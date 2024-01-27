// Copyright 2023 Phus Lu. All rights reserved.
// Copyright 2019 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an ISC-style
// license that can be found in the LICENSE file.

package lru

const (
	loadFactor  = 0.85                      // must be above 50%
	dibBitSize  = 8                         // 0xFF
	hashBitSize = 32 - dibBitSize           // 0xFFFFFF
	maxHash     = ^uint32(0) >> dibBitSize  // max 16777215
	maxDIB      = ^uint32(0) >> hashBitSize // max 255
)

func (s *shard[K, V]) table_Init(size int) {
	sz := max(nextPowOf2(size), 8)
	s.table.buckets = make([]struct{ hdib, index uint32 }, sz)
	s.table.mask = uint32(len(s.table.buckets) - 1)
	s.table.length = 0
}

// Set assigns an index to a key.
// Returns the previous index, or false when no index was assigned.
func (s *shard[K, V]) table_Set(hash uint32, key K, index uint32) (prev uint32, ok bool) {
	subhash := hash >> dibBitSize
	hdib := subhash<<dibBitSize | uint32(1)&maxDIB
	i := (hdib >> dibBitSize) & s.table.mask
	for {
		if s.table.buckets[i].hdib&maxDIB == 0 {
			s.table.buckets[i].hdib = hdib
			s.table.buckets[i].index = index
			s.table.length++
			return
		}
		if hdib>>dibBitSize == s.table.buckets[i].hdib>>dibBitSize && key == s.list[s.table.buckets[i].index].key {
			prev = s.table.buckets[i].index
			s.table.buckets[i].hdib = hdib
			s.table.buckets[i].index = index
			ok = true
			return
		}
		if s.table.buckets[i].hdib&maxDIB < hdib&maxDIB {
			hdib, s.table.buckets[i].hdib = s.table.buckets[i].hdib, hdib
			index, s.table.buckets[i].index = s.table.buckets[i].index, index
		}
		i = (i + 1) & s.table.mask
		hdib = hdib>>dibBitSize<<dibBitSize | (hdib&maxDIB+1)&maxDIB
	}
}

// Get returns an index for a key.
// Returns false when no index has been assign for key.
func (s *shard[K, V]) table_Get(hash uint32, key K) (prev uint32, ok bool) {
	subhash := hash >> dibBitSize
	mask := s.table.mask
	i := subhash & mask
	for {
		if s.table.buckets[i].hdib&maxDIB == 0 {
			return
		}
		if s.table.buckets[i].hdib>>dibBitSize == subhash && s.list[s.table.buckets[i].index].key == key {
			return s.table.buckets[i].index, true
		}
		i = (i + 1) & mask
	}
}

// Len returns the number of indexs in map.
func (s *shard[K, V]) table_Len() int {
	return s.table.length
}

// Delete deletes an index for a key.
// Returns the deleted index, or false when no index was assigned.
func (s *shard[K, V]) table_Delete(hash uint32, key K) (v uint32, ok bool) {
	subhash := hash >> dibBitSize
	i := subhash & s.table.mask
	for {
		if s.table.buckets[i].hdib&maxDIB == 0 {
			return
		}
		if s.table.buckets[i].hdib>>dibBitSize == subhash && s.list[s.table.buckets[i].index].key == key {
			old := s.table.buckets[i].index
			s.table_delete(i)
			return old, true
		}
		i = (i + 1) & s.table.mask
	}
}

func (s *shard[K, V]) table_delete(i uint32) {
	s.table.buckets[i].hdib = s.table.buckets[i].hdib>>dibBitSize<<dibBitSize | uint32(0)&maxDIB
	for {
		pi := i
		i = (i + 1) & s.table.mask
		if s.table.buckets[i].hdib&maxDIB <= 1 {
			s.table.buckets[pi].index = 0
			s.table.buckets[pi].hdib = 0
			break
		}
		s.table.buckets[pi].index = s.table.buckets[i].index
		s.table.buckets[pi].hdib = s.table.buckets[i].hdib>>dibBitSize<<dibBitSize | (s.table.buckets[i].hdib&maxDIB-1)&maxDIB
	}
	s.table.length--
}
