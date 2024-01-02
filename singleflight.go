// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package singleflight provides a duplicate function call suppression
// mechanism.
package lru

import "sync"

// singleflight_call is an in-flight or completed singleflight.Do singleflight_call
type singleflight_call[T any] struct {
	wg sync.WaitGroup

	// These fields are written once before the WaitGroup is done
	// and are only read after the WaitGroup is done.
	val T
	err error

	// These fields are read and written with the singleflight
	// mutex held before the WaitGroup is done, and are read but
	// not written after the WaitGroup is done.
	dups  int
	chans []chan<- singleflight_Result[T]
}

// Group represents a class of work and forms a namespace in
// which units of work can be executed with duplicate suppression.
type singleflight_Group[K comparable, V any] struct {
	mu sync.Mutex                  // protects m
	m  map[K]*singleflight_call[V] // lazily initialized
}

// Result holds the results of Do, so they can be passed
// on a channel.
type singleflight_Result[T any] struct {
	Val    T
	Err    error
	Shared bool
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time. If a duplicate comes in, the duplicate singleflight_caller waits for the
// original to complete and receives the same results.
// The return value shared indicates whether v was given to multiple singleflight_callers.
func (g *singleflight_Group[K, V]) Do(key K, fn func() (V, error)) (v V, err error, shared bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[K]*singleflight_call[V])
	}
	if c, ok := g.m[key]; ok {
		c.dups++
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, true
	}
	c := new(singleflight_call[V])
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	g.doCall(c, key, fn)
	return c.val, c.err, c.dups > 0
}

// DoChan is like Do but returns a channel that will receive the
// results when they are ready.
func (g *singleflight_Group[K, V]) DoChan(key K, fn func() (V, error)) <-chan singleflight_Result[V] {
	ch := make(chan singleflight_Result[V], 1)
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[K]*singleflight_call[V])
	}
	if c, ok := g.m[key]; ok {
		c.dups++
		c.chans = append(c.chans, ch)
		g.mu.Unlock()
		return ch
	}
	c := &singleflight_call[V]{chans: []chan<- singleflight_Result[V]{ch}}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	go g.doCall(c, key, fn)

	return ch
}

// doCall handles the single singleflight_call for a key.
func (g *singleflight_Group[K, V]) doCall(c *singleflight_call[V], key K, fn func() (V, error)) {
	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	for _, ch := range c.chans {
		ch <- singleflight_Result[V]{c.val, c.err, c.dups > 0}
	}
	g.mu.Unlock()
}

// ForgetUnshared tells the singleflight to forget about a key if it is not
// shared with any other goroutines. Future singleflight_calls to Do for a forgotten key
// will singleflight_call the function rather than waiting for an earlier singleflight_call to complete.
// Returns whether the key was forgotten or unknown--that is, whether no
// other goroutines are waiting for the result.
func (g *singleflight_Group[K, V]) ForgetUnshared(key K) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	c, ok := g.m[key]
	if !ok {
		return true
	}
	if c.dups == 0 {
		delete(g.m, key)
		return true
	}
	return false
}
