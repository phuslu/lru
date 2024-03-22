// Copyright 2023 Phus Lu. All rights reserved.

package lru

import (
	"sync/atomic"
	"time"
	"unsafe"
)

// TTLCache implements LRU Cache with TTL functionality.
type TTLCache[K comparable, V any] struct {
	shards [512]ttlshard[K, V]
	mask   uint32
	hasher func(key unsafe.Pointer, seed uintptr) uintptr
	seed   uintptr
	loader func(key K) (value V, ttl time.Duration, err error)
	group  singleflight_Group[K, V]
}

// NewTTLCache creates lru cache with size capacity.
func NewTTLCache[K comparable, V any](size int, options ...Option[K, V]) *TTLCache[K, V] {
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

	c := new(TTLCache[K, V])
	for _, o := range options {
		o.ApplyToTTLCache(c)
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
		shardlists := make([]ttlnode[K, V], (shardsize+1)*(c.mask+1))
		tablesize := ttlNewTableSize(uint32(shardsize))
		tablebuckets := make([]ttlbucket, tablesize*(c.mask+1))
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

// Get returns value for key.
func (c *TTLCache[K, V]) Get(key K) (value V, ok bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// return c.shards[hash&c.mask].Get(hash, key)
	return (*ttlshard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Get(hash, key)
}

// GetOrLoad returns value for key, call loader function by singleflight if value was not in cache.
func (c *TTLCache[K, V]) GetOrLoad(key K) (value V, err error, ok bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// value, ok = c.shards[hash&c.mask].Get(hash, key)
	value, ok = (*ttlshard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Get(hash, key)
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
func (c *TTLCache[K, V]) Peek(key K) (value V, expires int64, ok bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// return c.shards[hash&c.mask].Peek(hash, key)
	return (*ttlshard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Peek(hash, key)
}

// Set inserts key value pair and returns previous value.
func (c *TTLCache[K, V]) Set(key K, value V, ttl time.Duration) (prev V, replaced bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// return c.shards[hash&c.mask].Set(hash, key, value, ttl)
	return (*ttlshard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Set(hash, key, value, ttl)
}

// SetIfAbsent inserts key value pair and returns previous value, if key is absent in the cache.
func (c *TTLCache[K, V]) SetIfAbsent(key K, value V, ttl time.Duration) (prev V, replaced bool) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// return c.shards[hash&c.mask].SetIfAbsent(hash, key, value, ttl)
	return (*ttlshard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).SetIfAbsent(hash, key, value, ttl)
}

// Delete method deletes value associated with key and returns deleted value (or empty value if key was not in cache).
func (c *TTLCache[K, V]) Delete(key K) (prev V) {
	hash := uint32(c.hasher(noescape(unsafe.Pointer(&key)), c.seed))
	// return c.shards[hash&c.mask].Delete(hash, key)
	return (*ttlshard[K, V])(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Delete(hash, key)
}

// Len returns number of cached nodes.
func (c *TTLCache[K, V]) Len() int {
	var n uint32
	for i := uint32(0); i <= c.mask; i++ {
		n += c.shards[i].Len()
	}
	return int(n)
}

// AppendKeys appends all keys to keys and return the keys.
func (c *TTLCache[K, V]) AppendKeys(keys []K) []K {
	now := atomic.LoadUint32(&clock)
	for i := uint32(0); i <= c.mask; i++ {
		keys = c.shards[i].AppendKeys(keys, now)
	}
	return keys
}

// Stats returns cache stats.
func (c *TTLCache[K, V]) Stats() (stats Stats) {
	for i := uint32(0); i <= c.mask; i++ {
		s := &c.shards[i]
		s.mu.Lock()
		sl := s.table.length
		ss := s.stats
		s.mu.Unlock()
		stats.EntriesCount += uint64(sl)
		stats.GetCalls += ss.getcalls
		stats.SetCalls += ss.setcalls
		stats.Misses += ss.misses
	}
	return
}
