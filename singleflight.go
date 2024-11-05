// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lru

import "sync"

// singleflightCall is an in-flight or completed singleflight.Do singleflightCall
type singleflightCall[T any] struct {
	wg sync.WaitGroup

	// These fields are written once before the WaitGroup is done
	// and are only read after the WaitGroup is done.
	val T
	err error

	// These fields are read and written with the singleflight
	// mutex held before the WaitGroup is done, and are read but
	// not written after the WaitGroup is done.
	dups int
}

// Group represents a class of work and forms a namespace in
// which units of work can be executed with duplicate suppression.
type singleflightGroup[K comparable, V any] struct {
	mu sync.Mutex                 // protects m
	m  map[K]*singleflightCall[V] // lazily initialized
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time. If a duplicate comes in, the duplicate singleflight_caller waits for the
// original to complete and receives the same results.
// The return value shared indicates whether v was given to multiple singleflight_callers.
func (g *singleflightGroup[K, V]) Do(key K, fn func() (V, error)) (v V, err error, shared bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[K]*singleflightCall[V])
	}
	if c, ok := g.m[key]; ok {
		c.dups++
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, true
	}
	c := new(singleflightCall[V])
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	g.doCall(c, key, fn)
	return c.val, c.err, c.dups > 0
}

// doCall handles the single singleflightCall for a key.
func (g *singleflightGroup[K, V]) doCall(c *singleflightCall[V], key K, fn func() (V, error)) {
	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
}
