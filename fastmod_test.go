package lru

import (
	"testing"
)

func TestFastMod(t *testing.T) {
	cases := []struct {
		Number  uint32
		Divisor uint32
		Result  uint32
	}{
		{1, 100, 1},
		{2, 100, 2},
		{50, 100, 50},
		{99, 100, 99},
		{100, 100, 0},
		{101, 100, 1},
		{102, 100, 2},
		{1001, 100, 1},
		{1102, 100, 2},
		{2222, 100, 22},
	}

	for _, c := range cases {
		m := computemod(c.Divisor)
		if want, got := c.Result, fastmod(c.Number, m, c.Divisor); want != got {
			t.Errorf("fastmod(%v, %v) want %v, got %v", c.Number, c.Divisor, want, got)
		}
	}

}

func BenchmarkFastMod(b *testing.B) {
	d := uint32(128)
	m := computemod(d)

	b.ReportAllocs()
	b.ResetTimer()

	n := uint32(b.N)
	for i := uint32(0); i < n; i++ {
		_ = fastmod(i, m, d)
	}
}

func BenchmarkNormalMod(b *testing.B) {
	d := uint32(1000000 / 128)
	b.ReportAllocs()
	b.ResetTimer()

	n := uint32(b.N)
	for i := uint32(0); i < n; i++ {
		_ = i % d
	}
}
