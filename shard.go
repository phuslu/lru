// Copyright 2023 Phus Lu. All rights reserved.

package lru

import (
	"sync"
	"sync/atomic"
	"time"
)

type node[K comparable, V any] struct {
	expires uint32
	next    uint32
	prev    uint32
	ttl     uint32
	key     K
	value   V
}

// shard is a LRU partition contains a list and a hash table.
type shard[K comparable, V any] struct {
	mu sync.Mutex

	// hash table
	buckets []struct {
		hdib  uint32 // bitfield { hash:24 dib:8 }
		index uint32 // node index
	}

	// linked list
	list []node[K, V]

	mask   uint32
	length int

	// padding
	_ [56]byte
}

func (s *shard[K, V]) Get(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if index, exists := s.tableGet(hash, key); exists {
		if expires := s.list[index].expires; expires == 0 || atomic.LoadUint32(&clock) < expires {
			s.listMoveToFront(index)
			value = s.list[index].value
			ok = true
		} else {
			s.listMoveToBack(index)
			s.list[index].value = value
			s.tableDelete(hash, key)
		}
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) TouchGet(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if index, exists := s.tableGet(hash, key); exists {
		if expires := s.list[index].expires; expires == 0 {
			s.listMoveToFront(index)
			value = s.list[index].value
			ok = true
		} else if now := atomic.LoadUint32(&clock); now < expires {
			s.list[index].expires = now + s.list[index].ttl
			s.listMoveToFront(index)
			value = s.list[index].value
			ok = true
		} else {
			s.listMoveToBack(index)
			s.list[index].value = value
			s.tableDelete(hash, key)
		}
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Peek(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if index, exists := s.tableGet(hash, key); exists {
		value = s.list[index].value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Set(hash uint32, hashfun func(K) uint64, key K, value V, ttl time.Duration) (prev V, replaced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index, exists := s.tableGet(hash, key); exists {
		node := &s.list[index]
		previousValue := node.value
		s.listMoveToFront(index)
		node.value = value
		if ttl > 0 {
			node.ttl = uint32(ttl / time.Second)
			node.expires = atomic.LoadUint32(&clock) + node.ttl
		}
		prev = previousValue
		replaced = true
		return
	}

	index := s.listBack()
	node := &s.list[index]
	evictedValue := node.value
	s.tableDelete(uint32(hashfun(node.key)), node.key)

	node.key = key
	node.value = value
	if ttl > 0 {
		node.ttl = uint32(ttl / time.Second)
		node.expires = atomic.LoadUint32(&clock) + node.ttl
	}
	s.tableSet(hash, key, index)
	s.listMoveToFront(index)
	prev = evictedValue
	return
}

func (s *shard[K, V]) Delete(hash uint32, key K) (v V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index, exists := s.tableGet(hash, key); exists {
		node := &s.list[index]
		value := node.value
		s.listMoveToBack(index)
		node.value = v
		s.tableDelete(hash, key)
		v = value
	}

	return
}

func (s *shard[K, V]) Len() (n int) {
	s.mu.Lock()
	n = s.tableLen()
	s.mu.Unlock()

	return
}

func newshard[K comparable, V any](size int) *shard[K, V] {
	s := &shard[K, V]{}

	s.listInit(uint32(size))
	s.tableInit(int(float64(size) / (loadFactor - 0.05)))

	return s
}
