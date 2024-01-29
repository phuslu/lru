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
    - Adds only 26 extra bytes per cache object.
    - Minimized memory usage compared to others.
* Feature optional
    - Specifies shards count via `WithShards(count)` option.
    - Customize hasher function via `WithHasher(func(key K) (hash uint64))` option.
    - Using SlidingCache via `WithSilding(true)` option.
    - Create LoadingCache via `WithLoader(func(key K) (v V, ttl time.Duration, err error))` option.

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
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	cache := lru.New[string, int](8192, lru.WithShards[string, int](64))

	cache.Set("foobar", 42, 3*time.Second)
	println(cache.Get("foobar"))
}
```

Using a customized hasher function.
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
	"github.com/zeebo/xxh3"
)

func main() {
	cache := lru.New[string, int](8192, lru.WithHasher[string, int](xxh3.HashString))

	cache.Set("foobar", 42, 3*time.Second)
	println(cache.Get("foobar"))
}
```

Using as a sliding cache.
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	cache := lru.New[string, int](8192, lru.WithSilding(true))

	cache.Set("foobar", 42, 3*time.Second)

	time.Sleep(2 * time.Second)
	println(cache.Get("foobar"))

	time.Sleep(2 * time.Second)
	println(cache.Get("foobar"))

	time.Sleep(2 * time.Second)
	println(cache.Get("foobar"))
}
```

Create a loading cache.
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	loader := func(key string) (int, time.Duration, error) {
		return 42, time.Hour, nil
	}

	cache := lru.New[string, int](8192, lru.WithLoader(loader))

	println(cache.Get("b"))
	println(cache.GetOrLoad("b"))
	println(cache.Get("b"))
}
```

### Benchmarks

A Performance result as below. Check github [actions][actions] for more results and details.
<details>
  <summary>benchmark on keysize=16, itemsize=1000000, cachesize=50%, concurrency=8 with randomly read (90%) / write(10%)</summary>

```go
// go test -v -cpu=8 -run=none -bench=. -benchtime=5s -benchmem bench_test.go
package bench

import (
	"crypto/sha1"
	"fmt"
	"testing"
	"runtime"
	"time"
	_ "unsafe"

	theine "github.com/Yiling-J/theine-go"
	"github.com/cespare/xxhash/v2"
	cloudflare "github.com/cloudflare/golibs/lrucache"
	ristretto "github.com/dgraph-io/ristretto"
	freelru "github.com/elastic/go-freelru"
	lxzan "github.com/lxzan/memorycache"
	otter "github.com/maypok86/otter"
	ecache "github.com/orca-zhang/ecache"
	phuslu "github.com/phuslu/lru"
)

const (
	keysize     = 16
	cachesize   = 1000000
	parallelism = 8
	writepecent = 10
)

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

const threshold = ^uint32(0) / 100 * writepecent
var shardcount = func() int {
	n := runtime.GOMAXPROCS(0) * 16
	k := 1
	for k < n {
		k = k * 2
	}
	return k
}()

func BenchmarkCloudflareGetSet(b *testing.B) {
	cache := cloudflare.NewMultiLRUCache(uint(shardcount), uint(cachesize/shardcount))
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Now().Add(time.Hour))
	}
	expires := time.Now().Add(time.Hour)

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if fastrand() <= threshold {
				cache.Set(keys[i], i, expires)
			} else {
				cache.Get(keys[i])
			}
		}
	})
}

func BenchmarkEcacheGetSet(b *testing.B) {
	cache := ecache.NewLRUCache(uint16(shardcount), uint16(cachesize/shardcount), time.Hour)
	for i := 0; i < cachesize/2; i++ {
		cache.Put(keys[i], i)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if fastrand() <= threshold {
				cache.Put(keys[i], i)
			} else {
				cache.Get(keys[i])
			}
		}
	})
}

func BenchmarkLxzanGetSet(b *testing.B) {
	cache := lxzan.New[string, int](
		lxzan.WithBucketNum(shardcount),
		lxzan.WithBucketSize(cachesize/shardcount, cachesize/shardcount),
		lxzan.WithInterval(time.Hour, time.Hour),
	)
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Hour)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if fastrand() <= threshold {
				cache.Set(keys[i], i, time.Hour)
			} else {
				cache.Get(keys[i])
			}
		}
	})
}

func hashStringXXHASH(s string) uint32 {
	return uint32(xxhash.Sum64String(s))
}

func BenchmarkFreelruGetSet(b *testing.B) {
	cache, _ := freelru.NewSharded[string, int](cachesize, hashStringXXHASH)
	for i := 0; i < cachesize/2; i++ {
		cache.AddWithLifetime(keys[i], i, time.Hour)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if fastrand() <= threshold {
				cache.AddWithLifetime(keys[i], i, time.Hour)
			} else {
				cache.Get(keys[i])
			}
		}
	})
}

func BenchmarkRistrettoGetSet(b *testing.B) {
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 10 * cachesize, // number of keys to track frequency of (10M).
		MaxCost:     cachesize,      // maximum cost of cache (1M).
		BufferItems: 64,             // number of keys per Get buffer.
	})
	for i := 0; i < cachesize/2; i++ {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if fastrand() <= threshold {
				cache.SetWithTTL(keys[i], i, 1, time.Hour)
			} else {
				cache.Get(keys[i])
			}
		}
	})
}

func BenchmarkTheineGetSet(b *testing.B) {
	cache, _ := theine.NewBuilder[string, int](cachesize).Build()
	for i := 0; i < cachesize/2; i++ {
		cache.SetWithTTL(keys[i], i, 1, time.Hour)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if fastrand() <= threshold {
				cache.SetWithTTL(keys[i], i, 1, time.Hour)
			} else {
				cache.Get(keys[i])
			}
		}
	})
}

func BenchmarkOtterGetSet(b *testing.B) {
	cache, _ := otter.MustBuilder[string, int](cachesize).WithVariableTTL().Build()
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Hour)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if fastrand() <= threshold {
				cache.Set(keys[i], i, time.Hour)
			} else {
				cache.Get(keys[i])
			}
		}
	})
}

func BenchmarkPhusluGetSet(b *testing.B) {
	cache := phuslu.New[string, int](cachesize, phuslu.WithShards[string, int](uint32(shardcount)))
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keys[i], i, time.Hour)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if fastrand() <= threshold {
				cache.Set(keys[i], i, time.Hour)
			} else {
				cache.Get(keys[i])
			}
		}
	})
}
```
</details>

```
goos: linux
goarch: amd64
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkCloudflareGetSet
BenchmarkCloudflareGetSet-8   	33137439	       214.0 ns/op	      16 B/op	       1 allocs/op
BenchmarkEcacheGetSet
BenchmarkEcacheGetSet-8       	42560864	       154.0 ns/op	       2 B/op	       0 allocs/op
BenchmarkLxzanGetSet
BenchmarkLxzanGetSet-8        	36670929	       190.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkFreelruGetSet
BenchmarkFreelruGetSet-8      	54970399	       157.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkRistrettoGetSet
BenchmarkRistrettoGetSet-8    	38773869	       144.2 ns/op	      27 B/op	       1 allocs/op
BenchmarkTheineGetSet
BenchmarkTheineGetSet-8       	25598589	       239.5 ns/op	       4 B/op	       0 allocs/op
BenchmarkOtterGetSet
BenchmarkOtterGetSet-8        	34046569	       226.7 ns/op	       9 B/op	       0 allocs/op
BenchmarkPhusluGetSet
BenchmarkPhusluGetSet-8       	48137439	       141.3 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	71.219s
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
	freelru "github.com/elastic/go-freelru"
	ristretto "github.com/dgraph-io/ristretto"
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
	switch name {
	case "phuslu":
		SetupPhuslu()
	case "freelru":
		SetupFreelru()
	case "ristretto":
		SetupRistretto()
	case "otter":
		SetupOtter()
	case "lxzan":
		SetupLxzan()
	case "ecache":
		SetupEcache()
	case "cloudflare":
		SetupCloudflare()
	case "theine":
		SetupTheine()
	default:
		panic("no cache name")
	}

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
```
</details>

| MemStats   | Alloc   | TotalAlloc | Sys     |
| ---------- | ------- | ---------- | ------- |
| phuslu     | 48 MiB  | 56 MiB     | 61 MiB  |
| lxzan      | 95 MiB  | 103 MiB    | 106 MiB |
| ristretto  | 109 MiB | 186 MiB    | 128 MiB |
| freelru    | 112 MiB | 120 MiB    | 122 MiB |
| ecache     | 123 MiB | 131 MiB    | 127 MiB |
| otter      | 137 MiB | 211 MiB    | 177 MiB |
| theine     | 177 MiB | 223 MiB    | 193 MiB |
| cloudflare | 183 MiB | 191 MiB    | 188 MiB |

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
