package git

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// applyScript replays a FileDiff's hunks over the old text and returns what
// they say the new text is. It is the round-trip check both the table test and
// the fuzz target measure LineDiff against: a hunk set that cannot rebuild the
// new text describes some other pair of files.
func applyScript(oldText string, fd *FileDiff) (string, error) {
	oldLines := strings.Split(oldText, "\n")
	var out []string
	consumed := 0 // old lines already emitted or dropped
	for hi, h := range fd.Hunks {
		start := h.OldStart - 1
		if start < consumed || start > len(oldLines) {
			return "", fmt.Errorf("hunk %d starts at old line %d, past %d already consumed", hi, h.OldStart, consumed)
		}
		out = append(out, oldLines[consumed:start]...)
		consumed = start
		for _, dl := range h.Lines {
			switch dl.Type {
			case DiffLineContext:
				if consumed >= len(oldLines) || oldLines[consumed] != dl.Content {
					return "", fmt.Errorf("hunk %d context %q does not match old line %d", hi, dl.Content, consumed+1)
				}
				out = append(out, dl.Content)
				consumed++
			case DiffLineDelete:
				if consumed >= len(oldLines) || oldLines[consumed] != dl.Content {
					return "", fmt.Errorf("hunk %d deletes %q which is not old line %d", hi, dl.Content, consumed+1)
				}
				consumed++
			case DiffLineAdd:
				out = append(out, dl.Content)
			}
		}
	}
	out = append(out, oldLines[consumed:]...)
	return strings.Join(out, "\n"), nil
}

func TestLineDiff(t *testing.T) {
	tests := []struct {
		name      string
		old, new  string
		wantHunks int
		// wantStatus is checked for the listed new-file lines; the value is
		// what LineStatus must report there.
		wantStatus map[int]rune
	}{
		{
			name: "identical",
			old:  "a\nb\nc\n", new: "a\nb\nc\n",
			wantHunks: 0,
		},
		{
			name: "both empty",
			old:  "", new: "",
			wantHunks: 0,
		},
		{
			name: "insert only",
			old:  "a\nb\nc\n", new: "a\nb\nX\nY\nc\n",
			wantHunks:  1,
			wantStatus: map[int]rune{1: ' ', 3: '+', 4: '+', 5: ' '},
		},
		{
			name: "delete only",
			old:  "a\nb\nc\nd\n", new: "a\nd\n",
			wantHunks: 1,
			// b and c are gone; the marker lands on the survivor after them.
			wantStatus: map[int]rune{1: ' ', 2: '-'},
		},
		{
			name: "replace",
			old:  "a\nb\nc\n", new: "a\nB\nc\n",
			wantHunks:  1,
			wantStatus: map[int]rune{1: ' ', 2: '~', 3: ' '},
		},
		{
			name: "old empty",
			old:  "", new: "a\nb\n",
			wantHunks:  1,
			wantStatus: map[int]rune{1: '+', 2: '+'},
		},
		{
			name: "new empty",
			old:  "a\nb\n", new: "",
			wantHunks: 1,
			// Only the empty final line survives, and it carries the marker.
			wantStatus: map[int]rune{1: '-'},
		},
		{
			name: "no trailing newline on the new side",
			old:  "a\nb\n", new: "a\nb",
			wantHunks: 1,
			// The old text's empty final line is gone.
			wantStatus: map[int]rune{1: ' ', 2: '-'},
		},
		{
			name: "no trailing newline on the old side",
			old:  "a\nb", new: "a\nb\n",
			wantHunks:  1,
			wantStatus: map[int]rune{1: ' ', 2: ' ', 3: '+'},
		},
		{
			name:       "two changes six lines apart share one hunk",
			old:        numbered(1, 30),
			new:        strings.Replace(strings.Replace(numbered(1, 30), "l10\n", "X\n", 1), "l17\n", "Y\n", 1),
			wantHunks:  1,
			wantStatus: map[int]rune{10: '~', 17: '~'},
		},
		{
			name:       "two changes seven lines apart split into two hunks",
			old:        numbered(1, 30),
			new:        strings.Replace(strings.Replace(numbered(1, 30), "l10\n", "X\n", 1), "l18\n", "Y\n", 1),
			wantHunks:  2,
			wantStatus: map[int]rune{10: '~', 18: '~'},
		},
		{
			name:       "one change in two thousand lines is one hunk",
			old:        numbered(1, 2000),
			new:        strings.Replace(numbered(1, 2000), "l1000\n", "changed\n", 1),
			wantHunks:  1,
			wantStatus: map[int]rune{999: ' ', 1000: '~', 1001: ' '},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := LineDiff(tt.old, tt.new)
			if len(fd.Hunks) != tt.wantHunks {
				t.Errorf("LineDiff produced %d hunks, want %d", len(fd.Hunks), tt.wantHunks)
				for _, h := range fd.Hunks {
					t.Logf("  %s", h.Header)
				}
			}
			got, err := applyScript(tt.old, fd)
			if err != nil {
				t.Fatalf("applying the hunks to the old text: %v", err)
			}
			if got != tt.new {
				t.Errorf("hunks applied to old = %q, want %q", got, tt.new)
			}
			for line, want := range tt.wantStatus {
				if s := fd.LineStatus(line); s != want {
					t.Errorf("LineStatus(%d) = %q, want %q", line, s, want)
				}
			}
		})
	}
}

// A pair with nothing in common exceeds the Myers distance cap; the result
// still has to rebuild the new text.
func TestLineDiffBeyondDistanceCapStillRoundTrips(t *testing.T) {
	old := numbered(1, lineDiffMaxDistance)
	var b strings.Builder
	for i := 1; i <= lineDiffMaxDistance; i++ {
		fmt.Fprintf(&b, "different %d\n", i)
	}
	fd := LineDiff(old, b.String())
	if len(fd.Hunks) != 1 {
		t.Errorf("LineDiff produced %d hunks, want 1 wholesale replacement", len(fd.Hunks))
	}
	got, err := applyScript(old, fd)
	if err != nil {
		t.Fatalf("applying the hunks to the old text: %v", err)
	}
	if got != b.String() {
		t.Error("the fallback hunk does not rebuild the new text")
	}
}

func numbered(from, to int) string {
	var b strings.Builder
	for i := from; i <= to; i++ {
		fmt.Fprintf(&b, "l%d\n", i)
	}
	return b.String()
}

func FuzzLineDiff(f *testing.F) {
	seeds := [][2]string{
		{"", ""},
		{"a\nb\nc\n", "a\nb\nc\n"},
		{"a\nb\nc\n", "a\nX\nc\n"},
		{"a\nb\nc\n", ""},
		{"", "a\nb\nc\n"},
		{"a\nb", "a\nb\n"},
		{"one\ntwo\nthree\nfour\nfive\nsix\nseven\n", "one\nthree\nfour\nSIX\nseven\neight\n"},
		{"\n\n\n", "\n"},
		{"\x00\xff\n", "\xff\x00\n"},
	}
	for _, seed := range seeds {
		f.Add(seed[0], seed[1])
	}

	f.Fuzz(func(t *testing.T, oldText, newText string) {
		if len(oldText) > 32<<10 || len(newText) > 32<<10 {
			t.Skip()
		}
		fd := LineDiff(oldText, newText)
		if fd == nil {
			t.Fatal("LineDiff returned nil")
		}

		// The hunks must describe this pair of texts and no other.
		rebuilt, err := applyScript(oldText, fd)
		if err != nil {
			t.Fatalf("applying the hunks to the old text: %v", err)
		}
		if rebuilt != newText {
			t.Fatalf("hunks applied to old = %q, want %q", rebuilt, newText)
		}

		// The invariants FuzzParseUnifiedDiff holds a parsed diff to apply to a
		// synthesized one as well: the derived queries must agree with the
		// hunks' own content.
		added, deleted := fd.Stats()
		wantAdded, wantDeleted := 0, 0
		var wantChanged []int
		for _, hunk := range fd.Hunks {
			newLine := hunk.NewStart
			for _, line := range hunk.Lines {
				switch line.Type {
				case DiffLineAdd:
					wantAdded++
					wantChanged = append(wantChanged, newLine)
					newLine++
				case DiffLineDelete:
					wantDeleted++
				case DiffLineContext:
					newLine++
				default:
					t.Fatalf("invalid line type %d", line.Type)
				}
			}
			if newLine != hunk.NewStart+hunk.NewCount {
				t.Fatalf("hunk %q declares %d new lines, carries %d", hunk.Header, hunk.NewCount, newLine-hunk.NewStart)
			}
		}
		if added != wantAdded || deleted != wantDeleted {
			t.Fatalf("Stats() = (%d, %d), want (%d, %d)", added, deleted, wantAdded, wantDeleted)
		}
		if changed := fd.ChangedNewLines(); !slices.Equal(changed, wantChanged) {
			t.Fatalf("ChangedNewLines() = %v, want %v", changed, wantChanged)
		}
		for _, line := range wantChanged {
			if status := fd.LineStatus(line); status != '+' && status != '~' {
				t.Fatalf("LineStatus(%d) = %q, want a changed status", line, status)
			}
		}
		// Every marker must name a line the new text actually has.
		newLineCount := len(strings.Split(newText, "\n"))
		for line := range fd.lineStatusCache {
			if line < 1 || line > newLineCount {
				t.Fatalf("status recorded for line %d, outside the new text's 1..%d", line, newLineCount)
			}
		}
	})
}
