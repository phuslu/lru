// Copyright 2023 Phus Lu. All rights reserved.

package lru

type node[K comparable, V any] struct {
	key     K
	value   V
	expires uint32
	ttl     uint32
	next    uint32
	prev    uint32
}

// list is an arraylist to reduce GC efforts and improve performance.
type list[K comparable, V any] struct {
	nodes []node[K, V]
}

func (l *list[K, V]) Init(size uint32, value func(index uint32) (K, V)) {
	size += 1
	l.nodes = make([]node[K, V], size)
	for i := uint32(0); i < size; i++ {
		if value != nil && i != 0 {
			l.nodes[i].key, l.nodes[i].value = value(i)
		}
		l.nodes[i].next = (i + 1) % size
		l.nodes[i].prev = (i + size - 1) % size
	}
}

func (l *list[K, V]) Back() uint32 {
	return l.nodes[0].prev
}

func (l *list[K, V]) MoveToFront(i uint32) {
	root := &l.nodes[0]
	if root.next == i {
		return
	}

	node := &l.nodes[i]

	l.nodes[node.prev].next = node.next
	l.nodes[node.next].prev = node.prev

	node.prev = 0
	node.next = root.next

	root.next = i
	l.nodes[node.next].prev = i
}

func (l *list[K, V]) MoveToBack(i uint32) {
	j := l.nodes[0].prev
	if i == j {
		return
	}

	node, at := &l.nodes[i], &l.nodes[j]

	l.nodes[node.prev].next = node.next
	l.nodes[node.next].prev = node.prev

	node.prev = j
	node.next = at.next

	l.nodes[j].next = i
	l.nodes[node.next].prev = i
}
