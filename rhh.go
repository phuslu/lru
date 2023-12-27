// Copyright 2019 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an ISC-style
// license that can be found in the LICENSE file.

package lru

const (
	loadFactor  = 0.85                      // must be above 50%
	dibBitSize  = 8                         // 0xFF
	hashBitSize = 32 - dibBitSize           // 0xFFFFFF
	maxHash     = ^uint32(0) >> dibBitSize  // max 28,147,497,671,0655
	maxDIB      = ^uint32(0) >> hashBitSize // max 65,535
)

// rhhmap is a robin hood hashing map, see https://github.com/tidwall/hashmap
type rhhmap[K comparable] struct {
	hdib     []uint32 // bitfield { hash:24 dib:8 }
	buckets  []uint32
	getkey   func(i uint32) K
	cap      int
	length   int
	mask     uint32
	growAt   int
	shrinkAt int
}

func (m *rhhmap[K]) init(cap int, getkey func(i uint32) K) {
	m.cap = cap
	m.length = 0
	sz := 8
	for sz < m.cap {
		sz *= 2
	}
	if m.cap > 0 {
		m.cap = sz
	}
	m.getkey = getkey
	m.hdib = make([]uint32, sz)
	m.buckets = make([]uint32, sz)
	m.mask = uint32(len(m.buckets) - 1)
	m.growAt = int(float64(len(m.buckets)) * loadFactor)
	m.shrinkAt = int(float64(len(m.buckets)) * (1 - loadFactor))
}

func (m *rhhmap[K]) resize(newCap int) {
	var nmap rhhmap[K]
	nmap.init(newCap, m.getkey)
	for i := 0; i < len(m.buckets); i++ {
		if int(m.hdib[i]&maxDIB) > 0 {
			nmap.set(m.hdib[i]>>dibBitSize, m.getkey(m.buckets[i]), m.buckets[i])
		}
	}
	cap := m.cap
	*m = nmap
	m.cap = cap
}

// Set assigns a value to a key.
// Returns the previous value, or false when no value was assigned.
func (m *rhhmap[K]) Set(hash uint32, key K, value uint32) (uint32, bool) {
	if m.length >= m.growAt {
		m.resize(len(m.buckets) * 2)
	}
	return m.set(hash>>dibBitSize, key, value)
}

func (m *rhhmap[K]) set(hash uint32, key K, value uint32) (prev uint32, ok bool) {
	hdib := hash<<dibBitSize | uint32(1)&maxDIB
	e := value
	i := (hdib >> dibBitSize) & m.mask
	for {
		if m.hdib[i]&maxDIB == 0 {
			m.hdib[i] = hdib
			m.buckets[i] = e
			m.length++
			return
		}
		if hdib>>dibBitSize == m.hdib[i]>>dibBitSize && m.getkey(e) == m.getkey(m.buckets[i]) {
			old := m.buckets[i]
			m.hdib[i] = hdib
			m.buckets[i] = e
			return old, true
		}
		if m.hdib[i]&maxDIB < hdib&maxDIB {
			hdib, m.hdib[i] = m.hdib[i], hdib
			e, m.buckets[i] = m.buckets[i], e
		}
		i = (i + 1) & m.mask
		hdib = hdib>>dibBitSize<<dibBitSize | (hdib&maxDIB+1)&maxDIB
	}
}

// Get returns a value for a key.
// Returns false when no value has been assign for key.
func (m *rhhmap[K]) Get(hash uint32, key K) (prev uint32, ok bool) {
	if len(m.buckets) == 0 {
		return
	}
	subhash := hash >> dibBitSize
	i := subhash & m.mask
	for {
		if m.hdib[i]&maxDIB == 0 {
			return
		}
		if m.hdib[i]>>dibBitSize == subhash && m.getkey(m.buckets[i]) == key {
			return m.buckets[i], true
		}
		i = (i + 1) & m.mask
	}
}

// Len returns the number of values in map.
func (m *rhhmap[K]) Len() int {
	return m.length
}

// Delete deletes a value for a key.
// Returns the deleted value, or false when no value was assigned.
func (m *rhhmap[K]) Delete(hash uint32, key K) (v uint32, ok bool) {
	if len(m.buckets) == 0 {
		return
	}
	subhash := hash >> dibBitSize
	i := subhash & m.mask
	for {
		if m.hdib[i]&maxDIB == 0 {
			return
		}
		if m.hdib[i]>>dibBitSize == subhash && m.getkey(m.buckets[i]) == key {
			old := m.buckets[i]
			m.delete(i)
			return old, true
		}
		i = (i + 1) & m.mask
	}
}

func (m *rhhmap[K]) delete(i uint32) {
	m.hdib[i] = m.hdib[i]>>dibBitSize<<dibBitSize | uint32(0)&maxDIB
	for {
		pi := i
		i = (i + 1) & m.mask
		if m.hdib[i]&maxDIB <= 1 {
			m.buckets[pi] = 0
			m.hdib[pi] = 0
			break
		}
		m.buckets[pi] = m.buckets[i]
		m.hdib[pi] = m.hdib[i]>>dibBitSize<<dibBitSize | (m.hdib[i]&maxDIB-1)&maxDIB
	}
	m.length--
	if len(m.buckets) > m.cap && m.length <= m.shrinkAt {
		m.resize(m.length)
	}
}
