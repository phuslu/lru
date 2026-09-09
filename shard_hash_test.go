package lru

import (
	"testing"
	"unsafe"
)

func TestLRUCacheShardHashBits(t *testing.T) {
	c := NewLRUCache[uint32, uint32]((1<<shardBits)*16,
		WithShards[uint32, uint32](1<<shardBits),
		WithHasher[uint32, uint32](func(key unsafe.Pointer, seed uintptr) uintptr {
			return uintptr(*(*uint32)(key))
		}),
	)
	testShardHashBits(t,
		func(key, value uint32) { c.Set(key, value) }, c.Get, c.Delete,
		func(shard uint32) []uint64 { return c.shards[shard].tableBuckets },
	)
}

func TestTTLCacheShardHashBits(t *testing.T) {
	c := NewTTLCache[uint32, uint32]((1<<shardBits)*16,
		WithShards[uint32, uint32](1<<shardBits),
		WithHasher[uint32, uint32](func(key unsafe.Pointer, seed uintptr) uintptr {
			return uintptr(*(*uint32)(key))
		}),
	)
	testShardHashBits(t,
		func(key, value uint32) { c.Set(key, value, 0) }, c.Get, c.Delete,
		func(shard uint32) []uint64 { return c.shards[shard].tableBuckets },
	)
}

func testShardHashBits(t *testing.T, set func(uint32, uint32), get func(uint32) (uint32, bool), del func(uint32) uint32, buckets func(uint32) []uint64) {
	t.Helper()
	maxShards := uint32(1 << shardBits)
	for _, shard := range []uint32{0, maxShards / 2, maxShards - 1} {
		var keys [16]uint32
		for i := range keys {
			// Alternate between both halves of the table. Before shard and
			// table bits were separated, the top shard bit (bit shardBits-1)
			// also fed the bucket origin, making each pair collide.
			home := uint32(i/2 + i%2*16)
			keys[i] = home<<shardBits | shard
			set(keys[i], keys[i]+1)
		}
		var entries int
		for _, bucket := range buckets(shard) {
			b := (*lrubucket)(unsafe.Pointer(&bucket))
			if b.index == 0 {
				continue
			}
			entries++
			if dib := b.hdib & maxDIB; dib != 1 {
				t.Fatalf("shard %d: independent bucket origins should not collide: dib=%d", shard, dib)
			}
		}
		if entries != len(keys) {
			t.Fatalf("shard %d: got %d entries, want %d", shard, entries, len(keys))
		}
		for _, key := range keys {
			if value, ok := get(key); !ok || value != key+1 {
				t.Fatalf("Get(%d) = (%d, %v), want (%d, true)", key, value, ok, key+1)
			}
			if value := del(key); value != key+1 {
				t.Fatalf("Delete(%d) = %d, want %d", key, value, key+1)
			}
			if _, ok := get(key); ok {
				t.Fatalf("deleted key %d is still present", key)
			}
		}
	}
}
