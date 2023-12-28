package lru

import (
	"sync"
	"sync/atomic"
	"time"
)

type shard[K comparable, V any] struct {
	mu    sync.Mutex
	list  list[entry[K, V]]
	table rhh[K]
}

type entry[K comparable, V any] struct {
	key     K
	value   V
	expires int64
}

func (s *shard[K, V]) Get(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if i, exists := s.table.Get(hash, key); exists {
		item := &s.list.items[i]
		if ts := item.Value.expires; ts > 0 && atomic.LoadInt64(&now) > ts {
			s.list.MoveToBack(item)
			item.Value.value = value
			s.table.Delete(hash, key)
		} else {
			s.list.MoveToFront(item)
			value = item.Value.value
			ok = true
		}
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Peek(hash uint32, key K) (value V, ok bool) {
	s.mu.Lock()

	if i, exists := s.table.Get(hash, key); exists {
		value = s.list.items[i].Value.value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Set(hash uint32, hashfun func(K) uint32, key K, value V, ttl time.Duration) (prev V, replaced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i, exists := s.table.Get(hash, key); exists {
		item := &s.list.items[i]
		previousValue := item.Value.value
		s.list.MoveToFront(item)
		item.Value.value = value
		if ttl > 0 {
			item.Value.expires = atomic.LoadInt64(&now) + int64(ttl)
		}
		prev = previousValue
		replaced = true
		return
	}

	item := s.list.Back()
	evictedValue := item.Value.value
	s.table.Delete(hashfun(item.Value.key), item.Value.key)

	item.Value.key = key
	item.Value.value = value
	if ttl > 0 {
		item.Value.expires = atomic.LoadInt64(&now) + int64(ttl)
	}
	s.table.Set(hash, key, item.index)
	s.list.MoveToFront(item)
	prev = evictedValue
	return
}

func (s *shard[K, V]) Delete(hash uint32, key K) (v V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i, exists := s.table.Get(hash, key); exists {
		item := &s.list.items[i]
		value := item.Value.value
		s.list.MoveToBack(item)
		item.Value.value = v
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
	return s.list.items[i].Value.key
}

func newshard[K comparable, V any](size int) *shard[K, V] {
	s := &shard[K, V]{}

	s.list.Init(uint32(size), nil)
	s.table.init(size, s.getkey)

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
