package render

import "testing"

func TestDisplayColToRuneCol(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		displayCol int
		want       int
	}{
		{name: "start", line: "abc", displayCol: 0, want: 0},
		{name: "plain middle", line: "abc", displayCol: 1, want: 1},
		{name: "plain end", line: "abc", displayCol: 3, want: 3},
		{name: "tab first cell", line: "\tX", displayCol: 0, want: 0},
		{name: "tab first half", line: "\tX", displayCol: 1, want: 0},
		{name: "tab second half", line: "\tX", displayCol: 2, want: 1},
		{name: "tab last cell", line: "\tX", displayCol: 3, want: 1},
		{name: "after tab", line: "\tX", displayCol: 4, want: 1},
		{name: "after text", line: "a\tb", displayCol: 4, want: 2},
		{name: "beyond end", line: "a\tb", displayCol: 40, want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DisplayColToRuneCol(tt.line, tt.displayCol, 4); got != tt.want {
				t.Fatalf("DisplayColToRuneCol(%q, %d, 4) = %d, want %d", tt.line, tt.displayCol, got, tt.want)
			}
		})
	}
}

func TestTabColumnRoundTrip(t *testing.T) {
	line := "ab\tcd\te"
	for runeCol := 0; runeCol <= len([]rune(line)); runeCol++ {
		displayCol := RuneColToDisplayCol(line, runeCol, 4)
		if got := DisplayColToRuneCol(line, displayCol, 4); got != runeCol {
			t.Fatalf("rune col %d -> display %d -> rune %d", runeCol, displayCol, got)
		}
	}
}

func TestExpandTabsAndMatchDisplayLen(t *testing.T) {
	if got := ExpandTabs("a\tb\t", 4); got != "a   b   " {
		t.Fatalf("ExpandTabs = %q", got)
	}
	if got := ExpandTabs("plain", 4); got != "plain" {
		t.Fatalf("plain ExpandTabs = %q", got)
	}
	line := "α\t界x"
	if got := MatchDisplayLen(line, 1, len("\t界"), 4); got != 4 {
		t.Fatalf("MatchDisplayLen = %d, want 4", got)
	}
	if got := DisplayColToRuneCol("\tX", 1, 0); got != 1 {
		t.Fatalf("invalid tab size fallback = %d, want 1", got)
	}
}
