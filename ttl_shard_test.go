package lru

import (
	"testing"
	"unsafe"
)

func TestTTLShardPadding(t *testing.T) {
	var s ttlshard[string, int]

	if n := unsafe.Sizeof(s); n != 128 {
		t.Errorf("shard size is %d, not 128", n)
	}
}

func TestTTLShardListSet(t *testing.T) {
	var s ttlshard[string, uint32]
	s.Init(1024, getRuntimeHasher[string](), 0)

	key := "foobar"
	hash := uint32(s.table_hasher(noescape(unsafe.Pointer(&key)), s.table_seed))

	s.Set(hash, key, 42, 0)

	if index := s.list_Back(); s.list[index].key == key {
		t.Errorf("foobar should be list back: %v %v", index, s.list[index].key)
	}
}

func TestTTLShardTableSet(t *testing.T) {
	var s ttlshard[string, uint32]
	s.Init(1024, getRuntimeHasher[string](), 0)

	key := "foobar"
	hash := uint32(s.table_hasher(noescape(unsafe.Pointer(&key)), s.table_seed))

	s.Set(hash, key, 42, 0)

	i, ok := s.table_Set(hash, key, 123)
	if v := s.list[i].value; !ok || v != 42 {
		t.Errorf("foobar should be set to 42: %v %v", i, ok)
	}
}
