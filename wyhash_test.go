package lru

import (
	"strings"
	"testing"
)

func TestWyhashHashInputLengths(t *testing.T) {
	const seed = uint64(0x123456789abcdef0)

	lengths := []int{
		1, 3, 4, 8, 9, 16, 17, 24, 25, 32,
		33, 36, 42, 50, 58, 65, 97, 129, 161, 193,
		225, 241, 250, 270, 300,
	}
	seen := make(map[uint64]int, len(lengths))

	for _, n := range lengths {
		input := strings.Repeat("a", n)
		got := wyhash_hash(input, seed)
		if repeat := wyhash_hash(input, seed); repeat != got {
			t.Fatalf("hash should be deterministic for length %d: got=%#x repeat=%#x", n, got, repeat)
		}
		if otherSeed := wyhash_hash(input, seed+1); otherSeed == got {
			t.Fatalf("hash should change when seed changes for length %d: hash=%#x", n, got)
		}

		tweaked := input[:n-1] + "b"
		if otherInput := wyhash_hash(tweaked, seed); otherInput == got {
			t.Fatalf("hash should change when input changes for length %d: hash=%#x", n, got)
		}
		if prev, ok := seen[got]; ok {
			t.Fatalf("unexpected collision between lengths %d and %d: hash=%#x", prev, n, got)
		}
		seen[got] = n
	}
}

func TestWyhashHashBytesEmptyUsesSeed(t *testing.T) {
	const seed = uint64(0xfeedfacecafebeef)

	if got := wyhashHashbytes(nil, seed); got != seed {
		t.Fatalf("nil byte slice should return seed: got=%#x want=%#x", got, seed)
	}
	if got := wyhashHashbytes([]byte{}, seed); got != seed {
		t.Fatalf("empty byte slice should return seed: got=%#x want=%#x", got, seed)
	}
}
