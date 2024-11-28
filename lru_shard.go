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

// lrushard is an LRU partition contains a list and a hash table.
type lrushard[K comparable, V any] struct {
	mu sync.Mutex

	// the hash table, with 20% extra spacer than the list for fewer conflicts.
	tableBuckets []uint64 // []lrubucket
	tableMask    uint32
	tableLength  uint32
	tableHasher  func(key unsafe.Pointer, seed uintptr) uintptr
	tableSeed    uintptr

	// the list of nodes
	list []lrunode[K, V]

	// stats
	statsGetCalls uint64
	statsSetCalls uint64
	statsMisses   uint64

	// padding
	_ [24]byte
}

func (s *lrushard[K, V]) Init(size uint32, hasher func(key unsafe.Pointer, seed uintptr) uintptr, seed uintptr) {
	s.listInit(size)
	s.tableInit(size, hasher, seed)
}

func (s *lrushard[K, V]) Get(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	s.statsGetCalls++

	if index, exists := s.tableGet(hash, key); exists {
		s.listMoveToFront(index)
		// value = s.list[index].value
		value = (*lrunode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0]))).value
		ok = true
	} else {
		s.statsMisses++
	}

	s.mu.Unlock()

	return
}

func (s *lrushard[K, V]) Peek(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if index, exists := s.tableGet(hash, key); exists {
		value = s.list[index].value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *lrushard[K, V]) SetIfAbsent(hash uint32, key K, value V) (prev V, replaced bool) {
	s.mu.Lock()

	if index, exists := s.tableGet(hash, key); exists {
		prev = s.list[index].value
		s.mu.Unlock()
		return
	}

	s.statsSetCalls++

	// index := s.list_Back()
	// node := &s.list[index]
	index := s.list[0].prev
	node := (*lrunode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	s.tableDelete(uint32(s.tableHasher(noescape(unsafe.Pointer(&node.key)), s.tableSeed)), node.key)

	node.key = key
	node.value = value
	s.tableSet(hash, key, index)
	s.listMoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *lrushard[K, V]) Set(hash uint32, key K, value V) (prev V, replaced bool) {
	s.mu.Lock()

	s.statsSetCalls++

	if index, exists := s.tableGet(hash, key); exists {
		// node := &s.list[index]
		node := (*lrunode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
		previousValue := node.value
		s.listMoveToFront(index)
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

	// delete the old key if the list is full, note that the list length is size+1
	if uint32(len(s.list)-1) < s.tableLength+1 && key != node.key {
		s.tableDelete(uint32(s.tableHasher(noescape(unsafe.Pointer(&node.key)), s.tableSeed)), node.key)
	}

	node.key = key
	node.value = value
	s.tableSet(hash, key, index)
	s.listMoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *lrushard[K, V]) Delete(hash uint32, key K) (v V) {
	s.mu.Lock()

	if index, exists := s.tableGet(hash, key); exists {
		node := &s.list[index]
		value := node.value
		s.listMoveToBack(index)
		node.value = v
		s.tableDelete(hash, key)
		v = value
	}

	s.mu.Unlock()

	return
}

func (s *lrushard[K, V]) Len() (n uint32) {
	s.mu.Lock()
	// inlining s.table_Len()
	n = s.tableLength
	s.mu.Unlock()

	return
}

func (s *lrushard[K, V]) AppendKeys(dst []K) []K {
	s.mu.Lock()
	for _, bucket := range s.tableBuckets {
		b := (*lrubucket)(unsafe.Pointer(&bucket))
		if b.index == 0 {
			continue
		}
		dst = append(dst, s.list[b.index].key)
	}
	s.mu.Unlock()

	return dst
}
