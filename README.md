# lru - a high-performance and gc-friendly LRU cache

[![godoc][godoc-img]][godoc] [![release][release-img]][release] [![goreport][goreport-img]][goreport]

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

A Performance result as below
```
goos: linux
goarch: amd64
cpu: Intel(R) Xeon(R) Silver 4216 CPU @ 2.10GHz
BenchmarkCloudflareGet
BenchmarkCloudflareGet-8   	100000000	        59.11 ns/op	      16 B/op	       1 allocs/op
BenchmarkCcacheGet
BenchmarkCcacheGet-8       	105142296	        56.85 ns/op	      20 B/op	       2 allocs/op
BenchmarkRistrettoGet
BenchmarkRistrettoGet-8    	131303994	        45.99 ns/op	      16 B/op	       1 allocs/op
BenchmarkGoburrowGet
BenchmarkGoburrowGet-8     	100000000	        62.94 ns/op	      16 B/op	       1 allocs/op
BenchmarkPhusluGet
BenchmarkPhusluGet-8       	176671556	        34.11 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	43.895s
```

[godoc-img]: http://img.shields.io/badge/godoc-reference-blue.svg
[godoc]: https://godoc.org/github.com/phuslu/lru
[release-img]: https://img.shields.io/github/v/tag/phuslu/lru?label=release
[release]: https://github.com/phuslu/lru/releases
[goreport-img]: https://goreportcard.com/badge/github.com/phuslu/lru
[goreport]: https://goreportcard.com/report/github.com/phuslu/lru
