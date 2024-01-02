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

try on https://go.dev/play/p/XhMxFJe8jcy
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

A Performance result on keysize=16, cachesize=1000000, parallelism=32 with Read(75%)/Write(25%). Check [actions][actions] for more results and details.
```
goos: linux
goarch: amd64
cpu: AMD EPYC 7763 64-Core Processor                
BenchmarkCloudflareGet
BenchmarkCloudflareGet-8   	32122790	       183.9 ns/op	      18 B/op	       1 allocs/op
BenchmarkEcacheGet
BenchmarkEcacheGet-8       	49922084	       120.0 ns/op	       5 B/op	       0 allocs/op
BenchmarkRistrettoGet
BenchmarkRistrettoGet-8    	33818906	       195.6 ns/op	      41 B/op	       1 allocs/op
BenchmarkTheineGet
BenchmarkTheineGet-8       	29022984	       208.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkOtterGet
BenchmarkOtterGet-8        	71440144	        85.35 ns/op	       0 B/op	       0 allocs/op
BenchmarkPhusluGet
BenchmarkPhusluGet-8       	60555633	        98.51 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	61.404s
```

[godoc-img]: http://img.shields.io/badge/godoc-reference-blue.svg
[godoc]: https://godoc.org/github.com/phuslu/lru
[release-img]: https://img.shields.io/github/v/tag/phuslu/lru?label=release
[release]: https://github.com/phuslu/lru/releases
[goreport-img]: https://goreportcard.com/badge/github.com/phuslu/lru
[goreport]: https://goreportcard.com/report/github.com/phuslu/lru
[actions]: https://github.com/phuslu/lru/actions/workflows/benchmark.yml
