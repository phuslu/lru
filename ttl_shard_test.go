package lru

import (
	"testing"
	"time"
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
	hash := uint32(s.tableHasher(noescape(unsafe.Pointer(&key)), s.tableSeed))

	s.Set(hash, key, 42, 0)

	if index := s.listBack(); s.list[index].key == key {
		t.Errorf("foobar should be list back: %v %v", index, s.list[index].key)
	}
}

func TestTTLShardTableSet(t *testing.T) {
	var s ttlshard[string, uint32]
	s.Init(1024, getRuntimeHasher[string](), 0)

	key := "foobar"
	hash := uint32(s.tableHasher(noescape(unsafe.Pointer(&key)), s.tableSeed))

	s.Set(hash, key, 42, 0)

	i, ok := s.tableSet(hash, key, 123)
	if v := s.list[i].value; !ok || v != 42 {
		t.Errorf("foobar should be set to 42: %v %v", i, ok)
	}
}

func TestTTLShardTableDeleteMissing(t *testing.T) {
	var s ttlshard[string, int]
	s.Init(8, getRuntimeHasher[string](), 0)

	key := "present"
	hash := uint32(s.tableHasher(noescape(unsafe.Pointer(&key)), s.tableSeed))
	s.Set(hash, key, 1, time.Hour)

	missing := "missing"
	missingHash := uint32(s.tableHasher(noescape(unsafe.Pointer(&missing)), s.tableSeed))
	index, ok := s.tableDelete(missingHash, missing)
	if ok || index != 0 {
		t.Fatalf("missing key should not delete an index: index=%d ok=%v", index, ok)
	}
	if got, want := s.tableLength, uint32(1); got != want {
		t.Fatalf("table length should be unchanged: got=%d want=%d", got, want)
	}
	if got, ok := s.Get(hash, key); !ok || got != 1 {
		t.Fatalf("present key should remain cached: value=%d ok=%v", got, ok)
	}
}
