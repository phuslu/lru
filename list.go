package lru

type listitem[T any] struct {
	Value      T
	index      uint32
	next, prev uint32
}

type list[T any] struct {
	items []listitem[T]
}

func (l *list[T]) Init(size uint32, value func(index uint32) T) *list[T] {
	size += 1
	l.items = make([]listitem[T], size)
	for i := uint32(0); i < size; i++ {
		l.items[i].Value = value(i)
		l.items[i].index = i
		l.items[i].next = (i + 1) % size
		l.items[i].prev = (i + size - 1) % size
	}
	return l
}

func (l *list[T]) move(e, at *listitem[T]) {
	if e.index == at.index {
		return
	}

	l.items[e.prev].next = e.next
	l.items[e.next].prev = e.prev

	e.prev = at.index
	e.next = at.next

	l.items[at.index].next = e.index
	l.items[e.next].prev = e.index
}

func (l *list[T]) Back() *listitem[T] {
	return &l.items[l.items[0].prev]
}

func (l *list[T]) MoveToFront(e *listitem[T]) {
	if l.items[l.items[0].next].index == e.index {
		return
	}
	l.move(e, &l.items[0])
}

func (l *list[T]) MoveToBack(e *listitem[T]) {
	if l.items[l.items[0].prev].index == e.index {
		return
	}
	l.move(e, &l.items[l.items[0].prev])
}
