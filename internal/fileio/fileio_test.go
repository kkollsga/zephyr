package fileio

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
)

func TestOpenFile_ValidFile(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "small.txt")
	pt, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pt.Length() == 0 {
		t.Fatal("expected non-empty file")
	}
}

func TestOpenFile_NonExistent_Error(t *testing.T) {
	_, err := OpenFile("/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestSaveFile_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	pt := buffer.NewFromString("hello world\n")
	if err := SaveFile(pt, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world\n" {
		t.Fatalf("got %q", string(data))
	}
}

func TestSaveFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	os.WriteFile(path, []byte("old content"), 0644)

	pt := buffer.NewFromString("new content")
	if err := SaveFile(pt, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new content" {
		t.Fatalf("got %q", string(data))
	}
}

func TestSaveFile_PreservesNewlineStyle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "newlines.txt")
	content := "line1\nline2\nline3\n"
	pt := buffer.NewFromString(content)
	if err := SaveFile(pt, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("got %q, want %q", string(data), content)
	}
}

func TestSaveFile_PreservesExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "executable.sh")
	if err := os.WriteFile(path, []byte("old"), 0751); err != nil {
		t.Fatal(err)
	}

	if err := SaveFile(buffer.NewFromString("new"), path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0751); got != want {
		t.Fatalf("permissions = %04o, want %04o", got, want)
	}
	assertNoSaveTemps(t, dir)
}

func TestSaveFile_RenameFailurePreservesTargetAndCleansTemp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	err := SaveFile(buffer.NewFromString("replacement"), target)
	if err == nil {
		t.Fatal("expected rename failure when target is a non-empty directory")
	}
	data, readErr := os.ReadFile(marker)
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("target changed after failed save: data=%q err=%v", data, readErr)
	}
	assertNoSaveTemps(t, dir)
}

func TestSaveFile_MissingParentDoesNotCreateArtifacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "file.txt")
	if err := SaveFile(buffer.NewFromString("content"), path); err == nil {
		t.Fatal("expected save into missing parent to fail")
	}
	if FileExists(path) {
		t.Fatal("failed save created the target")
	}
	assertNoSaveTemps(t, dir)
}

func TestSaveFile_PermissionPreservationFailureKeepsOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "permissions.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("chmod failed")
	ops := defaultSaveFileOps
	ops.chmod = func(string, os.FileMode) error { return wantErr }

	err := saveFileWithOps(buffer.NewFromString("replacement"), path, ops)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "original" {
		t.Fatalf("original changed after permission failure: data=%q err=%v", data, readErr)
	}
	assertNoSaveTemps(t, dir)
}

func TestSaveFile_ExtendedMetadataFailureKeepsOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("metadata failed")
	ops := defaultSaveFileOps
	ops.preserveMetadata = func(string, string) error { return wantErr }

	err := saveFileWithOps(buffer.NewFromString("replacement"), path, ops)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "original" {
		t.Fatalf("original changed after metadata failure: data=%q err=%v", data, readErr)
	}
	assertNoSaveTemps(t, dir)
}

func TestSaveFile_DirectorySyncFailureReportsCommittedSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "directory-sync.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("directory sync failed")
	ops := defaultSaveFileOps
	ops.syncDir = func(gotDir string) error {
		if gotDir != dir {
			t.Fatalf("sync directory = %q, want %q", gotDir, dir)
		}
		return wantErr
	}

	err := saveFileWithOps(buffer.NewFromString("replacement"), path, ops)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "replacement" {
		t.Fatalf("renamed content = %q err=%v; save should be committed before directory sync", data, readErr)
	}
	assertNoSaveTemps(t, dir)
}

func TestSaveFile_BrokenSymlinkFailsWithoutReplacingLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires elevated privileges on Windows")
	}
	dir := t.TempDir()
	link := filepath.Join(dir, "broken-link.txt")
	if err := os.Symlink(filepath.Join(dir, "missing-target.txt"), link); err != nil {
		t.Fatal(err)
	}

	if err := SaveFile(buffer.NewFromString("replacement"), link); err == nil {
		t.Fatal("expected save through a broken symlink to fail")
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("failed save replaced the broken symlink")
	}
	assertNoSaveTemps(t, dir)
}

func TestSaveFile_OverwriteIsAtomicForReaders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.txt")
	oldContent := bytes.Repeat([]byte("old-content\n"), 20_000)
	newContent := bytes.Repeat([]byte("new-content\n"), 20_000)
	if err := os.WriteFile(path, oldContent, 0644); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	errCh := make(chan error, 1)
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			data, err := os.ReadFile(path)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			if !bytes.Equal(data, oldContent) && !bytes.Equal(data, newContent) {
				select {
				case errCh <- &partialReadError{size: len(data)}:
				default:
				}
				return
			}
		}
	}()

	err := SaveFile(buffer.NewFromString(string(newContent)), path)
	close(done)
	readers.Wait()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
	assertNoSaveTemps(t, dir)
}

type partialReadError struct {
	size int
}

func (e *partialReadError) Error() string {
	return "reader observed partial save of size " + fmt.Sprint(e.size)
}

func assertNoSaveTemps(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".zephyr-save-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("save left temporary files: %v", matches)
	}
}
