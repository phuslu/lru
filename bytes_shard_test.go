package lru

import (
	"testing"
	"unsafe"
)

func TestBytesShardPadding(t *testing.T) {
	var s bytesshard

	if n := unsafe.Sizeof(s); n != 128 {
		t.Errorf("shard size is %d, not 128", n)
	}
}

func TestBytesShardListSet(t *testing.T) {
	var s bytesshard
	s.Init(1024)

	key := []byte("foobar")
	value := []byte("42")
	hash := uint32(wyhashHashbytes(key, 0))

	s.Set(hash, key, value)

	if index := s.listBack(); string(s.list[index].key) == string(key) {
		t.Errorf("foobar should be list back: %v %s", index, s.list[index].key)
	}
}

func TestBytesShardTableInsert(t *testing.T) {
	var s bytesshard
	s.Init(4)
	keys := [5][]byte{nil, []byte("a"), []byte("b"), []byte("c"), []byte("d")}
	for i := 1; i < len(keys); i++ {
		s.list[i].key = keys[i]
	}
	testShardTableInsert(t, 8, s.tableInsert,
		func(hash, index uint32) (uint32, bool) { return s.tableGet(hash, keys[index]) },
		func(hash, index uint32) (uint32, bool) { return s.tableDelete(hash, keys[index]) },
		func() uint32 { return s.tableLength },
	)
}

func TestBytesShardTableDeleteMissing(t *testing.T) {
	var s bytesshard
	s.Init(8)

	key := []byte("present")
	value := []byte("value")
	hash := uint32(wyhashHashbytes(key, 0))
	s.Set(hash, key, value)

	missing := []byte("missing")
	index, ok := s.tableDelete(uint32(wyhashHashbytes(missing, 0)), missing)
	if ok || index != 0 {
		t.Fatalf("missing key should not delete an index: index=%d ok=%v", index, ok)
	}
	if got, want := s.tableLength, uint32(1); got != want {
		t.Fatalf("table length should be unchanged: got=%d want=%d", got, want)
	}
	if got, ok := s.Get(hash, key); !ok || b2s(got) != b2s(value) {
		t.Fatalf("present key should remain cached: value=%q ok=%v", got, ok)
	}
}
