# lru - a high-performance and gc-friendly LRU cache

[![godoc][godoc-img]][godoc] [![release][release-img]][release] [![goreport][goreport-img]][goreport]

### Features

* Simple
    - No Dependency
    - Less than 1000 sloc
* Fast
    - Faster than all well-known LRU caches
    - Zero memory allocs 
* GC friendly
    - Pointerless data structs
    - Continuous memory layout
* Memory efficient
    - Uses 24 extra bytes per cache object

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

A Performance result on keysize=16, cachesize=1000000, parallelism=32
```
goos: linux
goarch: arm64
pkg: bench
BenchmarkCloudflareGet
BenchmarkCloudflareGet-4   	32207244	       191.4 ns/op	      16 B/op	       1 allocs/op
BenchmarkCcacheGet
BenchmarkCcacheGet-4       	32112368	       183.4 ns/op	      20 B/op	       2 allocs/op
BenchmarkRistrettoGet
BenchmarkRistrettoGet-4    	39304964	       156.9 ns/op	      16 B/op	       1 allocs/op
BenchmarkEcacheGet
BenchmarkEcacheGet-4       	35331213	       166.2 ns/op	       0 B/op	       0 allocs/op
BenchmarkPhusluGet
BenchmarkPhusluGet-4       	45333045	       132.9 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	bench	42.942s
```

[godoc-img]: http://img.shields.io/badge/godoc-reference-blue.svg
[godoc]: https://godoc.org/github.com/phuslu/lru
[release-img]: https://img.shields.io/github/v/tag/phuslu/lru?label=release
[release]: https://github.com/phuslu/lru/releases
[goreport-img]: https://goreportcard.com/badge/github.com/phuslu/lru
[goreport]: https://goreportcard.com/report/github.com/phuslu/lru
