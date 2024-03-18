package lru

import (
	"runtime"
	"time"
	"unsafe"
)

// Option is an interface for LRUCache and TTLCache configuration.
type Option[K comparable, V any] interface {
	ApplyToLRUCache(*LRUCache[K, V])
	ApplyToTTLCache(*TTLCache[K, V])
}

// WithShards specifies the shards count of cache.
func WithShards[K comparable, V any](count uint32) Option[K, V] {
	return &shardsOption[K, V]{count: count}
}

type shardsOption[K comparable, V any] struct {
	count uint32
}

func (o *shardsOption[K, V]) ApplyToLRUCache(c *LRUCache[K, V]) {
	var shardcount uint32
	if o.count == 0 {
		shardcount = nextPowOf2(uint32(runtime.GOMAXPROCS(0) * 16))
	} else {
		shardcount = nextPowOf2(o.count)
	}
	if maxcount := uint32(len(c.shards)); shardcount > maxcount {
		shardcount = maxcount
	}

	c.mask = uint32(shardcount) - 1
}

func (o *shardsOption[K, V]) ApplyToTTLCache(c *TTLCache[K, V]) {
	var shardcount uint32
	if o.count == 0 {
		shardcount = nextPowOf2(uint32(runtime.GOMAXPROCS(0) * 16))
	} else {
		shardcount = nextPowOf2(o.count)
	}
	if maxcount := uint32(len(c.shards)); shardcount > maxcount {
		shardcount = maxcount
	}

	c.mask = uint32(shardcount) - 1
}

// WithHasher specifies the hasher function of cache.
func WithHasher[K comparable, V any](hasher func(key unsafe.Pointer, seed uintptr) (hash uintptr)) Option[K, V] {
	return &hasherOption[K, V]{hasher: hasher}
}

type hasherOption[K comparable, V any] struct {
	hasher func(key unsafe.Pointer, seed uintptr) (hash uintptr)
}

func (o *hasherOption[K, V]) ApplyToLRUCache(c *LRUCache[K, V]) {
	c.hasher = o.hasher
}

func (o *hasherOption[K, V]) ApplyToTTLCache(c *TTLCache[K, V]) {
	c.hasher = o.hasher
}

// WithSliding specifies that use sliding cache or not.
func WithSliding[K comparable, V any](sliding bool) Option[K, V] {
	return &slidingOption[K, V]{sliding: sliding}
}

type slidingOption[K comparable, V any] struct {
	sliding bool
}

func (o *slidingOption[K, V]) ApplyToLRUCache(c *LRUCache[K, V]) {
	panic("not_supported")
}

func (o *slidingOption[K, V]) ApplyToTTLCache(c *TTLCache[K, V]) {
	for i := uint32(0); i <= c.mask; i++ {
		c.shards[i].sliding = o.sliding
	}
}

// WithLoader specifies that loader function of LoadingCache.
func WithLoader[K comparable, V any, Loader func(key K) (value V, err error) | func(key K) (value V, ttl time.Duration, err error)](loader Loader) Option[K, V] {
	return &loaderOption[K, V]{loader: loader}
}

type loaderOption[K comparable, V any] struct {
	loader any
}

func (o *loaderOption[K, V]) ApplyToLRUCache(c *LRUCache[K, V]) {
	loader, ok := o.loader.(func(key K) (value V, err error))
	if !ok {
		panic("not_supported")
	}
	c.loader = loader
	c.group = singleflight_Group[K, V]{}
}

func (o *loaderOption[K, V]) ApplyToTTLCache(c *TTLCache[K, V]) {
	loader, ok := o.loader.(func(key K) (value V, ttl time.Duration, err error))
	if !ok {
		panic("not_supported")
	}
	c.loader = loader
	c.group = singleflight_Group[K, V]{}
}

func nextPowOf2(n uint32) uint32 {
	k := uint32(1)
	for k < n {
		k = k * 2
	}
	return k
}
