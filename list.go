// Copyright 2023 Phus Lu. All rights reserved.
package lru

type node[K comparable, V any] struct {
	key     K
	value   V
	expires int64
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

func (l *list[K, V]) move(i, j uint32) {
	if i == j {
		return
	}

	n, at := &l.nodes[i], &l.nodes[j]

	l.nodes[n.prev].next = n.next
	l.nodes[n.next].prev = n.prev

	n.prev = j
	n.next = at.next

	l.nodes[j].next = i
	l.nodes[n.next].prev = i
}

func (l *list[K, V]) Back() uint32 {
	return l.nodes[0].prev
}

func (l *list[K, V]) MoveToFront(i uint32) {
	if l.nodes[0].next == i {
		return
	}
	l.move(i, 0)
}

func (l *list[K, V]) MoveToBack(i uint32) {
	l.move(i, l.nodes[0].prev)
}
