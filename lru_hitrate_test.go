package lru

import (
	"container/list"
	"testing"
)

func TestLRUCacheSingleShardHitRateMatchesClassicLRU(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		trace    []int
	}{
		{
			name:     "working set fits",
			capacity: 8,
			trace: []int{
				0, 1, 2, 3, 4, 5, 6, 7,
				0, 1, 2, 3, 4, 5, 6, 7,
				8, 9, 0, 1, 2, 3, 4, 5,
				6, 7, 8, 9, 0, 1, 2, 3,
			},
		},
		{
			name:     "small capacity churn",
			capacity: 3,
			trace: []int{
				0, 1, 2, 0, 3, 0, 4, 0,
				1, 2, 0, 3, 4, 0, 2, 4,
				5, 4, 2, 0, 4, 2, 6, 2,
			},
		},
		{
			name:     "hot keys with scans",
			capacity: 32,
			trace:    newClassicLRUHitRateTrace(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pin one shard so this cache has the same policy surface as a classic global LRU.
			cache := NewLRUCache[int, int](tt.capacity, WithShards[int, int](1))
			classic := newClassicLRU[int, int](tt.capacity)
			keys := make(map[int]struct{})
			var cacheHits, cacheMisses, classicHits, classicMisses int

			for _, key := range tt.trace {
				keys[key] = struct{}{}

				if _, ok := cache.Get(key); ok {
					cacheHits++
				} else {
					cacheMisses++
					cache.Set(key, key)
				}

				if _, ok := classic.Get(key); ok {
					classicHits++
				} else {
					classicMisses++
					classic.Set(key, key)
				}
			}

			if cacheHits != classicHits || cacheMisses != classicMisses {
				t.Fatalf("hit rate differs: cache hits=%d misses=%d ratio=%.4f, classic hits=%d misses=%d ratio=%.4f",
					cacheHits, cacheMisses, hitRatio(cacheHits, cacheMisses),
					classicHits, classicMisses, hitRatio(classicHits, classicMisses))
			}

			stats := cache.Stats()
			if got, want := stats.GetCalls, uint64(len(tt.trace)); got != want {
				t.Fatalf("cache get calls should be %d: %d", want, got)
			}
			if got, want := stats.Misses, uint64(classicMisses); got != want {
				t.Fatalf("cache misses should match classic misses %d: %d", want, got)
			}
			if got, want := cache.Len(), classic.Len(); got != want {
				t.Fatalf("cache length should match classic length %d: %d", want, got)
			}

			for key := range keys {
				cacheValue, cacheOK := cache.Peek(key)
				classicValue, classicOK := classic.Peek(key)
				if cacheOK != classicOK || cacheValue != classicValue {
					t.Fatalf("resident key %d differs: cache=(%d,%v), classic=(%d,%v)",
						key, cacheValue, cacheOK, classicValue, classicOK)
				}
			}

			t.Logf("matched classic LRU hit rate: hits=%d misses=%d ratio=%.4f", cacheHits, cacheMisses, hitRatio(cacheHits, cacheMisses))
		})
	}
}

func newClassicLRUHitRateTrace() []int {
	trace := make([]int, 0, 4096)
	for round := 0; round < 64; round++ {
		for key := 0; key < 24; key++ {
			trace = append(trace, key)
		}
		for key := 0; key < 12; key++ {
			trace = append(trace, (round+key)%24)
		}
		if round%4 == 0 {
			for key := 0; key < 48; key++ {
				trace = append(trace, 1000+round*48+key)
			}
		}
		for key := 0; key < 16; key++ {
			trace = append(trace, key)
		}
	}
	return trace
}

func hitRatio(hits, misses int) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

type classicLRU[K comparable, V any] struct {
	capacity int
	ll       *list.List
	items    map[K]*list.Element
}

type classicLRUEntry[K comparable, V any] struct {
	key   K
	value V
}

func newClassicLRU[K comparable, V any](capacity int) *classicLRU[K, V] {
	return &classicLRU[K, V]{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[K]*list.Element, capacity),
	}
}

func (c *classicLRU[K, V]) Get(key K) (value V, ok bool) {
	if elem, exists := c.items[key]; exists {
		c.ll.MoveToFront(elem)
		return elem.Value.(classicLRUEntry[K, V]).value, true
	}
	return
}

func (c *classicLRU[K, V]) Peek(key K) (value V, ok bool) {
	if elem, exists := c.items[key]; exists {
		return elem.Value.(classicLRUEntry[K, V]).value, true
	}
	return
}

func (c *classicLRU[K, V]) Set(key K, value V) {
	if c.capacity <= 0 {
		return
	}
	if elem, exists := c.items[key]; exists {
		elem.Value = classicLRUEntry[K, V]{key: key, value: value}
		c.ll.MoveToFront(elem)
		return
	}

	elem := c.ll.PushFront(classicLRUEntry[K, V]{key: key, value: value})
	c.items[key] = elem
	if c.ll.Len() > c.capacity {
		c.removeOldest()
	}
}

func (c *classicLRU[K, V]) Len() int {
	return len(c.items)
}

func (c *classicLRU[K, V]) removeOldest() {
	elem := c.ll.Back()
	if elem == nil {
		return
	}
	c.ll.Remove(elem)
	entry := elem.Value.(classicLRUEntry[K, V])
	delete(c.items, entry.key)
}
