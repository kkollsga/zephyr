// Package benchcontrol holds benchmarks that measure nothing about Zephyr.
//
// Every other benchmark in this tree measures editor code, so a capture on its
// own cannot separate "the machine got slower" from "this change got slower":
// load moves every cell together, a real regression moves one (doctrine R11).
// These two cells are the drift meter that makes that distinction readable.
// Both are self-contained here — no internal/ package, no repository data — so
// no change to the editor can reach them.
//
//   - BenchmarkControl_Hash is a locally implemented FNV-1a over a fixed
//     64 KiB slice. It depends on nothing outside this file, not even the
//     standard library's hash package, so it moves only when the machine or the
//     compiler's code generation moves. It is the load meter.
//   - BenchmarkControl_SortInts is sort.Ints over a fixed-seed slice. It is
//     deliberately *not* immune to the Go release, which makes it the
//     toolchain-move diagnostic: both controls moving together means the
//     machine moved, this one moving alone means the standard library or the
//     compiler did. A control that moves deterministically has had its premise
//     expire, and that is a finding about the instrument rather than about the
//     machine (R11 corollary) — re-measuring returns the same number forever.
//
// Both cells sit in the microsecond range, orders of magnitude above the
// capture's per-cell noise floor, so neither is one tick away from measuring
// nothing. Neither performs a heap allocation per operation.
package benchcontrol

import (
	"sort"
	"testing"
)

const (
	controlHashBytes = 64 << 10
	controlSortLen   = 4096
)

// fnv1a is written out here rather than imported from hash/fnv so that a
// standard-library change cannot move the load meter.
func fnv1a(data []byte) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h := uint64(offset64)
	for _, c := range data {
		h ^= uint64(c)
		h *= prime64
	}
	return h
}

// nextLCG is a fixed 64-bit linear congruential step. A local generator keeps
// the input bytes identical across Go releases; math/rand's stream is stable
// today but is not part of what this control is allowed to depend on.
func nextLCG(state uint64) uint64 {
	return state*6364136223846793005 + 1442695040888963407
}

var (
	controlHashInput = func() []byte {
		b := make([]byte, controlHashBytes)
		state := uint64(0x9E3779B97F4A7C15)
		for i := range b {
			state = nextLCG(state)
			b[i] = byte(state >> 56)
		}
		return b
	}()

	controlSortInput = func() []int {
		s := make([]int, controlSortLen)
		state := uint64(0x0DDB1A5E5BAD5EED)
		for i := range s {
			state = nextLCG(state)
			s[i] = int(state >> 33)
		}
		return s
	}()

	controlHashSink uint64
	controlSortSink int
)

func BenchmarkControl_Hash(b *testing.B) {
	var h uint64
	for i := 0; i < b.N; i++ {
		h = fnv1a(controlHashInput)
	}
	controlHashSink = h
}

func BenchmarkControl_SortInts(b *testing.B) {
	buf := make([]int, len(controlSortInput))
	var sink int
	for i := 0; i < b.N; i++ {
		copy(buf, controlSortInput)
		sort.Ints(buf)
		sink += buf[0]
	}
	controlSortSink = sink
}
