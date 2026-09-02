package fileio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotDistinguishesContentAbsenceAndRecreation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := TakeSnapshot(path)
	if err != nil || !first.Exists || first.Size != 3 {
		t.Fatalf("first snapshot = %+v err=%v", first, err)
	}
	again, err := TakeSnapshot(path)
	if err != nil || !first.Equal(again) {
		t.Fatalf("unchanged file compared unequal: %+v vs %+v err=%v", first, again, err)
	}

	// Same length, different bytes: size alone would miss this.
	if err := os.WriteFile(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := TakeSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.Equal(changed) {
		t.Fatal("same-size rewrite compared equal")
	}

	// A missing file is reported, not an error, and never equals a present one.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	gone, err := TakeSnapshot(path)
	if err != nil {
		t.Fatalf("missing file returned an error: %v", err)
	}
	if gone.Exists || gone.Equal(changed) {
		t.Fatalf("deleted file snapshot = %+v", gone)
	}

	// Delete-and-recreate with the original bytes is the same content again:
	// identity is content, not inode, so this must compare equal to `first`.
	if err := os.WriteFile(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	recreated, err := TakeSnapshot(path)
	if err != nil || !recreated.Equal(first) {
		t.Fatalf("recreated snapshot = %+v err=%v", recreated, err)
	}
	if !recreated.Equal(SnapshotOfBytes([]byte("one"))) {
		t.Fatal("SnapshotOfBytes disagreed with the file on disk")
	}
}

func TestSnapshotRejectsUnreadablePath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "adirectory")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := TakeSnapshot(sub); err == nil {
		t.Fatal("a directory in the file's place was reported as a readable file")
	}
}
