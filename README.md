# a high-performance and gc-friendly LRU cache

[![godoc][godoc-img]][godoc] [![release][release-img]][release] [![goreport][goreport-img]][goreport]

### Features

* Simple
    - No Dependency
    - Less 500 lines of Go(excluding tests)
* Fast
    - Faster than all well-known **LRU** caches
    - Zero memory allocs 
* GC friendly
    - Pointerless data structs
    - Continuous memory layout
* Memory efficient
    - Uses only 24 extra bytes per cache object

### Getting Started

try on https://go.dev/play/p/tPcBftK0qJ8
```go
package main

import (
	"time"

	"github.com/phuslu/lru"
)

func main() {
	c := lru.New[string, int](1024)

	c.SetWithTTL("a", 1, 1000*time.Millisecond)
	println(c.Get("a"))

	time.Sleep(500 * time.Millisecond)
	println(c.Get("a"))

	time.Sleep(1500 * time.Millisecond)
	println(c.Get("a"))
}
```

### Benchmarks

A Performance result on keysize=16, cachesize=1000000, parallelism=32. Check [actions][actions] for more results and details.
```
goos: linux
goarch: amd64
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkCloudflareGet
BenchmarkCloudflareGet-8   	43232113	       145.9 ns/op	      16 B/op	       1 allocs/op
BenchmarkCcacheGet
BenchmarkCcacheGet-8       	48490944	       135.8 ns/op	      20 B/op	       2 allocs/op
BenchmarkEcacheGet
BenchmarkEcacheGet-8       	51344246	       115.7 ns/op	       0 B/op	       0 allocs/op
BenchmarkRistrettoGet
BenchmarkRistrettoGet-8    	56852104	       103.4 ns/op	      16 B/op	       1 allocs/op
BenchmarkTheineGet
BenchmarkTheineGet-8       	51490969	       108.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkOtterGet
BenchmarkOtterGet-8        	75847165	        74.34 ns/op	       0 B/op	       0 allocs/op
BenchmarkPhusluGet
BenchmarkPhusluGet-8       	72334320	        87.05 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	58.332s
```

[godoc-img]: http://img.shields.io/badge/godoc-reference-blue.svg
[godoc]: https://godoc.org/github.com/phuslu/lru
[release-img]: https://img.shields.io/github/v/tag/phuslu/lru?label=release
[release]: https://github.com/phuslu/lru/releases
[goreport-img]: https://goreportcard.com/badge/github.com/phuslu/lru
[goreport]: https://goreportcard.com/report/github.com/phuslu/lru
[actions]: https://github.com/phuslu/lru/actions/workflows/benchmark.yml
