package navigator

import (
	"strings"
	"testing"

	"github.com/kristianweb/zephyr/internal/git"
)

func makeTestStatusBuffer() *StatusBuffer {
	return &StatusBuffer{
		Branch:   "main",
		Hash:     "abc1234",
		Upstream: "origin/main",
		Ahead:    2,
		Behind:   0,
		Sections: []StatusSection{
			{
				Kind:  SectionUnstaged,
				Title: "Unstaged changes (2)  +15 -3",
				Entries: []StatusEntry{
					{Path: "main.go", Status: 'M', Added: 10, Deleted: 2},
					{Path: "util.go", Status: 'M', Added: 5, Deleted: 1},
				},
			},
			{
				Kind:  SectionStaged,
				Title: "Staged changes (1)  +20",
				Entries: []StatusEntry{
					{Path: "new.go", Status: 'A', Added: 20, Deleted: 0},
				},
			},
			{
				Kind:  SectionUntracked,
				Title: "Untracked files (1)",
				Entries: []StatusEntry{
					{Path: "scratch.go", Status: '?'},
				},
			},
		},
	}
}

func TestStatusBuffer_GenerateText(t *testing.T) {
	sb := makeTestStatusBuffer()
	text := sb.GenerateText()

	// Check header
	if !strings.Contains(text, "Head:     main (abc1234)") {
		t.Error("expected Head line")
	}
	if !strings.Contains(text, "Upstream: origin/main (ahead 2, behind 0)") {
		t.Error("expected Upstream line")
	}

	// Check sections
	if !strings.Contains(text, "Unstaged changes (2)") {
		t.Error("expected Unstaged section header")
	}
	if !strings.Contains(text, "Staged changes (1)") {
		t.Error("expected Staged section header")
	}
	if !strings.Contains(text, "Untracked files (1)") {
		t.Error("expected Untracked section header")
	}

	// Check entries
	if !strings.Contains(text, "M  main.go") {
		t.Error("expected main.go entry")
	}
	if !strings.Contains(text, "A  new.go") {
		t.Error("expected new.go entry")
	}
	if !strings.Contains(text, "?  scratch.go") {
		t.Error("expected scratch.go entry")
	}

	// Check stats
	if !strings.Contains(text, "+10 -2") {
		t.Error("expected diff stats for main.go")
	}
}

func TestStatusBuffer_GenerateText_Collapsed(t *testing.T) {
	sb := makeTestStatusBuffer()
	sb.Sections[0].Collapsed = true
	text := sb.GenerateText()

	// Unstaged header should exist but entries should not
	if !strings.Contains(text, "Unstaged changes (2)") {
		t.Error("section header should still appear")
	}
	if strings.Contains(text, "M  main.go") {
		t.Error("collapsed entries should not appear")
	}
	// Other sections should still be visible
	if !strings.Contains(text, "A  new.go") {
		t.Error("staged entries should still appear")
	}
}

func TestStatusBuffer_EntryAtLine(t *testing.T) {
	sb := makeTestStatusBuffer()
	sb.buildLineMap()

	text := sb.GenerateText()
	lines := strings.Split(text, "\n")

	// Find the line with "M  main.go"
	mainLine := -1
	for i, l := range lines {
		if strings.Contains(l, "M  main.go") {
			mainLine = i
			break
		}
	}
	if mainLine == -1 {
		t.Fatal("could not find main.go line")
	}

	entry, sec := sb.EntryAtLine(mainLine)
	if entry == nil {
		t.Fatal("expected entry at main.go line")
	}
	if entry.Path != "main.go" {
		t.Errorf("entry path = %q, want main.go", entry.Path)
	}
	if sec == nil || sec.Kind != SectionUnstaged {
		t.Error("expected unstaged section")
	}
}

func TestStatusBuffer_SectionAtLine(t *testing.T) {
	sb := makeTestStatusBuffer()
	sb.buildLineMap()

	text := sb.GenerateText()
	lines := strings.Split(text, "\n")

	// Find section header line
	headerLine := -1
	for i, l := range lines {
		if strings.Contains(l, "Staged changes") {
			headerLine = i
			break
		}
	}
	if headerLine == -1 {
		t.Fatal("could not find Staged section header")
	}

	sec := sb.SectionAtLine(headerLine)
	if sec == nil {
		t.Fatal("expected section")
	}
	if sec.Kind != SectionStaged {
		t.Errorf("section kind = %d, want SectionStaged", sec.Kind)
	}
}

func TestStatusBuffer_NextPrevSection(t *testing.T) {
	sb := makeTestStatusBuffer()
	sb.buildLineMap()

	text := sb.GenerateText()
	lines := strings.Split(text, "\n")

	// Find the first section header
	firstHeader := -1
	for i, l := range lines {
		if strings.Contains(l, "Unstaged changes") {
			firstHeader = i
			break
		}
	}
	if firstHeader == -1 {
		t.Fatal("could not find first section")
	}

	// Next section from first should go to staged
	next := sb.NextSection(firstHeader)
	if next <= firstHeader {
		t.Errorf("NextSection(%d) = %d, expected > %d", firstHeader, next, firstHeader)
	}
	if next < len(lines) && !strings.Contains(lines[next], "Staged changes") {
		t.Errorf("NextSection should go to Staged, got line: %q", lines[next])
	}

	// Prev section from staged should go to unstaged
	prev := sb.PrevSection(next)
	if prev != firstHeader {
		t.Errorf("PrevSection(%d) = %d, want %d", next, prev, firstHeader)
	}
}

func TestStatusBuffer_ToggleCollapse(t *testing.T) {
	sb := makeTestStatusBuffer()
	sb.buildLineMap()

	text := sb.GenerateText()
	lines := strings.Split(text, "\n")

	// Find unstaged header
	headerLine := -1
	for i, l := range lines {
		if strings.Contains(l, "Unstaged changes") {
			headerLine = i
			break
		}
	}

	// Collapse
	sb.ToggleCollapse(headerLine)
	if !sb.Sections[0].Collapsed {
		t.Error("section should be collapsed")
	}

	// Expand
	sb.ToggleCollapse(headerLine)
	if sb.Sections[0].Collapsed {
		t.Error("section should be expanded")
	}
}

func TestStatusBuffer_Empty(t *testing.T) {
	sb := &StatusBuffer{
		Branch: "main",
		Hash:   "abc",
	}
	text := sb.GenerateText()
	if !strings.Contains(text, "Head:     main (abc)") {
		t.Error("expected head line even with no changes")
	}
}

// Verify that FileStatus types from git package are used correctly
func TestStatusEntry_GitStatusTypes(t *testing.T) {
	_ = git.FileStatus{Path: "test.go", Index: 'M', Worktree: ' '}
}
