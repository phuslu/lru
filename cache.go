// Copyright 2023 Phus Lu. All rights reserved.

// Package lru implements cache with least recent used eviction policy.
package lru

import (
	"errors"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"
)

// Cache implements LRU Cache with least recent used eviction policy.
type Cache[K comparable, V any] struct {
	shards [512]shard[K, V]
	mask   uint32
	hasher func(K unsafe.Pointer, seed uintptr) uintptr
	seed   uintptr
	loader func(key K) (value V, ttl time.Duration, err error)
	group  singleflight_Group[K, V]
}

var compactCache = runtime.GOARCH == "amd64"

// New creates lru cache with size capacity.
func New[K comparable, V any](size int, options ...Option[K, V]) *Cache[K, V] {
	clocking()

	j := -1
	for i, o := range options {
		if _, ok := o.(*shardsOption[K, V]); ok {
			j = i
		}
	}
	switch {
	case j < 0:
		options = append([]Option[K, V]{WithShards[K, V](0)}, options...)
	case j > 0:
		options[0], options[j] = options[j], options[0]
	}

	c := new(Cache[K, V])
	for _, o := range options {
		o.ApplyToCache(c)
	}

	c.hasher = getRuntimeHasher[K]()
	c.seed = uintptr(fastrand64())

	if compactCache {
		// pre-alloc lists and tables for compactness
		shardsize := (uint32(size) + c.mask) / (c.mask + 1)
		shardlists := make([]node[K, V], (shardsize+1)*(c.mask+1))
		tablesize := newTableSize(uint32(shardsize))
		tablebuckets := make([]struct{ hdib, index uint32 }, tablesize*(c.mask+1))
		for i := uint32(0); i <= c.mask; i++ {
			c.shards[i].list = shardlists[i*(shardsize+1) : (i+1)*(shardsize+1)]
			c.shards[i].table.buckets = tablebuckets[i*tablesize : (i+1)*tablesize]
			c.shards[i].Init(shardsize, c.hasher, c.seed)
		}
	} else {
		shardsize := (uint32(size) + c.mask) / (c.mask + 1)
		for i := uint32(0); i <= c.mask; i++ {
			c.shards[i].Init(shardsize, c.hasher, c.seed)
		}
	}

	return c
}

// Options implements LRU Cache Option.
type Option[K comparable, V any] interface {
	ApplyToCache(*Cache[K, V])
}

// WithShards specifies the shards count of cache.
func WithShards[K comparable, V any](count uint32) Option[K, V] {
	return &shardsOption[K, V]{count: count}
}

type shardsOption[K comparable, V any] struct {
	count uint32
}

func (o *shardsOption[K, V]) ApplyToCache(c *Cache[K, V]) {
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

// WithLoader specifies that use sliding cache or not.
func WithSliding[K comparable, V any](sliding bool) Option[K, V] {
	return &slidingOption[K, V]{sliding: sliding}
}

type slidingOption[K comparable, V any] struct {
	sliding bool
}

func (o *slidingOption[K, V]) ApplyToCache(c *Cache[K, V]) {
	for i := uint32(0); i <= c.mask; i++ {
		c.shards[i].sliding = o.sliding
	}
}

// WithLoader specifies that loader function of LoadingCache.
func WithLoader[K comparable, V any](loader func(key K) (value V, ttl time.Duration, err error)) Option[K, V] {
	return &loaderOption[K, V]{loader: loader}
}

type loaderOption[K comparable, V any] struct {
	loader func(K) (V, time.Duration, error)
}

func (o *loaderOption[K, V]) ApplyToCache(c *Cache[K, V]) {
	c.loader = o.loader
	c.group = singleflight_Group[K, V]{}
}

func nextPowOf2(n uint32) uint32 {
	k := uint32(1)
	for k < n {
		k = k * 2
	}
	return k
}

// Get returns value for key.
func (c *Cache[K, V]) Get(key K) (value V, ok bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	return c.shards[hash&c.mask].Get(hash, key)
}

var ErrLoaderIsNil = errors.New("loader is nil")

// GetOrLoad returns value for key, call loader function by singleflight if value was not in cache.
func (c *Cache[K, V]) GetOrLoad(key K) (value V, err error, ok bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	value, ok = c.shards[hash&c.mask].Get(hash, key)
	if !ok {
		if c.loader == nil {
			err = ErrLoaderIsNil
			return
		}
		value, err, ok = c.group.Do(key, func() (V, error) {
			v, ttl, err := c.loader(key)
			if err != nil {
				return v, err
			}
			c.shards[hash&c.mask].Set(hash, key, v, ttl)
			return v, nil
		})
	}
	return
}

// Peek returns value and expires nanoseconds for key, but does not modify its recency.
func (c *Cache[K, V]) Peek(key K) (value V, expires int64, ok bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	return c.shards[hash&c.mask].Peek(hash, key)
}

// Set inserts key value pair and returns previous value.
func (c *Cache[K, V]) Set(key K, value V, ttl time.Duration) (prev V, replaced bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	return c.shards[hash&c.mask].Set(hash, key, value, ttl)
}

// SetIfAbsent inserts key value pair and returns previous value, if key is absent in the cache.
func (c *Cache[K, V]) SetIfAbsent(key K, value V, ttl time.Duration) (prev V, replaced bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	return c.shards[hash&c.mask].SetIfAbsent(hash, key, value, ttl)
}

// Delete method deletes value associated with key and returns deleted value (or empty value if key was not in cache).
func (c *Cache[K, V]) Delete(key K) (prev V) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	return c.shards[hash&c.mask].Delete(hash, key)
}

// Len returns number of cached nodes.
func (c *Cache[K, V]) Len() int {
	var n uint32
	for i := uint32(0); i <= c.mask; i++ {
		n += c.shards[i].Len()
	}
	return int(n)
}

// AppendKeys appends all keys to keys and return the keys.
func (c *Cache[K, V]) AppendKeys(keys []K) []K {
	now := atomic.LoadUint32(&clock)
	for i := uint32(0); i <= c.mask; i++ {
		keys = c.shards[i].AppendKeys(keys, now)
	}
	return keys
}

// Stats represents cache stats.
type Stats struct {
	// GetCalls is the number of Get calls.
	GetCalls uint64

	// SetCalls is the number of Set calls.
	SetCalls uint64

	// Misses is the number of cache misses.
	Misses uint64
}

// Stats returns cache stats.
func (c *Cache[K, V]) Stats() (stats Stats) {
	for i := uint32(0); i <= c.mask; i++ {
		c.shards[i].mu.Lock()
		s := c.shards[i].stats
		c.shards[i].mu.Unlock()
		stats.GetCalls += s.getcalls
		stats.SetCalls += s.setcalls
		stats.Misses += s.misses
	}
	return
}

// noescape hides a pointer from escape analysis.  noescape is
// the identity function but escape analysis doesn't think the
// output depends on the input.  noescape is inlined and currently
// compiles down to zero instructions.
// USE CAREFULLY!
//
//go:nosplit
//go:nocheckptr
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}
