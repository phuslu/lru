// Copyright 2023-2024 Phus Lu. All rights reserved.
// Copyright 2019 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an ISC-style
// license that can be found in the LICENSE file.

package lru

import (
	"unsafe"
)

const (
	loadFactor  = 0.85                      // must be above 50%
	dibBitSize  = 8                         // 0xFF
	hashBitSize = 32 - dibBitSize           // 0xFFFFFF
	maxHash     = ^uint32(0) >> dibBitSize  // max 16777215
	maxDIB      = ^uint32(0) >> hashBitSize // max 255
)

func (s *lrushard[K, V]) tableInit(size uint32, hasher func(key unsafe.Pointer, seed uintptr) uintptr, seed uintptr) {
	newsize := lruNewTableSize(size)
	if len(s.tableBuckets) == 0 {
		s.tableBuckets = make([]uint64, newsize)
	}
	s.tableMask = newsize - 1
	s.tableLength = 0
	s.tableHasher = hasher
	s.tableSeed = seed
}

func lruNewTableSize(size uint32) (newsize uint32) {
	newsize = nextPowOf2(size)
	if float64(newsize)*loadFactor < float64(size) {
		newsize = nextPowOf2(newsize + 1)
	}
	if newsize < 8 {
		newsize = 8
	}
	return
}

// Set assigns an index to a key.
// Returns the previous index, or false when no index was assigned.
func (s *lrushard[K, V]) tableSet(hash uint32, key K, index uint32) (prev uint32, ok bool) {
	subhash := hash >> dibBitSize
	hdib := subhash<<dibBitSize | uint32(1)&maxDIB
	mask := s.tableMask
	i := (hdib >> dibBitSize) & mask
	b0 := unsafe.Pointer(&s.tableBuckets[0])
	l0 := unsafe.Pointer(&s.list[0])
	for {
		b := (*lrubucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.hdib&maxDIB == 0 {
			b.hdib = hdib
			b.index = index
			s.tableLength++
			return
		}
		if hdib>>dibBitSize == b.hdib>>dibBitSize && (*lrunode[K, V])(unsafe.Add(l0, uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key == key {
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

// tableGet returns an index for a key.
// Returns false when no index has been assign for key.
func (s *lrushard[K, V]) tableGet(hash uint32, key K) (index uint32, ok bool) {
	subhash := hash >> dibBitSize
	mask := s.tableMask
	i := subhash & mask
	b0 := unsafe.Pointer(&s.tableBuckets[0])
	l0 := unsafe.Pointer(&s.list[0])
	for {
		b := (*lrubucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.hdib&maxDIB == 0 {
			return
		}
		if b.hdib>>dibBitSize == subhash && (*lrunode[K, V])(unsafe.Add(l0, uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key == key {
			return b.index, true
		}
		i = (i + 1) & mask
	}
}

// tableDelete deletes an index for a key.
// Returns the deleted index, or false when no index was assigned.
func (s *lrushard[K, V]) tableDelete(hash uint32, key K) (index uint32, ok bool) {
	subhash := hash >> dibBitSize
	mask := s.tableMask
	i := subhash & mask
	b0 := unsafe.Pointer(&s.tableBuckets[0])
	l0 := unsafe.Pointer(&s.list[0])
	for {
		b := (*lrubucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.hdib&maxDIB == 0 {
			return
		}
		if b.hdib>>dibBitSize == subhash && (*lrunode[K, V])(unsafe.Add(l0, uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key == key {
			old := b.index
			s.tableDeleteByIndex(i)
			return old, true
		}
		i = (i + 1) & mask
	}
}

func (s *lrushard[K, V]) tableDeleteByIndex(i uint32) {
	mask := s.tableMask
	b0 := unsafe.Pointer(&s.tableBuckets[0])
	bi := (*lrubucket)(unsafe.Add(b0, uintptr(i)*8))
	bi.hdib = bi.hdib>>dibBitSize<<dibBitSize | uint32(0)&maxDIB
	for {
		pi := i
		i = (i + 1) & mask
		bpi := (*lrubucket)(unsafe.Add(b0, uintptr(pi)*8))
		bi = (*lrubucket)(unsafe.Add(b0, uintptr(i)*8))
		if bi.hdib&maxDIB <= 1 {
			bpi.index = 0
			bpi.hdib = 0
			break
		}
		bpi.index = bi.index
		bpi.hdib = bi.hdib>>dibBitSize<<dibBitSize | (bi.hdib&maxDIB-1)&maxDIB
	}
	s.tableLength--
}
