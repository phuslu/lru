package lru

import (
	"testing"
)

func TestHashtable(t *testing.T) {
	table := &hashtable[int]{}

	hash := func(key int) uint32 {
		return uint32(key)
	}

	getkey := func(index uint32) int {
		return int(index)
	}

	table.init(1024, getkey)

	table.Set(hash(1), 1, 1)
	table.Set(hash(1), 1, 2)
	table.Set(hash(2), 2, 2)

	if v, ok := table.Get(hash(2), 2); v != 2 || !ok {
		t.Errorf("2 should be set to 2: %v", v)
	}
}
