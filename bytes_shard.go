// Copyright 2023-2024 Phus Lu. All rights reserved.

package lru

import (
	"sync"
	"unsafe"
)

// bytesnode is a list of bytes node, storing key-value pairs and related information
type bytesnode struct {
	key   []byte
	next  uint32
	prev  uint32
	value []byte
}

type bytesbucket struct {
	hdib  uint32 // bitfield { hash:24 dib:8 }
	index uint32 // node index
}

// bytesshard is a LRU partition contains a list and a hash table.
type bytesshard struct {
	mu sync.Mutex

	// the hash table, with 20% extra space than the list for fewer conflicts.
	table_buckets []uint64 // []bytesbucket
	table_mask    uint32
	table_length  uint32

	// the list of nodes
	list []bytesnode

	// stats
	stats_getcalls uint64
	stats_setcalls uint64
	stats_misses   uint64

	// padding
	_ [40]byte
}

func (s *bytesshard) Init(size uint32) {
	s.list_Init(size)
	s.table_Init(size)
}

func (s *bytesshard) Get(hash uint32, key []byte) (value []byte, ok bool) {
	s.mu.Lock()

	s.stats_getcalls++

	if index, exists := s.table_Get(hash, key); exists {
		s.list_MoveToFront(index)
		// value = s.list[index].value
		value = (*bytesnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0]))).value
		ok = true
	} else {
		s.stats_misses++
	}

	s.mu.Unlock()

	return
}

func (s *bytesshard) Peek(hash uint32, key []byte) (value []byte, ok bool) {
	s.mu.Lock()

	if index, exists := s.table_Get(hash, key); exists {
		value = s.list[index].value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *bytesshard) SetIfAbsent(hash uint32, key []byte, value []byte) (prev []byte, replaced bool) {
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
	node := (*bytesnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	s.table_Delete(uint32(wyhash_HashString(b2s(node.key), 0)), node.key)

	node.key = key
	node.value = value
	s.table_Set(hash, key, index)
	s.list_MoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *bytesshard) Set(hash uint32, key []byte, value []byte) (prev []byte, replaced bool) {
	s.mu.Lock()

	s.stats_setcalls++

	if index, exists := s.table_Get(hash, key); exists {
		// node := &s.list[index]
		node := (*bytesnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
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
	node := (*bytesnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	s.table_Delete(uint32(wyhash_HashString(b2s(node.key), 0)), node.key)

	node.key = key
	node.value = value
	s.table_Set(hash, key, index)
	s.list_MoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *bytesshard) Delete(hash uint32, key []byte) (v []byte) {
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

func (s *bytesshard) Len() (n uint32) {
	s.mu.Lock()
	// inlining s.table_Len()
	n = s.table_length
	s.mu.Unlock()

	return
}

func (s *bytesshard) AppendKeys(dst [][]byte) [][]byte {
	s.mu.Lock()
	for _, bucket := range s.table_buckets {
		b := (*bytesbucket)(unsafe.Pointer(&bucket))
		if b.index == 0 {
			continue
		}
		dst = append(dst, s.list[b.index].key)
	}
	s.mu.Unlock()

	return dst
}
