package render

import (
	"image/color"
	"reflect"
	"testing"
)

func TestComputeFoldRegionsIgnoresStringsAndComments(t *testing.T) {
	source := "func f() {\n" +
		"  quoted := \"}\"\n" +
		"  // ignored {\n" +
		"  if ok {\n" +
		"    call()\n" +
		"  }\n" +
		"},\n"

	got := ComputeFoldRegions(source)
	want := []FoldRegion{
		{StartLine: 0, EndLine: 6, OpenCol: 9, CloseChar: '}', TrailingText: ","},
		{StartLine: 3, EndLine: 5, OpenCol: 8, CloseChar: '}'},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ComputeFoldRegions() = %#v, want %#v", got, want)
	}
}

func TestFoldStateNestedMappingsAndRecursiveToggle(t *testing.T) {
	regions := []FoldRegion{
		{StartLine: 0, EndLine: 6, OpenCol: 0, CloseChar: '}'},
		{StartLine: 2, EndLine: 4, OpenCol: 2, CloseChar: '}'},
	}
	fs := NewFoldState()
	fs.SetRegions(regions, 8)

	if !fs.IsFoldStart(0) || !fs.IsFoldStart(2) || fs.IsFoldStart(1) {
		t.Fatal("fold-start index does not match regions")
	}
	fs.Toggle(2, 8)
	if !fs.HasCollapsed() || !fs.IsCollapsed(2) || !fs.IsHidden(3) || fs.IsHidden(2) {
		t.Fatal("nested fold did not collapse the expected lines")
	}
	if got := fs.DisplayToBuf(3); got != 5 {
		t.Fatalf("DisplayToBuf(3) = %d, want 5", got)
	}
	if got := fs.BufToDisplay(3); got != 2 {
		t.Fatalf("hidden BufToDisplay(3) = %d, want fold start display line 2", got)
	}
	if got := fs.ClampCursorLine(4); got != 2 {
		t.Fatalf("ClampCursorLine(4) = %d, want 2", got)
	}

	fs.ToggleRecursive(0, 8)
	if !fs.IsCollapsed(0) || !fs.IsCollapsed(2) || fs.DisplayLineCount() != 2 {
		t.Fatalf("recursive collapse state=%v visible=%d", fs.Collapsed, fs.DisplayLineCount())
	}
	fs.ToggleRecursive(0, 8)
	if fs.HasCollapsed() || fs.DisplayLineCount() != 8 {
		t.Fatalf("recursive expand state=%v visible=%d", fs.Collapsed, fs.DisplayLineCount())
	}
}

func TestFoldPresentationHelpers(t *testing.T) {
	r := &FoldRegion{StartLine: 1, EndLine: 43, OpenCol: 7, CloseChar: '}', TrailingText: ","}
	line := `"key": { contents`
	collapsed := CollapsedLineText(line, r)
	if collapsed != `"key": {...⁴²},` {
		t.Fatalf("CollapsedLineText() = %q", collapsed)
	}
	start, end, gotColor := CollapsedCountSpan(collapsed, r)
	if start != 11 || end != 13 {
		t.Fatalf("CollapsedCountSpan() = [%d,%d), want [11,13)", start, end)
	}
	if gotColor != (color.NRGBA{R: 220, G: 60, B: 60, A: 255}) {
		t.Fatalf("large fold color = %v", gotColor)
	}
	if CollapsedLineCount(nil) != 0 || CollapsedLineText(line, nil) != line {
		t.Fatal("nil fold helpers changed input")
	}
}

func TestSetRegionsDropsStaleCollapsedEntries(t *testing.T) {
	fs := NewFoldState()
	fs.SetRegions([]FoldRegion{{StartLine: 1, EndLine: 3}}, 5)
	fs.Toggle(1, 5)
	fs.SetRegions([]FoldRegion{{StartLine: 2, EndLine: 4}}, 5)
	if fs.HasCollapsed() || fs.RegionAt(1) != nil || fs.RegionAt(2) == nil {
		t.Fatalf("stale collapse survived region replacement: %#v", fs.Collapsed)
	}
}
