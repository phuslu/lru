//go:build linux && amd64
// +build linux,amd64

// Copyright 2023-2024 Phus Lu. All rights reserved.

package lru

import (
	"sync"
	"unsafe"
)

// mmapnode is a list of bytes node, storing key-value pairs and related information
type mmapnode struct {
	key   []byte
	next  uint32
	prev  uint32
	value []byte
}

type mmapbucket struct {
	hdib  uint32 // bitfield { hash:24 dib:8 }
	index uint32 // node index
}

// mmapshard is a LRU partition contains a list and a hash table.
type mmapshard struct {
	mu sync.Mutex

	// the hash table, with 20% extra space than the list for fewer conflicts.
	table_buckets []uint64 // []mmapbucket
	table_mask    uint32
	table_length  uint32
	table_hasher  func(key unsafe.Pointer, seed uintptr) uintptr
	table_seed    uintptr

	// the list of nodes
	list []mmapnode

	// stats
	stats_getcalls uint64
	stats_setcalls uint64
	stats_misses   uint64

	// padding
	_ [24]byte
}

func (s *mmapshard) Init(size uint32, hasher func(key unsafe.Pointer, seed uintptr) uintptr, seed uintptr) {
	s.list_Init(size)
	s.table_Init(size, hasher, seed)
}

func (s *mmapshard) Get(hash uint32, key []byte) (value []byte, ok bool) {
	s.mu.Lock()

	s.stats_getcalls++

	if index, exists := s.table_Get(hash, key); exists {
		s.list_MoveToFront(index)
		// value = s.list[index].value
		value = (*mmapnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0]))).value
		ok = true
	} else {
		s.stats_misses++
	}

	s.mu.Unlock()

	return
}

func (s *mmapshard) Peek(hash uint32, key []byte) (value []byte, ok bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		value = s.list[index].value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *mmapshard) SetIfAbsent(hash uint32, key []byte, value []byte) (prev []byte, replaced bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		prev = s.list[index].value
		s.mu.Unlock()
		return
	}

	s.stats_setcalls++

	// index := s.list_Back()
	// node := &s.list[index]
	index := s.list[0].prev
	node := (*mmapnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	s.table_Delete(uint32(s.table_hasher(noescape(unsafe.Pointer(&node.key)), s.table_seed)), node.key)

	node.key = key
	node.value = value
	s.table_Set(hash, key, index)
	s.list_MoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *mmapshard) Set(hash uint32, key []byte, value []byte) (prev []byte, replaced bool) {
	s.mu.Lock()

	s.stats_setcalls++

	if index, exists := s.table_Get(hash, key); exists {
		// node := &s.list[index]
		node := (*mmapnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
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
	node := (*mmapnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	s.table_Delete(uint32(s.table_hasher(noescape(unsafe.Pointer(&node.key)), s.table_seed)), node.key)

	node.key = key
	node.value = value
	s.table_Set(hash, key, index)
	s.list_MoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *mmapshard) Delete(hash uint32, key []byte) (v []byte) {
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

func (s *mmapshard) Len() (n uint32) {
	s.mu.Lock()
	// inlining s.table_Len()
	n = s.table_length
	s.mu.Unlock()

	return
}

func (s *mmapshard) AppendKeys(dst [][]byte) [][]byte {
	s.mu.Lock()
	for _, bucket := range s.table_buckets {
		b := (*mmapbucket)(unsafe.Pointer(&bucket))
		if b.index == 0 {
			continue
		}
		dst = append(dst, s.list[b.index].key)
	}
	s.mu.Unlock()

	return dst
}
