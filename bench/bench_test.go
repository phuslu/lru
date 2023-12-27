// go test -v -cpu=8 -run=none -bench=. -benchtime=5s -benchmem bench_test.go
package bench

import (
	"fmt"
	"testing"
	"time"
	_ "unsafe"

	cloudflare "github.com/cloudflare/golibs/lrucache"
	ristretto "github.com/dgraph-io/ristretto"
	ccache "github.com/karlseguin/ccache/v3"
	otter "github.com/maypok86/otter"
	phuslu "github.com/phuslu/lru"
)

const (
	keysize     = 16
	cachesize   = 65536
	parallelism = 2000
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
		cache.Set(keymap[i], i, time.Now().Add(time.Hour))
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get(keymap[fastrandn(cachesize)])
		}
	})
}

func BenchmarkCcacheGet(b *testing.B) {
	cache := ccache.New(ccache.Configure[int]().MaxSize(cachesize))
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keymap[i], i, time.Hour)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cache.Get(keymap[fastrandn(cachesize)])
		}
	})
}

func BenchmarkRistrettoGet(b *testing.B) {
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: cachesize, // number of keys to track frequency of (10M).
		MaxCost:     1 << 30,   // maximum cost of cache (1GB).
		BufferItems: 64,        // number of keys per Get buffer.
	})
	for i := 0; i < cachesize/2; i++ {
		cache.SetWithTTL(keymap[i], i, 1, time.Hour)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get(keymap[fastrandn(cachesize)])
		}
	})
}

func BenchmarkOtterGet(b *testing.B) {
	cache, err := otter.MustBuilder[string, int](cachesize).Build()
	if err != nil {
		panic(err)
	}
	for i := 0; i < cachesize/2; i++ {
		cache.SetWithTTL(keymap[i], i, time.Hour)
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
		cache.SetWithTTL(keymap[i], i, time.Hour)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get(keymap[fastrandn(cachesize)])
		}
	})
}
