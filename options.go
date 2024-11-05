package lru

import (
	"context"
	"errors"
	"runtime"
	"time"
	"unsafe"
)

// Option is an interface for LRUCache and TTLCache configuration.
type Option[K comparable, V any] interface {
	applyToLRUCache(*LRUCache[K, V])
	applyToTTLCache(*TTLCache[K, V])
}

// WithShards specifies the shards count of cache.
func WithShards[K comparable, V any](count uint32) Option[K, V] {
	return &shardsOption[K, V]{count: count}
}

type shardsOption[K comparable, V any] struct {
	count uint32
}

func (o *shardsOption[K, V]) getcount(maxcount uint32) uint32 {
	var shardcount uint32
	if o.count == 0 {
		shardcount = nextPowOf2(uint32(runtime.GOMAXPROCS(0) * 16))
	} else {
		shardcount = nextPowOf2(o.count)
	}
	if shardcount > maxcount {
		shardcount = maxcount
	}
	return shardcount
}

func (o *shardsOption[K, V]) applyToLRUCache(c *LRUCache[K, V]) {
	c.mask = o.getcount(uint32(len(c.shards))) - 1
}

func (o *shardsOption[K, V]) applyToTTLCache(c *TTLCache[K, V]) {
	c.mask = o.getcount(uint32(len(c.shards))) - 1
}

// WithHasher specifies the hasher function of cache.
func WithHasher[K comparable, V any](hasher func(key unsafe.Pointer, seed uintptr) (hash uintptr)) Option[K, V] {
	return &hasherOption[K, V]{hasher: hasher}
}

type hasherOption[K comparable, V any] struct {
	hasher func(key unsafe.Pointer, seed uintptr) (hash uintptr)
}

func (o *hasherOption[K, V]) applyToLRUCache(c *LRUCache[K, V]) {
	c.hasher = o.hasher
}

func (o *hasherOption[K, V]) applyToTTLCache(c *TTLCache[K, V]) {
	c.hasher = o.hasher
}

// WithSliding specifies that use sliding cache or not.
func WithSliding[K comparable, V any](sliding bool) Option[K, V] {
	return &slidingOption[K, V]{sliding: sliding}
}

type slidingOption[K comparable, V any] struct {
	sliding bool
}

func (o *slidingOption[K, V]) applyToLRUCache(c *LRUCache[K, V]) {
	panic("not_supported")
}

func (o *slidingOption[K, V]) applyToTTLCache(c *TTLCache[K, V]) {
	for i := uint32(0); i <= c.mask; i++ {
		c.shards[i].sliding = o.sliding
	}
}

var ErrLoaderIsNil = errors.New("loader is nil")

// WithLoader specifies that loader function of LoadingCache.
func WithLoader[K comparable, V any, Loader ~func(ctx context.Context, key K) (value V, err error) | ~func(ctx context.Context, key K) (value V, ttl time.Duration, err error)](loader Loader) Option[K, V] {
	return &loaderOption[K, V]{loader: loader}
}

type loaderOption[K comparable, V any] struct {
	loader any
}

func (o *loaderOption[K, V]) applyToLRUCache(c *LRUCache[K, V]) {
	loader, ok := o.loader.(func(ctx context.Context, key K) (value V, err error))
	if !ok {
		panic("not_supported")
	}
	c.loader = loader
	c.group = singleflightGroup[K, V]{}
}

func (o *loaderOption[K, V]) applyToTTLCache(c *TTLCache[K, V]) {
	loader, ok := o.loader.(func(ctx context.Context, key K) (value V, ttl time.Duration, err error))
	if !ok {
		panic("not_supported")
	}
	c.loader = loader
	c.group = singleflightGroup[K, V]{}
}

func nextPowOf2(n uint32) uint32 {
	k := uint32(1)
	for k < n {
		k = k * 2
	}
	return k
}

var isamd64 = runtime.GOARCH == "amd64"
