# a high-performance and gc-friendly LRU cache

[![godoc][godoc-img]][godoc] [![release][release-img]][release] [![goreport][goreport-img]][goreport] [![codecov][codecov-img]][codecov]

### Features

* Simple
    - No Dependencies.
    - Straightforward API and codes.
* Fast
    - Outperforms well-known *LRU* caches.
    - Zero memory allocations .
* GC friendly
    - Pointerless data structs.
    - Continuous memory layout.
* Memory efficient
    - Adds only 26 extra bytes per cache item.
    - Minimized memory usage compared to others.
* Feature optional
    - Using SlidingCache via `WithSliding(true)` option.
    - Create LoadingCache via `WithLoader(func(key K) (v V, ttl time.Duration, err error))` option.

### Limitation
1. The TTL is accurate to the nearest second.
2. Expired items are only removed when accessed again or the cache is full.

### Getting Started

An out of box example. https://go.dev/play/p/VdufWuwo4lE
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	cache := lru.New[string, int](8192)

	cache.Set("a", 1, 2*time.Second)
	println(cache.Get("a"))
	println(cache.Get("b"))

	time.Sleep(1 * time.Second)
	println(cache.Get("a"))

	time.Sleep(2 * time.Second)
	println(cache.Get("a"))

	stats := cache.Stats()
	println("GetCalls", stats.GetCalls, "SetCalls", stats.SetCalls, "Misses", stats.Misses)
}
```

### Throughput benchmarks

A Performance result as below. Check github [actions][actions] for more results and details.
<details>
  <summary>go1.22 benchmark on keysize=16, itemsize=1000000, cachesize=50%, concurrency=8</summary>

```go
// env writeratio=0.1 zipf=false go test -v -cpu=8 -run=none -bench=. -benchtime=5s -benchmem bench_test.go
package bench

import (
	"crypto/sha1"
	"fmt"
	"math/rand"
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

var threshold = func() uint32 {
	writeratio, _ := strconv.ParseFloat(os.Getenv("writeratio"), 64)
	return uint32(float64(^uint32(0)) * writeratio)
}()

var zipfian = func() (zipf func() uint64) {
	ok, _ := strconv.ParseBool(os.Getenv("zipf"))
	if !ok {
		return nil
	}
	return rand.NewZipf(rand.New(rand.NewSource(time.Now().UnixNano())), 1.0001, 10, cachesize-1).Uint64
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
	for i := 0; i < cachesize; i++ {
		x[i] = fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprint(i))))[:keysize]
	}
	return
}()

//go:noescape
//go:linkname fastrandn runtime.fastrandn
func fastrandn(x uint32) uint32

//go:noescape
//go:linkname fastrand runtime.fastrand
func fastrand() uint32

func BenchmarkHashicorpSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := hashicorp.NewLRU[string, int](cachesize, nil, time.Hour)
	for i := 0; i < cachesize/2; i++ {
		cache.Add(keys[i], i)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		zipf := zipfian()
		for pb.Next() {
			if threshold > 0 && fastrand() <= threshold {
				i := int(fastrandn(cachesize))
				cache.Add(keys[i], i)
			} else if zipf == nil {
				cache.Get(keys[fastrandn(cachesize)])
			} else {
				cache.Get(keys[zipf()])
			}
		}
	})
}

func BenchmarkCloudflareSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := cloudflare.NewMultiLRUCache(uint(shardcount), uint(cachesize/shardcount))
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Now().Add(time.Hour))
	}
	expires := time.Now().Add(time.Hour)

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		zipf := zipfian()
		for pb.Next() {
			if threshold > 0 && fastrand() <= threshold {
				i := int(fastrandn(cachesize))
				cache.Set(keys[i], i, expires)
			} else if zipf == nil {
				cache.Get(keys[fastrandn(cachesize)])
			} else {
				cache.Get(keys[zipf()])
			}
		}
	})
}

func BenchmarkEcacheSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := ecache.NewLRUCache(uint16(shardcount), uint16(cachesize/shardcount), time.Hour)
	for i := 0; i < cachesize/2; i++ {
		cache.Put(keys[i], i)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		zipf := zipfian()
		for pb.Next() {
			if threshold > 0 && fastrand() <= threshold {
				i := int(fastrandn(cachesize))
				cache.Put(keys[i], i)
			} else if zipf == nil {
				cache.Get(keys[fastrandn(cachesize)])
			} else {
				cache.Get(keys[zipf()])
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
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		zipf := zipfian()
		for pb.Next() {
			if threshold > 0 && fastrand() <= threshold {
				i := int(fastrandn(cachesize))
				cache.Set(keys[i], i, time.Hour)
			} else if zipf == nil {
				cache.Get(keys[fastrandn(cachesize)])
			} else {
				cache.Get(keys[zipf()])
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
	for i := 0; i < cachesize/2; i++ {
		cache.AddWithLifetime(keys[i], i, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		zipf := zipfian()
		for pb.Next() {
			if threshold > 0 && fastrand() <= threshold {
				i := int(fastrandn(cachesize))
				cache.AddWithLifetime(keys[i], i, time.Hour)
			} else if zipf == nil {
				cache.Get(keys[fastrandn(cachesize)])
			} else {
				cache.Get(keys[zipf()])
			}
		}
	})
}

func BenchmarkPhusluSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := phuslu.New[string, int](cachesize, phuslu.WithShards[string, int](uint32(shardcount)))
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		zipf := zipfian()
		for pb.Next() {
			if threshold > 0 && fastrand() <= threshold {
				i := int(fastrandn(cachesize))
				cache.Set(keys[i], i, time.Hour)
			} else if zipf == nil {
				cache.Get(keys[fastrandn(cachesize)])
			} else {
				cache.Get(keys[zipf()])
			}
		}
	})
}

func BenchmarkCcacheSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache := ccache.New(ccache.Configure[int]().MaxSize(cachesize).ItemsToPrune(100))
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		zipf := zipfian()
		for pb.Next() {
			if threshold > 0 && fastrand() <= threshold {
				i := int(fastrandn(cachesize))
				cache.Set(keys[i], i, time.Hour)
			} else if zipf == nil {
				cache.Get(keys[fastrandn(cachesize)])
			} else {
				cache.Get(keys[zipf()])
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
	for i := 0; i < cachesize/2; i++ {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		zipf := zipfian()
		for pb.Next() {
			if threshold > 0 && fastrand() <= threshold {
				i := int(fastrandn(cachesize))
				cache.SetWithTTL(keys[i], i, 1, time.Hour)
			} else if zipf == nil {
				cache.Get(keys[fastrandn(cachesize)])
			} else {
				cache.Get(keys[zipf()])
			}
		}
	})
}

func BenchmarkTheineSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache, _ := theine.NewBuilder[string, int](cachesize).Build()
	for i := 0; i < cachesize/2; i++ {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		zipf := zipfian()
		for pb.Next() {
			if threshold > 0 && fastrand() <= threshold {
				i := int(fastrandn(cachesize))
				cache.SetWithTTL(keys[i], i, 1, time.Hour)
			} else if zipf == nil {
				cache.Get(keys[fastrandn(cachesize)])
			} else {
				cache.Get(keys[zipf()])
			}
		}
	})
}

func BenchmarkOtterSetGet(b *testing.B) {
	c := perfbench.Open(b)
	cache, _ := otter.MustBuilder[string, int](cachesize).WithVariableTTL().Build()
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()
	c.Reset()
	b.RunParallel(func(pb *testing.PB) {
		zipf := zipfian()
		for pb.Next() {
			if threshold > 0 && fastrand() <= threshold {
				i := int(fastrandn(cachesize))
				cache.Set(keys[i], i, time.Hour)
			} else if zipf == nil {
				cache.Get(keys[fastrandn(cachesize)])
			} else {
				cache.Get(keys[zipf()])
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
BenchmarkHashicorpSetGet-8    	11607430	       574.4 ns/op	       3 B/op	       0 allocs/op
BenchmarkCloudflareSetGet
BenchmarkCloudflareSetGet-8   	34291885	       211.7 ns/op	      16 B/op	       1 allocs/op
BenchmarkEcacheSetGet
BenchmarkEcacheSetGet-8       	42259825	       159.2 ns/op	       2 B/op	       0 allocs/op
BenchmarkLxzanSetGet
BenchmarkLxzanSetGet-8        	43999303	       169.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkFreelruSetGet
BenchmarkFreelruSetGet-8      	47874778	       156.1 ns/op	       0 B/op	       0 allocs/op
BenchmarkPhusluSetGet
BenchmarkPhusluSetGet-8       	56541656	       130.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkCcacheSetGet
BenchmarkCcacheSetGet-8       	20182213	       369.2 ns/op	      34 B/op	       2 allocs/op
BenchmarkRistrettoSetGet
BenchmarkRistrettoSetGet-8    	33954415	       160.3 ns/op	      29 B/op	       1 allocs/op
BenchmarkTheineSetGet
BenchmarkTheineSetGet-8       	20122734	       322.5 ns/op	       5 B/op	       0 allocs/op
BenchmarkOtterSetGet
BenchmarkOtterSetGet-8        	36713918	       193.1 ns/op	       9 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	112.573s
```

with zipfian read (99%) and randomly write(1%)
```
goos: linux
goarch: amd64
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkHashicorpSetGet
BenchmarkHashicorpSetGet-8    	13828234	       448.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkCloudflareSetGet
BenchmarkCloudflareSetGet-8   	49162429	       130.9 ns/op	      16 B/op	       1 allocs/op
BenchmarkEcacheSetGet
BenchmarkEcacheSetGet-8       	58933701	       104.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkLxzanSetGet
BenchmarkLxzanSetGet-8        	58731279	       102.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkFreelruSetGet
BenchmarkFreelruSetGet-8      	55033344	       114.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkPhusluSetGet
BenchmarkPhusluSetGet-8       	72054231	        86.48 ns/op	       0 B/op	       0 allocs/op
BenchmarkCcacheSetGet
BenchmarkCcacheSetGet-8       	23310871	       270.9 ns/op	      21 B/op	       2 allocs/op
BenchmarkRistrettoSetGet
BenchmarkRistrettoSetGet-8    	48681564	       114.9 ns/op	      20 B/op	       1 allocs/op
BenchmarkTheineSetGet
BenchmarkTheineSetGet-8       	34271067	       176.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkOtterSetGet
BenchmarkOtterSetGet-8        	62846517	        85.34 ns/op	       1 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	95.788s
```

### Memory usage

The Memory usage result as below. Check github [actions][actions] for more results and details.
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
	for i := 0; i < cachesize; i++ {
		keys[i] = fmt.Sprintf(fmt.Sprintf("%%0%dd", keysize), i)
	}

	var o runtime.MemStats
	runtime.ReadMemStats(&o)

	map[string]func(int){
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

func SetupPhuslu(cachesize int) {
	cache := phuslu.New[string, int](cachesize)
	for i := 0; i < cachesize; i++ {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupFreelru(cachesize int) {
	cache, _ := freelru.NewSharded[string, int](uint32(cachesize), func(s string) uint32 { return uint32(xxhash.Sum64String(s)) })
	for i := 0; i < cachesize; i++ {
		cache.AddWithLifetime(keys[i], i, time.Hour)
	}
}

func SetupOtter(cachesize int) {
	cache, _ := otter.MustBuilder[string, int](cachesize).WithVariableTTL().Build()
	for i := 0; i < cachesize; i++ {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupEcache(cachesize int) {
	cache := ecache.NewLRUCache(1024, uint16(cachesize/1024), time.Hour)
	for i := 0; i < cachesize; i++ {
		cache.Put(keys[i], i)
	}
}

func SetupRistretto(cachesize int) {
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: int64(10 * cachesize), // number of keys to track frequency of (10M).
		MaxCost:     int64(cachesize),      // maximum cost of cache (1M).
		BufferItems: 64,             // number of keys per Get buffer.
	})
	for i := 0; i < cachesize; i++ {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}
}

func SetupLxzan(cachesize int) {
	cache := lxzan.New[string, int](
		lxzan.WithBucketNum(128),
		lxzan.WithBucketSize(cachesize/128, cachesize/128),
		lxzan.WithInterval(time.Hour, time.Hour),
	)
	for i := 0; i < cachesize; i++ {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupTheine(cachesize int) {
	cache, _ := theine.NewBuilder[string, int](int64(cachesize)).Build()
	for i := 0; i < cachesize; i++ {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}
}

func SetupCloudflare(cachesize int) {
	cache := cloudflare.NewMultiLRUCache(1024, uint(cachesize/1024))
	for i := 0; i < cachesize; i++ {
		cache.Set(keys[i], i, time.Now().Add(time.Hour))
	}
}

func SetupCcache(cachesize int) {
	cache := ccache.New(ccache.Configure[int]().MaxSize(int64(cachesize)).ItemsToPrune(100))
	for i := 0; i < cachesize; i++ {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupHashicorp(cachesize int) {
	cache := hashicorp.NewLRU[string, int](cachesize, nil, time.Hour)
	for i := 0; i < cachesize; i++ {
		cache.Add(keys[i], i)
	}
}
```
</details>

|            | 100000 | 200000 | 400000 | 1000000 | 2000000 | 4000000 |
| ---------- | ------ | ------ | ------ | ------- | ------- | ------- |
| phuslu     | 4 MB   | 8 MB   | 16 MB  | 46 MB   | 92 MB   | 186 MB  |
| lxzan      | 8 MB   | 17 MB  | 35 MB  | 95 MB   | 190 MB  | 379 MB  |
| ristretto* | 8 MB   | 14 MB  | 35 MB  | 89 MB   | 178 MB  | 413 MB  |
| freelru*   | 6 MB   | 13 MB  | 27 MB  | 112 MB  | 224 MB  | 448 MB  |
| ecache     | 11 MB  | 22 MB  | 44 MB  | 123 MB  | 238 MB  | 468 MB  |
| otter      | 14 MB  | 29 MB  | 62 MB  | 137 MB  | 274 MB  | 547 MB  |
| theine     | 14 MB  | 27 MB  | 62 MB  | 178 MB  | 357 MB  | 714 MB  |
| cloudflare | 16 MB  | 33 MB  | 64 MB  | 183 MB  | 358 MB  | 717 MB  |
| ccache     | 16 MB  | 33 MB  | 65 MB  | 182 MB  | 365 MB  | 730 MB  |
| hashicorp  | 18 MB  | 37 MB  | 58 MB  | 242 MB  | 484 MB  | 968 MB  |
- ristretto's usage is questionable, it rejects items by bloom filter and appears lower hit ratio.
- freelru overcommits the cache size to the next power of 2, leads to higher usage on larger sizes.

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
[actions]: https://github.com/phuslu/lru/actions/workflows/benchmark.yml
[codecov-img]: https://codecov.io/gh/phuslu/lru/graph/badge.svg?token=Q21AMQNM1K
[codecov]: https://codecov.io/gh/phuslu/lru
