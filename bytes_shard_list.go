// Copyright 2023-2024 Phus Lu. All rights reserved.

package lru

import (
	"unsafe"
)

func (s *bytesshard) listInit(size uint32) {
	size += 1
	if len(s.list) == 0 {
		s.list = make([]bytesnode, size)
	}
	for i := uint32(0); i < size; i++ {
		s.list[i].next = (i + 1) % size
		s.list[i].prev = (i + size - 1) % size
	}
}

func (s *bytesshard) listBack() uint32 {
	return s.list[0].prev
}

func (s *bytesshard) listMoveToFront(i uint32) {
	base := unsafe.Pointer(unsafe.SliceData(s.list))
	root := (*bytesnode)(base)
	if root.next == i {
		return
	}

	nodei := (*bytesnode)(unsafe.Add(base, uintptr(i)*unsafe.Sizeof(s.list[0])))

	((*bytesnode)(unsafe.Add(base, uintptr(nodei.prev)*unsafe.Sizeof(s.list[0])))).next = nodei.next
	((*bytesnode)(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = nodei.prev

	nodei.prev = 0
	nodei.next = root.next

	root.next = i
	((*bytesnode)(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = i
}

func (s *bytesshard) listMoveToBack(i uint32) {
	base := unsafe.Pointer(unsafe.SliceData(s.list))
	j := ((*bytesnode)(base)).prev
	if i == j {
		return
	}

	nodei := (*bytesnode)(unsafe.Add(base, uintptr(i)*unsafe.Sizeof(s.list[0])))
	at := (*bytesnode)(unsafe.Add(base, uintptr(j)*unsafe.Sizeof(s.list[0])))

	((*bytesnode)(unsafe.Add(base, uintptr(nodei.prev)*unsafe.Sizeof(s.list[0])))).next = nodei.next
	((*bytesnode)(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = nodei.prev

	nodei.prev = j
	nodei.next = at.next

	((*bytesnode)(unsafe.Add(base, uintptr(j)*unsafe.Sizeof(s.list[0])))).next = i
	((*bytesnode)(unsafe.Add(base, uintptr(nodei.next)*unsafe.Sizeof(s.list[0])))).prev = i
}
