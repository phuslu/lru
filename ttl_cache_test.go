package lru

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestTTLCacheCompactness(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		return
	}
	compact := isamd64
	defer func() {
		isamd64 = compact
	}()

	for _, b := range []bool{true, false} {
		isamd64 = b
		cache := NewTTLCache[string, []byte](32 * 1024)
		if length := cache.Len(); length != 0 {
			t.Fatalf("bad cache length: %v", length)
		}
	}
}

func TestTTLCacheDefaultkey(t *testing.T) {
	cache := NewTTLCache[string, int](1)
	var k string
	var i int = 10

	if prev, replaced := cache.Set(k, i, 0); replaced {
		t.Fatalf("value %v should not be replaced", prev)
	}

	if v, ok := cache.Get(k); !ok || v != i {
		t.Fatalf("bad returned value: %v != %v", v, i)
	}
}

func TestTTLCacheGetSet(t *testing.T) {
	cache := NewTTLCache[int, int](128)

	if v, ok := cache.Get(5); ok {
		t.Fatalf("bad returned value: %v", v)
	}

	if _, replaced := cache.Set(5, 10, 0); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get(5); !ok || v != 10 {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}

	if v, replaced := cache.Set(5, 9, 0); v != 10 || !replaced {
		t.Fatal("old value should be evicted")
	}

	if v, replaced := cache.Set(5, 9, 0); v != 9 || !replaced {
		t.Fatal("old value should be evicted")
	}

	if v, ok := cache.Get(5); !ok || v != 9 {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}
}

func TestTTLCacheSetIfAbsent(t *testing.T) {
	cache := NewTTLCache[int, int](128)

	cache.Set(5, 5, 0)

	if _, replaced := cache.SetIfAbsent(5, 10, 0); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get(5); !ok || v != 5 {
		t.Fatalf("bad returned value: %v = %v", v, 5)
	}

	cache.Delete(5)

	if _, replaced := cache.SetIfAbsent(5, 10, 0); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get(5); !ok || v != 10 {
		t.Fatalf("bad returned value: %v = %v", v, 10)
	}

	cache.Delete(5)

	if _, replaced := cache.SetIfAbsent(5, 10, 1*time.Second); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get(5); !ok || v != 10 {
		t.Fatalf("bad returned value: %v = %v", v, 10)
	}

	cache.Set(5, 5, 1*time.Second)
	time.Sleep(2 * time.Second)

	if _, replaced := cache.SetIfAbsent(5, 10, 1*time.Second); !replaced {
		t.Fatal("should have replaced")
	}

	if v, ok := cache.Get(5); !ok || v != 10 {
		t.Fatalf("bad returned value: %v = %v", v, 10)
	}

	cache.Set(5, 5, 1*time.Second)
	time.Sleep(2 * time.Second)

	if _, replaced := cache.SetIfAbsent(5, 10, 0); !replaced {
		t.Fatal("should have replaced")
	}

	if v, ok := cache.Get(5); !ok || v != 10 {
		t.Fatalf("bad returned value: %v = %v", v, 10)
	}
}

func TestTTLCacheEviction(t *testing.T) {
	cache := NewTTLCache[int, *int](256, WithShards[int, *int](1024))
	if cache.mask+1 != uint32(cap(cache.shards)) {
		t.Fatalf("bad shard mask: %v", cache.mask)
	}

	cache = NewTTLCache[int, *int](256, WithShards[int, *int](1))

	evictedCounter := 0
	for i := 0; i < 512; i++ {
		if v, _ := cache.Set(i, &i, 0); v != nil {
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
			t.Fatalf("key %v value %v should be evicted", i, *v)
		}
	}

	for i := 256; i < 512; i++ {
		if v, ok := cache.Get(i); !ok {
			t.Fatalf("key %v value %v should not be evicted", i, *v)
		}
	}

	for i := 256; i < 384; i++ {
		cache.Delete(i)
		if v, ok := cache.Get(i); ok {
			t.Fatalf("old key %v value %v should be deleted", i, *v)
		}
	}

	for i := 384; i < 512; i++ {
		if v, ok := cache.Get(i); !ok || v == nil {
			t.Fatalf("old key %v value %v should not be deleted", i, *v)
		}
	}

	if got, want := cache.Len(), 128; got != want {
		t.Fatalf("curent cache length %v should be %v", got, want)
	}

	cache.Set(400, &evictedCounter, 0)

	if got, want := len(cache.AppendKeys(nil)), 128; got != want {
		t.Fatalf("curent cache keys length %v should be %v", got, want)
	}
}

func TestTTLCachePeek(t *testing.T) {
	cache := NewTTLCache[int, int](64)

	cache.Set(10, 10, 0)
	cache.Set(20, 20, time.Hour)
	if v, expires, ok := cache.Peek(10); !ok || v != 10 || expires != 0 {
		t.Errorf("10 should be set to 10: %v, %v", v, expires)
	}

	if v, expires, ok := cache.Peek(20); !ok || v != 20 || expires == 0 {
		t.Errorf("20 should be set to 20: %v,", v)
	}

	if v, expires, ok := cache.Peek(30); ok || v != 0 || expires != 0 {
		t.Errorf("30 should be set to 0: %v,", v)
	}

	for k := 3; k < 1024; k++ {
		cache.Set(k, k, 0)
	}
	if v, _, ok := cache.Peek(10); ok || v == 10 {
		t.Errorf("%v should not have updated recent-ness of 10", v)
	}
	if v, _, ok := cache.Peek(30); ok || v != 0 {
		t.Errorf("%v should have updated recent-ness of 30", v)
	}
}

func TestTTLCacheHasher(t *testing.T) {
	cache := NewTTLCache[string, int](1024, WithHasher[string, int](func(key unsafe.Pointer, seed uintptr) (x uintptr) {
		x = 5381
		for _, c := range []byte(*(*string)(key)) {
			x = x*33 + uintptr(c)
		}
		return
	}))

	if v, ok := cache.Get("abcde"); ok {
		t.Fatalf("bad returned value: %v", v)
	}

	if _, replaced := cache.Set("abcde", 10, 0); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get("abcde"); !ok || v != 10 {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}
}

func TestTTLCacheLoader(t *testing.T) {
	cache := NewTTLCache[string, int](1024)
	if v, err, ok := cache.GetOrLoad("a", nil); ok || err == nil || v != 0 {
		t.Errorf("cache.GetOrLoad(\"a\", nil) again should be return error: %v, %v, %v", v, err, ok)
	}

	cache = NewTTLCache[string, int](1024, WithLoader[string, int](func(key string) (int, time.Duration, error) {
		if key == "" {
			return 0, 0, fmt.Errorf("invalid key: %v", key)
		}
		i := int(key[0] - 'a' + 1)
		return i, time.Duration(i) * time.Second, nil
	}))

	if v, err, ok := cache.GetOrLoad("", nil); ok || err == nil || v != 0 {
		t.Errorf("cache.GetOrLoad(\"a\", nil) again should be return error: %v, %v, %v", v, err, ok)
	}

	if v, err, ok := cache.GetOrLoad("b", nil); ok || err != nil || v != 2 {
		t.Errorf("cache.GetOrLoad(\"b\", nil) again should be return 2: %v, %v, %v", v, err, ok)
	}

	if v, err, ok := cache.GetOrLoad("a", nil); ok || err != nil || v != 1 {
		t.Errorf("cache.GetOrLoad(\"a\", nil) should be return 1: %v, %v, %v", v, err, ok)
	}

	if v, err, ok := cache.GetOrLoad("a", nil); !ok || err != nil || v != 1 {
		t.Errorf("cache.GetOrLoad(\"a\") again should be return 1: %v, %v, %v", v, err, ok)
	}

	time.Sleep(2 * time.Second)

	if v, err, ok := cache.GetOrLoad("a", nil); ok || err != nil || v != 1 {
		t.Errorf("cache.GetOrLoad(\"a\") again should be return 1: %v, %v, %v", v, err, ok)
	}
}

func TestTTLCacheLoaderPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if !strings.Contains(fmt.Sprint(r), "not_supported") {
				t.Errorf("should be not_supported")
			}
		}
	}()
	_ = NewTTLCache[string, int](1024, WithLoader[string, int](func(key string) (int, error) {
		return 1, nil
	}))
	t.Errorf("should be panic above")
}

func TestTTLCacheLoaderSingleflight(t *testing.T) {
	var loads uint32

	cache := NewTTLCache[string, int](1024, WithLoader[string, int](func(key string) (int, time.Duration, error) {
		atomic.AddUint32(&loads, 1)
		time.Sleep(100 * time.Millisecond)
		return int(key[0] - 'a' + 1), time.Hour, nil
	}))

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer wg.Done()
			v, err, ok := cache.GetOrLoad("a", nil)
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

func TestTTLCacheSlidingGet(t *testing.T) {
	cache := NewTTLCache[string, int](256, WithSliding[string, int](true), WithShards[string, int](1))

	cache.Set("a", 1, 0)
	cache.Set("b", 2, 3*time.Second)
	cache.Set("c", 3, 3*time.Second)
	cache.Set("d", 3, 1*time.Second)

	if got, want := cache.AppendKeys(nil), 4; len(got) != want {
		t.Fatalf("curent cache keys %v length should be %v", got, want)
	}

	if v, ok := cache.Get("a"); !ok || v != 1 {
		t.Fatalf("a should be set to 1: %v,", v)
	}

	time.Sleep(2 * time.Second)
	if v, ok := cache.Get("c"); !ok || v != 3 {
		t.Errorf("c should be set to 3: %v,", v)
	}
	if v, ok := cache.Get("d"); ok || v != 0 {
		t.Errorf("d should be set to 0: %v,", v)
	}

	if got, want := cache.AppendKeys(nil), 3; len(got) != want {
		t.Fatalf("curent cache keys %v length should be %v", got, want)
	}

	cache.Set("c", 4, 3*time.Second)

	time.Sleep(2 * time.Second)
	if v, ok := cache.Get("c"); !ok || v != 4 {
		t.Errorf("c should be still set to 4: %v,", v)
	}

	time.Sleep(1 * time.Second)

	if got, want := cache.AppendKeys(nil), 2; len(got) != want {
		t.Fatalf("curent cache keys %v length should be %v", got, want)
	}

}

func TestTTLCacheStats(t *testing.T) {
	cache := NewTTLCache[string, int](256, WithShards[string, int](1))

	cache.Set("a", 1, 0)
	cache.Set("b", 2, 3*time.Second)
	cache.Set("c", 3, 3*time.Second)
	cache.Set("d", 3, 2*time.Second)

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
	cache.Set("c", 13, 3*time.Second)

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

func BenchmarkTTLCacheRand(b *testing.B) {
	cache := NewTTLCache[int64, int64](8192)

	trace := make([]int64, b.N*2)
	for i := 0; i < b.N*2; i++ {
		trace[i] = rand.Int63() % 32768
	}

	b.ReportAllocs()
	b.ResetTimer()

	var hit, miss int
	for i := 0; i < 2*b.N; i++ {
		if i%2 == 0 {
			cache.Set(trace[i], trace[i], 0)
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

func BenchmarkTTLCacheFreq(b *testing.B) {
	cache := NewTTLCache[int64, int64](8192)

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
		cache.Set(trace[i], trace[i], 0)
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

func BenchmarkTTLCacheTTL(b *testing.B) {
	cache := NewTTLCache[int64, int64](8192)

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
		cache.Set(trace[i], trace[i], 60*time.Second)
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
