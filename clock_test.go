package lru

import (
	"runtime"
	"testing"
	"time"
)

func TestClocking(t *testing.T) {
	for i := 0; i < 10000; i++ {
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
