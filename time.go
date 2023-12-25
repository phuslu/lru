package lru

import (
	"sync"
	"sync/atomic"
	"time"
)

var timeUnixNano = func() int64 { return time.Now().UnixNano() }

var now int64
var nowOnce sync.Once

func fastTimeUnixNano() int64 { return atomic.LoadInt64(&now) }

func EnableFastTimeUnixNano() {
	nowOnce.Do(func() {
		atomic.StoreInt64(&now, time.Now().UnixNano())
		go func() {
			for {
				time.Sleep(time.Second)
				atomic.StoreInt64(&now, time.Now().UnixNano())
			}
		}()
	})
	timeUnixNano = fastTimeUnixNano
}
