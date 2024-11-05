// Copyright 2023-2024 Phus Lu. All rights reserved.

package lru

import (
	"unsafe"
)

func (s *ttlshard[K, V]) listInit(size uint32) {
	size += 1
	if len(s.list) == 0 {
		s.list = make([]ttlnode[K, V], size)
	}
	for i := uint32(0); i < size; i++ {
		s.list[i].next = (i + 1) % size
		s.list[i].prev = (i + size - 1) % size
	}
}

func (s *ttlshard[K, V]) listBack() uint32 {
	return s.list[0].prev
}

func (s *ttlshard[K, V]) listMoveToFront(i uint32) {
	root := &s.list[0]
	if root.next == i {
		return
	}

	base := unsafe.Pointer(root)
	nodei := (*ttlnode[K, V])(unsafe.Add(base, uintptr(i)*unsafe.Sizeof(s.list[0])))

	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.prev)*unsafe.Sizeof(s.list[0])))).next = nodei.next
	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = nodei.prev

	nodei.prev = 0
	nodei.next = root.next

	root.next = i
	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = i
}

func (s *ttlshard[K, V]) listMoveToBack(i uint32) {
	j := s.list[0].prev
	if i == j {
		return
	}

	base := unsafe.Pointer(&s.list[0])
	nodei := (*ttlnode[K, V])(unsafe.Add(base, uintptr(i)*unsafe.Sizeof(s.list[0])))
	at := (*ttlnode[K, V])(unsafe.Add(base, uintptr(j)*unsafe.Sizeof(s.list[0])))

	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.prev)*unsafe.Sizeof(s.list[0])))).next = nodei.next
	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = nodei.prev

	nodei.prev = j
	nodei.next = at.next

	((*ttlnode[K, V])(unsafe.Add(base, uintptr(j)*unsafe.Sizeof(s.list[0])))).next = i
	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = i
}
