// Copyright 2023 Phus Lu. All rights reserved.

// Package lru implements cache with least recent used eviction policy.
package lru

import (
	"runtime"
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
func New[K comparable, V any](size int) *Cache[K, V] {
	shardcount := 1
	for shardcount < runtime.NumCPU()*16 {
		shardcount *= 2
	}
	shardsize := 1
	for shardsize*shardcount < size {
		shardsize *= 2
	}
	return newWithShards[K, V](shardcount, shardsize)
}

func newWithShards[K comparable, V any](shardcount, shardsize int) *Cache[K, V] {
	c := &Cache[K, V]{
		shards: make([]shard[K, V], shardcount),
		mask:   uint32(shardcount) - 1,
		hasher: maphash_NewHasher[K](),
	}
	for i := range c.shards {
		c.shards[i] = *newshard[K, V](shardsize)
	}

	return c
}

// Get returns value for key.
func (c *Cache[K, V]) Get(key K) (value V, ok bool) {
	hash := uint32(c.hasher.Hash(key))
	return c.shards[hash&c.mask].Get(hash, key)
}

// TouchGet returns value for key and reset the expires with TTL(aka, Sliding Cache).
func (c *Cache[K, V]) TouchGet(key K) (value V, ok bool) {
	hash := uint32(c.hasher.Hash(key))
	return c.shards[hash&c.mask].TouchGet(hash, key)
}

// Peek returns value for key, but does not modify its recency.
func (c *Cache[K, V]) Peek(key K) (value V, ok bool) {
	hash := uint32(c.hasher.Hash(key))
	return c.shards[hash&c.mask].Peek(hash, key)
}

// Set inserts key value pair and returns previous value, if cache was full.
func (c *Cache[K, V]) Set(key K, value V) (prev V, replaced bool) {
	hash := uint32(c.hasher.Hash(key))
	return c.shards[hash&c.mask].Set(hash, c.hasher.Hash, key, value, 0)
}

// SetWithTTL inserts key value pair with ttl and returns previous value, if cache was full.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) (prev V, replaced bool) {
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

// NewWithLoader creates lru cache with size capacity and loader function.
func NewWithLoader[K comparable, V any](size int, loader func(K) (value V, ttl time.Duration, err error)) *Cache[K, V] {
	cache := New[K, V](size)
	cache.group = singleflight_Group[K, V]{}
	cache.loader = loader
	return cache
}

// Loader returns the global default loader function.
func (c *Cache[K, V]) Loader() func(K) (value V, ttl time.Duration, err error) {
	return c.loader
}

func (c *Cache[K, V]) getOrLoad(key K, loader func(K) (V, time.Duration, error), touch bool) (value V, err error, ok bool) {
	hash := uint32(c.hasher.Hash(key))
	if touch {
		value, ok = c.shards[hash&c.mask].TouchGet(hash, key)
	} else {
		value, ok = c.shards[hash&c.mask].Get(hash, key)
	}
	if !ok {
		if loader == nil {
			loader = c.loader
		}
		if loader == nil {
			return
		}
		value, err, ok = c.group.Do(key, func() (v V, err error) {
			var ttl time.Duration
			v, ttl, err = loader(key)
			if err != nil {
				return v, err
			}
			c.shards[hash&c.mask].Set(hash, c.hasher.Hash, key, v, ttl)
			return v, nil
		})
	}
	return
}

// GetOrLoad returns value for key, call loader function by singleflight if value was not in cache.
// If loader parameter is nil, use global loader function provided by NewWithLoader instead.
func (c *Cache[K, V]) GetOrLoad(key K, loader func(K) (V, time.Duration, error)) (value V, err error, ok bool) {
	return c.getOrLoad(key, loader, false)
}

// TouchGetOrLoad returns value for key and reset expires with TTL, call loader function by singleflight if value was not in cache.
// If loader parameter is nil, use global loader function provided by NewWithLoader instead.
func (c *Cache[K, V]) TouchGetOrLoad(key K, loader func(K) (V, time.Duration, error)) (value V, err error, ok bool) {
	return c.getOrLoad(key, loader, true)
}
