package lru

import (
	"sync"
	"time"
)

type shard[K comparable, V any] struct {
	mu    sync.Mutex
	list  *list[*entry[K, V]]
	table rhhmap[K, *element[*entry[K, V]]]
	_     [24]byte
}

type entry[K comparable, V any] struct {
	key     K
	value   V
	expires int64
}

func (s *shard[K, V]) Get(hash uint64, key K) (value V, ok bool) {
	s.mu.Lock()

	if e, exists := s.table.Get(hash, key); exists {
		if ts := e.Value.expires; ts > 0 && timeUnixNano() > ts {
			s.list.MoveToBack(e)
			e.Value.value = value
			s.table.Delete(hash, key)
		} else {
			s.list.MoveToFront(e)
			value = e.Value.value
			ok = true
		}
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Peek(hash uint64, key K) (value V, ok bool) {
	s.mu.Lock()

	if e, exists := s.table.Get(hash, key); exists {
		value = e.Value.value
		ok = true
	}

	s.mu.Unlock()

	return
}

func (s *shard[K, V]) Set(hash uint64, hashfun func(K) uint64, key K, value V, ttl time.Duration) (prev V, replaced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, exists := s.table.Get(hash, key); exists {
		previousValue := e.Value.value
		s.list.MoveToFront(e)
		e.Value.value = value
		if ttl > 0 {
			e.Value.expires = timeUnixNano() + int64(ttl)
		}
		prev = previousValue
		replaced = true
		return
	}

	e := s.list.Back()
	i := e.Value
	evictedValue := i.value
	s.table.Delete(hashfun(i.key), i.key)

	i.key = key
	i.value = value
	if ttl > 0 {
		i.expires = timeUnixNano() + int64(ttl)
	}
	s.table.Set(hash, key, e)
	s.list.MoveToFront(e)
	prev = evictedValue
	return
}

func (s *shard[K, V]) Delete(hash uint64, key K) (v V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, exists := s.table.Get(hash, key); exists {
		value := e.Value.value
		s.list.MoveToBack(e)
		e.Value.value = v
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

func newshard[K comparable, V any](size int) *shard[K, V] {
	s := &shard[K, V]{}

	s.list = new(list[*entry[K, V]])
	s.list.init()
	for i := 0; i < size; i++ {
		s.list.PushBack(&entry[K, V]{})
	}

	s.table.init(size)

	return s
}
