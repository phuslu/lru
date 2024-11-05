// Copyright 2023-2024 Phus Lu. All rights reserved.

// Package lru implements cache with least recent used eviction policy.
package lru

import (
	"context"
	"unsafe"
)

// LRUCache implements LRU Cache with least recent used eviction policy.
type LRUCache[K comparable, V any] struct {
	shards [512]lrushard[K, V]
	mask   uint32
	hasher func(key unsafe.Pointer, seed uintptr) uintptr
	seed   uintptr
	loader func(ctx context.Context, key K) (value V, err error)
	group  singleflightGroup[K, V]
}

// NewLRUCache creates lru cache with size capacity.
func NewLRUCache[K comparable, V any](size int, options ...Option[K, V]) *LRUCache[K, V] {
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

	c := new(LRUCache[K, V])
	for _, o := range options {
		o.applyToLRUCache(c)
	}

	if c.hasher == nil {
		c.hasher = getRuntimeHasher[K]()
	}
	if c.seed == 0 {
		c.seed = uintptr(fastrand64())
	}

	if isamd64 {
		// pre-alloc lists and tables for compactness
		shardsize := (uint32(size) + c.mask) / (c.mask + 1)
		shardlists := make([]lrunode[K, V], (shardsize+1)*(c.mask+1))
		tablesize := lruNewTableSize(uint32(shardsize))
		tablebuckets := make([]uint64, tablesize*(c.mask+1))
		for i := uint32(0); i <= c.mask; i++ {
			c.shards[i].list = shardlists[i*(shardsize+1) : (i+1)*(shardsize+1)]
			c.shards[i].tableBuckets = tablebuckets[i*tablesize : (i+1)*tablesize]
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

// Get returns value for key.
func (c *LRUCache[K, V]) Get(key K) (value V, ok bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// return c.shards[hash&c.mask].Get(hash, key)
	return (*lrushard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Get(hash, key)
}

// GetOrLoad returns value for key, call loader function by singleflight if value was not in cache.
func (c *LRUCache[K, V]) GetOrLoad(ctx context.Context, key K, loader func(context.Context, K) (V, error)) (value V, err error, ok bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	value, ok = c.shards[hash&c.mask].Get(hash, key)
	if !ok {
		if loader == nil {
			loader = c.loader
		}
		if loader == nil {
			err = ErrLoaderIsNil
			return
		}
		value, err, ok = c.group.Do(key, func() (V, error) {
			v, err := loader(ctx, key)
			if err != nil {
				return v, err
			}
			c.shards[hash&c.mask].Set(hash, key, v)
			return v, nil
		})
	}
	return
}

// Peek returns value, but does not modify its recency.
func (c *LRUCache[K, V]) Peek(key K) (value V, ok bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// return c.shards[hash&c.mask].Peek(hash, key)
	return (*lrushard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Peek(hash, key)
}

// Set inserts key value pair and returns previous value.
func (c *LRUCache[K, V]) Set(key K, value V) (prev V, replaced bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// return c.shards[hash&c.mask].Set(hash, key, value)
	return (*lrushard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Set(hash, key, value)
}

// SetIfAbsent inserts key value pair and returns previous value, if key is absent in the cache.
func (c *LRUCache[K, V]) SetIfAbsent(key K, value V) (prev V, replaced bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// return c.shards[hash&c.mask].SetIfAbsent(hash, key, value)
	return (*lrushard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).SetIfAbsent(hash, key, value)
}

// Delete method deletes value associated with key and returns deleted value (or empty value if key was not in cache).
func (c *LRUCache[K, V]) Delete(key K) (prev V) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// return c.shards[hash&c.mask].Delete(hash, key)
	return (*lrushard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Delete(hash, key)
}

// Len returns number of cached nodes.
func (c *LRUCache[K, V]) Len() int {
	var n uint32
	for i := uint32(0); i <= c.mask; i++ {
		n += c.shards[i].Len()
	}
	return int(n)
}

// AppendKeys appends all keys to keys and return the keys.
func (c *LRUCache[K, V]) AppendKeys(keys []K) []K {
	for i := uint32(0); i <= c.mask; i++ {
		keys = c.shards[i].AppendKeys(keys)
	}
	return keys
}

// Stats returns cache stats.
func (c *LRUCache[K, V]) Stats() (stats Stats) {
	for i := uint32(0); i <= c.mask; i++ {
		s := &c.shards[i]
		s.mu.Lock()
		stats.EntriesCount += uint64(s.tableLength)
		stats.GetCalls += s.statsGetCalls
		stats.SetCalls += s.statsSetCalls
		stats.Misses += s.statsMisses
		s.mu.Unlock()
	}
	return
}
