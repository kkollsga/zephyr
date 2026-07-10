package main

import "testing"

func TestJSONCompactToggleUndo(t *testing.T) {
	expanded := "{\n  \"a\": 1,\n  \"b\": [\n    2,\n    3\n  ]\n}"
	st, ed, _ := testAppWithText(expanded, "JSON")
	st.indentWidth = 2

	// Expanded -> compact.
	st.toggleJSONCompact()
	if got := ed.Buffer.Text(); got != `{"a":1,"b":[2,3]}` {
		t.Fatalf("compact result = %q", got)
	}
	if !ed.Modified {
		t.Fatal("expected tab marked dirty after format")
	}

	// A single Cmd+Z reverts the whole reformat.
	ed.Undo()
	if got := ed.Buffer.Text(); got != expanded {
		t.Fatalf("undo did not restore expanded form in one step: %q", got)
	}

	// Compact -> expanded round-trips back.
	st2, ed2, _ := testAppWithText(`{"a":1,"b":[2,3]}`, "JSON")
	st2.indentWidth = 2
	st2.toggleJSONCompact()
	if got := ed2.Buffer.Text(); got != expanded {
		t.Fatalf("expand result = %q, want %q", got, expanded)
	}
}

func TestJSONCompactToggleInvalidNoOp(t *testing.T) {
	src := `{"a": }`
	st, ed, _ := testAppWithText(src, "JSON")
	st.indentWidth = 2
	st.toggleJSONCompact()
	if ed.Buffer.Text() != src || ed.Modified {
		t.Fatalf("invalid JSON was not a no-op: text=%q modified=%v", ed.Buffer.Text(), ed.Modified)
	}
}

func TestFixIndentOnEnterDedentsCloser(t *testing.T) {
	// A mis-indented closing brace is fixed when Enter is pressed on its line.
	src := "function f() {\n    return 1;\n        }"
	st, ed, _ := testAppWithText(src, "JavaScript")
	st.autoIndent = true
	st.indentWidth = 4
	// Cursor at end of the mis-indented "}" line (line index 2).
	ed.Cursor.SetPosition(ed.Buffer, 2, len("        }"))

	st.insertNewlineAutoIndent()

	line, _ := ed.Buffer.Line(2)
	if line != "}" {
		t.Fatalf("leaving line not dedented: %q", line)
	}
	// New line sits at depth 0 (after the closer) with the cursor at its start.
	if line, _ := ed.Buffer.Line(3); line != "" {
		t.Fatalf("new line = %q, want empty", line)
	}
	if ed.Cursor.Line != 3 || ed.Cursor.Col != 0 {
		t.Fatalf("cursor = %d:%d, want 3:0", ed.Cursor.Line, ed.Cursor.Col)
	}

	// Undo fully reverts (fix is a separate step from the newline insert).
	ed.Undo()
	ed.Undo()
	if got := ed.Buffer.Text(); got != src {
		t.Fatalf("undo did not restore original: %q", got)
	}
}

func TestEnterIndentsNewLineAtDepth(t *testing.T) {
	// The new line created by Enter gets the engine indent for its position,
	// not a copy of the leaving line's pre-fix whitespace.
	src := "function f() {\nif (x) {\nreturn parts.greeting;\n}\n}"
	st, ed, _ := testAppWithText(src, "JavaScript")
	st.autoIndent = true
	st.indentWidth = 2
	ed.Cursor.SetPosition(ed.Buffer, 2, len("return parts.greeting;"))

	st.insertNewlineAutoIndent()

	if line, _ := ed.Buffer.Line(2); line != "    return parts.greeting;" {
		t.Fatalf("leaving line = %q", line)
	}
	if line, _ := ed.Buffer.Line(3); line != "    " {
		t.Fatalf("new line = %q, want 4-space indent", line)
	}
	if ed.Cursor.Line != 3 || ed.Cursor.Col != 4 {
		t.Fatalf("cursor = %d:%d, want 3:4", ed.Cursor.Line, ed.Cursor.Col)
	}

	// Three undo steps: newline insert, leaving-line fix, new-line indent.
	ed.Undo()
	ed.Undo()
	ed.Undo()
	if got := ed.Buffer.Text(); got != src {
		t.Fatalf("undo did not restore original: %q", got)
	}
}

func TestEnterMidLineSplitIndentsRemainder(t *testing.T) {
	// Splitting mid-line re-indents both halves and puts the cursor after the
	// remainder line's new indentation.
	src := "if (x) {\nfoo();bar();\n}"
	st, ed, _ := testAppWithText(src, "JavaScript")
	st.autoIndent = true
	st.indentWidth = 2
	ed.Cursor.SetPosition(ed.Buffer, 1, len("foo();"))

	st.insertNewlineAutoIndent()

	if line, _ := ed.Buffer.Line(1); line != "  foo();" {
		t.Fatalf("leaving line = %q", line)
	}
	if line, _ := ed.Buffer.Line(2); line != "  bar();" {
		t.Fatalf("remainder line = %q", line)
	}
	if ed.Cursor.Line != 2 || ed.Cursor.Col != 2 {
		t.Fatalf("cursor = %d:%d, want 2:2", ed.Cursor.Line, ed.Cursor.Col)
	}

	ed.Undo()
	ed.Undo()
	ed.Undo()
	if got := ed.Buffer.Text(); got != src {
		t.Fatalf("undo did not restore original: %q", got)
	}
}

func TestFixIndentOnEnterDisabled(t *testing.T) {
	src := "function f() {\n        }"
	st, ed, _ := testAppWithText(src, "JavaScript")
	st.autoIndent = false // legacy behavior
	st.indentWidth = 4
	ed.Cursor.SetPosition(ed.Buffer, 1, len("        }"))

	st.insertNewlineAutoIndent()

	line, _ := ed.Buffer.Line(1)
	if line != "        }" {
		t.Fatalf("line was modified with autoIndent disabled: %q", line)
	}
	// Legacy path copies the leaving line's leading whitespace to the new line.
	if line, _ := ed.Buffer.Line(2); line != "        " {
		t.Fatalf("legacy new-line indent = %q", line)
	}
}

func TestFixIndentOnEnterSkipsUnsupportedLang(t *testing.T) {
	src := "def f():\n        pass"
	st, ed, _ := testAppWithText(src, "Python")
	st.autoIndent = true
	st.indentWidth = 4
	ed.Cursor.SetPosition(ed.Buffer, 1, len("        pass"))

	st.insertNewlineAutoIndent()

	line, _ := ed.Buffer.Line(1)
	if line != "        pass" {
		t.Fatalf("unsupported language line was modified: %q", line)
	}
}

func TestFixIndentOnEnterLeavesStringLineUntouched(t *testing.T) {
	// The leaving line begins inside a multi-line template literal, so its
	// leading whitespace must not be changed.
	src := "const t = `line one\n      still template`;\nx;"
	st, ed, _ := testAppWithText(src, "JavaScript")
	st.autoIndent = true
	st.indentWidth = 2
	ed.Cursor.SetPosition(ed.Buffer, 1, len("      still template`;"))

	st.insertNewlineAutoIndent()

	line, _ := ed.Buffer.Line(1)
	if line != "      still template`;" {
		t.Fatalf("template-interior line was modified: %q", line)
	}
}
