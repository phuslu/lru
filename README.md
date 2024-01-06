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
    - Adds only 24 extra bytes per cache object.
    - Saves at least [60%](#memory-usage) memory than others.
* Feature selected
    - LoadingCache with `GetOrLoad` method.
    - SlidingCache with `TouchGet` method.

### Getting Started

An out of box example. https://go.dev/play/p/QmllF57birV
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	cache := lru.New[string, int](8192)

	cache.SetWithTTL("a", 1, 2*time.Second)
	println(cache.Get("a"))

	time.Sleep(1 * time.Second)
	println(cache.Get("a"))

	time.Sleep(2 * time.Second)
	println(cache.Get("a"))
}
```

Create a loading cache. https://go.dev/play/p/S261F8ij2BL
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	cache := lru.NewWithLoader[string, int](8192, func(string) (int, time.Duration, error) {
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
	cache := lru.New[string, int](8192)

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

A Performance result as below. Check github [actions][actions] for more results and details.
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

  var keys = func() (x []string) {
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
  		cache.Set(keys[i], i, time.Now().Add(time.Hour))
  	}
  	b.SetParallelism(parallelism)
  	b.ResetTimer()
  	b.RunParallel(func(pb *testing.PB) {
  		expires := time.Now().Add(time.Hour)
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.Set(keys[i], i, expires)
  			} else {
  				cache.Get(keys[i])
  			}
  		}
  	})
  }

  func BenchmarkEcacheGet(b *testing.B) {
  	cache := ecache.NewLRUCache(1024, cachesize/1024, time.Hour)
  	for i := 0; i < cachesize/2; i++ {
  		cache.Put(keys[i], i)
  	}
  	b.SetParallelism(parallelism)
  	b.ResetTimer()
  	b.RunParallel(func(pb *testing.PB) {
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.Put(keys[i], i)
  			} else {
  				cache.Get(keys[i])
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
  		cache.SetWithTTL(keys[i], i, 1, time.Hour)
  	}

  	b.SetParallelism(parallelism)
  	b.ResetTimer()

  	b.RunParallel(func(pb *testing.PB) {
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.SetWithTTL(keys[i], i, 1, time.Hour)
  			} else {
  				cache.Get(keys[i])
  			}
  		}
  	})
  }

  func BenchmarkTheineGet(b *testing.B) {
  	cache, _ := theine.NewBuilder[string, int](cachesize).Build()
  	for i := 0; i < cachesize/2; i++ {
  		cache.SetWithTTL(keys[i], i, 1, time.Hour)
  	}

  	b.SetParallelism(parallelism)
  	b.ResetTimer()

  	b.RunParallel(func(pb *testing.PB) {
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.SetWithTTL(keys[i], i, 1, time.Hour)
  			} else {
  				cache.Get(keys[i])
  			}
  		}
  	})
  }

  func BenchmarkOtterGet(b *testing.B) {
  	cache, _ := otter.MustBuilder[string, int](cachesize).Build()
  	for i := 0; i < cachesize/2; i++ {
  		cache.SetWithTTL(keys[i], i, time.Hour)
  	}

  	b.SetParallelism(parallelism)
  	b.ResetTimer()

  	b.RunParallel(func(pb *testing.PB) {
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.SetWithTTL(keys[i], i, time.Hour)
  			} else {
  				cache.Get(keys[i])
  			}
  		}
  	})
  }

  func BenchmarkPhusluGet(b *testing.B) {
  	cache := phuslu.New[string, int](cachesize)
  	for i := 0; i < cachesize/2; i++ {
  		cache.SetWithTTL(keys[i], i, time.Hour)
  	}

  	b.SetParallelism(parallelism)
  	b.ResetTimer()

  	b.RunParallel(func(pb *testing.PB) {
  		waterlevel := int(float32(cachesize) * writeradio)
  		for pb.Next() {
  			i := int(fastrandn(cachesize))
  			if i <= waterlevel {
  				cache.SetWithTTL(keys[i], i, time.Hour)
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
BenchmarkCloudflareGet
BenchmarkCloudflareGet-8    35573719         158.8 ns/op        16 B/op        1 allocs/op
BenchmarkEcacheGet
BenchmarkEcacheGet-8        50885976         116.8 ns/op         2 B/op        0 allocs/op
BenchmarkRistrettoGet
BenchmarkRistrettoGet-8     41817271         146.3 ns/op        27 B/op        1 allocs/op
BenchmarkTheineGet
BenchmarkTheineGet-8        33138063         180.4 ns/op         0 B/op        0 allocs/op
BenchmarkOtterGet
BenchmarkOtterGet-8         69140085          79.58 ns/op        0 B/op        0 allocs/op
BenchmarkPhusluGet
BenchmarkPhusluGet-8        71629431          86.70 ns/op        0 B/op        0 allocs/op
PASS
ok    command-line-arguments  46.622s
```

### Memory usage

The Memory usage result as below. Check github [actions][actions] for more results and details.
<details>
  <summary>memory usage on keysize=16(string), valuesize=8(int), cachesize=1000000(1M)</summary>

  ```go
  // memusage.go
  package main

  import (
  	"fmt"
  	"os"
  	"runtime"
  	"time"

  	theine "github.com/Yiling-J/theine-go"
  	cloudflare "github.com/cloudflare/golibs/lrucache"
  	ristretto "github.com/dgraph-io/ristretto"
  	otter "github.com/maypok86/otter"
  	ecache "github.com/orca-zhang/ecache"
  	phuslu "github.com/phuslu/lru"
  )

  const (
  	keysize   = 16
  	cachesize = 1000000
  )

  var keys = func() (x []string) {
  	x = make([]string, cachesize)
  	for i := 0; i < cachesize; i++ {
  		x[i] = fmt.Sprintf(fmt.Sprintf("%%0%dd", keysize), i)
  	}
  	return
  }()

  func main() {
  	var o runtime.MemStats
  	runtime.ReadMemStats(&o)

  	name := os.Args[1]
  	switch name {
  	case "phuslu":
  		SetupPhuslu()
  	case "ristretto":
  		SetupRistretto()
  	case "otter":
  		SetupOtter()
  	case "ecache":
  		SetupEcache()
  	case "cloudflare":
  		SetupCloudflare()
  	case "theine":
  		SetupTheine()
  	default:
  		panic("no cache name")
  	}

  	mb := func(n uint64) uint64 { return n / 1024 / 1024 }

  	var m runtime.MemStats
  	runtime.ReadMemStats(&m)
  	fmt.Printf("%s\t%v MiB\t%v MiB\t%v MiB\n", name, mb(m.Alloc-o.Alloc), mb(m.TotalAlloc-o.TotalAlloc), mb(m.Sys-o.Sys))
  }

  func SetupPhuslu() {
  	cache := phuslu.New[string, int](cachesize)
  	for i := 0; i < cachesize; i++ {
  		cache.SetWithTTL(keys[i], i, time.Hour)
  	}
  }

  func SetupOtter() {
  	cache, _ := otter.MustBuilder[string, int](cachesize).Build()
  	for i := 0; i < cachesize; i++ {
  		cache.SetWithTTL(keys[i], i, time.Hour)
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
  		NumCounters: cachesize,
  		MaxCost:     2 << 30,
  		BufferItems: 64,
  	})
  	for i := 0; i < cachesize; i++ {
  		cache.SetWithTTL(keys[i], i, 1, time.Hour)
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
| ecache     | 123 MiB | 131 MiB    | 127 MiB |
| ristretto  | 135 MiB | 219 MiB    | 144 MiB |
| otter      | 137 MiB | 211 MiB    | 181 MiB |
| theine     | 177 MiB | 223 MiB    | 194 MiB |
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
