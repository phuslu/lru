// Copyright 2023 Phus Lu. All rights reserved.

package lru

func (s *shard[K, V]) list_Init(size uint32) {
	size += 1
	if len(s.list) == 0 {
		s.list = make([]node[K, V], size)
	}
	for i := uint32(0); i < size; i++ {
		s.list[i].next = (i + 1) % size
		s.list[i].prev = (i + size - 1) % size
	}
}

func (s *shard[K, V]) list_Back() uint32 {
	return s.list[0].prev
}

func (s *shard[K, V]) list_MoveToFront(i uint32) {
	root := &s.list[0]
	if root.next == i {
		return
	}

	node := &s.list[i]

	s.list[node.prev].next = node.next
	s.list[node.next].prev = node.prev

	node.prev = 0
	node.next = root.next

	root.next = i
	s.list[node.next].prev = i
}

func (s *shard[K, V]) list_MoveToBack(i uint32) {
	j := s.list[0].prev
	if i == j {
		return
	}

	node, at := &s.list[i], &s.list[j]

	s.list[node.prev].next = node.next
	s.list[node.next].prev = node.prev

	node.prev = j
	node.next = at.next

	s.list[j].next = i
	s.list[node.next].prev = i
}
