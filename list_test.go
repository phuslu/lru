package lru

import (
	"testing"
)

func TestListMove(t *testing.T) {
	l := &list[byte, error]{}
	l.Init(10, func(i uint32) (byte, error) { return 'a' + byte(i) - 1, nil })

	nexts := func() (s string) {
		node := &l.nodes[0]
		for node.next != 0 {
			node = &l.nodes[node.next]
			s += string(node.key)
		}
		return
	}

	prevs := func() (s string) {
		node := &l.nodes[0]
		for node.prev != 0 {
			node = &l.nodes[node.prev]
			s += string(node.key)
		}
		return
	}

	if want, got := "abcdefghij", nexts(); want != got {
		t.Fatalf("nexts want=%#v got=%#v\n", want, got)
	}
	if want, got := "jihgfedcba", prevs(); want != got {
		t.Fatalf("prevs want=%#v got=%#v\n", want, got)
	}

	l.MoveToFront(l.Back())
	if want, got := "jabcdefghi", nexts(); want != got {
		t.Fatalf("nexts want=%#v got=%#v\n", want, got)
	}
	if want, got := "ihgfedcbaj", prevs(); want != got {
		t.Fatalf("prevs want=%#v got=%#v\n", want, got)
	}

	l.MoveToFront(l.Back())
	if want, got := "ijabcdefgh", nexts(); want != got {
		t.Fatalf("nexts want=%#v got=%#v\n", want, got)
	}
	if want, got := "hgfedcbaji", prevs(); want != got {
		t.Fatalf("prevs want=%#v got=%#v\n", want, got)
	}

	l.MoveToBack(4)
	if want, got := "ijabcefghd", nexts(); want != got {
		t.Fatalf("nexts want=%#v got=%#v\n", want, got)
	}
	if want, got := "dhgfecbaji", prevs(); want != got {
		t.Fatalf("prevs want=%#v got=%#v\n", want, got)
	}

	l.MoveToFront(4)
	if want, got := "dijabcefgh", nexts(); want != got {
		t.Fatalf("nexts want=%#v got=%#v\n", want, got)
	}
	if want, got := "hgfecbajid", prevs(); want != got {
		t.Fatalf("prevs want=%#v got=%#v\n", want, got)
	}
}
