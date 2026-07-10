package git

import (
	"reflect"
	"slices"
	"testing"
)

func FuzzParseUnifiedDiff(f *testing.F) {
	seeds := []string{
		"",
		"diff --git a/a b/a\nindex 1..2 100644\n--- a/a\n+++ b/a\n@@ -1 +1 @@\n-old\n+new\n",
		"diff --git a/new b/new\nnew file mode 100644\n@@ -0,0 +1,2 @@\n+one\n+two\n",
		"diff --git a/old b/old\ndeleted file mode 100644\n@@ -1,2 +0,0 @@\n-one\n-two\n\\ No newline at end of file\n",
		"diff --git a/old name b/new name\nsimilarity index 100%\nrename from old name\nrename to new name\n",
		"diff --git a/image b/image\nBinary files a/image and b/image differ\n",
		"noise\x00\xff\ndiff --git a/x b/x\n@@ -999999999999999999999 +0,0 @@\n+x\n",
		"diff --git a/x b/x\n@@ -9 +0,9999999999999999990 @@\n+x\n",
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64<<10 {
			t.Skip()
		}
		got := ParseUnifiedDiff(data)
		again := ParseUnifiedDiff(data)
		if !reflect.DeepEqual(got, again) {
			t.Fatal("ParseUnifiedDiff is not deterministic")
		}

		for fileIndex := range got {
			fd := &got[fileIndex]
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
						t.Fatalf("file %d contains invalid line type %d", fileIndex, line.Type)
					}
				}
			}
			if added != wantAdded || deleted != wantDeleted {
				t.Fatalf("file %d Stats() = (%d, %d), want (%d, %d)", fileIndex, added, deleted, wantAdded, wantDeleted)
			}
			if changed := fd.ChangedNewLines(); !slices.Equal(changed, wantChanged) {
				t.Fatalf("file %d ChangedNewLines() = %v, want %v", fileIndex, changed, wantChanged)
			}
			for _, line := range wantChanged {
				status := fd.LineStatus(line)
				if status != '+' && status != '~' {
					t.Fatalf("file %d LineStatus(%d) = %q, want changed status", fileIndex, line, status)
				}
			}
		}
	})
}

func TestParseUnifiedDiff_LargeNewCountDoesNotPanic(t *testing.T) {
	const input = "diff --git a/x b/x\n@@ -9 +0,9999999999999999990 @@\n+x\n"
	diffs := ParseUnifiedDiff([]byte(input))
	if len(diffs) != 1 {
		t.Fatalf("ParseUnifiedDiff returned %d files, want 1", len(diffs))
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("ChangedNewLines panicked for an oversized hunk count: %v", recovered)
		}
	}()
	_ = diffs[0].ChangedNewLines()
}
