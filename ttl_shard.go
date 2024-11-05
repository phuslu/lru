// Copyright 2023-2024 Phus Lu. All rights reserved.

package lru

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// ttlnode is a list of ttl node, storing key-value pairs and related information
type ttlnode[K comparable, V any] struct {
	key     K
	expires uint32
	next    uint32
	prev    uint32
	ttl     uint32
	value   V
	hit     bool
}

type ttlbucket struct {
	hdib  uint32 // bitfield { hash:24 dib:8 }
	index uint32 // node index
}

// ttlshard is an LRU partition contains a list and a hash table.
type ttlshard[K comparable, V any] struct {
	mu sync.Mutex

	// the hash table, with 20% extra spacer than the list for fewer conflicts.
	tableBuckets []uint64 // []ttlbucket
	tableMask    uint32
	tableLength  uint32
	tableHasher  func(key unsafe.Pointer, seed uintptr) uintptr
	tableSeed    uintptr

	// the list of nodes
	list []ttlnode[K, V]

	sliding bool

	// stats
	statsGetCalls uint64
	statsSetCalls uint64
	statsMisses   uint64

	// padding
	_ [16]byte
}

func (s *ttlshard[K, V]) Init(size uint32, hasher func(key unsafe.Pointer, seed uintptr) uintptr, seed uintptr) {
	s.listInit(size)
	s.tableInit(size, hasher, seed)
}

func (s *ttlshard[K, V]) Get(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	s.statsGetCalls++

	if index, exists := s.tableGet(hash, key); exists {
		if expires := s.list[index].expires; expires == 0 {
			s.listMoveToFront(index)
			// value = s.list[index].value
			value = (*ttlnode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0]))).value
			ok = true
		} else if now := atomic.LoadUint32(&clock); now < expires {
			if s.sliding {
				s.list[index].expires = now + s.list[index].ttl
			}
			s.listMoveToFront(index)
			// value = s.list[index].value
			value = (*ttlnode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0]))).value
			ok = true
		} else {
			s.listMoveToBack(index)
			// s.list[index].value = value
			(*ttlnode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0]))).value = value
			s.tableDelete(hash, key)
			s.statsMisses++
		}
	} else {
		s.statsMisses++
	}

	s.mu.Unlock()

	return
}

func (s *ttlshard[K, V]) Peek(hash uint32, key K) (value V, expires int64, ok bool) {
	s.mu.Lock()

	if index, exists := s.tableGet(hash, key); exists {
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

	if index, exists := s.tableGet(hash, key); exists {
		// node := &s.list[index]
		node := (*ttlnode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
		prev = node.value
		if node.expires == 0 || atomic.LoadUint32(&clock) < node.expires {
			s.mu.Unlock()
			return
		}

		s.statsSetCalls++

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

	s.statsSetCalls++

	// index := s.list_Back()
	// node := &s.list[index]
	index := s.list[0].prev
	node := (*ttlnode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	s.tableDelete(uint32(s.tableHasher(noescape(unsafe.Pointer(&node.key)), s.tableSeed)), node.key)

	node.key = key
	node.value = value
	if ttl > 0 {
		node.ttl = uint32(ttl / time.Second)
		node.expires = atomic.LoadUint32(&clock) + node.ttl
	}
	s.tableSet(hash, key, index)
	s.listMoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *ttlshard[K, V]) Set(hash uint32, key K, value V, ttl time.Duration) (prev V, replaced bool) {
	s.mu.Lock()

	s.statsSetCalls++

	if index, exists := s.tableGet(hash, key); exists {
		// node := &s.list[index]
		node := (*ttlnode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
		previousValue := node.value
		s.listMoveToFront(index)
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

	// index := s.list_Back()
	// node := &s.list[index]
	index := s.list[0].prev
	node := (*ttlnode[K, V])(unsafe.Add(unsafe.Pointer(&s.list[0]), uintptr(index)*unsafe.Sizeof(s.list[0])))
	evictedValue := node.value
	if key != node.key && node.hit {
		s.tableDelete(uint32(s.tableHasher(noescape(unsafe.Pointer(&node.key)), s.tableSeed)), node.key)
	}

	node.hit = true
	node.key = key
	node.value = value
	if ttl > 0 {
		node.ttl = uint32(ttl / time.Second)
		node.expires = atomic.LoadUint32(&clock) + node.ttl
	}
	s.tableSet(hash, key, index)
	s.listMoveToFront(index)
	prev = evictedValue

	s.mu.Unlock()
	return
}

func (s *ttlshard[K, V]) Delete(hash uint32, key K) (v V) {
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

func (s *ttlshard[K, V]) Len() (n uint32) {
	s.mu.Lock()
	// inlining s.table_Len()
	n = s.tableLength
	s.mu.Unlock()

	return
}

func (s *ttlshard[K, V]) AppendKeys(dst []K, now uint32) []K {
	s.mu.Lock()
	for _, bucket := range s.tableBuckets {
		b := (*ttlbucket)(unsafe.Pointer(&bucket))
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
