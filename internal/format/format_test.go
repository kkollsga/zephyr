package format

import "testing"

func TestJSONIndentRoundTrip(t *testing.T) {
	compact := `{"b":1,"a":2,"nested":{"x":[1,2,3]},"u":"café"}`
	indented, ok := JSONIndent([]byte(compact), "  ")
	if !ok {
		t.Fatal("JSONIndent reported invalid JSON")
	}
	// Key order must be preserved (b before a).
	got := string(indented)
	bi, ai := indexOf(got, `"b"`), indexOf(got, `"a"`)
	if bi < 0 || ai < 0 || bi > ai {
		t.Fatalf("key order not preserved:\n%s", got)
	}
	if !contains(got, "\n") {
		t.Fatalf("expected multi-line output, got %q", got)
	}
	// Round-trip back to compact must equal the original.
	back, ok := JSONCompact(indented)
	if !ok {
		t.Fatal("JSONCompact reported invalid JSON")
	}
	if string(back) != compact {
		t.Fatalf("round-trip mismatch:\n got: %s\nwant: %s", back, compact)
	}
}

func TestJSONIndentUnicodePreserved(t *testing.T) {
	// json.Indent must not re-escape or reorder; the \u escape is kept verbatim.
	src := `{"emoji":"😀","n":-1.5e10}`
	out, ok := JSONIndent([]byte(src), "    ")
	if !ok {
		t.Fatal("invalid JSON")
	}
	if !contains(string(out), `😀`) {
		t.Fatalf("unicode escape not preserved:\n%s", out)
	}
	if !contains(string(out), "-1.5e10") {
		t.Fatalf("number formatting not preserved:\n%s", out)
	}
}

func TestJSONInvalidNoOp(t *testing.T) {
	if _, ok := JSONIndent([]byte(`{"a":}`), "  "); ok {
		t.Fatal("expected invalid JSON to fail")
	}
	if _, ok := JSONCompact([]byte(`not json`)); ok {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestIsSingleLine(t *testing.T) {
	if !IsSingleLine(`  {"a":1}  `) {
		t.Fatal("compact JSON should be single-line")
	}
	if IsSingleLine("{\n  \"a\": 1\n}") {
		t.Fatal("expanded JSON should not be single-line")
	}
}

func TestJSReindentBasic(t *testing.T) {
	src := "function f() {\nreturn {\na: 1,\nb: [\n1,\n2,\n],\n};\n}\n"
	want := "function f() {\n    return {\n        a: 1,\n        b: [\n            1,\n            2,\n        ],\n    };\n}\n"
	got := JSReindent(src, "    ")
	if got != want {
		t.Fatalf("reindent mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestJSReindentIdempotent(t *testing.T) {
	src := "if (x) {\n        foo();\n  if (y) {\nbar();\n}\n}\n"
	once := JSReindent(src, "  ")
	twice := JSReindent(once, "  ")
	if once != twice {
		t.Fatalf("not idempotent:\n once:\n%s\ntwice:\n%s", once, twice)
	}
}

func TestJSReindentDedentClosers(t *testing.T) {
	src := "obj = {\na: (\n1\n),\n}\n"
	want := "obj = {\n    a: (\n        1\n    ),\n}\n"
	if got := JSReindent(src, "    "); got != want {
		t.Fatalf("closer dedent mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestJSReindentStringsUntouched(t *testing.T) {
	// Braces inside strings and comments must not change nesting, and the
	// content of those lines is preserved apart from leading whitespace.
	src := "const a = {\nx: \"a { b } c\",\ny: '} } }',\n// a } comment {\nz: 1,\n}\n"
	want := "const a = {\n    x: \"a { b } c\",\n    y: '} } }',\n    // a } comment {\n    z: 1,\n}\n"
	if got := JSReindent(src, "    "); got != want {
		t.Fatalf("strings/comments miscounted:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestJSReindentTemplateLiteralUntouched(t *testing.T) {
	// The interior lines of a multi-line template literal must be left exactly
	// as-is, including their leading whitespace and any braces.
	src := "function f() {\nconst t = `line one\n  weird } indent {\n${ok}`;\nreturn t;\n}\n"
	got := JSReindent(src, "    ")
	if !contains(got, "\n  weird } indent {\n") {
		t.Fatalf("template interior line was modified:\n%s", got)
	}
	// The line after the template closes should be re-indented normally.
	if !contains(got, "\n    return t;\n") {
		t.Fatalf("post-template line not reindented:\n%s", got)
	}
}

func TestJSReindentBlockComment(t *testing.T) {
	src := "x = {\n/* a\n   } b { */\ny: 1,\n}\n"
	got := JSReindent(src, "  ")
	// Interior block-comment line untouched.
	if !contains(got, "\n   } b { */\n") {
		t.Fatalf("block comment interior modified:\n%s", got)
	}
	if !contains(got, "\n  y: 1,\n") {
		t.Fatalf("line after comment not reindented:\n%s", got)
	}
}

func TestJSReindentSwitch(t *testing.T) {
	src := "switch (x) {\ncase 1:\nfoo();\nbreak;\ndefault:\nbar();\n}\n"
	want := "switch (x) {\n    case 1:\n        foo();\n        break;\n    default:\n        bar();\n}\n"
	if got := JSReindent(src, "    "); got != want {
		t.Fatalf("switch mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestJSReindentRegexBraces(t *testing.T) {
	// A regex literal containing braces must not affect nesting depth.
	src := "const a = {\nre: /^\\{.*\\}$/,\nb: 1,\n}\n"
	want := "const a = {\n    re: /^\\{.*\\}$/,\n    b: 1,\n}\n"
	if got := JSReindent(src, "    "); got != want {
		t.Fatalf("regex miscounted:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestLineIndent(t *testing.T) {
	src := "function f() {\nreturn 1;\n}\n"
	ind, skip, ok := LineIndent(src, "  ", 1)
	if !ok || skip {
		t.Fatalf("unexpected ok=%v skip=%v", ok, skip)
	}
	if ind != "  " {
		t.Fatalf("expected 2-space indent, got %q", ind)
	}
	if _, _, ok := LineIndent(src, "  ", 99); ok {
		t.Fatal("out-of-range line should return ok=false")
	}

	// Blank lines report the indent a statement at that position would get.
	blank := "if (x) {\n\n}\n"
	ind, skip, ok = LineIndent(blank, "  ", 1)
	if !ok || skip || ind != "  " {
		t.Fatalf("blank line indent = %q skip=%v ok=%v, want %q", ind, skip, ok, "  ")
	}
	// JSReindent still strips whitespace-only lines.
	if got := JSReindent("if (x) {\n    \n}\n", "  "); got != "if (x) {\n\n}\n" {
		t.Fatalf("blank line not stripped: %q", got)
	}
}

// tiny helpers to avoid importing strings in a test that only needs these.
func contains(s, sub string) bool { return indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
