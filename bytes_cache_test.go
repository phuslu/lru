package lru

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestBytesCacheDefaultkey(t *testing.T) {
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
		t.Fatal("old value should be evicted")
	}

	if v, replaced := cache.Set([]byte("5"), []byte("9")); b2s(v) != "9" || !replaced {
		t.Fatal("old value should be evicted")
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
		t.Fatalf("curent cache length %v should be %v", got, want)
	}

	cache.Set([]byte("400"), []byte("400"))

	if got, want := len(cache.AppendKeys(nil)), 128; got != want {
		t.Fatalf("curent cache keys length %v should be %v", got, want)
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
		t.Errorf("%v should not have updated recent-ness of 10", v)
	}
	if v, ok := cache.Peek([]byte("30")); ok || len(v) != 0 {
		t.Errorf("%v should have updated recent-ness of 30", v)
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

	trace := make([]int64, b.N*2)
	for i := 0; i < b.N*2; i++ {
		trace[i] = rand.Int63() % 32768
	}

	b.ReportAllocs()
	b.ResetTimer()

	var hit, miss int
	for i := 0; i < 2*b.N; i++ {
		if i%2 == 0 {
			cache.Set([]byte(fmt.Sprint(trace[i])), []byte(fmt.Sprint(trace[i])))
		} else {
			if _, ok := cache.Get([]byte(fmt.Sprint(trace[i]))); ok {
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
		cache.Set([]byte(fmt.Sprint(trace[i])), []byte(fmt.Sprint(trace[i])))
	}
	var hit, miss int
	for i := 0; i < b.N; i++ {
		if _, ok := cache.Get([]byte(fmt.Sprint(trace[i]))); ok {
			hit++
		} else {
			miss++
		}
	}
	b.Logf("hit: %d miss: %d ratio: %f", hit, miss, float64(hit)/float64(hit+miss))
}
