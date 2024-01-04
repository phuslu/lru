# a high-performance and gc-friendly LRU cache

[![godoc][godoc-img]][godoc] [![release][release-img]][release] [![goreport][goreport-img]][goreport] [![codecov][codecov-img]][codecov]

### Features

* Simple
    - No Dependency
    - Less 1000 lines of Go
* Fast
    - Faster than all well-known **LRU** caches
    - Zero memory allocs 
* GC friendly
    - Pointerless data structs
    - Continuous memory layout
* Memory efficient
    - Uses only 24 extra bytes per cache object
    - TTL (2 x 4 bytes) + ArrayList node (2 x 4 bytes) + Key hash (4 bytes) + Hash Entry index (4 bytes)

### Getting Started

An out of box example. https://go.dev/play/p/QmllF57birV
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	cache := lru.New[string, int](1024)

	cache.SetWithTTL("a", 1, 2*time.Second)
	println(cache.Get("a"))

	time.Sleep(1 * time.Second)
	println(cache.Get("a"))

	time.Sleep(2 * time.Second)
	println(cache.Get("a"))
}
```

New a LRU loading cache. https://go.dev/play/p/S261F8ij2BL
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	cache := lru.NewWithLoader[string, int](1024, func(string) (int, time.Duration, error) {
		return 42, time.Hour, nil
	})

	println(cache.Get("b"))
	println(cache.GetOrLoad("b", nil))
	println(cache.Get("b"))
}
```

Using as a sliding cache. https://go.dev/play/p/usCPrTN34Xp
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	cache := lru.New[string, int](4096)

	cache.SetWithTTL("foobar", 42, 3*time.Second)

	time.Sleep(2 * time.Second)
	println(cache.TouchGet("foobar"))

	time.Sleep(2 * time.Second)
	println(cache.TouchGet("foobar"))

	time.Sleep(2 * time.Second)
	println(cache.TouchGet("foobar"))
}
```

### Benchmarks

A Performance result as below. Check [actions][actions] for more results and details.
<details>
  <summary>benchmark on keysize=16, cachesize=1000000, parallelism=32 with read (90%) / write(10%)</summary>

  ```go
  // go test -v -cpu=8 -run=none -bench=. -benchtime=5s -benchmem bench_test.go
  package bench

  import (
  	"fmt"
  	"testing"
  	"time"
  	_ "unsafe"

  	theine "github.com/Yiling-J/theine-go"
  	cloudflare "github.com/cloudflare/golibs/lrucache"
  	ristretto "github.com/dgraph-io/ristretto"
  	otter "github.com/maypok86/otter"
  	ecache "github.com/orca-zhang/ecache"
  	phuslu "github.com/phuslu/lru"
  )

  const (
  	keysize     = 16
  	cachesize   = 1000000
  	parallelism = 32
  	writeradio  = 0.1
  )

  var keymap = func() (x []string) {
  	x = make([]string, cachesize)
  	for i := 0; i < cachesize; i++ {
  		x[i] = fmt.Sprintf(fmt.Sprintf("%%0%dd", keysize), i)
  	}
  	return
  }()

  //go:noescape
  //go:linkname fastrandn runtime.fastrandn
  func fastrandn(x uint32) uint32

  func BenchmarkCloudflareGet(b *testing.B) {
  	cache := cloudflare.NewMultiLRUCache(1024, cachesize/1024)
  	for i := 0; i < cachesize/2; i++ {
  		cache.Set(keymap[i], i, time.Now().Add(time.Hour))
  	}
  	b.SetParallelism(parallelism)
  	b.ResetTimer()
  	b.RunParallel(func(pb *testing.PB) {
  		expires := time.Now().Add(time.Hour)
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.Set(keymap[i], i, expires)
  			} else {
  				cache.Get(keymap[i])
  			}
  		}
  	})
  }

  func BenchmarkEcacheGet(b *testing.B) {
  	cache := ecache.NewLRUCache(1024, cachesize/1024, time.Hour)
  	for i := 0; i < cachesize/2; i++ {
  		cache.Put(keymap[i], i)
  	}
  	b.SetParallelism(parallelism)
  	b.ResetTimer()
  	b.RunParallel(func(pb *testing.PB) {
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.Put(keymap[i], i)
  			} else {
  				cache.Get(keymap[i])
  			}
  		}
  	})
  }

  func BenchmarkRistrettoGet(b *testing.B) {
  	cache, _ := ristretto.NewCache(&ristretto.Config{
  		NumCounters: cachesize, // number of keys to track frequency of (10M).
  		MaxCost:     2 << 30,   // maximum cost of cache (2GB).
  		BufferItems: 64,        // number of keys per Get buffer.
  	})
  	for i := 0; i < cachesize/2; i++ {
  		cache.SetWithTTL(keymap[i], i, 1, time.Hour)
  	}

  	b.SetParallelism(parallelism)
  	b.ResetTimer()

  	b.RunParallel(func(pb *testing.PB) {
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.SetWithTTL(keymap[i], i, 1, time.Hour)
  			} else {
  				cache.Get(keymap[i])
  			}
  		}
  	})
  }

  func BenchmarkTheineGet(b *testing.B) {
  	cache, _ := theine.NewBuilder[string, int](cachesize).Build()
  	for i := 0; i < cachesize/2; i++ {
  		cache.SetWithTTL(keymap[i], i, 1, time.Hour)
  	}

  	b.SetParallelism(parallelism)
  	b.ResetTimer()

  	b.RunParallel(func(pb *testing.PB) {
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.SetWithTTL(keymap[i], i, 1, time.Hour)
  			} else {
  				cache.Get(keymap[i])
  			}
  		}
  	})
  }

  func BenchmarkOtterGet(b *testing.B) {
  	cache, _ := otter.MustBuilder[string, int](cachesize).Build()
  	for i := 0; i < cachesize/2; i++ {
  		cache.SetWithTTL(keymap[i], i, time.Hour)
  	}

  	b.SetParallelism(parallelism)
  	b.ResetTimer()

  	b.RunParallel(func(pb *testing.PB) {
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.SetWithTTL(keymap[i], i, time.Hour)
  			} else {
  				cache.Get(keymap[i])
  			}
  		}
  	})
  }

  func BenchmarkPhusluGet(b *testing.B) {
  	cache := phuslu.New[string, int](cachesize)
  	for i := 0; i < cachesize/2; i++ {
  		cache.SetWithTTL(keymap[i], i, time.Hour)
  	}

  	b.SetParallelism(parallelism)
  	b.ResetTimer()

  	b.RunParallel(func(pb *testing.PB) {
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.SetWithTTL(keymap[i], i, time.Hour)
  			} else {
  				cache.Get(keymap[i])
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
BenchmarkCloudflareGet
BenchmarkCloudflareGet-8   	34575751	       171.1 ns/op	      17 B/op	       1 allocs/op
BenchmarkEcacheGet
BenchmarkEcacheGet-8       	52730455	       114.2 ns/op	       5 B/op	       0 allocs/op
BenchmarkRistrettoGet
BenchmarkRistrettoGet-8    	34192080	       172.0 ns/op	      41 B/op	       1 allocs/op
BenchmarkTheineGet
BenchmarkTheineGet-8       	29694378	       202.5 ns/op	       0 B/op	       0 allocs/op
BenchmarkOtterGet
BenchmarkOtterGet-8        	76333077	        74.27 ns/op	       0 B/op	       0 allocs/op
BenchmarkPhusluGet
BenchmarkPhusluGet-8       	73868793	        82.88 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	46.827s
```

[godoc-img]: http://img.shields.io/badge/godoc-reference-blue.svg
[godoc]: https://godoc.org/github.com/phuslu/lru
[release-img]: https://img.shields.io/github/v/tag/phuslu/lru?label=release
[release]: https://github.com/phuslu/lru/releases
[goreport-img]: https://goreportcard.com/badge/github.com/phuslu/lru
[goreport]: https://goreportcard.com/report/github.com/phuslu/lru
[actions]: https://github.com/phuslu/lru/actions/workflows/benchmark.yml
[codecov-img]: https://codecov.io/gh/phuslu/lru/graph/badge.svg?token=Q21AMQNM1K
[codecov]: https://codecov.io/gh/phuslu/lru
