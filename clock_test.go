package lru

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestClockingGood(t *testing.T) {
	for i := 0; i < 1000; i++ {
		go func() {
			time.Sleep(100 * time.Millisecond)
			clocking()
		}()
	}

	time.Sleep(time.Second)

	if n := runtime.NumGoroutine(); n > 10 {
		t.Errorf("bad clocking, too many gorouinte number: %v", n)
	}
}

func TestClockingBad(t *testing.T) {
	c := make(chan struct{})
	for i := 0; i < 1000; i++ {
		go func() {
			<-c
			clocking()
		}()
	}

	go func() {
		for i := 0; i < 9223372036854775807; i++ {
			atomic.StoreUint32(&clock, 0)
		}
	}()

	close(c)

	time.Sleep(time.Second)

	if n := runtime.NumGoroutine(); n < 100 {
		t.Errorf("bad clocking, too few gorouinte number: %v", n)
	}
}
