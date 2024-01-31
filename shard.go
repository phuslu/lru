// Copyright 2023 Phus Lu. All rights reserved.

package lru

import (
	"sync"
	"sync/atomic"
	"time"
)

// node is a list node of LRU, storing key-value pairs and related information
type node[K comparable, V any] struct {
	key     K
	value   V
	expires uint32
	next    uint32
	prev    uint32
	ttl     uint32
}

// shard is a LRU partition contains a list and a hash table.
type shard[K comparable, V any] struct {
	mu sync.Mutex

	// the hash table, with 25% extra space than the list for fewer conflicts.
	table struct {
		buckets []struct {
			hdib  uint32 // bitfield { hash:24 dib:8 }
			index uint32 // node index
		}
		size   uint32
		length int
	}

	// the list of nodes
	list []node[K, V]

	sliding bool

	// padding
	_ [48]byte
}

func (s *shard[K, V]) Init(size uint32) {
	s.list_Init(size)
	s.table_Init(uint32(float64(size) / (loadFactor - 0.05)))
}

func (s *shard[K, V]) Get(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		if expires := s.list[index].expires; expires == 0 {
			s.list_MoveToFront(index)
			value = s.list[index].value
			ok = true
		} else if now := atomic.LoadUint32(&clock); now < expires {
			if s.sliding {
				s.list[index].expires = now + s.list[index].ttl
			}
			s.list_MoveToFront(index)
			value = s.list[index].value
			ok = true
		} else {
			s.list_MoveToBack(index)
			s.list[index].value = value
			s.table_Delete(hash, key)
		}
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Peek(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		value = s.list[index].value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) SetIfAbsent(hash uint32, hashfun func(K) uint64, key K, value V, ttl time.Duration) (prev V, replaced bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		node := &s.list[index]
		prev = node.value
		if node.expires == 0 || atomic.LoadUint32(&clock) < node.expires {
			s.mu.Unlock()
			return
		}

		node.value = value
		if ttl > 0 {
			node.ttl = uint32(ttl / time.Second)
			node.expires = atomic.LoadUint32(&clock) + node.ttl
		} else {
			node.ttl = 0
			node.expires = 0
		}
		replaced = true

		s.mu.Unlock()
		return
	}

	index := s.list_Back()
	node := &s.list[index]
	evictedValue := node.value
	s.table_Delete(uint32(hashfun(node.key)), node.key)

	node.key = key
	node.value = value
	if ttl > 0 {
		node.ttl = uint32(ttl / time.Second)
		node.expires = atomic.LoadUint32(&clock) + node.ttl
	}
	s.table_Set(hash, key, index)
	s.list_MoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *shard[K, V]) Set(hash uint32, hashfun func(K) uint64, key K, value V, ttl time.Duration) (prev V, replaced bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		node := &s.list[index]
		previousValue := node.value
		s.list_MoveToFront(index)
		node.value = value
		if ttl > 0 {
			node.ttl = uint32(ttl / time.Second)
			node.expires = atomic.LoadUint32(&clock) + node.ttl
		}
		prev = previousValue
		replaced = true

		s.mu.Unlock()
		return
	}

	index := s.list_Back()
	node := &s.list[index]
	evictedValue := node.value
	s.table_Delete(uint32(hashfun(node.key)), node.key)

	node.key = key
	node.value = value
	if ttl > 0 {
		node.ttl = uint32(ttl / time.Second)
		node.expires = atomic.LoadUint32(&clock) + node.ttl
	}
	s.table_Set(hash, key, index)
	s.list_MoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *shard[K, V]) Delete(hash uint32, key K) (v V) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		node := &s.list[index]
		value := node.value
		s.list_MoveToBack(index)
		node.value = v
		s.table_Delete(hash, key)
		v = value
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Len() (n int) {
	s.mu.Lock()
	// inlining s.table_Len()
	n = s.table.length
	s.mu.Unlock()

	return
}

func (s *shard[K, V]) AppendKeys(dst []K, now uint32) []K {
	s.mu.Lock()
	for _, b := range s.table.buckets {
		if b.index == 0 {
			continue
		}
		node := &s.list[b.index]
		if expires := node.expires; expires == 0 || now <= expires {
			dst = append(dst, node.key)
		}
	}
	s.mu.Unlock()

	return dst
}
