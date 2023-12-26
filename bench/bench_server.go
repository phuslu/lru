package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	_ "unsafe"

	"github.com/phuslu/lru"
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

func main() {
	go func() {
		log.Println(http.ListenAndServe("127.0.0.1:6060", nil))
	}()

	cache := lru.New[string, int](cachesize)
	for i := 0; i < cachesize/2; i++ {
		cache.Set(keymap[i], i)
	}

	for {
		_, _ = cache.Get(keymap[fastrandn(cachesize)])
	}
}
