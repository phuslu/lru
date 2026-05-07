package lru

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestBytesCacheDefaultKey(t *testing.T) {
	cache := NewBytesCache(1, 1)
	var k []byte
	var i = []byte("42")

	if prev, replaced := cache.Set(k, i); replaced {
		t.Fatalf("value %v should not be replaced", prev)
	}

	if v, ok := cache.Get(k); !ok || b2s(v) != b2s(i) {
		t.Fatalf("bad returned value: %v != %v", v, i)
	}
}

func TestBytesCacheGetSet(t *testing.T) {
	cache := NewBytesCache(1, 128)

	if v, ok := cache.Get([]byte("5")); ok {
		t.Fatalf("bad returned value: %v", v)
	}

	if _, replaced := cache.Set([]byte("5"), []byte("10")); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get([]byte("5")); !ok || b2s(v) != "10" {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}

	if v, replaced := cache.Set([]byte("5"), []byte("9")); b2s(v) != "10" || !replaced {
		t.Fatalf("set should return previous value 10 and replaced=true: value=%q replaced=%v", v, replaced)
	}

	if v, replaced := cache.Set([]byte("5"), []byte("9")); b2s(v) != "9" || !replaced {
		t.Fatalf("set should return previous value 9 and replaced=true: value=%q replaced=%v", v, replaced)
	}

	if v, ok := cache.Get([]byte("5")); !ok || b2s(v) != "9" {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}
}

func TestBytesCacheSetIfAbsent(t *testing.T) {
	cache := NewBytesCache(1, 128)

	cache.Set([]byte("5"), []byte("5"))

	if _, replaced := cache.SetIfAbsent([]byte("5"), []byte("10")); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get([]byte("5")); !ok || b2s(v) != "5" {
		t.Fatalf("bad returned value: %v = %v", v, []byte("5"))
	}

	cache.Delete([]byte("5"))

	if _, replaced := cache.SetIfAbsent([]byte("5"), []byte("10")); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get([]byte("5")); !ok || b2s(v) != "10" {
		t.Fatalf("bad returned value: %v = %v", v, 10)
	}

	cache.Delete([]byte("5"))

	if _, replaced := cache.SetIfAbsent([]byte("5"), []byte("10")); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := cache.Get([]byte("5")); !ok || b2s(v) != "10" {
		t.Fatalf("bad returned value: %v = %v", v, 10)
	}
}

func TestBytesCacheSetIfAbsentPreservesNilKey(t *testing.T) {
	cache := NewBytesCache(1, 128)

	cache.Set(nil, []byte("nil"))
	cache.SetIfAbsent([]byte("a"), []byte("a"))

	if v, ok := cache.Get(nil); !ok || b2s(v) != "nil" {
		t.Fatalf("nil key should remain cached: %q, %v", v, ok)
	}
}

func TestBytesCacheSetIfAbsentEvictsWhenFull(t *testing.T) {
	cache := NewBytesCache(1, 1)

	if prev, replaced := cache.Set([]byte("old"), []byte("1")); replaced || prev != nil {
		t.Fatalf("initial insert should not replace: prev=%q replaced=%v", prev, replaced)
	}

	prev, replaced := cache.SetIfAbsent([]byte("new"), []byte("2"))
	if replaced || b2s(prev) != "1" {
		t.Fatalf("absent insert should evict old value without replacing same key: prev=%q replaced=%v", prev, replaced)
	}
	if v, ok := cache.Get([]byte("old")); ok || len(v) != 0 {
		t.Fatalf("old key should be evicted: value=%q ok=%v", v, ok)
	}
	if v, ok := cache.Get([]byte("new")); !ok || b2s(v) != "2" {
		t.Fatalf("new key should be cached: value=%q ok=%v", v, ok)
	}
}

func TestBytesCacheSetPreservesNilKey(t *testing.T) {
	cache := NewBytesCache(1, 128)

	cache.Set(nil, []byte("nil"))
	cache.Set([]byte("a"), []byte("a"))

	if v, ok := cache.Get(nil); !ok || b2s(v) != "nil" {
		t.Fatalf("nil key should remain cached: %q, %v", v, ok)
	}
}

func TestBytesCacheLengthWithNilValue(t *testing.T) {
	cache := NewBytesCache(1, 2)

	cache.Set(nil, nil)
	cache.Set([]byte("1"), nil)

	if got, want := cache.Len(), 2; got != want {
		t.Fatalf("cache length should count nil values: got=%d want=%d", got, want)
	}
	if v, ok := cache.Get(nil); !ok || v != nil {
		t.Fatalf("nil key with nil value should be present: value=%q ok=%v", v, ok)
	}
	if v, ok := cache.Get([]byte("1")); !ok || v != nil {
		t.Fatalf("non-nil key with nil value should be present: value=%q ok=%v", v, ok)
	}

	cache.Set([]byte("2"), []byte("2"))
	if got, want := cache.Len(), 2; got != want {
		t.Fatalf("cache length should stay at capacity: got=%d want=%d", got, want)
	}
}

func TestBytesCacheEviction(t *testing.T) {
	cache := NewBytesCache(128, 256)
	if cache.mask+1 != uint32(cap(cache.shards)) {
		t.Fatalf("bad shard mask: %v", cache.mask)
	}

	cache = NewBytesCache(1, 256)

	evictedCounter := 0
	for i := 0; i < 512; i++ {
		if v, _ := cache.Set([]byte(fmt.Sprint(i)), []byte(fmt.Sprint(i))); v != nil {
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
		if v, ok := cache.Get([]byte(fmt.Sprint(i))); ok || len(v) != 0 {
			t.Fatalf("key %v value %v should be evicted", i, v)
		}
	}

	for i := 256; i < 512; i++ {
		if v, ok := cache.Get([]byte(fmt.Sprint(i))); !ok {
			t.Fatalf("key %v value %v should not be evicted", i, v)
		}
	}

	for i := 256; i < 384; i++ {
		cache.Delete([]byte(fmt.Sprint(i)))
		if v, ok := cache.Get([]byte(fmt.Sprint(i))); ok {
			t.Fatalf("old key %v value %v should be deleted", i, v)
		}
	}

	for i := 384; i < 512; i++ {
		if v, ok := cache.Get([]byte(fmt.Sprint(i))); !ok || len(v) == 0 {
			t.Fatalf("old key %v value %v should not be deleted", i, v)
		}
	}

	if got, want := cache.Len(), 128; got != want {
		t.Fatalf("current cache length %v should be %v", got, want)
	}

	cache.Set([]byte("400"), []byte("400"))

	if got, want := len(cache.AppendKeys(nil)), 128; got != want {
		t.Fatalf("current cache keys length %v should be %v", got, want)
	}
}

func TestBytesCachePeek(t *testing.T) {
	cache := NewBytesCache(1, 64)

	cache.Set([]byte("10"), []byte("10"))
	cache.Set([]byte("20"), []byte("20"))
	if v, ok := cache.Peek([]byte("10")); !ok || b2s(v) != "10" {
		t.Errorf("10 should be set to 10: %v", v)
	}

	if v, ok := cache.Peek([]byte("20")); !ok || b2s(v) != "20" {
		t.Errorf("20 should be set to 20: %v,", v)
	}

	if v, ok := cache.Peek([]byte("30")); ok || len(v) != 0 {
		t.Errorf("30 should be set to nil: %v,", v)
	}

	for k := 3; k < 1024; k++ {
		cache.Set([]byte(fmt.Sprint(k)), []byte(fmt.Sprint(k)))
	}
	if v, ok := cache.Peek([]byte("10")); ok || b2s(v) == "10" {
		t.Errorf("peek should not update recency for key 10: value=%q ok=%v", v, ok)
	}
	if v, ok := cache.Peek([]byte("30")); ok || len(v) != 0 {
		t.Errorf("missing key 30 should remain absent: value=%q ok=%v", v, ok)
	}
}

func TestBytesCacheStats(t *testing.T) {
	cache := NewBytesCache(1, 256)

	cache.Set([]byte("a"), []byte("1"))
	cache.Set([]byte("b"), []byte("2"))
	cache.Set([]byte("c"), []byte("3"))
	cache.Set([]byte("d"), []byte("3"))

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

	cache.Get([]byte("a"))
	cache.Get([]byte("b"))
	cache.Get([]byte("x"))
	cache.Get([]byte("y"))
	cache.Get([]byte("z"))
	cache.Set([]byte("c"), []byte("13"))

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

func BenchmarkBytesCacheRand(b *testing.B) {
	cache := NewBytesCache(1, 8192)
	keys, values := newBytesBenchmarkItems(32768)

	trace := make([]uint32, b.N*2)
	for i := 0; i < b.N*2; i++ {
		trace[i] = uint32(rand.Intn(len(keys)))
	}

	b.ReportAllocs()
	b.ResetTimer()

	var hit, miss int
	for i := 0; i < 2*b.N; i++ {
		index := trace[i]
		if i%2 == 0 {
			cache.Set(keys[index], values[index])
		} else {
			if _, ok := cache.Get(keys[index]); ok {
				hit++
			} else {
				miss++
			}
		}
	}
	b.Logf("hit: %d miss: %d ratio: %f", hit, miss, float64(hit)/float64(hit+miss))
}

func BenchmarkBytesCacheFreq(b *testing.B) {
	cache := NewBytesCache(1, 8192)
	keys, values := newBytesBenchmarkItems(32768)

	trace := make([]uint32, b.N*2)
	for i := 0; i < b.N*2; i++ {
		if i%2 == 0 {
			trace[i] = uint32(rand.Intn(len(keys) / 2))
		} else {
			trace[i] = uint32(rand.Intn(len(keys)))
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		index := trace[i]
		cache.Set(keys[index], values[index])
	}
	var hit, miss int
	for i := 0; i < b.N; i++ {
		if _, ok := cache.Get(keys[trace[i]]); ok {
			hit++
		} else {
			miss++
		}
	}
	b.Logf("hit: %d miss: %d ratio: %f", hit, miss, float64(hit)/float64(hit+miss))
}

func newBytesBenchmarkItems(n int) (keys [][]byte, values [][]byte) {
	keys = make([][]byte, n)
	values = make([][]byte, n)
	for i := 0; i < n; i++ {
		item := []byte(fmt.Sprint(i))
		keys[i] = item
		values[i] = item
	}
	return keys, values
}
