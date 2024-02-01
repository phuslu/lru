package lru

import (
	"testing"
	"unsafe"
)

func TestShardPadding(t *testing.T) {
	var s shard[string, int]

	if n := unsafe.Sizeof(s); n != 128 {
		t.Errorf("shard size is %d, not 128", n)
	}

}

func TestShardTableSet(t *testing.T) {
	var s shard[string, uint32]
	s.Init(1024)

	hashfun := func(key string) (x uint64) {
		x = 5381
		for _, c := range []byte(key) {
			x = x*33 + uint64(c)
		}
		return
	}

	s.Set(uint32(hashfun("foobar")), hashfun, "foobar", 42, 0)

	i, ok := s.table_Set(uint32(hashfun("foobar")), "foobar", 123)
	if v := s.list[i].value; !ok || v != 42 {
		t.Errorf("foobar should be set to 42: %v %v", i, ok)
	}
}
