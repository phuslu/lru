package lru

import (
	"testing"
)

func TestShardTable(t *testing.T) {
	s := newshard[string, uint32](1024)

	hashfun := func(key string) (x uint64) {
		x = 5381
		for _, c := range []byte(key) {
			x = x*33 + uint64(c)
		}
		return
	}

	s.Set(uint32(hashfun("foobar")), hashfun, "foobar", 42, 0)

	i, ok := s.tableSet(uint32(hashfun("foobar")), "foobar", 123)
	if v := s.list[i].value; !ok || v != 42 {
		t.Errorf("foobar should be set to 42: %v %v", i, ok)
	}
}
