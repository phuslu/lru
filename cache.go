// Copyright 2023 Phus Lu. All rights reserved.

// Package lru implements cache with least recent used eviction policy.
package lru

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

// Cache implements LRU Cache with least recent used eviction policy.
type Cache[K comparable, V any] struct {
	shards []shard[K, V]
	mask   uint32
	hasher maphash_Hasher[K]
	loader func(key K) (value V, ttl time.Duration, err error)
	group  singleflight_Group[K, V]
}

// New creates lru cache with size capacity.
func New[K comparable, V any](size int, options ...Option[K, V]) *Cache[K, V] {
	shardcount := nextPowOf2(runtime.GOMAXPROCS(0) * 16)
	shardsize := nextPowOf2(size / shardcount)
	return newWithShards[K, V](shardcount, shardsize, options...)
}

// Options implements LRU Cache Option.
type Option[K comparable, V any] interface {
	Apply(*Cache[K, V])
}

// WithLoader specifies that use sliding cache or not.
func WithSliding[K comparable, V any](sliding bool) Option[K, V] {
	return &slidingOption[K, V]{sliding: sliding}
}

type slidingOption[K comparable, V any] struct {
	sliding bool
}

func (o *slidingOption[K, V]) Apply(c *Cache[K, V]) {
	for i := range c.shards {
		c.shards[i].sliding = o.sliding
	}
}

// WithLoader specifies that loader function of LoadingCache.
func WithLoader[K comparable, V any](loader func(K) (V, time.Duration, error)) Option[K, V] {
	return &loaderOption[K, V]{loader: loader}
}

type loaderOption[K comparable, V any] struct {
	loader func(K) (V, time.Duration, error)
}

func (o *loaderOption[K, V]) Apply(c *Cache[K, V]) {
	c.loader = o.loader
	c.group = singleflight_Group[K, V]{}
}

func nextPowOf2(n int) int {
	k := 1
	for k < n {
		k = k * 2
	}
	return k
}

func newWithShards[K comparable, V any](shardcount, shardsize int, options ...Option[K, V]) *Cache[K, V] {
	c := &Cache[K, V]{
		shards: make([]shard[K, V], shardcount),
		mask:   uint32(shardcount) - 1,
		hasher: maphash_NewHasher[K](),
	}
	for i := range c.shards {
		c.shards[i].Init(uint32(shardsize))
	}
	for _, option := range options {
		option.Apply(c)
	}

	return c
}

// Get returns value for key.
func (c *Cache[K, V]) Get(key K) (value V, ok bool) {
	hash := uint32(c.hasher.Hash(key))
	return c.shards[hash&c.mask].Get(hash, key)
}

// GetOrLoad returns value for key, call loader function by singleflight if value was not in cache.
func (c *Cache[K, V]) GetOrLoad(key K) (value V, err error, ok bool) {
	hash := uint32(c.hasher.Hash(key))
	value, ok = c.shards[hash&c.mask].Get(hash, key)
	if !ok {
		if c.loader == nil {
			err = fmt.Errorf("loader is nil")
			return
		}
		value, err, ok = c.group.Do(key, func() (V, error) {
			v, ttl, err := c.loader(key)
			if err != nil {
				return v, err
			}
			c.shards[hash&c.mask].Set(hash, c.hasher.Hash, key, v, ttl)
			return v, nil
		})
	}
	return
}

// Peek returns value for key, but does not modify its recency.
func (c *Cache[K, V]) Peek(key K) (value V, ok bool) {
	hash := uint32(c.hasher.Hash(key))
	return c.shards[hash&c.mask].Peek(hash, key)
}

// Set inserts key value pair and returns previous value, if cache was full.
func (c *Cache[K, V]) Set(key K, value V, ttl time.Duration) (prev V, replaced bool) {
	hash := uint32(c.hasher.Hash(key))
	return c.shards[hash&c.mask].Set(hash, c.hasher.Hash, key, value, ttl)
}

// Delete method deletes value associated with key and returns deleted value (or empty value if key was not in cache).
func (c *Cache[K, V]) Delete(key K) (prev V) {
	hash := uint32(c.hasher.Hash(key))
	return c.shards[hash&c.mask].Delete(hash, key)
}

// Len returns number of cached nodes.
func (c *Cache[K, V]) Len() int {
	n := 0
	for i := range c.shards {
		n += c.shards[i].Len()
	}
	return n
}

// AppendKeys appends all keys to keys and return the keys.
func (c *Cache[K, V]) AppendKeys(keys []K) []K {
	now := atomic.LoadUint32(&clock)
	for i := range c.shards {
		keys = c.shards[i].AppendKeys(keys, now)
	}
	return keys
}
