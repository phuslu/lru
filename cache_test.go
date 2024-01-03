package lru

import (
	"math/rand"
	"testing"
	"time"
)

func TestCacheDefaultkey(t *testing.T) {
	l := New[string, int](1)
	var k string
	var i int = 10

	if prev, replaced := l.Set(k, i); replaced {
		t.Fatalf("value %v should not be replaced", prev)
	}

	if v, ok := l.Get(k); !ok || v != i {
		t.Fatalf("bad returned value: %v != %v", v, i)
	}
}

func TestCacheSetget(t *testing.T) {
	l := New[int, int](128)

	if v, ok := l.Get(5); ok {
		t.Fatalf("bad returned value: %v", v)
	}

	if _, replaced := l.Set(5, 10); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := l.Get(5); !ok || v != 10 {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}

	if v, replaced := l.Set(5, 9); v != 10 || !replaced {
		t.Fatal("old value should be evicted")
	}

	if v, ok := l.Get(5); !ok || v != 9 {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}
}

func TestCacheEviction(t *testing.T) {
	l := newWithShards[int, *int](1, 256)

	evictedCounter := 0
	for i := 0; i < 512; i++ {
		if v, _ := l.Set(i, &i); v != nil {
			evictedCounter++
		}
	}

	if l.Len() != 256 {
		t.Fatalf("bad len: %v", l.Len())
	}

	if evictedCounter != 256 {
		t.Fatalf("bad evicted count: %v", evictedCounter)
	}

	for i := 0; i < 256; i++ {
		if v, ok := l.Get(i); ok || v != nil {
			t.Fatalf("key %v value %v should be evicted", i, *v)
		}
	}

	for i := 256; i < 512; i++ {
		if v, ok := l.Get(i); !ok {
			t.Fatalf("key %v value %v should not be evicted", i, *v)
		}
	}

	for i := 256; i < 384; i++ {
		l.Delete(i)
		if v, ok := l.Get(i); ok {
			t.Fatalf("old key %v value %v should be deleted", i, *v)
		}
	}

	for i := 384; i < 512; i++ {
		if v, ok := l.Get(i); !ok || v == nil {
			t.Fatalf("old key %v value %v should not be deleted", i, *v)
		}
	}

	if got, want := l.Len(), 128; got != want {
		t.Fatalf("curent cache length %v should be %v", got, want)
	}
}

func TestCacheSlidingGet(t *testing.T) {
	l := newWithShards[string, int](1, 256)

	l.SetWithTTL("foobar", 42, 400*time.Millisecond)

	time.Sleep(200 * time.Millisecond)
	if v, ok := l.SlidingGet("foobar"); !ok || v != 42 {
		t.Errorf("foobar should be set to 42: %v,", v)
	}

	time.Sleep(300 * time.Millisecond)
	if v, ok := l.Get("foobar"); !ok || v != 42 {
		t.Errorf("foobar should be still set to 42: %v,", v)
	}
}

func TestCachePeek(t *testing.T) {
	l := New[int, int](64)

	l.Set(10, 10)
	l.Set(20, 20)
	if v, ok := l.Peek(10); !ok || v != 10 {
		t.Errorf("10 should be set to 10: %v,", v)
	}

	if v, ok := l.Peek(20); !ok || v != 20 {
		t.Errorf("20 should be set to 20: %v,", v)
	}

	if v, ok := l.Peek(30); ok || v != 0 {
		t.Errorf("30 should be set to 0: %v,", v)
	}

	for k := 3; k < 1024; k++ {
		l.Set(k, k)
	}
	if v, ok := l.Peek(10); ok || v == 10 {
		t.Errorf("%v should not have updated recent-ness of 10", v)
	}
	if v, ok := l.Peek(30); ok || v != 0 {
		t.Errorf("%v should have updated recent-ness of 30", v)
	}
}

func BenchmarkCacheRand(b *testing.B) {
	l := New[int64, int64](8192)

	trace := make([]int64, b.N*2)
	for i := 0; i < b.N*2; i++ {
		trace[i] = rand.Int63() % 32768
	}

	b.ReportAllocs()
	b.ResetTimer()

	var hit, miss int
	for i := 0; i < 2*b.N; i++ {
		if i%2 == 0 {
			l.Set(trace[i], trace[i])
		} else {
			if _, ok := l.Get(trace[i]); ok {
				hit++
			} else {
				miss++
			}
		}
	}
	b.Logf("hit: %d miss: %d ratio: %f", hit, miss, float64(hit)/float64(hit+miss))
}

func BenchmarkCacheFreq(b *testing.B) {
	l := New[int64, int64](8192)

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
		l.Set(trace[i], trace[i])
	}
	var hit, miss int
	for i := 0; i < b.N; i++ {
		if _, ok := l.Get(trace[i]); ok {
			hit++
		} else {
			miss++
		}
	}
	b.Logf("hit: %d miss: %d ratio: %f", hit, miss, float64(hit)/float64(hit+miss))
}

func BenchmarkCacheTTL(b *testing.B) {
	l := New[int64, int64](8192)

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
		l.SetWithTTL(trace[i], trace[i], 60*time.Second)
	}
	var hit, miss int
	for i := 0; i < b.N; i++ {
		if _, ok := l.Get(trace[i]); ok {
			hit++
		} else {
			miss++
		}
	}
	b.Logf("hit: %d miss: %d ratio: %f", hit, miss, float64(hit)/float64(hit+miss))
}
