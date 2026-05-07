package lru

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestLRUCacheCompactness(t *testing.T) {
	compact := isamd64
	defer func() {
		isamd64 = compact
	}()

	for _, b := range []bool{true, false} {
		isamd64 = b
		cache := NewLRUCache[string, []byte](32, WithShards[string, []byte](4))
		if length := cache.Len(); length != 0 {
			t.Fatalf("bad cache length: %v", length)
		}
		if got, want := cache.mask+1, uint32(4); got != want {
			t.Fatalf("bad shard count: got=%d want=%d", got, want)
		}
		if got, want := len(cache.shards[0].list), 9; got != want {
			t.Fatalf("bad shard list size for compact=%v: got=%d want=%d", b, got, want)
		}
		cache.Set("a", []byte("1"))
		if v, ok := cache.Get("a"); !ok || string(v) != "1" {
			t.Fatalf("cache should work with compact=%v: value=%q ok=%v", b, v, ok)
		}
	}
}

func TestLRUCacheDefaultKey(t *testing.T) {
	cache := NewLRUCache[string, int](1)
	var k string
	var i int = 10

	if prev, replaced := cache.Set(k, i); replaced {
		t.Fatalf("value %v should not be replaced", prev)
	}

	if v, ok := cache.Get(k); !ok || v != i {
		t.Fatalf("bad returned value: %v != %v", v, i)
	}
}

func TestLRUCacheGetSet(t *testing.T) {
	cache := NewLRUCache[int, int](128)

	if v, ok := cache.Get(5); ok {
		t.Fatalf("bad returned value: %v", v)
	}

	if _, replaced := cache.Set(5, 10); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get(5); !ok || v != 10 {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}

	if v, replaced := cache.Set(5, 9); v != 10 || !replaced {
		t.Fatalf("set should return previous value 10 and replaced=true: value=%d replaced=%v", v, replaced)
	}

	if v, replaced := cache.Set(5, 9); v != 9 || !replaced {
		t.Fatalf("set should return previous value 9 and replaced=true: value=%d replaced=%v", v, replaced)
	}

	if v, ok := cache.Get(5); !ok || v != 9 {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}
}

func TestLRUCacheSetIfAbsent(t *testing.T) {
	cache := NewLRUCache[int, int](128)

	cache.Set(5, 5)

	if _, replaced := cache.SetIfAbsent(5, 10); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get(5); !ok || v != 5 {
		t.Fatalf("bad returned value: %v = %v", v, 5)
	}

	cache.Delete(5)

	if _, replaced := cache.SetIfAbsent(5, 10); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get(5); !ok || v != 10 {
		t.Fatalf("bad returned value: %v = %v", v, 10)
	}

	cache.Delete(5)

	if _, replaced := cache.SetIfAbsent(5, 10); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get(5); !ok || v != 10 {
		t.Fatalf("bad returned value: %v = %v", v, 10)
	}
}

func TestLRUCacheSetIfAbsentEvictsWhenFull(t *testing.T) {
	cache := NewLRUCache[string, int](1, WithShards[string, int](1))

	if prev, replaced := cache.Set("old", 1); replaced || prev != 0 {
		t.Fatalf("initial insert should not replace: prev=%d replaced=%v", prev, replaced)
	}

	prev, replaced := cache.SetIfAbsent("new", 2)
	if replaced || prev != 1 {
		t.Fatalf("absent insert should evict old value without replacing same key: prev=%d replaced=%v", prev, replaced)
	}
	if v, ok := cache.Get("old"); ok || v != 0 {
		t.Fatalf("old key should be evicted: value=%d ok=%v", v, ok)
	}
	if v, ok := cache.Get("new"); !ok || v != 2 {
		t.Fatalf("new key should be cached: value=%d ok=%v", v, ok)
	}
}

func TestLRUCacheSetIfAbsentPreservesZeroKey(t *testing.T) {
	cache := NewLRUCache[string, int](128, WithShards[string, int](1))

	cache.Set("", 1)
	cache.SetIfAbsent("a", 2)

	if v, ok := cache.Get(""); !ok || v != 1 {
		t.Fatalf("zero key should remain cached: %v, %v", v, ok)
	}
}

func TestLRUCacheEviction(t *testing.T) {
	cache := NewLRUCache[int, *int](256, WithShards[int, *int](1024))
	if cache.mask+1 != uint32(cap(cache.shards)) {
		t.Fatalf("bad shard mask: %v", cache.mask)
	}

	cache = NewLRUCache[int, *int](256, WithShards[int, *int](1))

	evictedCounter := 0
	for i := 0; i < 512; i++ {
		if v, _ := cache.Set(i, &i); v != nil {
			evictedCounter++
		}
	}

	if cache.Len() != 256 {
		t.Fatalf("bad len: %v", cache.Len())
	}

	if evictedCounter != 256 {
		t.Fatalf("bad evicted count: %v", evictedCounter)
	}

	for i := 0; i < 256; i++ {
		if v, ok := cache.Get(i); ok || v != nil {
			t.Fatalf("key %d should be evicted: value=%v ok=%v", i, v, ok)
		}
	}

	for i := 256; i < 512; i++ {
		if v, ok := cache.Get(i); !ok || v == nil {
			t.Fatalf("key %d should not be evicted: value=%v ok=%v", i, v, ok)
		}
	}

	for i := 256; i < 384; i++ {
		cache.Delete(i)
		if v, ok := cache.Get(i); ok {
			t.Fatalf("old key %d should be deleted: value=%v ok=%v", i, v, ok)
		}
	}

	for i := 384; i < 512; i++ {
		if v, ok := cache.Get(i); !ok || v == nil {
			t.Fatalf("old key %d should not be deleted: value=%v ok=%v", i, v, ok)
		}
	}

	if got, want := cache.Len(), 128; got != want {
		t.Fatalf("current cache length %v should be %v", got, want)
	}

	cache.Set(400, &evictedCounter)

	if got, want := len(cache.AppendKeys(nil)), 128; got != want {
		t.Fatalf("current cache keys length %v should be %v", got, want)
	}
}

func TestLRUCachePeek(t *testing.T) {
	cache := NewLRUCache[int, int](64, WithShards[int, int](1))

	cache.Set(10, 10)
	cache.Set(20, 20)
	if v, ok := cache.Peek(10); !ok || v != 10 {
		t.Errorf("10 should be set to 10: %v", v)
	}

	if v, ok := cache.Peek(20); !ok || v != 20 {
		t.Errorf("20 should be set to 20: %v,", v)
	}

	if v, ok := cache.Peek(30); ok || v != 0 {
		t.Errorf("30 should be set to 0: %v,", v)
	}

	for k := 3; k < 1024; k++ {
		cache.Set(k, k)
	}
	if v, ok := cache.Peek(10); ok || v == 10 {
		t.Errorf("peek should not update recency for key 10: value=%d ok=%v", v, ok)
	}
	if v, ok := cache.Peek(30); ok || v != 0 {
		t.Errorf("missing key 30 should remain absent: value=%d ok=%v", v, ok)
	}
}

func TestLRUCacheHasher(t *testing.T) {
	cache := NewLRUCache[string, int](1024,
		WithHasher[string, int](func(key unsafe.Pointer, seed uintptr) (x uintptr) {
			x = 5381
			for _, c := range []byte(*(*string)(key)) {
				x = x*33 + uintptr(c)
			}
			return
		}),
		WithShards[string, int](1),
	)

	if v, ok := cache.Get("abcde"); ok {
		t.Fatalf("bad returned value: %v", v)
	}

	if _, replaced := cache.Set("abcde", 10); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get("abcde"); !ok || v != 10 {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}
}

func TestLRUCacheSliding(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("WithSliding should panic for LRUCache")
		}
		if !strings.Contains(fmt.Sprint(r), "not_supported") {
			t.Fatalf("panic should contain not_supported: %v", r)
		}
	}()
	_ = NewLRUCache[string, int](1024, WithSliding[string, int](true))
}

func TestLRUCacheLoader(t *testing.T) {
	cache := NewLRUCache[string, int](1024)
	if v, err, ok := cache.GetOrLoad(context.Background(), "a", nil); ok || err == nil || v != 0 {
		t.Fatalf("GetOrLoad without loader should fail: value=%d err=%v ok=%v", v, err, ok)
	}

	cache = NewLRUCache[string, int](1024, WithLoader[string, int](func(ctx context.Context, key string) (int, error) {
		if key == "" {
			return 0, fmt.Errorf("invalid key: %v", key)
		}
		i := int(key[0] - 'a' + 1)
		return i, nil
	}))

	if v, err, ok := cache.GetOrLoad(context.Background(), "", nil); ok || err == nil || v != 0 {
		t.Fatalf("GetOrLoad with invalid key should fail: value=%d err=%v ok=%v", v, err, ok)
	}

	if v, err, ok := cache.GetOrLoad(context.Background(), "b", nil); ok || err != nil || v != 2 {
		t.Fatalf("GetOrLoad should load b=2: value=%d err=%v ok=%v", v, err, ok)
	}

	if v, err, ok := cache.GetOrLoad(context.Background(), "a", nil); ok || err != nil || v != 1 {
		t.Fatalf("GetOrLoad should load a=1: value=%d err=%v ok=%v", v, err, ok)
	}

	if v, err, ok := cache.GetOrLoad(context.Background(), "a", nil); !ok || err != nil || v != 1 {
		t.Fatalf("GetOrLoad should hit cached a=1: value=%d err=%v ok=%v", v, err, ok)
	}
}

func TestLRUCacheLoaderPanic(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("TTL-style loader should panic for LRUCache")
		}
		if !strings.Contains(fmt.Sprint(r), "not_supported") {
			t.Fatalf("panic should contain not_supported: %v", r)
		}
	}()
	_ = NewLRUCache[string, int](1024, WithLoader[string, int](func(ctx context.Context, key string) (int, time.Duration, error) {
		return 1, time.Hour, nil
	}))
}

func TestLRUCacheLoaderSingleflight(t *testing.T) {
	var loads uint32

	cache := NewLRUCache[string, int](1024, WithLoader[string, int](func(ctx context.Context, key string) (int, error) {
		atomic.AddUint32(&loads, 1)
		time.Sleep(100 * time.Millisecond)
		return int(key[0] - 'a' + 1), nil
	}))

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer wg.Done()
			v, err, ok := cache.GetOrLoad(context.Background(), "a", nil)
			if v != 1 || err != nil || !ok {
				t.Errorf("a should be set to 1: %v,%v,%v", v, err, ok)
			}
		}(i)
	}
	wg.Wait()

	if n := atomic.LoadUint32(&loads); n != 1 {
		t.Errorf("a should be loaded only once: %v", n)
	}
}

func TestLRUCacheStats(t *testing.T) {
	cache := NewLRUCache[string, int](256, WithShards[string, int](1))

	cache.Set("a", 1)
	cache.Set("b", 2)
	cache.Set("c", 3)
	cache.Set("d", 3)

	stats := cache.Stats()
	if got, want := stats.EntriesCount, uint64(4); got != want {
		t.Fatalf("cache entries should be %v: %v", want, got)
	}
	if got, want := stats.GetCalls, uint64(0); got != want {
		t.Fatalf("cache get calls should be %v: %v", want, got)
	}
	if got, want := stats.SetCalls, uint64(4); got != want {
		t.Fatalf("cache set calls should be %v: %v", want, got)
	}
	if got, want := stats.Misses, uint64(0); got != want {
		t.Fatalf("cache misses should be %v: %v", want, got)
	}

	cache.Get("a")
	cache.Get("b")
	cache.Get("x")
	cache.Get("y")
	cache.Get("z")
	cache.Set("c", 13)

	stats = cache.Stats()
	if got, want := stats.EntriesCount, uint64(4); got != want {
		t.Fatalf("cache entries should be %v: %v", want, got)
	}
	if got, want := stats.GetCalls, uint64(5); got != want {
		t.Fatalf("cache get calls should be %v: %v", want, got)
	}
	if got, want := stats.SetCalls, uint64(5); got != want {
		t.Fatalf("cache set calls should be %v: %v", want, got)
	}
	if got, want := stats.Misses, uint64(3); got != want {
		t.Fatalf("cache misses should be %v: %v", want, got)
	}
}

func BenchmarkLRUCacheRand(b *testing.B) {
	cache := NewLRUCache[int64, int64](8192)

	trace := make([]int64, b.N*2)
	for i := 0; i < b.N*2; i++ {
		trace[i] = rand.Int63() % 32768
	}

	b.ReportAllocs()
	b.ResetTimer()

	var hit, miss int
	for i := 0; i < 2*b.N; i++ {
		if i%2 == 0 {
			cache.Set(trace[i], trace[i])
		} else {
			if _, ok := cache.Get(trace[i]); ok {
				hit++
			} else {
				miss++
			}
		}
	}
	b.Logf("hit: %d miss: %d ratio: %f", hit, miss, float64(hit)/float64(hit+miss))
}

func BenchmarkLRUCacheFreq(b *testing.B) {
	cache := NewLRUCache[int64, int64](8192)

	trace := make([]int64, b.N*2)
	for i := 0; i < b.N*2; i++ {
		if i%2 == 0 {
			trace[i] = rand.Int63() % 16384
		} else {
			trace[i] = rand.Int63() % 32768
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.Set(trace[i], trace[i])
	}
	var hit, miss int
	for i := 0; i < b.N; i++ {
		if _, ok := cache.Get(trace[i]); ok {
			hit++
		} else {
			miss++
		}
	}
	b.Logf("hit: %d miss: %d ratio: %f", hit, miss, float64(hit)/float64(hit+miss))
}
