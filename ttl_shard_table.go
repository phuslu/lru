// Copyright 2023 Phus Lu. All rights reserved.
// Copyright 2019 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an ISC-style
// license that can be found in the LICENSE file.

package lru

import (
	"unsafe"
)

func (s *ttlshard[K, V]) table_Init(size uint32, hasher func(K unsafe.Pointer, seed uintptr) uintptr, seed uintptr) {
	newsize := ttlNewTableSize(size)
	if len(s.table.buckets) == 0 {
		s.table.buckets = make([]ttlbucket, newsize)
	}
	s.table.mask = newsize - 1
	s.table.length = 0
	s.table.hasher = hasher
	s.table.seed = seed
}

func ttlNewTableSize(size uint32) (newsize uint32) {
	newsize = nextPowOf2(size)
	if float64(newsize)*loadFactor < float64(size) {
		newsize = nextPowOf2(newsize + 1)
	}
	if newsize < 8 {
		newsize = 8
	}
	return
}

// table_Set assigns an index to a key.
// Returns the previous index, or false when no index was assigned.
func (s *ttlshard[K, V]) table_Set(hash uint32, key K, index uint32) (prev uint32, ok bool) {
	subhash := hash >> dibBitSize
	hdib := subhash<<dibBitSize | uint32(1)&maxDIB
	mask := s.table.mask
	i := (hdib >> dibBitSize) & mask
	b0 := unsafe.Pointer(&s.table.buckets[0])
	l0 := unsafe.Pointer(&s.list[0])
	for {
		b := (*ttlbucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.hdib&maxDIB == 0 {
			b.hdib = hdib
			b.index = index
			s.table.length++
			return
		}
		if hdib>>dibBitSize == b.hdib>>dibBitSize && (*ttlnode[K, V])(unsafe.Add(l0, uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key == key {
			prev = b.index
			b.hdib = hdib
			b.index = index
			ok = true
			return
		}
		if b.hdib&maxDIB < hdib&maxDIB {
			hdib, b.hdib = b.hdib, hdib
			index, b.index = b.index, index
		}
		i = (i + 1) & mask
		hdib = hdib>>dibBitSize<<dibBitSize | (hdib&maxDIB+1)&maxDIB
	}
}

// table_Get returns an index for a key.
// Returns false when no index has been assign for key.
func (s *ttlshard[K, V]) table_Get(hash uint32, key K) (index uint32, ok bool) {
	subhash := hash >> dibBitSize
	mask := s.table.mask
	i := subhash & mask
	b0 := unsafe.Pointer(&s.table.buckets[0])
	l0 := unsafe.Pointer(&s.list[0])
	for {
		b := (*ttlbucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.hdib&maxDIB == 0 {
			return
		}
		if b.hdib>>dibBitSize == subhash && (*ttlnode[K, V])(unsafe.Add(l0, uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key == key {
			return b.index, true
		}
		i = (i + 1) & mask
	}
}

// table_Delete deletes an index for a key.
// Returns the deleted index, or false when no index was assigned.
func (s *ttlshard[K, V]) table_Delete(hash uint32, key K) (v uint32, ok bool) {
	subhash := hash >> dibBitSize
	mask := s.table.mask
	i := subhash & mask
	b0 := unsafe.Pointer(&s.table.buckets[0])
	l0 := unsafe.Pointer(&s.list[0])
	for {
		b := (*ttlbucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.hdib&maxDIB == 0 {
			return
		}
		if b.hdib>>dibBitSize == subhash && (*ttlnode[K, V])(unsafe.Add(l0, uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key == key {
			old := b.index
			s.table_delete(i)
			return old, true
		}
		i = (i + 1) & mask
	}
}

func (s *ttlshard[K, V]) table_delete(i uint32) {
	mask := s.table.mask
	b0 := unsafe.Pointer(&s.table.buckets[0])
	bi := (*ttlbucket)(unsafe.Add(b0, uintptr(i)*8))
	bi.hdib = bi.hdib>>dibBitSize<<dibBitSize | uint32(0)&maxDIB
	for {
		pi := i
		i = (i + 1) & mask
		bpi := (*ttlbucket)(unsafe.Add(b0, uintptr(pi)*8))
		bi = (*ttlbucket)(unsafe.Add(b0, uintptr(i)*8))
		if bi.hdib&maxDIB <= 1 {
			bpi.index = 0
			bpi.hdib = 0
			break
		}
		bpi.index = bi.index
		bpi.hdib = bi.hdib>>dibBitSize<<dibBitSize | (bi.hdib&maxDIB-1)&maxDIB
	}
	s.table.length--
}
