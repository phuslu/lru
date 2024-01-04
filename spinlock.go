// Copyright 2023 Phus Lu. All rights reserved.

package lru

import (
	"sync/atomic"
)

type spinlock struct {
	lock uint32
}

func (l *spinlock) Lock() {
	for {
		for atomic.LoadUint32(&l.lock) == 1 {
		}
		if atomic.CompareAndSwapUint32(&l.lock, 0, 1) {
			return
		}
	}
}

func (l *spinlock) Unlock() {
	atomic.StoreUint32(&l.lock, 0)
}
