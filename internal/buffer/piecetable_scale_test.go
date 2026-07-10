package buffer

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

const benchmarkDocumentLine = "0123456789 abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ\n"

var benchmarkPieceTableSink *PieceTable

func buildSizedBenchmarkDocument(size int) string {
	var text strings.Builder
	text.Grow(size)
	for text.Len()+len(benchmarkDocumentLine) <= size {
		text.WriteString(benchmarkDocumentLine)
	}
	remaining := size - text.Len()
	if remaining > 0 {
		text.WriteString(benchmarkDocumentLine[:remaining])
	}
	return text.String()
}

func TestPieceTable_LargeConstructionAllocationGuard(t *testing.T) {
	text := buildSizedBenchmarkDocument(1 << 20)
	allocations := testing.AllocsPerRun(10, func() {
		benchmarkPieceTableSink = NewFromString(text)
	})
	if allocations > 5 {
		t.Fatalf("NewFromString(1 MiB) made %.0f allocations, want at most 5", allocations)
	}
	const runs = 32
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < runs; i++ {
		benchmarkPieceTableSink = NewFromString(text)
	}
	runtime.ReadMemStats(&after)
	bytes := (after.TotalAlloc - before.TotalAlloc) / runs
	if bytes > 150<<10 {
		t.Fatalf("NewFromString(1 MiB) allocated %d bytes/op, want at most %d", bytes, 150<<10)
	}
}

func TestPieceTable_EditUndoSequencePreservesLineIndex(t *testing.T) {
	const original = "first line\nsecond 世界\nthird line\n"
	pt := NewFromString(original)
	wantLineStarts := slices.Clone(pt.lineStarts)

	for _, text := range []string{"x", "two\nlines", "🙂", "\n", "\r\n"} {
		for _, offset := range []int{0, pt.Length() / 2, pt.Length()} {
			if err := pt.Insert(offset, text); err != nil {
				t.Fatalf("Insert(%d, %q): %v", offset, text, err)
			}
			if err := pt.Delete(offset, len(text)); err != nil {
				t.Fatalf("Delete(%d, %d): %v", offset, len(text), err)
			}
			if got := pt.Text(); got != original {
				t.Fatalf("after undo-adjacent edit, Text() = %q, want %q", got, original)
			}
			if !slices.Equal(pt.lineStarts, wantLineStarts) {
				t.Fatalf("after undo-adjacent edit, line starts = %v, want %v", pt.lineStarts, wantLineStarts)
			}
		}
	}
}

// BenchmarkPieceTable_OpenBySize measures construction and eager line indexing.
func BenchmarkPieceTable_OpenBySize(b *testing.B) {
	for _, size := range []struct {
		name  string
		bytes int
	}{
		{name: "1MiB", bytes: 1 << 20},
		{name: "10MiB", bytes: 10 << 20},
		{name: "100MiB", bytes: 100 << 20},
	} {
		b.Run(size.name, func(b *testing.B) {
			text := buildSizedBenchmarkDocument(size.bytes)
			b.ReportAllocs()
			b.SetBytes(int64(size.bytes))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkPieceTableSink = NewFromString(text)
			}
		})
	}
}

// BenchmarkPieceTable_EditUndoBySize measures a steady-state insert/delete pair
// on an already-open buffer. The table is warmed and edit metadata is drained
// after every pair, while the append buffer is reserved before timing, so B/op
// describes transient piece-edit allocations rather than session growth.
func BenchmarkPieceTable_EditUndoBySize(b *testing.B) {
	for _, size := range []struct {
		name  string
		bytes int
	}{
		{name: "1MiB", bytes: 1 << 20},
		{name: "10MiB", bytes: 10 << 20},
		{name: "100MiB", bytes: 100 << 20},
	} {
		b.Run(size.name, func(b *testing.B) {
			document := buildSizedBenchmarkDocument(size.bytes)
			pt := NewFromString(document)
			offset := pt.Length() / 2
			payload := "typed-edit"
			if err := pt.Insert(offset, payload); err != nil {
				b.Fatal(err)
			}
			if err := pt.Delete(offset, len(payload)); err != nil {
				b.Fatal(err)
			}
			_ = pt.DrainEdits()
			pt.add.Grow(b.N * len(payload))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := pt.Insert(offset, payload); err != nil {
					b.Fatal(err)
				}
				if err := pt.Delete(offset, len(payload)); err != nil {
					b.Fatal(err)
				}
				_ = pt.DrainEdits()
			}
			b.StopTimer()
			if got := pt.Length(); got != len(document) {
				b.Fatalf("Length() = %d after edit/undo pairs, want %d", got, len(document))
			}
			b.ReportMetric(2, "edits/op")
		})
	}
}
