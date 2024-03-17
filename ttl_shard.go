// Copyright 2023 Phus Lu. All rights reserved.

package lru

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// ttlnode is a list ttlnode of LRU, storing key-value pairs and related information
type ttlnode[K comparable, V any] struct {
	key     K
	expires uint32
	next    uint32
	prev    uint32
	ttl     uint32
	value   V
}

type ttlbucket struct {
	hdib  uint32 // bitfield { hash:24 dib:8 }
	index uint32 // node index
}

// ttlshard is a LRU partition contains a list and a hash table.
type ttlshard[K comparable, V any] struct {
	mu sync.Mutex

	// the hash table, with 20% extra space than the list for fewer conflicts.
	table struct {
		buckets []ttlbucket
		mask    uint32
		length  uint32
		hasher  func(K unsafe.Pointer, seed uintptr) uintptr
		seed    uintptr
	}

	// the list of nodes
	list []ttlnode[K, V]

	sliding bool

	stats struct {
		getcalls uint64
		setcalls uint64
		misses   uint64
	}

	// padding
	_ [16]byte
}

func (s *ttlshard[K, V]) Init(size uint32, hasher func(K unsafe.Pointer, seed uintptr) uintptr, seed uintptr) {
	s.list_Init(size)
	s.table_Init(size, hasher, seed)
}

func (s *ttlshard[K, V]) Get(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	s.stats.getcalls++

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
			s.stats.misses++
		}
	} else {
		s.stats.misses++
	}

	s.mu.Unlock()

	return
}

func (s *ttlshard[K, V]) Peek(hash uint32, key K) (value V, expires int64, ok bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		value = s.list[index].value
		if e := s.list[index].expires; e > 0 {
			expires = (int64(e) + clockBase) * int64(time.Second)
		}
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *ttlshard[K, V]) SetIfAbsent(hash uint32, key K, value V, ttl time.Duration) (prev V, replaced bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		node := &s.list[index]
		prev = node.value
		if node.expires == 0 || atomic.LoadUint32(&clock) < node.expires {
			s.mu.Unlock()
			return
		}

		s.stats.setcalls++

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

	s.stats.setcalls++

	index := s.list_Back()
	node := &s.list[index]
	evictedValue := node.value
	s.table_Delete(uint32(s.table.hasher(noescape(unsafe.Pointer(&node.key)), s.table.seed)), node.key)

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

func (s *ttlshard[K, V]) Set(hash uint32, key K, value V, ttl time.Duration) (prev V, replaced bool) {
	s.mu.Lock()

	s.stats.setcalls++

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
	if key != node.key {
		s.table_Delete(uint32(s.table.hasher(noescape(unsafe.Pointer(&node.key)), s.table.seed)), node.key)
	}

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

func (s *ttlshard[K, V]) Delete(hash uint32, key K) (v V) {
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

func (s *ttlshard[K, V]) Len() (n uint32) {
	s.mu.Lock()
	// inlining s.table_Len()
	n = s.table.length
	s.mu.Unlock()

	return
}

func (s *ttlshard[K, V]) AppendKeys(dst []K, now uint32) []K {
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
