package lru_test

import (
	"context"
	"fmt"
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
	fmt.Println(cache.Get("foobar"))

	// Output:
	// 42 true
}

func ExampleWithLoader() {
	loader := func(ctx context.Context, key string) (int, time.Duration, error) {
		return 42, time.Hour, nil
	}

	cache := lru.NewTTLCache[string, int](4096, lru.WithLoader[string, int](loader))

	fmt.Println(cache.Get("a"))
	fmt.Println(cache.Get("b"))
	fmt.Println(cache.GetOrLoad(context.Background(), "a", nil))
	fmt.Println(cache.GetOrLoad(context.Background(), "b", func(context.Context, string) (int, time.Duration, error) { return 100, 0, nil }))
	fmt.Println(cache.Get("a"))
	fmt.Println(cache.Get("b"))

	// Output:
	// 0 false
	// 0 false
	// 42 <nil> false
	// 100 <nil> false
	// 42 true
	// 100 true
}

func ExampleWithShards() {
	cache := lru.NewTTLCache[string, int](4096, lru.WithShards[string, int](1))

	cache.Set("foobar", 42, 3*time.Second)
	fmt.Println(cache.Get("foobar"))

	// Output:
	// 42 true
}

func ExampleWithSliding() {
	cache := lru.NewTTLCache[string, int](4096, lru.WithSliding[string, int](true))

	cache.Set("foobar", 42, 3*time.Second)

	time.Sleep(2 * time.Second)
	fmt.Println(cache.Get("foobar"))

	time.Sleep(2 * time.Second)
	fmt.Println(cache.Get("foobar"))

	time.Sleep(2 * time.Second)
	fmt.Println(cache.Get("foobar"))

	// Output:
	// 42 true
	// 42 true
	// 42 true
}
