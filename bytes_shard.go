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

// bytesshard is an LRU partition contains a list and a hash table.
type bytesshard struct {
	mu sync.Mutex

	// the hash table, with 20% extra spacer than the list for fewer conflicts.
	tableBuckets []uint64 // []bytesbucket
	tableMask    uint32
	tableLength  uint32

	// the list of nodes
	list []bytesnode

	// stats
	statsGetCalls uint64
	statsSetCalls uint64
	statsMisses   uint64

	// padding
	_ [40]byte
}

func (s *bytesshard) Init(size uint32) {
	s.listInit(size)
	s.tableInit(size)
}

func (s *bytesshard) Get(hash uint32, key []byte) (value []byte, ok bool) {
	s.mu.Lock()

	s.statsGetCalls++

	if index, exists := s.tableGet(hash, key); exists {
		s.listMoveToFront(index)
		// value = s.list[index].value
		value = (*bytesnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0]))).value
		ok = true
	} else {
		s.statsMisses++
	}

	s.mu.Unlock()

	return
}

func (s *bytesshard) Peek(hash uint32, key []byte) (value []byte, ok bool) {
	s.mu.Lock()

	if index, exists := s.tableGet(hash, key); exists {
		value = s.list[index].value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *bytesshard) SetIfAbsent(hash uint32, key []byte, value []byte) (prev []byte, replaced bool) {
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
	node := (*bytesnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	s.tableDelete(uint32(wyhashHashbytes(node.key, 0)), node.key)

	node.key = key
	node.value = value
	s.tableSet(hash, key, index)
	s.listMoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *bytesshard) Set(hash uint32, key []byte, value []byte) (prev []byte, replaced bool) {
	s.mu.Lock()

	s.statsSetCalls++

	if index, exists := s.tableGet(hash, key); exists {
		// node := &s.list[index]
		node := (*bytesnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
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
	node := (*bytesnode)(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	s.tableDelete(uint32(wyhashHashbytes(node.key, 0)), node.key)

	node.key = key
	node.value = value
	s.tableSet(hash, key, index)
	s.listMoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *bytesshard) Delete(hash uint32, key []byte) (v []byte) {
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

func (s *bytesshard) Len() (n uint32) {
	s.mu.Lock()
	// inlining s.table_Len()
	n = s.tableLength
	s.mu.Unlock()

	return
}

func (s *bytesshard) AppendKeys(dst [][]byte) [][]byte {
	s.mu.Lock()
	for _, bucket := range s.tableBuckets {
		b := (*bytesbucket)(unsafe.Pointer(&bucket))
		if b.index == 0 {
			continue
		}
		dst = append(dst, s.list[b.index].key)
	}
	s.mu.Unlock()

	return dst
}
