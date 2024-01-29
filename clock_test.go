package lru

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestClockStop(t *testing.T) {
	StopClock()
	now := atomic.LoadUint32(&clock)
	time.Sleep(3 * time.Second)
	if atomic.LoadUint32(&clock)-now >= 2 {
		t.Error("stop clock failed")
	}
}
