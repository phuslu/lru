// Copyright 2023-2024 Phus Lu. All rights reserved.

package lru

import (
	"sync"
	"unsafe"
)

// lrunode is a list of lru node, storing key-value pairs and related information
type lrunode[K comparable, V any] struct {
	key   K
	next  uint32
	prev  uint32
	value V
}

type lrubucket struct {
	hdib  uint32 // bitfield { hash:24 dib:8 }
	index uint32 // node index
}

// lrushard is a LRU partition contains a list and a hash table.
type lrushard[K comparable, V any] struct {
	mu sync.Mutex

	// the hash table, with 20% extra space than the list for fewer conflicts.
	table struct {
		buckets []lrubucket
		mask    uint32
		length  uint32
		hasher  func(key unsafe.Pointer, seed uintptr) uintptr
		seed    uintptr
	}

	// the list of nodes
	list []lrunode[K, V]

	stats struct {
		getcalls uint64
		setcalls uint64
		misses   uint64
	}

	// padding
	_ [24]byte
}

func (s *lrushard[K, V]) Init(size uint32, hasher func(key unsafe.Pointer, seed uintptr) uintptr, seed uintptr) {
	s.list_Init(size)
	s.table_Init(size, hasher, seed)
}

func (s *lrushard[K, V]) Get(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	s.stats.getcalls++

	if index, exists := s.table_Get(hash, key); exists {
		s.list_MoveToFront(index)
		// value = s.list[index].value
		value = (*lrunode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0]))).value
		ok = true
	} else {
		s.stats.misses++
	}

	s.mu.Unlock()

	return
}

func (s *lrushard[K, V]) Peek(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		value = s.list[index].value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *lrushard[K, V]) SetIfAbsent(hash uint32, key K, value V) (prev V, replaced bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		prev = s.list[index].value
		s.mu.Unlock()
		return
	}

	s.stats.setcalls++

	// index := s.list_Back()
	// node := &s.list[index]
	index := s.list[0].prev
	node := (*lrunode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	s.table_Delete(uint32(s.table.hasher(noescape(unsafe.Pointer(&node.key)), s.table.seed)), node.key)

	node.key = key
	node.value = value
	s.table_Set(hash, key, index)
	s.list_MoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *lrushard[K, V]) Set(hash uint32, key K, value V) (prev V, replaced bool) {
	s.mu.Lock()

	s.stats.setcalls++

	if index, exists := s.table_Get(hash, key); exists {
		// node := &s.list[index]
		node := (*lrunode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
		previousValue := node.value
		s.list_MoveToFront(index)
		node.value = value
		prev = previousValue
		replaced = true

		s.mu.Unlock()
		return
	}

	// index := s.list_Back()
	// node := &s.list[index]
	index := s.list[0].prev
	node := (*lrunode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	if key != node.key {
		s.table_Delete(uint32(s.table.hasher(noescape(unsafe.Pointer(&node.key)), s.table.seed)), node.key)
	}

	node.key = key
	node.value = value
	s.table_Set(hash, key, index)
	s.list_MoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *lrushard[K, V]) Delete(hash uint32, key K) (v V) {
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

func (s *lrushard[K, V]) Len() (n uint32) {
	s.mu.Lock()
	// inlining s.table_Len()
	n = s.table.length
	s.mu.Unlock()

	return
}

func (s *lrushard[K, V]) AppendKeys(dst []K, now uint32) []K {
	s.mu.Lock()
	for _, b := range s.table.buckets {
		if b.index == 0 {
			continue
		}
		dst = append(dst, s.list[b.index].key)
	}
	s.mu.Unlock()

	return dst
}
