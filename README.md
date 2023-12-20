# lru - a thread-safe LRU cache with ttl support

[![godoc][godoc-img]][godoc] [![release][release-img]][release] [![goreport][goreport-img]][goreport]

### Getting Started

try on https://go.dev/play/p/yiKM7AAaynl
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	c := lru.New[string, int](1024)

	c.SetWithTTL("a", 1, 50*time.Millisecond)
	println(c.Get("a"))

	time.Sleep(10 * time.Millisecond)
	println(c.Get("a"))

	time.Sleep(100 * time.Millisecond)
	println(c.Get("a"))
}
```

### Benchmarks

<details>
<summary>The most common benchmarks(Parallel Get) against with cloudflare/elastic/hashicorp implementation:</summary>

```go
// go test -v -cpu=8 -run=none -bench=. -benchtime=5s -benchmem bench_test.go
package bench

import (
	"fmt"
	"testing"
	"time"
	_ "unsafe"

	"github.com/cespare/xxhash/v2"
	cloudflare "github.com/cloudflare/golibs/lrucache"
	elastic "github.com/elastic/go-freelru"
	hashicorp "github.com/hashicorp/golang-lru/v2"
	phuslu "github.com/phuslu/lru"
)

const (
	keysize     = 16
	cachesize   = 16384
	parallelism = 1000
)

var keymap = func() (x [cachesize]string) {
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
		cache.Set(keymap[i], i, time.Time{})
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get(keymap[fastrandn(cachesize)])
		}
	})
}

func elasticHashString(s string) uint32 {
	return uint32(xxhash.Sum64String(s))
}

func BenchmarkElasticGet(b *testing.B) {
	cache, _ := elastic.NewSynced[string, int](cachesize, elasticHashString)
	for i := 0; i < cachesize/2; i++ {
		cache.Add(keymap[i], i)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get(keymap[fastrandn(cachesize)])
		}
	})
}

func BenchmarkHashicorpGet(b *testing.B) {
	cache, _ := hashicorp.New[string, int](cachesize)
	for i := 0; i < cachesize/2; i++ {
		cache.Add(keymap[i], i)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get(keymap[fastrandn(cachesize)])
		}
	})
}

func BenchmarkPhusluGet(b *testing.B) {
	cache := phuslu.New[string, int](cachesize)
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keymap[i], i)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get(keymap[fastrandn(cachesize)])
		}
	})
}
```
</details>

A Performance result as below
```
goos: linux
goarch: amd64
cpu: Intel(R) Xeon(R) Silver 4216 CPU @ 2.10GHz
BenchmarkCloudflareGet
BenchmarkCloudflareGet-8   	100000000	        60.48 ns/op	      16 B/op	       1 allocs/op
BenchmarkElasticGet
BenchmarkElasticGet-8      	15093165	       424.4 ns/op	       0 B/op	       0 allocs/op
BenchmarkHashicorpGet
BenchmarkHashicorpGet-8    	27390493	       349.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkPhusluGet
BenchmarkPhusluGet-8       	199958748	        30.45 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	32.350s
```

[godoc-img]: http://img.shields.io/badge/godoc-reference-blue.svg
[godoc]: https://godoc.org/github.com/phuslu/lru
[release-img]: https://img.shields.io/github/v/tag/phuslu/lru?label=release
[release]: https://github.com/phuslu/lru/releases
[goreport-img]: https://goreportcard.com/badge/github.com/phuslu/lru
[goreport]: https://goreportcard.com/report/github.com/phuslu/lru
