package lru

import "testing"

func testShardTableInsert(t *testing.T, shift uint, insert func(uint32, uint32), get, del func(uint32, uint32) (uint32, bool), length func() uint32) {
	t.Helper()
	// With eight buckets, inserting a second key at home 6 displaces the
	// key at home 7 across the end of the table. Keys 1, 3 and 4 also share
	// their full hash, so lookup must distinguish them by key.
	hashes := [5]uint32{0, 6 << shift, 7 << shift, 6 << shift, 6 << shift}
	check := func(want ...uint32) {
		t.Helper()
		if n := length(); n != uint32(len(want)) {
			t.Fatalf("table length = %d, want %d", n, len(want))
		}
		for key := uint32(1); key < uint32(len(hashes)); key++ {
			var expected uint32
			for _, index := range want {
				if index == key {
					expected = index
				}
			}
			index, ok := get(hashes[key], key)
			if index != expected || ok != (expected != 0) {
				t.Fatalf("key %d: index=%d, ok=%v, want index=%d", key, index, ok, expected)
			}
		}
	}
	remove := func(key uint32) {
		t.Helper()
		if index, ok := del(hashes[key], key); !ok || index != key {
			t.Fatalf("delete key %d: index=%d, ok=%v", key, index, ok)
		}
	}

	for key := uint32(1); key <= 3; key++ {
		insert(hashes[key], key)
	}
	check(1, 2, 3)
	remove(1)
	check(2, 3)
	insert(hashes[4], 4)
	check(2, 3, 4)
	remove(3)
	check(2, 4)
	remove(4)
	check(2)
	remove(2)
	check()
}
