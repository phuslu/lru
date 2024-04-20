// Copyright 2023-2024 Phus Lu. All rights reserved.

package lru

import (
	"context"
	"unsafe"
)

// BytesCache implements Bytes Cache with least recent used eviction policy.
type BytesCache struct {
	shards []bytesshard
	mask   uint32
	group  singleflight_Group[string, []byte]
}

// NewBytesCache creates bytes cache with size capacity.
func NewBytesCache[K comparable, V any](size int) *BytesCache {
	c := new(BytesCache)

	c.mask = 511
	c.shards = make([]bytesshard, c.mask+1)

	shardsize := (uint32(size) + c.mask) / (c.mask + 1)
	for i := uint32(0); i <= c.mask; i++ {
		c.shards[i].Init(shardsize)
	}

	return c
}

// Get returns value for key.
func (c *BytesCache) Get(key []byte) (value []byte, ok bool) {
	hash := uint32(wyhash_HashString(b2s(key), 0))
	// return c.shards[hash&c.mask].Get(hash, key)
	return (*bytesshard)(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Get(hash, key)
}

// GetOrLoad returns value for key, call loader function by singleflight if value was not in cache.
func (c *BytesCache) GetOrLoad(ctx context.Context, key []byte, loader func(context.Context, []byte) ([]byte, error)) (value []byte, err error, ok bool) {
	hash := uint32(wyhash_HashString(b2s(key), 0))
	value, ok = c.shards[hash&c.mask].Get(hash, key)
	if !ok {
		if loader == nil {
			err = ErrLoaderIsNil
			return
		}
		value, err, ok = c.group.Do(b2s(key), func() ([]byte, error) {
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
func (c *BytesCache) Peek(key []byte) (value []byte, ok bool) {
	hash := uint32(wyhash_HashString(b2s(key), 0))
	// return c.shards[hash&c.mask].Peek(hash, key)
	return (*bytesshard)(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Peek(hash, key)
}

// Set inserts key value pair and returns previous value.
func (c *BytesCache) Set(key []byte, value []byte) (prev []byte, replaced bool) {
	hash := uint32(wyhash_HashString(b2s(key), 0))
	// return c.shards[hash&c.mask].Set(hash, key, value)
	return (*bytesshard)(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Set(hash, key, value)
}

// SetIfAbsent inserts key value pair and returns previous value, if key is absent in the cache.
func (c *BytesCache) SetIfAbsent(key []byte, value []byte) (prev []byte, replaced bool) {
	hash := uint32(wyhash_HashString(b2s(key), 0))
	// return c.shards[hash&c.mask].SetIfAbsent(hash, key, value)
	return (*bytesshard)(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).SetIfAbsent(hash, key, value)
}

// Delete method deletes value associated with key and returns deleted value (or empty value if key was not in cache).
func (c *BytesCache) Delete(key []byte) (prev []byte) {
	hash := uint32(wyhash_HashString(b2s(key), 0))
	// return c.shards[hash&c.mask].Delete(hash, key)
	return (*bytesshard)(unsafe.Add(unsafe.Pointer(&c.shards[0]), uintptr(hash&c.mask)*unsafe.Sizeof(c.shards[0]))).Delete(hash, key)
}

// Len returns number of cached nodes.
func (c *BytesCache) Len() int {
	var n uint32
	for i := uint32(0); i <= c.mask; i++ {
		n += c.shards[i].Len()
	}
	return int(n)
}

// AppendKeys appends all keys to keys and return the keys.
func (c *BytesCache) AppendKeys(keys [][]byte) [][]byte {
	for i := uint32(0); i <= c.mask; i++ {
		keys = c.shards[i].AppendKeys(keys)
	}
	return keys
}

// Stats returns cache stats.
func (c *BytesCache) Stats() (stats Stats) {
	for i := uint32(0); i <= c.mask; i++ {
		s := &c.shards[i]
		s.mu.Lock()
		stats.EntriesCount += uint64(s.table_length)
		stats.GetCalls += s.stats_getcalls
		stats.SetCalls += s.stats_setcalls
		stats.Misses += s.stats_misses
		s.mu.Unlock()
	}
	return
}
