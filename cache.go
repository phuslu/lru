// Package lru implements cache with least recent used eviction policy.
package lru

import (
	"runtime"
	"time"
	"unsafe"
)

// Cache implements LRU Cache with least recent used eviction policy.
type Cache[K comparable, V any] struct {
	shards  []shard[K, V]
	mask    uint64
	keysize int
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

	c := &Cache[K, V]{
		shards: make([]shard[K, V], shardcount),
		mask:   uint64(shardcount) - 1,
	}
	for i := range c.shards {
		c.shards[i] = *newshard[K, V](shardsize)
	}

	var k K
	switch ((any)(k)).(type) {
	case string:
		c.keysize = 0
	default:
		c.keysize = int(unsafe.Sizeof(k))
	}

	return c
}

func (c *Cache[K, V]) hash(key K) uint64 {
	var hash uint64
	if c.keysize == 0 {
		hash = wyhash_HashString(*(*string)(unsafe.Pointer(&key)), 0)
	} else {
		hash = wyhash_HashString(*(*string)(unsafe.Pointer(&struct {
			data unsafe.Pointer
			len  int
		}{unsafe.Pointer(&key), c.keysize})), 0)
	}
	return hash
}

// Get returns value for key.
func (c *Cache[K, V]) Get(key K) (value V, ok bool) {
	hash := c.hash(key)
	return c.shards[hash&c.mask].Get(hash, key)
}

// Peek returns value for key (if key was in cache), but does not modify its recency.
func (c *Cache[K, V]) Peek(key K) (value V, ok bool) {
	hash := c.hash(key)
	return c.shards[hash&c.mask].Peek(hash, key)
}

// Set inserts key value pair and returns evicted value, if cache was full.
func (c *Cache[K, V]) Set(key K, value V) (prev V, replaced bool) {
	hash := c.hash(key)
	return c.shards[hash&c.mask].Set(hash, c.hash, key, value, 0)
}

// SetWithTTL inserts key value pair with ttl and returns evicted value, if cache was full.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) (prev V, replaced bool) {
	hash := c.hash(key)
	return c.shards[hash&c.mask].Set(hash, c.hash, key, value, ttl)
}

// Delete method deletes entry associated with key and returns pointer to deleted value (or nil if entry was not in cache).
func (c *Cache[K, V]) Delete(key K) (prev V) {
	hash := c.hash(key)
	return c.shards[hash&c.mask].Delete(hash, key)
}

// Len returns number of cached items.
func (c *Cache[K, V]) Len() int {
	n := 0
	for i := range c.shards {
		n += c.shards[i].Len()
	}
	return n
}
