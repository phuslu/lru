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

// rhh is a robin hood hashing, only stores key getter and key-value index to reduce GC efforts.
type rhh[K comparable] struct {
	buckets []struct {
		hdib  uint32 // bitfield { hash:24 dib:8 }
		index uint32 // key-value index
	}
	getkey   func(index uint32) K
	cap      int
	length   int
	mask     uint32
	growAt   int
	shrinkAt int
}

func (m *rhh[K]) init(cap int, getkey func(i uint32) K) {
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
	m.buckets = make([]struct{ hdib, index uint32 }, sz)
	m.mask = uint32(len(m.buckets) - 1)
	m.growAt = int(float64(len(m.buckets)) * loadFactor)
	m.shrinkAt = int(float64(len(m.buckets)) * (1 - loadFactor))
}

func (m *rhh[K]) resize(newCap int) {
	var nmap rhh[K]
	nmap.init(newCap, m.getkey)
	for i := 0; i < len(m.buckets); i++ {
		if int(m.buckets[i].hdib&maxDIB) > 0 {
			nmap.set(m.buckets[i].hdib>>dibBitSize, m.getkey(m.buckets[i].index), m.buckets[i].index)
		}
	}
	cap := m.cap
	*m = nmap
	m.cap = cap
}

// Set assigns a value to a key.
// Returns the previous value, or false when no value was assigned.
func (m *rhh[K]) Set(hash uint32, key K, value uint32) (uint32, bool) {
	if m.length >= m.growAt {
		m.resize(len(m.buckets) * 2)
	}
	return m.set(hash>>dibBitSize, key, value)
}

func (m *rhh[K]) set(hash uint32, key K, value uint32) (prev uint32, ok bool) {
	hdib := hash<<dibBitSize | uint32(1)&maxDIB
	e := value
	i := (hdib >> dibBitSize) & m.mask
	for {
		if m.buckets[i].hdib&maxDIB == 0 {
			m.buckets[i].hdib = hdib
			m.buckets[i].index = e
			m.length++
			return
		}
		if hdib>>dibBitSize == m.buckets[i].hdib>>dibBitSize && m.getkey(e) == m.getkey(m.buckets[i].index) {
			old := m.buckets[i].index
			m.buckets[i].hdib = hdib
			m.buckets[i].index = e
			return old, true
		}
		if m.buckets[i].hdib&maxDIB < hdib&maxDIB {
			hdib, m.buckets[i].hdib = m.buckets[i].hdib, hdib
			e, m.buckets[i].index = m.buckets[i].index, e
		}
		i = (i + 1) & m.mask
		hdib = hdib>>dibBitSize<<dibBitSize | (hdib&maxDIB+1)&maxDIB
	}
}

// Get returns a value for a key.
// Returns false when no value has been assign for key.
func (m *rhh[K]) Get(hash uint32, key K) (prev uint32, ok bool) {
	if len(m.buckets) == 0 {
		return
	}
	subhash := hash >> dibBitSize
	i := subhash & m.mask
	for {
		if m.buckets[i].hdib&maxDIB == 0 {
			return
		}
		if m.buckets[i].hdib>>dibBitSize == subhash && m.getkey(m.buckets[i].index) == key {
			return m.buckets[i].index, true
		}
		i = (i + 1) & m.mask
	}
}

// Len returns the number of values in map.
func (m *rhh[K]) Len() int {
	return m.length
}

// Delete deletes a value for a key.
// Returns the deleted value, or false when no value was assigned.
func (m *rhh[K]) Delete(hash uint32, key K) (v uint32, ok bool) {
	if len(m.buckets) == 0 {
		return
	}
	subhash := hash >> dibBitSize
	i := subhash & m.mask
	for {
		if m.buckets[i].hdib&maxDIB == 0 {
			return
		}
		if m.buckets[i].hdib>>dibBitSize == subhash && m.getkey(m.buckets[i].index) == key {
			old := m.buckets[i].index
			m.delete(i)
			return old, true
		}
		i = (i + 1) & m.mask
	}
}

func (m *rhh[K]) delete(i uint32) {
	m.buckets[i].hdib = m.buckets[i].hdib>>dibBitSize<<dibBitSize | uint32(0)&maxDIB
	for {
		pi := i
		i = (i + 1) & m.mask
		if m.buckets[i].hdib&maxDIB <= 1 {
			m.buckets[pi].index = 0
			m.buckets[pi].hdib = 0
			break
		}
		m.buckets[pi].index = m.buckets[i].index
		m.buckets[pi].hdib = m.buckets[i].hdib>>dibBitSize<<dibBitSize | (m.buckets[i].hdib&maxDIB-1)&maxDIB
	}
	m.length--
	if len(m.buckets) > m.cap && m.length <= m.shrinkAt {
		m.resize(m.length)
	}
}
