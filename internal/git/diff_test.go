package git

import (
	"testing"
)

func TestParseUnifiedDiff_SimpleAdd(t *testing.T) {
	input := `diff --git a/newfile.go b/newfile.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/newfile.go
@@ -0,0 +1,5 @@
+package main
+
+func main() {
+	fmt.Println("hello")
+}
`
	diffs := ParseUnifiedDiff([]byte(input))
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	d := diffs[0]
	if d.Path != "newfile.go" {
		t.Errorf("path = %q, want %q", d.Path, "newfile.go")
	}
	if d.Status != 'A' {
		t.Errorf("status = %c, want A", d.Status)
	}
	if len(d.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(d.Hunks))
	}
	h := d.Hunks[0]
	if h.OldStart != 0 || h.OldCount != 0 {
		t.Errorf("old = %d,%d, want 0,0", h.OldStart, h.OldCount)
	}
	if h.NewStart != 1 || h.NewCount != 5 {
		t.Errorf("new = %d,%d, want 1,5", h.NewStart, h.NewCount)
	}
	if len(h.Lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(h.Lines))
	}
	for _, l := range h.Lines {
		if l.Type != DiffLineAdd {
			t.Errorf("line type = %d, want DiffLineAdd", l.Type)
		}
	}
}

func TestParseUnifiedDiff_SimpleDelete(t *testing.T) {
	input := `diff --git a/old.go b/old.go
deleted file mode 100644
index abc1234..0000000
--- a/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package old
-
-func deprecated() {}
`
	diffs := ParseUnifiedDiff([]byte(input))
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	d := diffs[0]
	if d.Status != 'D' {
		t.Errorf("status = %c, want D", d.Status)
	}
	if len(d.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(d.Hunks))
	}
	h := d.Hunks[0]
	if h.OldStart != 1 || h.OldCount != 3 {
		t.Errorf("old = %d,%d, want 1,3", h.OldStart, h.OldCount)
	}
	if h.NewStart != 0 || h.NewCount != 0 {
		t.Errorf("new = %d,%d, want 0,0", h.NewStart, h.NewCount)
	}
	for _, l := range h.Lines {
		if l.Type != DiffLineDelete {
			t.Errorf("line type = %d, want DiffLineDelete", l.Type)
		}
	}
}

func TestParseUnifiedDiff_ModifySingleHunk(t *testing.T) {
	input := "diff --git a/main.go b/main.go\n" +
		"index abc1234..def5678 100644\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -3,7 +3,8 @@ package main\n" +
		" import \"fmt\"\n" +
		" \n" +
		" func main() {\n" +
		"-\tfmt.Println(\"old\")\n" +
		"+\tfmt.Println(\"new\")\n" +
		"+\tfmt.Println(\"extra\")\n" +
		" }\n" +
		" \n" +
		" func helper() {}\n"
	diffs := ParseUnifiedDiff([]byte(input))
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	d := diffs[0]
	if d.Status != 'M' {
		t.Errorf("status = %c, want M", d.Status)
	}
	h := d.Hunks[0]
	if h.OldStart != 3 || h.OldCount != 7 {
		t.Errorf("old = %d,%d, want 3,7", h.OldStart, h.OldCount)
	}
	if h.NewStart != 3 || h.NewCount != 8 {
		t.Errorf("new = %d,%d, want 3,8", h.NewStart, h.NewCount)
	}

	// Count line types
	var adds, dels, ctx int
	for _, l := range h.Lines {
		switch l.Type {
		case DiffLineAdd:
			adds++
		case DiffLineDelete:
			dels++
		case DiffLineContext:
			ctx++
		}
	}
	if adds != 2 {
		t.Errorf("adds = %d, want 2", adds)
	}
	if dels != 1 {
		t.Errorf("dels = %d, want 1", dels)
	}
	if ctx != 6 {
		t.Errorf("context = %d, want 6", ctx)
	}
}

func TestParseUnifiedDiff_MultiHunk(t *testing.T) {
	input := `diff --git a/big.go b/big.go
index abc1234..def5678 100644
--- a/big.go
+++ b/big.go
@@ -1,4 +1,4 @@
 package big

-var x = 1
+var x = 2

@@ -20,4 +20,5 @@ func foo() {
 	bar()
-	baz()
+	qux()
+	extra()
 }
`
	diffs := ParseUnifiedDiff([]byte(input))
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if len(diffs[0].Hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(diffs[0].Hunks))
	}
	h1 := diffs[0].Hunks[0]
	h2 := diffs[0].Hunks[1]
	if h1.NewStart != 1 {
		t.Errorf("hunk1 NewStart = %d, want 1", h1.NewStart)
	}
	if h2.NewStart != 20 {
		t.Errorf("hunk2 NewStart = %d, want 20", h2.NewStart)
	}
}

func TestParseUnifiedDiff_MultiFile(t *testing.T) {
	input := `diff --git a/a.go b/a.go
index abc..def 100644
--- a/a.go
+++ b/a.go
@@ -1,3 +1,3 @@
 package a
-var x = 1
+var x = 2
diff --git a/b.go b/b.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/b.go
@@ -0,0 +1,2 @@
+package b
+var y = 3
`
	diffs := ParseUnifiedDiff([]byte(input))
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}
	if diffs[0].Path != "a.go" {
		t.Errorf("first path = %q, want a.go", diffs[0].Path)
	}
	if diffs[1].Path != "b.go" {
		t.Errorf("second path = %q, want b.go", diffs[1].Path)
	}
	if diffs[0].Status != 'M' {
		t.Errorf("first status = %c, want M", diffs[0].Status)
	}
	if diffs[1].Status != 'A' {
		t.Errorf("second status = %c, want A", diffs[1].Status)
	}
}

func TestParseUnifiedDiff_Rename(t *testing.T) {
	input := `diff --git a/old.go b/new.go
similarity index 95%
rename from old.go
rename to new.go
index abc..def 100644
--- a/old.go
+++ b/new.go
@@ -1,3 +1,3 @@
 package renamed
-var a = 1
+var a = 2
`
	diffs := ParseUnifiedDiff([]byte(input))
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Status != 'R' {
		t.Errorf("status = %c, want R", diffs[0].Status)
	}
	if diffs[0].Path != "new.go" {
		t.Errorf("path = %q, want new.go", diffs[0].Path)
	}
}

func TestParseUnifiedDiff_Binary(t *testing.T) {
	input := `diff --git a/image.png b/image.png
index abc..def 100644
Binary files a/image.png and b/image.png differ
`
	diffs := ParseUnifiedDiff([]byte(input))
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !diffs[0].Binary {
		t.Error("expected Binary = true")
	}
}

func TestParseUnifiedDiff_NoNewlineAtEnd(t *testing.T) {
	input := `diff --git a/f.go b/f.go
index abc..def 100644
--- a/f.go
+++ b/f.go
@@ -1,3 +1,3 @@
 package f
-var x = 1
+var x = 2
\ No newline at end of file
`
	diffs := ParseUnifiedDiff([]byte(input))
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	h := diffs[0].Hunks[0]
	// Should have 3 lines: context, delete, add (the "no newline" marker is skipped)
	if len(h.Lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(h.Lines))
	}
}

func TestParseUnifiedDiff_Empty(t *testing.T) {
	diffs := ParseUnifiedDiff(nil)
	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs, got %d", len(diffs))
	}
	diffs = ParseUnifiedDiff([]byte(""))
	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs, got %d", len(diffs))
	}
}

func TestFileDiff_LineStatus(t *testing.T) {
	// Simulate a hunk: lines 5-9 in new file
	// old: line5=context, line6=deleted, line7=context
	// new: line5=context, line6=added, line7=added, line8=context
	fd := &FileDiff{
		Path:   "test.go",
		Status: 'M',
		Hunks: []Hunk{
			{
				OldStart: 5, OldCount: 3,
				NewStart: 5, NewCount: 4,
				Lines: []DiffLine{
					{Type: DiffLineContext, Content: "context1"},
					{Type: DiffLineDelete, Content: "old line"},
					{Type: DiffLineAdd, Content: "new line"},
					{Type: DiffLineAdd, Content: "extra line"},
					{Type: DiffLineContext, Content: "context2"},
				},
			},
		},
	}

	tests := []struct {
		line int
		want rune
	}{
		{1, ' '},  // before hunk
		{5, ' '},  // context line
		{6, '~'},  // modified (add after delete)
		{7, '+'},  // pure add (no preceding delete at this point)
		{8, ' '},  // context
		{20, ' '}, // after hunk
	}

	for _, tt := range tests {
		got := fd.LineStatus(tt.line)
		if got != tt.want {
			t.Errorf("LineStatus(%d) = %c, want %c", tt.line, got, tt.want)
		}
	}
}

func TestFileDiff_ChangedNewLines(t *testing.T) {
	fd := &FileDiff{
		Hunks: []Hunk{
			{
				NewStart: 3, NewCount: 4,
				Lines: []DiffLine{
					{Type: DiffLineContext, Content: "ctx"},
					{Type: DiffLineDelete, Content: "old"},
					{Type: DiffLineAdd, Content: "new"},
					{Type: DiffLineContext, Content: "ctx"},
				},
			},
			{
				NewStart: 20, NewCount: 2,
				Lines: []DiffLine{
					{Type: DiffLineAdd, Content: "added1"},
					{Type: DiffLineAdd, Content: "added2"},
				},
			},
		},
	}

	lines := fd.ChangedNewLines()
	expected := []int{4, 20, 21}
	if len(lines) != len(expected) {
		t.Fatalf("ChangedNewLines = %v, want %v", lines, expected)
	}
	for i, v := range expected {
		if lines[i] != v {
			t.Errorf("ChangedNewLines[%d] = %d, want %d", i, lines[i], v)
		}
	}
}

func TestFileDiff_HunkAt(t *testing.T) {
	fd := &FileDiff{
		Hunks: []Hunk{
			{NewStart: 5, NewCount: 3},
			{NewStart: 20, NewCount: 5},
		},
	}

	if h := fd.HunkAt(1); h != nil {
		t.Error("HunkAt(1) should be nil")
	}
	if h := fd.HunkAt(5); h == nil || h.NewStart != 5 {
		t.Error("HunkAt(5) should return first hunk")
	}
	if h := fd.HunkAt(7); h == nil || h.NewStart != 5 {
		t.Error("HunkAt(7) should return first hunk")
	}
	if h := fd.HunkAt(8); h != nil {
		t.Error("HunkAt(8) should be nil (between hunks)")
	}
	if h := fd.HunkAt(22); h == nil || h.NewStart != 20 {
		t.Error("HunkAt(22) should return second hunk")
	}
	if h := fd.HunkAt(25); h != nil {
		t.Error("HunkAt(25) should be nil (past end)")
	}
}

func TestFileDiff_Stats(t *testing.T) {
	fd := &FileDiff{
		Hunks: []Hunk{
			{
				Lines: []DiffLine{
					{Type: DiffLineAdd},
					{Type: DiffLineAdd},
					{Type: DiffLineDelete},
					{Type: DiffLineContext},
				},
			},
			{
				Lines: []DiffLine{
					{Type: DiffLineAdd},
					{Type: DiffLineDelete},
					{Type: DiffLineDelete},
				},
			},
		},
	}
	added, deleted := fd.Stats()
	if added != 3 {
		t.Errorf("added = %d, want 3", added)
	}
	if deleted != 3 {
		t.Errorf("deleted = %d, want 3", deleted)
	}
}

func TestFileDiff_NilSafe(t *testing.T) {
	var fd *FileDiff
	if fd.LineStatus(1) != ' ' {
		t.Error("nil FileDiff.LineStatus should return ' '")
	}
	if fd.ChangedNewLines() != nil {
		t.Error("nil FileDiff.ChangedNewLines should return nil")
	}
	if fd.HunkAt(1) != nil {
		t.Error("nil FileDiff.HunkAt should return nil")
	}
}
