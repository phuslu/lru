package lru

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheDefaultkey(t *testing.T) {
	l := New[string, int](1)
	var k string
	var i int = 10

	if prev, replaced := l.Set(k, i, 0); replaced {
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

	if _, replaced := l.Set(5, 10, 0); replaced {
		t.Fatal("should not have replaced")
	}

	if v, ok := l.Get(5); !ok || v != 10 {
		t.Fatalf("bad returned value: %v != %v", v, 10)
	}

	if v, replaced := l.Set(5, 9, 0); v != 10 || !replaced {
		t.Fatal("old value should be evicted")
	}

	if v, replaced := l.Set(5, 9, 0); v != 9 || !replaced {
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
		if v, _ := l.Set(i, &i, 0); v != nil {
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

	l.Set(400, &evictedCounter, 0)

	if got, want := len(l.AppendKeys(nil)), 128; got != want {
		t.Fatalf("curent cache keys length %v should be %v", got, want)
	}
}

func TestCachePeek(t *testing.T) {
	l := New[int, int](64)

	l.Set(10, 10, 0)
	l.Set(20, 20, 0)
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
		l.Set(k, k, 0)
	}
	if v, ok := l.Peek(10); ok || v == 10 {
		t.Errorf("%v should not have updated recent-ness of 10", v)
	}
	if v, ok := l.Peek(30); ok || v != 0 {
		t.Errorf("%v should have updated recent-ness of 30", v)
	}
}

func TestCacheLoader(t *testing.T) {
	l := New[string, int](1024)
	if v, err, ok := l.GetOrLoad("a"); ok || err == nil || v != 0 {
		t.Errorf("l.GetOrLoad(\"a\") again should be return error: %v, %v, %v", v, err, ok)
	}

	l = New[string, int](1024, WithLoader(func(key string) (int, time.Duration, error) {
		if key == "" {
			return 0, 0, fmt.Errorf("invalid key: %v", key)
		}
		i := int(key[0] - 'a' + 1)
		return i, time.Duration(i) * time.Second, nil
	}))

	if v, err, ok := l.GetOrLoad(""); ok || err == nil || v != 0 {
		t.Errorf("l.GetOrLoad(\"a\") again should be return error: %v, %v, %v", v, err, ok)
	}

	if v, err, ok := l.GetOrLoad("b"); ok || err != nil || v != 2 {
		t.Errorf("l.GetOrLoad(\"b\") again should be return 2: %v, %v, %v", v, err, ok)
	}

	if v, err, ok := l.GetOrLoad("a"); ok || err != nil || v != 1 {
		t.Errorf("l.GetOrLoad(\"a\") should be return 1: %v, %v, %v", v, err, ok)
	}

	if v, err, ok := l.GetOrLoad("a"); !ok || err != nil || v != 1 {
		t.Errorf("l.GetOrLoad(\"a\") again should be return 1: %v, %v, %v", v, err, ok)
	}

	time.Sleep(1 * time.Second)

	if v, err, ok := l.GetOrLoad("a"); ok || err != nil || v != 1 {
		t.Errorf("l.GetOrLoad(\"a\") again should be return 1: %v, %v, %v", v, err, ok)
	}
}

func TestCacheLoaderSingleflight(t *testing.T) {
	var loads uint32

	l := New[string, int](1024, WithLoader(func(key string) (int, time.Duration, error) {
		atomic.AddUint32(&loads, 1)
		time.Sleep(100 * time.Millisecond)
		return int(key[0] - 'a' + 1), time.Hour, nil
	}))

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer wg.Done()
			v, err, ok := l.GetOrLoad("a")
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

func TestCacheSlidingGet(t *testing.T) {
	l := newWithShards[string, int](1, 256, WithSliding[string, int](true))

	l.Set("a", 1, 0)
	l.Set("b", 2, 3*time.Second)
	l.Set("c", 3, 3*time.Second)
	l.Set("d", 3, 1*time.Second)

	if got, want := l.AppendKeys(nil), 4; len(got) != want {
		t.Fatalf("curent cache keys %v length should be %v", got, want)
	}

	if v, ok := l.Get("a"); !ok || v != 1 {
		t.Fatalf("a should be set to 1: %v,", v)
	}

	time.Sleep(2 * time.Second)
	if v, ok := l.Get("c"); !ok || v != 3 {
		t.Errorf("c should be set to 3: %v,", v)
	}
	if v, ok := l.Get("d"); ok || v != 0 {
		t.Errorf("d should be set to 0: %v,", v)
	}

	if got, want := l.AppendKeys(nil), 3; len(got) != want {
		t.Fatalf("curent cache keys %v length should be %v", got, want)
	}

	l.Set("c", 4, 3*time.Second)

	time.Sleep(2 * time.Second)
	if v, ok := l.Get("c"); !ok || v != 4 {
		t.Errorf("c should be still set to 4: %v,", v)
	}

	if got, want := l.AppendKeys(nil), 2; len(got) != want {
		t.Fatalf("curent cache keys %v length should be %v", got, want)
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
			l.Set(trace[i], trace[i], 0)
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
		l.Set(trace[i], trace[i], 0)
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
		l.Set(trace[i], trace[i], 60*time.Second)
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
