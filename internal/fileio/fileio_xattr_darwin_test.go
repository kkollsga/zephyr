//go:build darwin

package fileio

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
)

func TestSaveFile_PreservesExistingExtendedAttributes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tagged.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	const name = "com.zephyr.test"
	want := "preserve-me"
	if output, err := exec.Command("/usr/bin/xattr", "-w", name, want, path).CombinedOutput(); err != nil {
		t.Fatalf("setting extended attribute: %v: %s", err, output)
	}

	if err := SaveFile(buffer.NewFromString("after"), path); err != nil {
		t.Fatal(err)
	}
	got, err := exec.Command("/usr/bin/xattr", "-p", name, path).Output()
	if err != nil {
		t.Fatalf("extended attribute was lost after save: %v", err)
	}
	if strings.TrimSuffix(string(got), "\n") != want {
		t.Fatalf("extended attribute = %q, want %q", got, want)
	}
}

func TestSaveFile_ThroughSymlinkUpdatesTargetWithoutReplacingLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")
	if err := os.WriteFile(target, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Fatal(err)
	}

	if err := SaveFile(buffer.NewFromString("after"), link); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("save replaced the symlink instead of updating its target")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "after"; got != want {
		t.Fatalf("symlink target content = %q, want %q", got, want)
	}
}
