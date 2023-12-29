// Copyright 2023 Phus Lu. All rights reserved.
package lru

type node[K comparable, V any] struct {
	key     K
	value   V
	expires int64
	index   uint32
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
		l.nodes[i].index = i
		l.nodes[i].next = (i + 1) % size
		l.nodes[i].prev = (i + size - 1) % size
	}
}

func (l *list[K, V]) move(n, at *node[K, V]) {
	if n.index == at.index {
		return
	}

	l.nodes[n.prev].next = n.next
	l.nodes[n.next].prev = n.prev

	n.prev = at.index
	n.next = at.next

	l.nodes[at.index].next = n.index
	l.nodes[n.next].prev = n.index
}

func (l *list[K, V]) Back() *node[K, V] {
	return &l.nodes[l.nodes[0].prev]
}

func (l *list[K, V]) MoveToFront(n *node[K, V]) {
	if l.nodes[l.nodes[0].next].index == n.index {
		return
	}
	l.move(n, &l.nodes[0])
}

func (l *list[K, V]) MoveToBack(n *node[K, V]) {
	if l.nodes[l.nodes[0].prev].index == n.index {
		return
	}
	l.move(n, &l.nodes[l.nodes[0].prev])
}
