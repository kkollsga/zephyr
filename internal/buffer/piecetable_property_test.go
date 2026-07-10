package buffer

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func checkPieceTableModel(t testing.TB, pt *PieceTable, model string) {
	t.Helper()
	if got := pt.Text(); got != model {
		t.Fatalf("Text() = %q, want %q", got, model)
	}
	if got := pt.Length(); got != len(model) {
		t.Fatalf("Length() = %d, want %d", got, len(model))
	}

	wantLines := strings.Split(model, "\n")
	if got := pt.LineCount(); got != len(wantLines) {
		t.Fatalf("LineCount() = %d, want %d for %q", got, len(wantLines), model)
	}

	start := 0
	for line, want := range wantLines {
		if got := pt.LineStartOffset(line); got != start {
			t.Fatalf("LineStartOffset(%d) = %d, want %d", line, got, start)
		}
		if got, err := pt.Line(line); err != nil || got != want {
			t.Fatalf("Line(%d) = %q, %v; want %q, nil", line, got, err, want)
		}
		lc, err := pt.OffsetToLineCol(start)
		if err != nil || lc != (LineCol{Line: line, Col: 0}) {
			t.Fatalf("OffsetToLineCol(%d) = %+v, %v; want %d:0, nil", start, lc, err, line)
		}

		byteOffset := 0
		column := 0
		for byteOffset < len(want) {
			offset, err := pt.LineColToOffset(LineCol{Line: line, Col: column})
			if err != nil || offset != start+byteOffset {
				t.Fatalf("LineColToOffset(%d:%d) = %d, %v; want %d, nil", line, column, offset, err, start+byteOffset)
			}
			_, size := utf8.DecodeRuneInString(want[byteOffset:])
			byteOffset += size
			column++
		}
		if line+1 < len(wantLines) {
			start += len(want) + 1
		}
	}
}

func modelBytePoint(text string, offset int) (row, col int) {
	row = strings.Count(text[:offset], "\n")
	lineStart := strings.LastIndexByte(text[:offset], '\n') + 1
	return row, offset - lineStart
}

func modelInsertEdit(text string, offset int, inserted string) EditInfo {
	startRow, startCol := modelBytePoint(text, offset)
	newEndRow, newEndCol := startRow, startCol+len(inserted)
	if lastNewline := strings.LastIndexByte(inserted, '\n'); lastNewline >= 0 {
		newEndRow += strings.Count(inserted, "\n")
		newEndCol = len(inserted) - lastNewline - 1
	}
	return EditInfo{
		StartByte:  offset,
		OldEndByte: offset,
		NewEndByte: offset + len(inserted),
		StartRow:   startRow,
		StartCol:   startCol,
		OldEndRow:  startRow,
		OldEndCol:  startCol,
		NewEndRow:  newEndRow,
		NewEndCol:  newEndCol,
	}
}

func modelDeleteEdit(text string, offset, length int) EditInfo {
	startRow, startCol := modelBytePoint(text, offset)
	oldEndRow, oldEndCol := modelBytePoint(text, offset+length)
	return EditInfo{
		StartByte:  offset,
		OldEndByte: offset + length,
		NewEndByte: offset,
		StartRow:   startRow,
		StartCol:   startCol,
		OldEndRow:  oldEndRow,
		OldEndCol:  oldEndCol,
		NewEndRow:  startRow,
		NewEndCol:  startCol,
	}
}

func TestPieceTable_EditInfoRandomizedModel(t *testing.T) {
	const (
		seed       = int64(0xed17_1af0)
		operations = 500
	)
	rng := rand.New(rand.NewSource(seed))
	model := "first line\nsecond 世界\nthird\n"
	pt := NewFromString(model)
	payloads := []string{"x", "\n", "two\nlines", "世界", "🙂\nnext", "\r\n"}

	for step := 0; step < operations; step++ {
		var want EditInfo
		if len(model) == 0 || (len(model) < 2_048 && rng.Intn(2) == 0) {
			offset := rng.Intn(len(model) + 1)
			inserted := payloads[rng.Intn(len(payloads))]
			want = modelInsertEdit(model, offset, inserted)
			if err := pt.Insert(offset, inserted); err != nil {
				t.Fatalf("seed=%d step=%d Insert(%d, %q): %v", seed, step, offset, inserted, err)
			}
			model = model[:offset] + inserted + model[offset:]
		} else {
			offset := rng.Intn(len(model))
			length := 1 + rng.Intn(len(model)-offset)
			want = modelDeleteEdit(model, offset, length)
			if err := pt.Delete(offset, length); err != nil {
				t.Fatalf("seed=%d step=%d Delete(%d, %d): %v", seed, step, offset, length, err)
			}
			model = model[:offset] + model[offset+length:]
		}

		if got := pt.DrainEdits(); !reflect.DeepEqual(got, []EditInfo{want}) {
			t.Fatalf("seed=%d step=%d DrainEdits() = %+v, want [%+v]", seed, step, got, want)
		}
		if got := pt.DrainEdits(); len(got) != 0 {
			t.Fatalf("seed=%d step=%d second DrainEdits() = %+v, want empty", seed, step, got)
		}
		checkPieceTableModel(t, pt, model)
	}
}

func TestPieceTable_RandomizedEditModel(t *testing.T) {
	const (
		seed       = int64(0x5eed_cafe)
		operations = 2_000
	)
	rng := rand.New(rand.NewSource(seed))
	model := "first line\nsecond 世界\n"
	pt := NewFromString(model)
	payloads := []string{"", "x", "\n", "two\nlines", "世界", "🙂", "\x00", "\r\n"}

	for step := 0; step < operations; step++ {
		if len(model) == 0 || (len(model) < 2_048 && rng.Intn(2) == 0) {
			offset := rng.Intn(len(model) + 1)
			text := payloads[rng.Intn(len(payloads))]
			if err := pt.Insert(offset, text); err != nil {
				t.Fatalf("seed=%d step=%d Insert(%d, %q): %v", seed, step, offset, text, err)
			}
			model = model[:offset] + text + model[offset:]
		} else {
			offset := rng.Intn(len(model))
			length := 1 + rng.Intn(len(model)-offset)
			if err := pt.Delete(offset, length); err != nil {
				t.Fatalf("seed=%d step=%d Delete(%d, %d): %v", seed, step, offset, length, err)
			}
			model = model[:offset] + model[offset+length:]
		}
		checkPieceTableModel(t, pt, model)
	}
}

func FuzzPieceTableEditModel(f *testing.F) {
	f.Add([]byte("hello\nworld\n"), []byte{0, 0, 2, '\n', 'x', 1, 3, 4})
	f.Add([]byte("世界🙂\n"), []byte{0, 4, 5, 0xf0, 0x9f, 0x99, 0x82, '\n', 1, 1, 3})
	f.Add([]byte{}, []byte{0, 0, 3, 'a', '\n', 'b', 1, 0, 2})
	f.Add([]byte("\n\n\n"), []byte{1, 0, 2, 0, 1, 1, '\n'})

	f.Fuzz(func(t *testing.T, initial, operations []byte) {
		if len(initial) > 256 || len(operations) > 512 {
			t.Skip()
		}
		model := string(initial)
		pt := NewFromString(model)
		checkPieceTableModel(t, pt, model)

		for i := 0; i+2 < len(operations); {
			op, position, amount := operations[i], operations[i+1], int(operations[i+2])
			i += 3
			if op&1 == 0 {
				length := amount % 9
				if i+length > len(operations) {
					length = len(operations) - i
				}
				text := string(operations[i : i+length])
				i += length
				if len(model)+len(text) > 1_024 {
					continue
				}
				offset := int(position) % (len(model) + 1)
				if err := pt.Insert(offset, text); err != nil {
					t.Fatalf("Insert(%d, %q): %v", offset, text, err)
				}
				model = model[:offset] + text + model[offset:]
			} else if len(model) > 0 {
				offset := int(position) % len(model)
				length := 1 + amount%(len(model)-offset)
				if err := pt.Delete(offset, length); err != nil {
					t.Fatalf("Delete(%d, %d): %v", offset, length, err)
				}
				model = model[:offset] + model[offset+length:]
			}
			checkPieceTableModel(t, pt, model)
		}
	})
}
