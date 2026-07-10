package format

import (
	"strings"
	"testing"
)

func TestJSONErrorLine(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantLine int
		wantOK   bool
	}{
		{"valid object", `{"a": 1, "b": [2, 3]}`, 0, false},
		{"valid multiline", "{\n  \"a\": 1,\n  \"b\": 2\n}", 0, false},
		{"empty", "", 0, false},
		{"whitespace only", "   \n\t\n  ", 0, false},
		{"error at start", "}", 0, true},
		{"error mid-file", "{\n  \"a\": 1,\n  \"b\": ,\n  \"c\": 3\n}", 2, true},
		{"error at end (unclosed)", "{\n  \"a\": 1", 1, true},
		{"trailing garbage", "{\"a\":1} extra", 0, true},
		{"missing comma multiline", "{\n  \"a\": 1\n  \"b\": 2\n}", 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, ok := JSONErrorLine([]byte(tt.src))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (line=%d)", ok, tt.wantOK, line)
			}
			if ok && line != tt.wantLine {
				t.Fatalf("line = %d, want %d", line, tt.wantLine)
			}
		})
	}
}

// TestJSONErrorLinePathological verifies detection never panics on hostile input.
func TestJSONErrorLinePathological(t *testing.T) {
	cases := [][]byte{
		[]byte(strings.Repeat("a", 1_000_000)),      // huge single line
		[]byte("\x00\x01\x02\xff\xfe garbage \x00"), // binary
		[]byte("😀🎉🔥 not json 界"),                    // emoji / multibyte
		[]byte(strings.Repeat("[", 100_000)),        // deep unbalanced nesting
		{},                                          // empty slice
		[]byte("\n\n\n\n"),                          // only newlines
	}
	for i, src := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("case %d panicked: %v", i, r)
				}
			}()
			line, ok := JSONErrorLine(src)
			if ok && line < 0 {
				t.Fatalf("case %d returned negative line %d", i, line)
			}
		}()
	}
}
