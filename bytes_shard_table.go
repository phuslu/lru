// Copyright 2023-2024 Phus Lu. All rights reserved.
// Copyright 2019 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an ISC-style
// license that can be found in the LICENSE file.

package lru

import (
	"unsafe"
)

func (s *bytesshard) table_Init(size uint32, hasher func(key unsafe.Pointer, seed uintptr) uintptr, seed uintptr) {
	newsize := bytesNewTableSize(size)
	if len(s.table_buckets) == 0 {
		s.table_buckets = make([]uint64, newsize)
	}
	s.table_mask = newsize - 1
	s.table_length = 0
	s.table_hasher = hasher
	s.table_seed = seed
}

func bytesNewTableSize(size uint32) (newsize uint32) {
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
func (s *bytesshard) table_Set(hash uint32, key []byte, index uint32) (prev uint32, ok bool) {
	subhash := hash >> dibBitSize
	hdib := subhash<<dibBitSize | uint32(1)&maxDIB
	mask := s.table_mask
	i := (hdib >> dibBitSize) & mask
	b0 := unsafe.Pointer(&s.table_buckets[0])
	l0 := unsafe.Pointer(&s.list[0])
	for {
		b := (*bytesbucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.hdib&maxDIB == 0 {
			b.hdib = hdib
			b.index = index
			s.table_length++
			return
		}
		if hdib>>dibBitSize == b.hdib>>dibBitSize && b2s((*bytesnode)(unsafe.Add(l0, uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key) == b2s(key) {
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
func (s *bytesshard) table_Get(hash uint32, key []byte) (index uint32, ok bool) {
	subhash := hash >> dibBitSize
	mask := s.table_mask
	i := subhash & mask
	b0 := unsafe.Pointer(&s.table_buckets[0])
	l0 := unsafe.Pointer(&s.list[0])
	for {
		b := (*bytesbucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.hdib&maxDIB == 0 {
			return
		}
		if b.hdib>>dibBitSize == subhash && b2s((*bytesnode)(unsafe.Add(l0, uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key) == b2s(key) {
			return b.index, true
		}
		i = (i + 1) & mask
	}
}

// table_Delete deletes an index for a key.
// Returns the deleted index, or false when no index was assigned.
func (s *bytesshard) table_Delete(hash uint32, key []byte) (index uint32, ok bool) {
	subhash := hash >> dibBitSize
	mask := s.table_mask
	i := subhash & mask
	b0 := unsafe.Pointer(&s.table_buckets[0])
	l0 := unsafe.Pointer(&s.list[0])
	for {
		b := (*bytesbucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.hdib&maxDIB == 0 {
			return
		}
		if b.hdib>>dibBitSize == subhash && b2s((*bytesnode)(unsafe.Add(l0, uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key) == b2s(key) {
			old := b.index
			s.table_delete(i)
			return old, true
		}
		i = (i + 1) & mask
	}
}

func (s *bytesshard) table_delete(i uint32) {
	mask := s.table_mask
	b0 := unsafe.Pointer(&s.table_buckets[0])
	bi := (*bytesbucket)(unsafe.Add(b0, uintptr(i)*8))
	bi.hdib = bi.hdib>>dibBitSize<<dibBitSize | uint32(0)&maxDIB
	for {
		pi := i
		i = (i + 1) & mask
		bpi := (*bytesbucket)(unsafe.Add(b0, uintptr(pi)*8))
		bi = (*bytesbucket)(unsafe.Add(b0, uintptr(i)*8))
		if bi.hdib&maxDIB <= 1 {
			bpi.index = 0
			bpi.hdib = 0
			break
		}
		bpi.index = bi.index
		bpi.hdib = bi.hdib>>dibBitSize<<dibBitSize | (bi.hdib&maxDIB-1)&maxDIB
	}
	s.table_length--
}
