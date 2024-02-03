# a high-performance and gc-friendly LRU cache

[![godoc][godoc-img]][godoc] [![release][release-img]][release] [![goreport][goreport-img]][goreport] [![codecov][codecov-img]][codecov]

### Features

* Simple
    - No Dependencies.
    - 100% code coverage.
    - Less than 1000 lines of Go code.
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
    - Specifies shards count via `WithShards(count)` option.
    - Customize hasher function via `WithHasher(func(key K) (hash uint64))` option.
    - Using SlidingCache via `WithSilding(true)` option.
    - Create LoadingCache via `WithLoader(func(key K) (v V, ttl time.Duration, err error))` option.

### Limitation
1. The TTL is accurate to the nearest second.
2. Expired items are only removed when accessed again or the cache is full.

### Getting Started

An out of box example. https://go.dev/play/p/01hUdKwp2MC
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

	time.Sleep(1 * time.Second)
	println(cache.Get("a"))

	time.Sleep(2 * time.Second)
	println(cache.Get("a"))
}
```

Using a customized shards count.
```go
cache := lru.New[string, int](8192, lru.WithShards[string, int](64))

cache.Set("foobar", 42, 3*time.Second)
println(cache.Get("foobar"))
```

Using a customized hasher function.
```go
hasher := func(key string) (hash uint64) {
	hash = 5381
	for _, c := range []byte(key) {
		hash = hash*33 + uint64(c)
	}
	return
}

cache := lru.New[string, int](8192, lru.WithHasher[string, int](hasher))

cache.Set("foobar", 42, 3*time.Second)
println(cache.Get("foobar"))
```

Using as a sliding cache.
```go
cache := lru.New[string, int](8192, lru.WithSilding(true))

cache.Set("foobar", 42, 3*time.Second)

time.Sleep(2 * time.Second)
println(cache.Get("foobar"))

time.Sleep(2 * time.Second)
println(cache.Get("foobar"))

time.Sleep(2 * time.Second)
println(cache.Get("foobar"))
```

Create a loading cache.
```go
loader := func(key string) (int, time.Duration, error) {
	return 42, time.Hour, nil
}

cache := lru.New[string, int](8192, lru.WithLoader(loader))

println(cache.Get("b"))
println(cache.GetOrLoad("b"))
println(cache.Get("b"))
```

### Throughput benchmarks

A Performance result as below. Check github [actions][actions] for more results and details.
<details>
  <summary>benchmark on keysize=16, itemsize=1000000, cachesize=50%, concurrency=8</summary>

```go
// env writepecent=10 zipf=0 go test -v -cpu=8 -run=none -bench=. -benchtime=5s -benchmem bench_test.go
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
	hashicorp "github.com/hashicorp/golang-lru/v2/expirable"
	cloudflare "github.com/cloudflare/golibs/lrucache"
	ristretto "github.com/dgraph-io/ristretto"
	freelru "github.com/elastic/go-freelru"
	lxzan "github.com/lxzan/memorycache"
	otter "github.com/maypok86/otter"
	ecache "github.com/orca-zhang/ecache"
	phuslu "github.com/phuslu/lru"
)

const (
	keysize   = 16
	cachesize = 1000000
)

var threshold = func() uint32 {
	writepecent, _ := strconv.Atoi(os.Getenv("writepecent"))
	return ^uint32(0) / 100 * uint32(writepecent)
}()

var zipfian = func() (f func() uint64) {
	if os.Getenv("zipf") == "1" {
		f = rand.NewZipf(rand.New(rand.NewSource(time.Now().UnixNano())), 1.0001, 10, cachesize-1).Uint64
	}
	return
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
	cache := hashicorp.NewLRU[string, int](cachesize, nil, time.Hour)
	for i := 0; i < cachesize/2; i++ {
		cache.Add(keys[i], i)
	}

	b.ResetTimer()

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
	cache := cloudflare.NewMultiLRUCache(uint(shardcount), uint(cachesize/shardcount))
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Now().Add(time.Hour))
	}
	expires := time.Now().Add(time.Hour)

	b.ResetTimer()

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
	cache := ecache.NewLRUCache(uint16(shardcount), uint16(cachesize/shardcount), time.Hour)
	for i := 0; i < cachesize/2; i++ {
		cache.Put(keys[i], i)
	}

	b.ResetTimer()

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
	cache := lxzan.New[string, int](
		lxzan.WithBucketNum(shardcount),
		lxzan.WithBucketSize(cachesize/shardcount, cachesize/shardcount),
		lxzan.WithInterval(time.Hour, time.Hour),
	)
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()

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
	cache, _ := freelru.NewSharded[string, int](cachesize, hashStringXXHASH)
	for i := 0; i < cachesize/2; i++ {
		cache.AddWithLifetime(keys[i], i, time.Hour)
	}

	b.ResetTimer()

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

func BenchmarkRistrettoSetGet(b *testing.B) {
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 10 * cachesize, // number of keys to track frequency of (10M).
		MaxCost:     cachesize,      // maximum cost of cache (1M).
		BufferItems: 64,             // number of keys per Get buffer.
	})
	for i := 0; i < cachesize/2; i++ {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}

	b.ResetTimer()

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
	cache, _ := theine.NewBuilder[string, int](cachesize).Build()
	for i := 0; i < cachesize/2; i++ {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}

	b.ResetTimer()

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
	cache, _ := otter.MustBuilder[string, int](cachesize).WithVariableTTL().Build()
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()

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

func BenchmarkPhusluSetGet(b *testing.B) {
	cache := phuslu.New[string, int](cachesize, phuslu.WithShards[string, int](uint32(shardcount)))
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Hour)
	}

	b.ResetTimer()

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
BenchmarkHashicorpSetGet-8    	13397827	       556.6 ns/op	      11 B/op	       0 allocs/op
BenchmarkCloudflareSetGet
BenchmarkCloudflareSetGet-8   	36744186	       203.5 ns/op	      16 B/op	       1 allocs/op
BenchmarkEcacheSetGet
BenchmarkEcacheSetGet-8       	45918138	       144.3 ns/op	       2 B/op	       0 allocs/op
BenchmarkLxzanSetGet
BenchmarkLxzanSetGet-8        	46131553	       163.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkFreelruSetGet
BenchmarkFreelruSetGet-8      	54157987	       136.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkRistrettoSetGet
BenchmarkRistrettoSetGet-8    	36123840	       144.0 ns/op	      28 B/op	       1 allocs/op
BenchmarkTheineSetGet
BenchmarkTheineSetGet-8       	22808450	       295.6 ns/op	       4 B/op	       0 allocs/op
BenchmarkOtterSetGet
BenchmarkOtterSetGet-8        	39550353	       176.4 ns/op	       9 B/op	       0 allocs/op
BenchmarkPhusluSetGet
BenchmarkPhusluSetGet-8       	64300864	       123.0 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	90.217s
```

with zipfian read (99%) and randomly write(1%)
```
goos: linux
goarch: amd64
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkHashicorpSetGet
BenchmarkHashicorpSetGet-8    	14883876	       404.0 ns/op	       0 B/op	       0 allocs/op
BenchmarkCloudflareSetGet
BenchmarkCloudflareSetGet-8   	51935983	       124.6 ns/op	      16 B/op	       1 allocs/op
BenchmarkEcacheSetGet
BenchmarkEcacheSetGet-8       	62620203	        96.70 ns/op	       0 B/op	       0 allocs/op
BenchmarkLxzanSetGet
BenchmarkLxzanSetGet-8        	65998576	        95.72 ns/op	       0 B/op	       0 allocs/op
BenchmarkFreelruSetGet
BenchmarkFreelruSetGet-8      	59447082	       103.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkRistrettoSetGet
BenchmarkRistrettoSetGet-8    	44582882	       116.0 ns/op	      21 B/op	       1 allocs/op
BenchmarkTheineSetGet
BenchmarkTheineSetGet-8       	32880466	       171.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkOtterSetGet
BenchmarkOtterSetGet-8        	85051764	        79.06 ns/op	       1 B/op	       0 allocs/op
BenchmarkPhusluSetGet
BenchmarkPhusluSetGet-8       	75368127	        82.65 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	78.301s
```

### Memory usage

The Memory usage result as below. Check github [actions][actions] for more results and details.
<details>
  <summary>memory usage on keysize=16(string), valuesize=8(int), itemsize=1000000(1M), cachesize=100%</summary>

```go
// memusage.go
package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	theine "github.com/Yiling-J/theine-go"
	"github.com/cespare/xxhash/v2"
	cloudflare "github.com/cloudflare/golibs/lrucache"
	ristretto "github.com/dgraph-io/ristretto"
	freelru "github.com/elastic/go-freelru"
	hashicorp "github.com/hashicorp/golang-lru/v2/expirable"
	lxzan "github.com/lxzan/memorycache"
	otter "github.com/maypok86/otter"
	ecache "github.com/orca-zhang/ecache"
	phuslu "github.com/phuslu/lru"
)

const (
	keysize   = 16
	cachesize = 1000000
)

var keys []string

func main() {
	keys = make([]string, cachesize)
	for i := 0; i < cachesize; i++ {
		keys[i] = fmt.Sprintf(fmt.Sprintf("%%0%dd", keysize), i)
	}

	var o runtime.MemStats
	runtime.ReadMemStats(&o)

	name := os.Args[1]
	setup := map[string]func(){
		"phuslu":     SetupPhuslu,
		"freelru":    SetupFreelru,
		"ristretto":  SetupRistretto,
		"otter":      SetupOtter,
		"lxzan":      SetupLxzan,
		"ecache":     SetupEcache,
		"cloudflare": SetupCloudflare,
		"hashicorp":  SetupHashicorp,
		"theine":     SetupTheine,
	}[name]
	setup()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("%s\t%v MiB\t%v MiB\t%v MiB\n",
		name,
		(m.Alloc-o.Alloc)/1048576,
		(m.TotalAlloc-o.TotalAlloc)/1048576,
		(m.Sys-o.Sys)/1048576,
	)
}

func SetupPhuslu() {
	cache := phuslu.New[string, int](cachesize)
	for i := 0; i < cachesize; i++ {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupFreelru() {
	cache, _ := freelru.NewSharded[string, int](cachesize, func(s string) uint32 { return uint32(xxhash.Sum64String(s)) })
	for i := 0; i < cachesize; i++ {
		cache.AddWithLifetime(keys[i], i, time.Hour)
	}
}

func SetupOtter() {
	cache, _ := otter.MustBuilder[string, int](cachesize).WithVariableTTL().Build()
	for i := 0; i < cachesize; i++ {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupEcache() {
	cache := ecache.NewLRUCache(1024, cachesize/1024, time.Hour)
	for i := 0; i < cachesize; i++ {
		cache.Put(keys[i], i)
	}
}

func SetupRistretto() {
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 10 * cachesize, // number of keys to track frequency of (10M).
		MaxCost:     cachesize,      // maximum cost of cache (1M).
		BufferItems: 64,             // number of keys per Get buffer.
	})
	for i := 0; i < cachesize; i++ {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}
}

func SetupLxzan() {
	cache := lxzan.New[string, int](
		lxzan.WithBucketNum(128),
		lxzan.WithBucketSize(cachesize/128, cachesize/128),
		lxzan.WithInterval(time.Hour, time.Hour),
	)
	for i := 0; i < cachesize; i++ {
		cache.Set(keys[i], i, time.Hour)
	}
}

func SetupTheine() {
	cache, _ := theine.NewBuilder[string, int](cachesize).Build()
	for i := 0; i < cachesize; i++ {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}
}

func SetupCloudflare() {
	cache := cloudflare.NewMultiLRUCache(1024, cachesize/1024)
	for i := 0; i < cachesize; i++ {
		cache.Set(keys[i], i, time.Now().Add(time.Hour))
	}
}

func SetupHashicorp() {
	cache := hashicorp.NewLRU[string, int](cachesize, nil, time.Hour)
	for i := 0; i < cachesize; i++ {
		cache.Add(keys[i], i)
	}
}
```
</details>

| MemStats   | Alloc   | TotalAlloc | Sys     |
| ---------- | ------- | ---------- | ------- |
| phuslu     | 46 MiB  | 54 MiB     | 57 MiB  |
| lxzan      | 95 MiB  | 103 MiB    | 106 MiB |
| ristretto  | 107 MiB | 185 MiB    | 132 MiB |
| freelru    | 112 MiB | 120 MiB    | 122 MiB |
| ecache     | 123 MiB | 131 MiB    | 127 MiB |
| otter      | 137 MiB | 211 MiB    | 177 MiB |
| theine     | 177 MiB | 223 MiB    | 194 MiB |
| cloudflare | 183 MiB | 191 MiB    | 188 MiB |
| hashicorp  | 238 MiB | 306 MiB    | 270 MiB |

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
