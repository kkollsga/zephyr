package git

import (
	"fmt"
	"testing"
)

// buildLargeFileDiff creates a FileDiff with many hunks for benchmarking.
func buildLargeFileDiff(numHunks, linesPerHunk int) *FileDiff {
	fd := &FileDiff{Path: "bench.go", Status: 'M'}
	line := 1
	for i := 0; i < numHunks; i++ {
		h := Hunk{
			NewStart: line,
			NewCount: linesPerHunk + 2, // context + changes
		}
		// context line
		h.Lines = append(h.Lines, DiffLine{Type: DiffLineContext, Content: "ctx"})
		for j := 0; j < linesPerHunk; j++ {
			h.Lines = append(h.Lines, DiffLine{Type: DiffLineDelete, Content: "old"})
			h.Lines = append(h.Lines, DiffLine{Type: DiffLineAdd, Content: "new"})
		}
		// context line
		h.Lines = append(h.Lines, DiffLine{Type: DiffLineContext, Content: "ctx"})
		fd.Hunks = append(fd.Hunks, h)
		line += linesPerHunk + 2 + 20 // gap between hunks
	}
	return fd
}

func BenchmarkLineStatus_10hunks(b *testing.B) {
	fd := buildLargeFileDiff(10, 5)
	// Query a line in the middle
	queryLine := fd.Hunks[5].NewStart + 1
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fd.LineStatus(queryLine)
	}
}

func BenchmarkLineStatus_100hunks(b *testing.B) {
	fd := buildLargeFileDiff(100, 5)
	queryLine := fd.Hunks[50].NewStart + 1
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fd.LineStatus(queryLine)
	}
}

// BenchmarkLineStatusAllVisible simulates the render loop:
// query LineStatus for every visible line (40 lines).
func BenchmarkLineStatusAllVisible_10hunks(b *testing.B) {
	fd := buildLargeFileDiff(10, 5)
	startLine := fd.Hunks[3].NewStart
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for line := startLine; line < startLine+40; line++ {
			fd.LineStatus(line)
		}
	}
}

func BenchmarkLineStatusAllVisible_100hunks(b *testing.B) {
	fd := buildLargeFileDiff(100, 5)
	startLine := fd.Hunks[50].NewStart
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for line := startLine; line < startLine+40; line++ {
			fd.LineStatus(line)
		}
	}
}

// BenchmarkChangedNewLines benchmarks the list of changed lines.
func BenchmarkChangedNewLines_100hunks(b *testing.B) {
	fd := buildLargeFileDiff(100, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fd.ChangedNewLines()
	}
}

// BenchmarkHunkStartLines benchmarks getting hunk start lines.
func BenchmarkHunkStartLines_100hunks(b *testing.B) {
	fd := buildLargeFileDiff(100, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fd.HunkStartLines()
	}
}

// BenchmarkParseUnifiedDiff benchmarks parsing a large diff.
func BenchmarkParseUnifiedDiff(b *testing.B) {
	// Build a synthetic diff with 50 files, 3 hunks each
	var diff string
	for f := 0; f < 50; f++ {
		diff += fmt.Sprintf("diff --git a/file%d.go b/file%d.go\nindex abc..def 100644\n--- a/file%d.go\n+++ b/file%d.go\n", f, f, f, f)
		for h := 0; h < 3; h++ {
			start := h*20 + 1
			diff += fmt.Sprintf("@@ -%d,10 +%d,12 @@\n", start, start)
			diff += " context\n-old line 1\n-old line 2\n+new line 1\n+new line 2\n+new line 3\n+new line 4\n context\n context\n context\n"
		}
	}
	data := []byte(diff)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseUnifiedDiff(data)
	}
}
