package lru

import (
	"fmt"
	"testing"
	"unsafe"
)

func TestLRUShardPadding(t *testing.T) {
	var s lrushard[string, int]

	if n := unsafe.Sizeof(s); n != 128 {
		t.Errorf("shard size is %d, not 128", n)
	}
}

func TestLRUShardListSet(t *testing.T) {
	var s lrushard[string, uint32]
	s.Init(1024, getRuntimeHasher[string](), 0)

	key := "foobar"
	hash := uint32(s.tableHasher(noescape(unsafe.Pointer(&key)), s.tableSeed))

	s.Set(hash, key, 42)

	if index := s.listBack(); s.list[index].key == key {
		t.Errorf("foobar should be list back: %v %v", index, s.list[index].key)
	}
}

func TestLRUShardTableSet(t *testing.T) {
	var s lrushard[string, uint32]
	s.Init(1024, getRuntimeHasher[string](), 0)

	key := "foobar"
	hash := uint32(s.tableHasher(noescape(unsafe.Pointer(&key)), s.tableSeed))

	s.Set(hash, key, 42)

	i, ok := s.tableSet(hash, key, 123)
	if v := s.list[i].value; !ok || v != 42 {
		t.Errorf("foobar should be set to 42: %v %v", i, ok)
	}
}

func TestLRUShardTableDeleteMissing(t *testing.T) {
	var s lrushard[string, int]
	s.Init(8, getRuntimeHasher[string](), 0)

	key := "present"
	hash := uint32(s.tableHasher(noescape(unsafe.Pointer(&key)), s.tableSeed))
	s.Set(hash, key, 1)

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

func TestLRUCacheLengthWithZeroValue(t *testing.T) {
	cache := NewLRUCache[string, string](128, WithShards[string, string](1))

	cache.Set("", "")
	cache.Set("1", "1")

	if got, want := cache.Len(), 2; got != want {
		t.Fatalf("current cache length %v should be %v", got, want)
	}

	for i := 2; i < 128; i++ {
		k := fmt.Sprintf("%d", i)
		if _, replace := cache.Set(k, k); replace {
			t.Fatalf("key %v should not be replaced", k)
		}
	}

	if l := cache.Len(); l != 128 {
		t.Fatalf("cache length %v should be 128", l)
	}

	for i := 128; i < 256; i++ {
		k := fmt.Sprintf("%d", i)
		v := ""
		if i-128 > 0 {
			v = fmt.Sprintf("%d", i-128)
		}
		if prev, _ := cache.Set(k, k); prev != v {
			t.Fatalf("value %v should be evicted", prev)
		}
	}

	if l := cache.Len(); l != 128 {
		t.Fatalf("cache length %v should be 128", l)
	}
}
