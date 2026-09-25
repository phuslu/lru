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
	list := s.list[:size]
	for i := range list {
		list[i].next = uint32(i) + 1
		list[i].prev = uint32(i) - 1
	}
	list[0].prev = size - 1
	list[size-1].next = 0
}

func (s *ttlshard[K, V]) listBack() uint32 {
	return s.list[0].prev
}

func (s *ttlshard[K, V]) listMoveToFront(i uint32) {
	base := unsafe.Pointer(unsafe.SliceData(s.list))
	root := (*ttlnode[K, V])(base)
	if root.next == i {
		return
	}

	nodei := (*ttlnode[K, V])(unsafe.Add(base, uintptr(i)*unsafe.Sizeof(s.list[0])))

	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.prev)*unsafe.Sizeof(s.list[0])))).next = nodei.next
	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = nodei.prev

	nodei.prev = 0
	nodei.next = root.next

	root.next = i
	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = i
}

func (s *ttlshard[K, V]) listMoveToBack(i uint32) {
	base := unsafe.Pointer(unsafe.SliceData(s.list))
	j := ((*ttlnode[K, V])(base)).prev
	if i == j {
		return
	}

	nodei := (*ttlnode[K, V])(unsafe.Add(base, uintptr(i)*unsafe.Sizeof(s.list[0])))
	at := (*ttlnode[K, V])(unsafe.Add(base, uintptr(j)*unsafe.Sizeof(s.list[0])))

	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.prev)*unsafe.Sizeof(s.list[0])))).next = nodei.next
	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = nodei.prev

	nodei.prev = j
	nodei.next = at.next

	((*ttlnode[K, V])(unsafe.Add(base, uintptr(j)*unsafe.Sizeof(s.list[0])))).next = i
	((*ttlnode[K, V])(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = i
}
