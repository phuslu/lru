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
	s.Init(1024, getRuntimeHasher[string](), 0)

	key := []byte("foobar")
	value := []byte("42")
	hash := uint32(s.table_hasher(noescape(unsafe.Pointer(&key)), s.table_seed))

	s.Set(hash, key, value)

	if index := s.list_Back(); string(s.list[index].key) == string(key) {
		t.Errorf("foobar should be list back: %v %s", index, s.list[index].key)
	}
}

func TestBytesShardTableSet(t *testing.T) {
	var s bytesshard
	s.Init(1024, getRuntimeHasher[string](), 0)

	key := []byte("foobar")
	value := []byte("42")
	hash := uint32(s.table_hasher(noescape(unsafe.Pointer(&key)), s.table_seed))

	s.Set(hash, key, value)

	i, ok := s.table_Set(hash, key, 123)
	if v := s.list[i].value; !ok || string(v) != string(value) {
		t.Errorf("foobar should be set to %s: %v %v", value, i, ok)
	}
}
