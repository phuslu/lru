// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lru

// element is an element of a linked list.
type element[T any] struct {
	// Next and previous pointers in the doubly-linked list of elements.
	// To simplify the implementation, internally a list l is implemented
	// as a ring, such that &l.root is both the next element of the last
	// list element (l.Back()) and the previous element of the first list
	// element (l.Front()).
	next, prev *element[T]

	// The value stored with this element.
	Value T
}

// List represents a doubly linked list.
// The zero value for List is an empty list ready to use.
type list[T any] struct {
	root element[T] // sentinel list element, only &root, root.prev, and root.next are used
}

// Init initializes or clears list l.
func (l *list[T]) init() *list[T] {
	l.root.next = &l.root
	l.root.prev = &l.root
	return l
}

// insert inserts e after at, increments l.len, and returns e.
func (l *list[T]) insert(e, at *element[T]) *element[T] {
	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e
	return e
}

// move moves e to next to at.
func (l *list[T]) move(e, at *element[T]) {
	if e == at {
		return
	}
	e.prev.next = e.next
	e.next.prev = e.prev

	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e
}

// Back returns the last element of list l or nil if the list is empty.
func (l *list[T]) Back() *element[T] {
	return l.root.prev
}

// PushBack inserts a new element e with value v at the back of list l and returns e.
func (l *list[T]) PushBack(v T) *element[T] {
	return l.insert(&element[T]{Value: v}, l.root.prev)
}

// MoveToFront moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
func (l *list[T]) MoveToFront(e *element[T]) {
	if l.root.next == e {
		return
	}
	// see comment in List.Remove about initialization of l
	l.move(e, &l.root)
}

// MoveToBack moves element e to the back of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
func (l *list[T]) MoveToBack(e *element[T]) {
	if l.root.prev == e {
		return
	}
	// see comment in List.Remove about initialization of l
	l.move(e, l.root.prev)
}
