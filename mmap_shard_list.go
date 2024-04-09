// Copyright 2023-2024 Phus Lu. All rights reserved.

package lru

import (
	"unsafe"
)

func (s *mmapshard) list_Init(size uint32) {
	size += 1
	if len(s.list) == 0 {
		s.list = make([]mmapnode, size)
	}
	for i := uint32(0); i < size; i++ {
		s.list[i].next = (i + 1) % size
		s.list[i].prev = (i + size - 1) % size
	}
}

func (s *mmapshard) list_Back() uint32 {
	return s.list[0].prev
}

func (s *mmapshard) list_MoveToFront(i uint32) {
	root := &s.list[0]
	if root.next == i {
		return
	}

	base := unsafe.Pointer(root)
	nodei := (*mmapnode)(unsafe.Add(base, uintptr(i)*unsafe.Sizeof(s.list[0])))

	((*mmapnode)(unsafe.Add(base, uintptr(nodei.prev)*unsafe.Sizeof(s.list[0])))).next = nodei.next
	((*mmapnode)(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = nodei.prev

	nodei.prev = 0
	nodei.next = root.next

	root.next = i
	((*mmapnode)(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = i
}

func (s *mmapshard) list_MoveToBack(i uint32) {
	j := s.list[0].prev
	if i == j {
		return
	}

	base := unsafe.Pointer(&s.list[0])
	nodei := (*mmapnode)(unsafe.Add(base, uintptr(i)*unsafe.Sizeof(s.list[0])))
	at := (*mmapnode)(unsafe.Add(base, uintptr(j)*unsafe.Sizeof(s.list[0])))

	((*mmapnode)(unsafe.Add(base, uintptr(nodei.prev)*unsafe.Sizeof(s.list[0])))).next = nodei.next
	((*mmapnode)(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = nodei.prev

	nodei.prev = j
	nodei.next = at.next

	((*mmapnode)(unsafe.Add(base, uintptr(j)*unsafe.Sizeof(s.list[0])))).next = i
	((*mmapnode)(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = i
}
