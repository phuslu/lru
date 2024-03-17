package lru_test

import (
	"time"
	"unsafe"

	"github.com/phuslu/lru"
)

func ExampleWithHasher() {
	hasher := func(key unsafe.Pointer, seed uintptr) (hash uintptr) {
		hash = 5381
		for _, c := range []byte(*(*string)(key)) {
			hash = hash*33 + uintptr(c)
		}
		return
	}

	cache := lru.NewTTLCache[string, int](4096, lru.WithHasher[string, int](hasher))

	cache.Set("foobar", 42, 3*time.Second)
	println(cache.Get("foobar"))
}

func ExampleWithLoader() {
	loader := func(key string) (int, time.Duration, error) {
		return 42, time.Hour, nil
	}

	cache := lru.NewTTLCache[string, int](4096, lru.WithLoader(loader))

	println(cache.Get("b"))
	println(cache.GetOrLoad("b"))
	println(cache.Get("b"))
}

func ExampleWithShards() {
	cache := lru.NewTTLCache[string, int](4096, lru.WithShards[string, int](1))

	cache.Set("foobar", 42, 3*time.Second)
	println(cache.Get("foobar"))
}

func ExampleWithSliding() {
	cache := lru.NewTTLCache[string, int](4096, lru.WithSliding[string, int](true))

	cache.Set("foobar", 42, 3*time.Second)

	time.Sleep(2 * time.Second)
	println(cache.Get("foobar"))

	time.Sleep(2 * time.Second)
	println(cache.Get("foobar"))

	time.Sleep(2 * time.Second)
	println(cache.Get("foobar"))
}
