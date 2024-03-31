# a high-performance and gc-friendly LRU cache

[![godoc][godoc-img]][godoc] [![release][release-img]][release] [![goreport][goreport-img]][goreport] [![codecov][codecov-img]][codecov]

### Features

* Simple
    - No Dependencies.
    - Straightforward API.
* Fast
    - Outperforms well-known *LRU* caches.
    - Zero memory allocations.
* GC friendly
    - Pointerless and continuous data structs.
    - Minimized GC scan times.
* Memory efficient
    - Adds only 26 extra bytes per entry.
    - Minimized memory usage.
* Feature optional
    - Using SlidingCache via `WithSliding(true)` option.
    - Create LoadingCache via `WithLoader(func(context.Context, K) (V, time.Duration, error))` option.

### Limitations
1. The TTL is accurate to the nearest second.
2. Expired items are only removed when accessed again or the cache is full.

### Getting Started

```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	cache := lru.NewTTLCache[string, int](8192)

	cache.Set("a", 1, 2*time.Second)
	println(cache.Get("a"))
	println(cache.Get("b"))

	time.Sleep(1 * time.Second)
	println(cache.Get("a"))

	time.Sleep(2 * time.Second)
	println(cache.Get("a"))

	stats := cache.Stats()
	println("SetCalls", stats.SetCalls, "GetCalls", stats.GetCalls, "Misses", stats.Misses)
}
```

### Throughput benchmarks

A Performance result as below. Check github [benchmark][benchmark] action for more results and details.
<details>
  <summary>go1.22 benchmark on keysize=16, itemsize=1000000, cachesize=50%, concurrency=8</summary>

```go
// env writeratio=0.1 zipfian=false go test -v -cpu=8 -run=none -bench=. -benchtime=5s -benchmem bench_test.go
package bench

import (
	"crypto/sha1"
	"fmt"
	"math/rand/v2"
	"math/bits"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"
	_ "unsafe"

	theine "github.com/Yiling-J/theine-go"
	"github.com/cespare/xxhash/v2"
	cloudflare "github.com/cloudflare/golibs/lrucache"
	ristretto "github.com/dgraph-io/ristretto"
	freelru "github.com/elastic/go-freelru"
	hashicorp "github.com/hashicorp/golang-lru/v2/expirable"
	ccache "github.com/karlseguin/ccache/v3"
	lxzan "github.com/lxzan/memorycache"
	otter "github.com/maypok86/otter"
	ecache "github.com/orca-zhang/ecache"
	phuslu "github.com/phuslu/lru"
	"github.com/aclements/go-perfevent/perfbench"
)

const (
	keysize   = 16
	cachesize = 1000000
)

var writeratio, _ = strconv.ParseFloat(os.Getenv("writeratio"), 64)
var zipfian, _ = strconv.ParseBool(os.Getenv("zipfian"))

type CheapRand struct {
	Seed uint64
}

func (rand *CheapRand) Uint32() uint32 {
	rand.Seed += 0xa0761d6478bd642f
	hi, lo := bits.Mul64(rand.Seed, rand.Seed^0xe7037ed1a0b428db)
	return uint32(hi ^ lo)
}

func (rand *CheapRand) Uint32n(n uint32) uint32 {
	return uint32((uint64(rand.Uint32()) * uint64(n)) >> 32)
}

func (rand *CheapRand) Uint64() uint64 {
	return uint64(rand.Uint32())<<32 ^ uint64(rand.Uint32())
}

var shardcount = func() int {
	n := runtime.GOMAXPROCS(0) * 16
	k := 1
	for k < n {
		k = k * 2
	}
	return k
}()

var keys = func() (x []string) {
	x = make([]string, cachesize)
	for i := range cachesize {
		x[i] = fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprint(i))))[:keysize]
	}
	return
}()

func BenchmarkHashicorpSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := hashicorp.NewLRU[string, int](cachesize, nil, time.Hour)
	for i := range cachesize/2 {
		cache.Add(keys[i], i)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.Add(keys[i], i)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}

func BenchmarkCloudflareSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := cloudflare.NewMultiLRUCache(uint(shardcount), uint(cachesize/shardcount))
	for i := range cachesize/2 {
		cache.Set(keys[i], i, time.Now().Add(time.Hour))
	}
	expires := time.Now().Add(time.Hour)

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.Set(keys[i], i, expires)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}

func BenchmarkEcacheSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := ecache.NewLRUCache(uint16(shardcount), uint16(cachesize/shardcount), time.Hour)
	for i := range cachesize/2 {
		cache.Put(keys[i], i)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.Put(keys[i], i)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}

func BenchmarkLxzanSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := lxzan.New[string, int](
		lxzan.WithBucketNum(shardcount),
		lxzan.WithBucketSize(cachesize/shardcount, cachesize/shardcount),
		lxzan.WithInterval(time.Hour, time.Hour),
	)
	for i := range cachesize/2 {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.Set(keys[i], i, time.Hour)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}

func hashStringXXHASH(s string) uint32 {
	return uint32(xxhash.Sum64String(s))
}

func BenchmarkFreelruSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache, _ := freelru.NewSharded[string, int](cachesize, hashStringXXHASH)
	for i := range cachesize/2 {
		cache.AddWithLifetime(keys[i], i, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.AddWithLifetime(keys[i], i, time.Hour)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}

func BenchmarkPhusluSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := phuslu.NewTTLCache[string, int](cachesize, phuslu.WithShards[string, int](uint32(shardcount)))
	for i := range cachesize/2 {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.Set(keys[i], i, time.Hour)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}

func BenchmarkNoTTLSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := phuslu.NewLRUCache[string, int](cachesize, phuslu.WithShards[string, int](uint32(shardcount)))
	for i := range cachesize/2 {
		cache.Set(keys[i], i)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.Set(keys[i], i)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}

func BenchmarkCcacheSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := ccache.New(ccache.Configure[int]().MaxSize(cachesize).ItemsToPrune(100))
	for i := range cachesize/2 {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.Set(keys[i], i, time.Hour)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}

func BenchmarkRistrettoSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 10 * cachesize, // number of keys to track frequency of (10M).
		MaxCost:     cachesize,      // maximum cost of cache (1M).
		BufferItems: 64,             // number of keys per Get buffer.
	})
	for i := range cachesize/2 {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.SetWithTTL(keys[i], i, 1, time.Hour)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}

func BenchmarkTheineSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache, _ := theine.NewBuilder[string, int](cachesize).Build()
	for i := range cachesize/2 {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.SetWithTTL(keys[i], i, 1, time.Hour)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}

func BenchmarkOtterSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache, _ := otter.MustBuilder[string, int](cachesize).WithVariableTTL().Build()
	for i := range cachesize/2 {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		threshold := uint32(float64(^uint32(0)) * writeratio)
		cheaprand := &CheapRand{uint64(time.Now().UnixNano())}
		zipf := rand.NewZipf(rand.New(cheaprand), 1.0001, 10, cachesize-1)
		for pb.Next() {
			if threshold > 0 && cheaprand.Uint32() <= threshold {
				i := int(cheaprand.Uint32n(cachesize))
				cache.Set(keys[i], i, time.Hour)
			} else if zipfian {
				cache.Get(keys[zipf.Uint64()])
			} else {
				cache.Get(keys[cheaprand.Uint32n(cachesize)])
			}
		}
	})
}
```
</details>

with randomly read (90%) and randomly write(10%)
```
goos: linux
goarch: amd64
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkHashicorpSetGet
BenchmarkHashicorpSetGet-8    	13146066	       553.3 ns/op	      11 B/op	       0 allocs/op
BenchmarkCloudflareSetGet
BenchmarkCloudflareSetGet-8   	36612316	       208.6 ns/op	      16 B/op	       1 allocs/op
BenchmarkEcacheSetGet
BenchmarkEcacheSetGet-8       	47435138	       138.0 ns/op	       2 B/op	       0 allocs/op
BenchmarkLxzanSetGet
BenchmarkLxzanSetGet-8        	48343774	       153.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkFreelruSetGet
BenchmarkFreelruSetGet-8      	56105211	       137.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkPhusluSetGet
BenchmarkPhusluSetGet-8       	66522236	       114.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkCcacheSetGet
BenchmarkCcacheSetGet-8       	21252092	       369.4 ns/op	      34 B/op	       2 allocs/op
BenchmarkRistrettoSetGet
BenchmarkRistrettoSetGet-8    	35511078	       152.7 ns/op	      29 B/op	       1 allocs/op
BenchmarkTheineSetGet
BenchmarkTheineSetGet-8       	21374548	       311.4 ns/op	       5 B/op	       0 allocs/op
BenchmarkOtterSetGet
BenchmarkOtterSetGet-8        	51896601	       159.1 ns/op	       8 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	113.829s
```

with zipfian read (99%) and randomly write(1%)
```
goos: linux
goarch: amd64
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkHashicorpSetGet
BenchmarkHashicorpSetGet-8    	14631464	       418.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkCloudflareSetGet
BenchmarkCloudflareSetGet-8   	48996306	       129.3 ns/op	      16 B/op	       1 allocs/op
BenchmarkEcacheSetGet
BenchmarkEcacheSetGet-8       	61667361	       101.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkLxzanSetGet
BenchmarkLxzanSetGet-8        	59331700	        99.66 ns/op	       0 B/op	       0 allocs/op
BenchmarkFreelruSetGet
BenchmarkFreelruSetGet-8      	57392088	       113.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkPhusluSetGet
BenchmarkPhusluSetGet-8       	78875428	        81.73 ns/op	       0 B/op	       0 allocs/op
BenchmarkCcacheSetGet
BenchmarkCcacheSetGet-8       	23366601	       270.9 ns/op	      21 B/op	       2 allocs/op
BenchmarkRistrettoSetGet
BenchmarkRistrettoSetGet-8    	44893608	       114.3 ns/op	      20 B/op	       1 allocs/op
BenchmarkTheineSetGet
BenchmarkTheineSetGet-8       	32717158	       172.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkOtterSetGet
BenchmarkOtterSetGet-8        	70170547	        85.62 ns/op	       1 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	96.989s
```

### GC scan

The GC scan times as below. Check github [gcscan][gcscan] action for more results and details.
<details>
  <summary>GC scan times on keysize=16(string), valuesize=8(int), cachesize in (100000,200000,400000,1000000)</summary>

```go
// env GODEBUG=gctrace=1 go run gcscan.go phuslu 1000000 
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"

	theine "github.com/Yiling-J/theine-go"
	"github.com/cespare/xxhash/v2"
	cloudflare "github.com/cloudflare/golibs/lrucache"
	ristretto "github.com/dgraph-io/ristretto"
	freelru "github.com/elastic/go-freelru"
	hashicorp "github.com/hashicorp/golang-lru/v2/expirable"
	ccache "github.com/karlseguin/ccache/v3"
	lxzan "github.com/lxzan/memorycache"
	otter "github.com/maypok86/otter"
	ecache "github.com/orca-zhang/ecache"
	phuslu "github.com/phuslu/lru"
)

const keysize = 16
var repeat, _ = strconv.Atoi(os.Getenv("repeat"))

var keys []string

func main() {
	name := os.Args[1]
	cachesize, _ := strconv.Atoi(os.Args[2])

	keys = make([]string, cachesize)
	for i := range cachesize {
		keys[i] = fmt.Sprintf(fmt.Sprintf("%%0%dd", keysize), i)
	}

	map[string]func(int){
		"nottl":      SetupNottl,
		"phuslu":     SetupPhuslu,
		"freelru":    SetupFreelru,
		"ristretto":  SetupRistretto,
		"otter":      SetupOtter,
		"lxzan":      SetupLxzan,
		"ecache":     SetupEcache,
		"cloudflare": SetupCloudflare,
		"ccache":     SetupCcache,
		"hashicorp":  SetupHashicorp,
		"theine":     SetupTheine,
	}[name](cachesize)
}

func SetupNottl(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache := phuslu.NewLRUCache[string, int](cachesize)
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.Set(keys[i], i)
		}
		runtime.GC()
	}
}

func SetupPhuslu(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache := phuslu.NewTTLCache[string, int](cachesize)
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.Set(keys[i], i, time.Hour)
		}
		runtime.GC()
	}
}

func SetupFreelru(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache, _ := freelru.NewSharded[string, int](uint32(cachesize), func(s string) uint32 { return uint32(xxhash.Sum64String(s)) })
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.AddWithLifetime(keys[i], i, time.Hour)
		}
		runtime.GC()
	}
}

func SetupOtter(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache, _ := otter.MustBuilder[string, int](cachesize).WithVariableTTL().Build()
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.Set(keys[i], i, time.Hour)
		}
		runtime.GC()
	}
}

func SetupEcache(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache := ecache.NewLRUCache(1024, uint16(cachesize/1024), time.Hour)
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.Put(keys[i], i)
		}
		runtime.GC()
	}
}

func SetupRistretto(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: int64(10 * cachesize), // number of keys to track frequency of (10M).
		MaxCost:     int64(cachesize),      // maximum cost of cache (1M).
		BufferItems: 64,                    // number of keys per Get buffer.
	})
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.SetWithTTL(keys[i], i, 1, time.Hour)
		}
		runtime.GC()
	}
}

func SetupLxzan(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache := lxzan.New[string, int](
		lxzan.WithBucketNum(128),
		lxzan.WithBucketSize(cachesize/128, cachesize/128),
		lxzan.WithInterval(time.Hour, time.Hour),
	)
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.Set(keys[i], i, time.Hour)
		}
		runtime.GC()
	}
}

func SetupTheine(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache, _ := theine.NewBuilder[string, int](int64(cachesize)).Build()
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.SetWithTTL(keys[i], i, 1, time.Hour)
		}
		runtime.GC()
	}
}

func SetupCloudflare(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache := cloudflare.NewMultiLRUCache(1024, uint(cachesize/1024))
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.Set(keys[i], i, time.Now().Add(time.Hour))
		}
		runtime.GC()
	}
}

func SetupCcache(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache := ccache.New(ccache.Configure[int]().MaxSize(int64(cachesize)).ItemsToPrune(100))
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.Set(keys[i], i, time.Hour)
		}
		runtime.GC()
	}
}

func SetupHashicorp(cachesize int) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	cache := hashicorp.NewLRU[string, int](cachesize, nil, time.Hour)
	runtime.GC()
	for range repeat {
		for i := range cachesize {
			cache.Add(keys[i], i)
		}
		runtime.GC()
	}
}
```
</details>

| GCScan     | 100000 | 200000 | 400000 | 1000000 |
| ---------- | ------ | ------ | ------ | ------- |
| phuslu     | 1 ms   | 3 ms   | 6 ms   | 15 ms   |
| freelru    | 2 ms   | 3 ms   | 6 ms   | 16 ms   |
| ristretto  | 4 ms   | 6 ms   | 9 ms   | 17 ms   |
| lxzan      | 2 ms   | 4 ms   | 7 ms   | 18 ms   |
| cloudflare | 5 ms   | 10 ms  | 21 ms  | 56 ms   |
| ecache     | 5 ms   | 10 ms  | 21 ms  | 57 ms   |
| ccache     | 5 ms   | 10 ms  | 22 ms  | 58 ms   |
| otter      | 6 ms   | 12 ms  | 28 ms  | 64 ms   |
| hashicorp  | 7 ms   | 16 ms  | 30 ms  | 79 ms   |
| theine     | 6 ms   | 13 ms  | 34 ms  | 83 ms   |

### Memory usage

The Memory usage result as below. Check github [memory][memory] action for more results and details.
<details>
  <summary>memory usage on keysize=16(string), valuesize=8(int), cachesize in (100000,200000,400000,1000000,2000000,4000000)</summary>

```go
// memusage.go
package main

import (
	"fmt"
	"os"
	"runtime"
	"time"
	"strconv"

	theine "github.com/Yiling-J/theine-go"
	"github.com/cespare/xxhash/v2"
	cloudflare "github.com/cloudflare/golibs/lrucache"
	ristretto "github.com/dgraph-io/ristretto"
	freelru "github.com/elastic/go-freelru"
	hashicorp "github.com/hashicorp/golang-lru/v2/expirable"
	ccache "github.com/karlseguin/ccache/v3"
	lxzan "github.com/lxzan/memorycache"
	otter "github.com/maypok86/otter"
	ecache "github.com/orca-zhang/ecache"
	phuslu "github.com/phuslu/lru"
)

const keysize = 16

var keys []string

func main() {
	name := os.Args[1]
	cachesize, _ := strconv.Atoi(os.Args[2])

	keys = make([]string, cachesize)
	for i := range cachesize {
		keys[i] = fmt.Sprintf(fmt.Sprintf("%%0%dd", keysize), i)
	}

	var o runtime.MemStats
	runtime.ReadMemStats(&o)

	map[string]func(int){
		"nottl":      SetupNottl,
		"phuslu":     SetupPhuslu,
		"freelru":    SetupFreelru,
		"ristretto":  SetupRistretto,
		"otter":      SetupOtter,
		"lxzan":      SetupLxzan,
		"ecache":     SetupEcache,
		"cloudflare": SetupCloudflare,
		"ccache":     SetupCcache,
		"hashicorp":  SetupHashicorp,
		"theine":     SetupTheine,
	}[name](cachesize)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("%s\t%d\t%v MB\t%v MB\t%v MB\n",
		name,
		cachesize,
		(m.Alloc-o.Alloc)/1048576,
		(m.TotalAlloc-o.TotalAlloc)/1048576,
		(m.Sys-o.Sys)/1048576,
	)
}

func SetupNottl(cachesize int) {
	cache := phuslu.NewLRUCache[string, int](cachesize)
	for i := range cachesize {
		cache.Set(keys[i], i)
	}
}

func SetupPhuslu(cachesize int) {
	cache := phuslu.NewTTLCache[string, int](cachesize)
	for i := range cachesize {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupFreelru(cachesize int) {
	cache, _ := freelru.NewSharded[string, int](uint32(cachesize), func(s string) uint32 { return uint32(xxhash.Sum64String(s)) })
	for i := range cachesize {
		cache.AddWithLifetime(keys[i], i, time.Hour)
	}
}

func SetupOtter(cachesize int) {
	cache, _ := otter.MustBuilder[string, int](cachesize).WithVariableTTL().Build()
	for i := range cachesize {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupEcache(cachesize int) {
	cache := ecache.NewLRUCache(1024, uint16(cachesize/1024), time.Hour)
	for i := range cachesize {
		cache.Put(keys[i], i)
	}
}

func SetupRistretto(cachesize int) {
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: int64(10 * cachesize), // number of keys to track frequency of (10M).
		MaxCost:     int64(cachesize),      // maximum cost of cache (1M).
		BufferItems: 64,             // number of keys per Get buffer.
	})
	for i := range cachesize {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}
}

func SetupLxzan(cachesize int) {
	cache := lxzan.New[string, int](
		lxzan.WithBucketNum(128),
		lxzan.WithBucketSize(cachesize/128, cachesize/128),
		lxzan.WithInterval(time.Hour, time.Hour),
	)
	for i := range cachesize {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupTheine(cachesize int) {
	cache, _ := theine.NewBuilder[string, int](int64(cachesize)).Build()
	for i := range cachesize {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}
}

func SetupCloudflare(cachesize int) {
	cache := cloudflare.NewMultiLRUCache(1024, uint(cachesize/1024))
	for i := range cachesize {
		cache.Set(keys[i], i, time.Now().Add(time.Hour))
	}
}

func SetupCcache(cachesize int) {
	cache := ccache.New(ccache.Configure[int]().MaxSize(int64(cachesize)).ItemsToPrune(100))
	for i := range cachesize {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupHashicorp(cachesize int) {
	cache := hashicorp.NewLRU[string, int](cachesize, nil, time.Hour)
	for i := range cachesize {
		cache.Add(keys[i], i)
	}
}
```
</details>

|            | 100000 | 200000 | 400000 | 1000000 | 2000000 | 4000000 |
| ---------- | ------ | ------ | ------ | ------- | ------- | ------- |
| nottl*     | 3 MB   | 7 MB   | 13 MB  | 39 MB   | 78 MB   | 155 MB  |
| phuslu     | 4 MB   | 8 MB   | 16 MB  | 46 MB   | 93 MB   | 185 MB  |
| lxzan      | 8 MB   | 17 MB  | 35 MB  | 95 MB   | 190 MB  | 379 MB  |
| ristretto* | 14 MB  | 15 MB  | 34 MB  | 89 MB   | 213 MB  | 412 MB  |
| otter      | 13 MB  | 22 MB  | 54 MB  | 104 MB  | 209 MB  | 419 MB  |
| freelru*   | 6 MB   | 14 MB  | 27 MB  | 112 MB  | 224 MB  | 448 MB  |
| ecache     | 11 MB  | 22 MB  | 44 MB  | 123 MB  | 238 MB  | 468 MB  |
| theine     | 15 MB  | 31 MB  | 62 MB  | 178 MB  | 357 MB  | 714 MB  |
| cloudflare | 15 MB  | 33 MB  | 64 MB  | 183 MB  | 358 MB  | 717 MB  |
| ccache     | 16 MB  | 33 MB  | 65 MB  | 183 MB  | 365 MB  | 730 MB  |
| hashicorp  | 18 MB  | 37 MB  | 57 MB  | 242 MB  | 484 MB  | 968 MB  |
- nottl saves 20% memory usage compared to phuslu by removing its ttl functionality, resulting in a slight increase in throughput.
- ristretto employs a questionable usage pattern due to its rejection of items via a bloom filter, resulting in a lower hit ratio.
- freelru overcommits the cache size to the next power of 2, leading to higher memory usage particularly at larger cache sizes.

### Hit ratio
It is a classic sharded LRU implementation, so the hit ratio is comparable to or slightly lower than a regular LRU.

### License
LRU is licensed under the MIT License. See the LICENSE file for details.

### Contact
For inquiries or support, contact phus.lu@gmail.com or raise github issues.

[godoc-img]: http://img.shields.io/badge/godoc-reference-blue.svg
[godoc]: https://pkg.go.dev/github.com/phuslu/lru
[release-img]: https://img.shields.io/github/v/tag/phuslu/lru?label=release
[release]: https://github.com/phuslu/lru/tags
[goreport-img]: https://goreportcard.com/badge/github.com/phuslu/lru
[goreport]: https://goreportcard.com/report/github.com/phuslu/lru
[benchmark]: https://github.com/phuslu/lru/actions/workflows/benchmark.yml
[memory]: https://github.com/phuslu/lru/actions/workflows/memory.yml
[gcscan]: https://github.com/phuslu/lru/actions/workflows/gcscan.yml
[codecov-img]: https://codecov.io/gh/phuslu/lru/graph/badge.svg?token=Q21AMQNM1K
[codecov]: https://codecov.io/gh/phuslu/lru
