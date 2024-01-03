// go test -v -cpu=8 -run=none -bench=. -benchtime=5s -benchmem bench_test.go
package bench

import (
	"fmt"
	"testing"
	"time"
	_ "unsafe"

	theine "github.com/Yiling-J/theine-go"
	"github.com/cespare/xxhash/v2"
	cloudflare "github.com/cloudflare/golibs/lrucache"
	ristretto "github.com/dgraph-io/ristretto"
	freelru "github.com/elastic/go-freelru"
	otter "github.com/maypok86/otter"
	ecache "github.com/orca-zhang/ecache"
	phuslu "github.com/phuslu/lru"
)

const (
	keysize     = 16
	cachesize   = 1000000
	parallelism = 32
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
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if i >= cachesize/10 {
				cache.Get(keymap[i])
			} else {
				cache.Set(keymap[i], i, expires)
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
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if i >= cachesize/10 {
				cache.Get(keymap[i])
			} else {
				cache.Put(keymap[i], i)
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
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if i >= cachesize/10 {
				cache.Get(keymap[i])
			} else {
				cache.SetWithTTL(keymap[i], i, 1, time.Hour)
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
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if i >= cachesize/10 {
				cache.Get(keymap[i])
			} else {
				cache.SetWithTTL(keymap[i], i, 1, time.Hour)
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
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if i >= cachesize/10 {
				cache.Get(keymap[i])
			} else {
				cache.SetWithTTL(keymap[i], i, time.Hour)
			}
		}
	})
}

func hashStringXXHASH(s string) uint32 {
	return uint32(xxhash.Sum64String(s))
}

func BenchmarkFreelruGet(b *testing.B) {
	cache, _ := freelru.NewSharded[string, int](cachesize, hashStringXXHASH)
	for i := 0; i < cachesize/2; i++ {
		cache.AddWithLifetime(keymap[i], i, time.Hour)
	}

	b.SetParallelism(parallelism)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if i >= cachesize/10 {
				cache.Get(keymap[i])
			} else {
				cache.AddWithLifetime(keymap[i], i, time.Hour)
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
		for pb.Next() {
			i := int(fastrandn(cachesize))
			if i >= cachesize/10 {
				cache.Get(keymap[i])
			} else {
				cache.SetWithTTL(keymap[i], i, time.Hour)
			}
		}
	})
}
