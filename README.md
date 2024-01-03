# a high-performance and gc-friendly LRU cache

[![godoc][godoc-img]][godoc] [![release][release-img]][release] [![goreport][goreport-img]][goreport]

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
    - TTL (8 bytes) + ArrayList node (2 x 4 bytes) + Key hash (4 bytes) + Hash Entry index (4 bytes)

### Getting Started

try on https://go.dev/play/p/f2653y1Kj6i
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	cache1 := lru.New[string, int](1024)

	cache1.SetWithTTL("a", 1, 200*time.Millisecond)
	println(cache1.Get("a"))

	time.Sleep(100 * time.Millisecond)
	println(cache1.Get("a"))

	time.Sleep(150 * time.Millisecond)
	println(cache1.Get("a"))

	cache2 := lru.NewWithLoader[string, int](1024, func(string) (int, time.Duration, error) {
		return 42, time.Hour, nil
	})

	println(cache2.Get("b"))
	println(cache2.GetOrLoad("b", nil))
	println(cache2.Get("b"))
}
```

### Benchmarks

A Performance result on keysize=16, cachesize=1000000, parallelism=32 with Read(90%)/Write(10%). Check [actions][actions] for more results and details.
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
