package git

import "testing"

func TestParseStatus_Modified(t *testing.T) {
	// " M file.go\x00"
	data := []byte(" M file.go\x00")
	statuses := ParseStatus(data)
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	s := statuses[0]
	if s.Path != "file.go" {
		t.Errorf("path = %q, want file.go", s.Path)
	}
	if s.Index != ' ' {
		t.Errorf("index = %c, want ' '", s.Index)
	}
	if s.Worktree != 'M' {
		t.Errorf("worktree = %c, want M", s.Worktree)
	}
}

func TestParseStatus_Added(t *testing.T) {
	data := []byte("A  newfile.go\x00")
	statuses := ParseStatus(data)
	if len(statuses) != 1 {
		t.Fatalf("expected 1, got %d", len(statuses))
	}
	if statuses[0].Index != 'A' {
		t.Errorf("index = %c, want A", statuses[0].Index)
	}
	if statuses[0].Worktree != ' ' {
		t.Errorf("worktree = %c, want ' '", statuses[0].Worktree)
	}
}

func TestParseStatus_Deleted(t *testing.T) {
	data := []byte("D  removed.go\x00")
	statuses := ParseStatus(data)
	if len(statuses) != 1 {
		t.Fatalf("expected 1, got %d", len(statuses))
	}
	if statuses[0].Index != 'D' {
		t.Errorf("index = %c, want D", statuses[0].Index)
	}
}

func TestParseStatus_Untracked(t *testing.T) {
	data := []byte("?? scratch.go\x00")
	statuses := ParseStatus(data)
	if len(statuses) != 1 {
		t.Fatalf("expected 1, got %d", len(statuses))
	}
	if statuses[0].Index != '?' {
		t.Errorf("index = %c, want ?", statuses[0].Index)
	}
	if statuses[0].Worktree != '?' {
		t.Errorf("worktree = %c, want ?", statuses[0].Worktree)
	}
}

func TestParseStatus_PartiallyStagedModify(t *testing.T) {
	data := []byte("MM both.go\x00")
	statuses := ParseStatus(data)
	if len(statuses) != 1 {
		t.Fatalf("expected 1, got %d", len(statuses))
	}
	if statuses[0].Index != 'M' {
		t.Errorf("index = %c, want M", statuses[0].Index)
	}
	if statuses[0].Worktree != 'M' {
		t.Errorf("worktree = %c, want M", statuses[0].Worktree)
	}
}

func TestParseStatus_Rename(t *testing.T) {
	// Rename entry: "R  new.go\x00old.go\x00"
	data := []byte("R  new.go\x00old.go\x00")
	statuses := ParseStatus(data)
	if len(statuses) != 1 {
		t.Fatalf("expected 1, got %d", len(statuses))
	}
	s := statuses[0]
	if s.Index != 'R' {
		t.Errorf("index = %c, want R", s.Index)
	}
	if s.Path != "new.go" {
		t.Errorf("path = %q, want new.go", s.Path)
	}
	if s.OrigPath != "old.go" {
		t.Errorf("origPath = %q, want old.go", s.OrigPath)
	}
}

func TestParseStatus_MultipleFiles(t *testing.T) {
	data := []byte(" M file1.go\x00A  file2.go\x00?? file3.go\x00")
	statuses := ParseStatus(data)
	if len(statuses) != 3 {
		t.Fatalf("expected 3, got %d", len(statuses))
	}
	if statuses[0].Path != "file1.go" {
		t.Errorf("path[0] = %q, want file1.go", statuses[0].Path)
	}
	if statuses[1].Path != "file2.go" {
		t.Errorf("path[1] = %q, want file2.go", statuses[1].Path)
	}
	if statuses[2].Path != "file3.go" {
		t.Errorf("path[2] = %q, want file3.go", statuses[2].Path)
	}
}

func TestParseStatus_Empty(t *testing.T) {
	statuses := ParseStatus(nil)
	if len(statuses) != 0 {
		t.Errorf("expected 0, got %d", len(statuses))
	}
	statuses = ParseStatus([]byte(""))
	if len(statuses) != 0 {
		t.Errorf("expected 0, got %d", len(statuses))
	}
}
