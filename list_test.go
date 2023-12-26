package lru

import (
	"fmt"
	"testing"
)

func TestListRange(t *testing.T) {
	l := &list[string]{}
	l.Init(10, func(i uint32) string { return fmt.Sprintf("%c", 'a'+i-1) })

	allnext := func() (s string) {
		item := &l.items[0]
		for item.next != 0 {
			item = &l.items[item.next]
			s += item.Value
		}
		return
	}

	allprev := func() (s string) {
		item := &l.items[0]
		for item.prev != 0 {
			item = &l.items[item.prev]
			s += item.Value
		}
		return
	}

	if want, got := "abcdefghij", allnext(); want != got {
		t.Fatalf("allnext want=%#v got=%#v\n", want, got)
	}
	if want, got := "jihgfedcba", allprev(); want != got {
		t.Fatalf("allprev want=%#v got=%#v\n", want, got)
	}

	l.MoveToFront(l.Back())
	if want, got := "jabcdefghi", allnext(); want != got {
		t.Fatalf("allnext want=%#v got=%#v\n", want, got)
	}
	if want, got := "ihgfedcbaj", allprev(); want != got {
		t.Fatalf("allprev want=%#v got=%#v\n", want, got)
	}

	l.MoveToFront(l.Back())
	if want, got := "ijabcdefgh", allnext(); want != got {
		t.Fatalf("allnext want=%#v got=%#v\n", want, got)
	}
	if want, got := "hgfedcbaji", allprev(); want != got {
		t.Fatalf("allprev want=%#v got=%#v\n", want, got)
	}

	l.MoveToBack(&l.items[4])
	if want, got := "ijabcefghd", allnext(); want != got {
		t.Fatalf("allnext want=%#v got=%#v\n", want, got)
	}
	if want, got := "dhgfecbaji", allprev(); want != got {
		t.Fatalf("allprev want=%#v got=%#v\n", want, got)
	}

	l.MoveToFront(&l.items[4])
	if want, got := "dijabcefgh", allnext(); want != got {
		t.Fatalf("allnext want=%#v got=%#v\n", want, got)
	}
	if want, got := "hgfecbajid", allprev(); want != got {
		t.Fatalf("allprev want=%#v got=%#v\n", want, got)
	}
}
