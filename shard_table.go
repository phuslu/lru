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

func (s *shard[K, V]) tableInit(size int) {
	sz := 8
	for sz < size {
		sz *= 2
	}
	s.buckets = make([]struct{ hdib, index uint32 }, sz)
	s.mask = uint32(len(s.buckets) - 1)
	s.length = 0
}

// Set assigns a value to a key.
// Returns the previous value, or false when no value was assigned.
func (s *shard[K, V]) tableSet(hash uint32, key K, value uint32) (uint32, bool) {
	return s.tableset(hash>>dibBitSize, key, value)
}

func (s *shard[K, V]) tableset(hash uint32, key K, value uint32) (prev uint32, ok bool) {
	hdib := hash<<dibBitSize | uint32(1)&maxDIB
	i := (hdib >> dibBitSize) & s.mask
	for {
		if s.buckets[i].hdib&maxDIB == 0 {
			s.buckets[i].hdib = hdib
			s.buckets[i].index = value
			s.length++
			return
		}
		if hdib>>dibBitSize == s.buckets[i].hdib>>dibBitSize && key == s.list[s.buckets[i].index].key {
			old := s.buckets[i].index
			s.buckets[i].hdib = hdib
			s.buckets[i].index = value
			return old, true
		}
		if s.buckets[i].hdib&maxDIB < hdib&maxDIB {
			hdib, s.buckets[i].hdib = s.buckets[i].hdib, hdib
			value, s.buckets[i].index = s.buckets[i].index, value
		}
		i = (i + 1) & s.mask
		hdib = hdib>>dibBitSize<<dibBitSize | (hdib&maxDIB+1)&maxDIB
	}
}

// Get returns a value for a key.
// Returns false when no value has been assign for key.
func (s *shard[K, V]) tableGet(hash uint32, key K) (prev uint32, ok bool) {
	subhash := hash >> dibBitSize
	i := subhash & s.mask
	for {
		if s.buckets[i].hdib&maxDIB == 0 {
			return
		}
		if s.buckets[i].hdib>>dibBitSize == subhash && s.list[s.buckets[i].index].key == key {
			return s.buckets[i].index, true
		}
		i = (i + 1) & s.mask
	}
}

// Len returns the number of values in map.
func (s *shard[K, V]) tableLen() int {
	return s.length
}

// Delete deletes a value for a key.
// Returns the deleted value, or false when no value was assigned.
func (s *shard[K, V]) tableDelete(hash uint32, key K) (v uint32, ok bool) {
	subhash := hash >> dibBitSize
	i := subhash & s.mask
	for {
		if s.buckets[i].hdib&maxDIB == 0 {
			return
		}
		if s.buckets[i].hdib>>dibBitSize == subhash && s.list[s.buckets[i].index].key == key {
			old := s.buckets[i].index
			s.tabledelete(i)
			return old, true
		}
		i = (i + 1) & s.mask
	}
}

func (s *shard[K, V]) tabledelete(i uint32) {
	s.buckets[i].hdib = s.buckets[i].hdib>>dibBitSize<<dibBitSize | uint32(0)&maxDIB
	for {
		pi := i
		i = (i + 1) & s.mask
		if s.buckets[i].hdib&maxDIB <= 1 {
			s.buckets[pi].index = 0
			s.buckets[pi].hdib = 0
			break
		}
		s.buckets[pi].index = s.buckets[i].index
		s.buckets[pi].hdib = s.buckets[i].hdib>>dibBitSize<<dibBitSize | (s.buckets[i].hdib&maxDIB-1)&maxDIB
	}
	s.length--
}
