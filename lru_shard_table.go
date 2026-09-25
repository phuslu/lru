// Copyright 2023-2024 Phus Lu. All rights reserved.
// Copyright 2019 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an ISC-style
// license that can be found in the LICENSE file.

package lru

import (
	"math/bits"
	"unsafe"
)

const (
	// shardBits and shardCount describe the cache-level shard geometry.
	// LRUCache/TTLCache embed shards [shardCount]..., and sharded table
	// probing rotates the hash left by -shardBits so that table indexes stay
	// independent of the low hash bits that select the shard.
	shardBits  = 9
	shardCount = 1 << shardBits

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

// tableInsert assigns an index to a key that the caller has established is absent.
func (s *lrushard[K, V]) tableInsert(hash uint32, index uint32) {
	subhash := hash >> dibBitSize
	hdib := subhash<<dibBitSize | uint32(1)&maxDIB
	mask := s.tableMask
	// Skip the shard bits without discarding bits needed by large tables.
	i := bits.RotateLeft32(hash, -shardBits) & mask
	b0 := unsafe.Pointer(unsafe.SliceData(s.tableBuckets))
	for {
		b := (*lrubucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.hdib&maxDIB == 0 {
			b.hdib = hdib
			b.index = index
			s.tableLength++
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
	// The bucket of key at probe distance dib holds exactly subhash|dib, so one
	// comparison checks both. hdib is 64-bit so that it never wraps around to
	// match an empty bucket; kept small enough to be inlined into callers.
	hdib := uint64(hash>>dibBitSize<<dibBitSize | 1)
	i := bits.RotateLeft32(hash, -shardBits)
	for {
		b := (*lrubucket)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(s.tableBuckets)), uintptr(i&s.tableMask)*8))
		if uint64(b.hdib) == hdib && (*lrunode[K, V])(unsafe.Add(unsafe.Pointer(unsafe.SliceData(s.list)), uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key == key {
			return b.index, true
		}
		if b.hdib&maxDIB < uint32(hdib)&maxDIB {
			return
		}
		i++
		hdib++
	}
}

// tableDelete deletes an index for a key.
// Returns the deleted index, or false when no index was assigned.
func (s *lrushard[K, V]) tableDelete(hash uint32, key K) (index uint32, ok bool) {
	hdib := uint64(hash>>dibBitSize<<dibBitSize | 1)
	mask := s.tableMask
	i := bits.RotateLeft32(hash, -shardBits) & mask
	b0 := unsafe.Pointer(unsafe.SliceData(s.tableBuckets))
	l0 := unsafe.Pointer(unsafe.SliceData(s.list))
	for {
		b := (*lrubucket)(unsafe.Add(b0, uintptr(i)*8))
		if uint64(b.hdib) == hdib && (*lrunode[K, V])(unsafe.Add(l0, uintptr(b.index)*unsafe.Sizeof(s.list[0]))).key == key {
			old := b.index
			s.tableDeleteByIndex(i)
			return old, true
		}
		if b.hdib&maxDIB < uint32(hdib)&maxDIB {
			return
		}
		i = (i + 1) & mask
		hdib++
	}
}

// tableDeleteIndex deletes the bucket assigned to a node index whose key hashes to hash.
// Node indexes are unique in the table, so no key comparison is needed.
func (s *lrushard[K, V]) tableDeleteIndex(hash uint32, index uint32) {
	mask := s.tableMask
	i := bits.RotateLeft32(hash, -shardBits) & mask
	b0 := unsafe.Pointer(unsafe.SliceData(s.tableBuckets))
	for {
		b := (*lrubucket)(unsafe.Add(b0, uintptr(i)*8))
		if b.index == index {
			s.tableDeleteByIndex(i)
			return
		}
		if b.hdib&maxDIB == 0 {
			return
		}
		i = (i + 1) & mask
	}
}

func (s *lrushard[K, V]) tableDeleteByIndex(i uint32) {
	mask := s.tableMask
	b0 := unsafe.Pointer(unsafe.SliceData(s.tableBuckets))
	for {
		pi := i
		i = (i + 1) & mask
		bpi := (*lrubucket)(unsafe.Add(b0, uintptr(pi)*8))
		bi := (*lrubucket)(unsafe.Add(b0, uintptr(i)*8))
		if bi.hdib&maxDIB <= 1 {
			bpi.index = 0
			bpi.hdib = 0
			break
		}
		bpi.index = bi.index
		bpi.hdib = bi.hdib - 1
	}
	s.tableLength--
}
