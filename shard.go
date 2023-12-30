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

	if index, exists := s.table.Get(hash, key); exists {
		if timestamp := s.list.nodes[index].expires; timestamp > 0 && atomic.LoadInt64(&now) > timestamp {
			s.list.MoveToBack(index)
			s.list.nodes[index].value = value
			s.table.Delete(hash, key)
		} else {
			s.list.MoveToFront(index)
			value = s.list.nodes[index].value
			ok = true
		}
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Peek(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if index, exists := s.table.Get(hash, key); exists {
		value = s.list.nodes[index].value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Set(hash uint32, hashfun func(K) uint32, key K, value V, ttl time.Duration) (prev V, replaced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index, exists := s.table.Get(hash, key); exists {
		node := &s.list.nodes[index]
		previousValue := node.value
		s.list.MoveToFront(index)
		node.value = value
		if ttl > 0 {
			node.expires = atomic.LoadInt64(&now) + int64(ttl)
		}
		prev = previousValue
		replaced = true
		return
	}

	index := s.list.Back()
	node := &s.list.nodes[index]
	evictedValue := node.value
	s.table.Delete(hashfun(node.key), node.key)

	node.key = key
	node.value = value
	if ttl > 0 {
		node.expires = atomic.LoadInt64(&now) + int64(ttl)
	}
	s.table.Set(hash, key, index)
	s.list.MoveToFront(index)
	prev = evictedValue
	return
}

func (s *shard[K, V]) Delete(hash uint32, key K) (v V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index, exists := s.table.Get(hash, key); exists {
		node := &s.list.nodes[index]
		value := node.value
		s.list.MoveToBack(index)
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

func (s *shard[K, V]) getkey(index uint32) K {
	return s.list.nodes[index].key
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
