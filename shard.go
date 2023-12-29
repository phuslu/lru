package lru

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// shard is a LRU partition contains a list and a hash table.
type shard[K comparable, V any] struct {
	mu    sync.Mutex
	list  list[K, V]
	table rhh[K]

	_ [128 - unsafe.Sizeof(sync.Mutex{}) - unsafe.Sizeof(list[K, V]{}) - unsafe.Sizeof(rhh[K]{})]byte
}

func (s *shard[K, V]) Get(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if i, exists := s.table.Get(hash, key); exists {
		node := &s.list.nodes[i]
		if ts := node.expires; ts > 0 && atomic.LoadInt64(&now) > ts {
			s.list.MoveToBack(node)
			node.value = value
			s.table.Delete(hash, key)
		} else {
			s.list.MoveToFront(node)
			value = node.value
			ok = true
		}
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Peek(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if i, exists := s.table.Get(hash, key); exists {
		value = s.list.nodes[i].value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Set(hash uint32, hashfun func(K) uint32, key K, value V, ttl time.Duration) (prev V, replaced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i, exists := s.table.Get(hash, key); exists {
		node := &s.list.nodes[i]
		previousValue := node.value
		s.list.MoveToFront(node)
		node.value = value
		if ttl > 0 {
			node.expires = atomic.LoadInt64(&now) + int64(ttl)
		}
		prev = previousValue
		replaced = true
		return
	}

	node := s.list.Back()
	evictedValue := node.value
	s.table.Delete(hashfun(node.key), node.key)

	node.key = key
	node.value = value
	if ttl > 0 {
		node.expires = atomic.LoadInt64(&now) + int64(ttl)
	}
	s.table.Set(hash, key, node.index)
	s.list.MoveToFront(node)
	prev = evictedValue
	return
}

func (s *shard[K, V]) Delete(hash uint32, key K) (v V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i, exists := s.table.Get(hash, key); exists {
		node := &s.list.nodes[i]
		value := node.value
		s.list.MoveToBack(node)
		node.value = v
		s.table.Delete(hash, key)
		v = value
	}

	return
}

func (s *shard[K, V]) Len() (n int) {
	s.mu.Lock()
	n = s.table.Len()
	s.mu.Unlock()

	return
}

func (s *shard[K, V]) getkey(i uint32) K {
	return s.list.nodes[i].key
}

func newshard[K comparable, V any](size int) *shard[K, V] {
	s := &shard[K, V]{}

	s.list.Init(uint32(size), nil)
	s.table.init(int(float64(size)/0.8), s.getkey)

	return s
}

var now int64

func init() {
	atomic.StoreInt64(&now, time.Now().UnixNano())
	go func() {
		for {
			time.Sleep(time.Second)
			atomic.StoreInt64(&now, time.Now().UnixNano())
		}
	}()
}
