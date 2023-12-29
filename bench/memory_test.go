// go test -v -cpu=8 -run=none -bench=. -benchtime=5s -benchmem bench_test.go
package bench

import (
	"fmt"
	"testing"
	"time"

	"github.com/DmitriyVTitov/size"
	cloudflare "github.com/cloudflare/golibs/lrucache"
	ristretto "github.com/dgraph-io/ristretto"
	goburrow "github.com/goburrow/cache"
	ccache "github.com/karlseguin/ccache/v3"
	phuslu "github.com/phuslu/lru"
)

const (
	keysize   = 16
	cachesize = 1_000_000
)

var keymap = func() (x []string) {
	x = make([]string, cachesize)
	for i := 0; i < cachesize; i++ {
		x[i] = fmt.Sprintf(fmt.Sprintf("%%0%dd", keysize), i)
	}
	return
}()

func TestCloudflareSize(t *testing.T) {
	cache := cloudflare.NewMultiLRUCache(1024, cachesize/1024)
	for i := 0; i < cachesize; i++ {
		cache.Set(keymap[i], i, time.Now().Add(time.Hour))
	}

	t.Logf("cache memory size %v", size.Of(cache))
}

func TTestCcacheSize(t *testing.T) {
	cache := ccache.New(ccache.Configure[int]().MaxSize(cachesize))
	for i := 0; i < cachesize; i++ {
		cache.Set(keymap[i], i, time.Hour)
	}

	t.Logf("cache memory size %v", size.Of(cache))
}

func TTestRistrettoSize(t *testing.T) {
	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: cachesize, // number of keys to track frequency of (10M).
		MaxCost:     1 << 30,   // maximum cost of cache (1GB).
		BufferItems: 64,        // number of keys per Get buffer.
	})
	for i := 0; i < cachesize; i++ {
		cache.SetWithTTL(keymap[i], i, 1, time.Hour)
	}

	t.Logf("cache memory size %v", size.Of(cache))
}

func TestGoburrowSize(t *testing.T) {
	cache := goburrow.New(
		goburrow.WithMaximumSize(cachesize),       // Limit number of entries in the cache.
		goburrow.WithExpireAfterAccess(time.Hour), // Expire entries after 1 minute since last accessed.
		goburrow.WithRefreshAfterWrite(time.Hour), // Expire entries after 2 minutes since last created.
	)
	for i := 0; i < cachesize; i++ {
		cache.Put(keymap[i], i)
	}

	t.Logf("cache memory size %v", size.Of(cache))
}

func TestPhusluSize(t *testing.T) {
	cache := phuslu.New[string, int](cachesize)
	for i := 0; i < cachesize/2; i++ {
		cache.SetWithTTL(keymap[i], i, time.Hour)
	}

	t.Logf("cache memory size %v", size.Of(cache))
}
