// Copyright 2023 Phus Lu. All rights reserved.

package lru

import (
	"sync/atomic"
	"time"
)

// clock is the number of seconds since January 1, 2024 UTC
// always use `atomic.LoadUint32(&clock)` for accessing clock value.
var clock uint32

func init() {
	const clockBase = 1704067200 // 2024-01-01T00:00:00Z
	atomic.StoreUint32(&clock, uint32(time.Now().Unix()-clockBase))
	go func() {
		for {
			for i := 0; i < 9; i++ {
				time.Sleep(100 * time.Millisecond)
				atomic.StoreUint32(&clock, uint32(time.Now().Unix()-clockBase))
			}
			time.Sleep(100 * time.Millisecond)
			atomic.StoreUint32(&clock, uint32(time.Now().Unix()-clockBase))
		}
	}()
}
