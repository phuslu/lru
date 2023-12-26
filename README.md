# lru - a thread-safe and gc-friendly LRU cache

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

A Performance result as below
```
goos: linux
goarch: amd64
cpu: Intel(R) Xeon(R) Silver 4216 CPU @ 2.10GHz
BenchmarkCloudflareGet
BenchmarkCloudflareGet-8   	97366081	        65.24 ns/op	      16 B/op	       1 allocs/op
BenchmarkOistrettoGet
BenchmarkOistrettoGet-8    	100000000	        50.97 ns/op	      16 B/op	       1 allocs/op
BenchmarkOtterGet
BenchmarkOtterGet-8        	356862066	        17.20 ns/op	       0 B/op	       0 allocs/op
BenchmarkPhusluGet
BenchmarkPhusluGet-8       	174274854	        34.73 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	command-line-arguments	24.309s
```

[godoc-img]: http://img.shields.io/badge/godoc-reference-blue.svg
[godoc]: https://godoc.org/github.com/phuslu/lru
[release-img]: https://img.shields.io/github/v/tag/phuslu/lru?label=release
[release]: https://github.com/phuslu/lru/releases
[goreport-img]: https://goreportcard.com/badge/github.com/phuslu/lru
[goreport]: https://goreportcard.com/report/github.com/phuslu/lru
