//go:build windows

package fileio

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

// Regression: a save used to fail outright when another program held the target
// open (CI saw "Access is denied" from the rename in
// TestSaveFile_OverwriteIsAtomicForReaders).
func TestRenameReplaceWith_RetriesSharingViolations(t *testing.T) {
	for _, tc := range []struct {
		name string
		code syscall.Errno
	}{
		{"sharing violation", errorSharingViolation},
		{"access denied", errorAccessDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			err := renameReplaceWith(func(string, string) error {
				calls++
				if calls < 3 {
					return &os.LinkError{Op: "rename", Err: tc.code}
				}
				return nil
			}, "old", "new")
			if err != nil {
				t.Fatalf("renameReplaceWith = %v, want nil after retries", err)
			}
			if calls != 3 {
				t.Fatalf("rename calls = %d, want 3", calls)
			}
		})
	}
}

func TestRenameReplaceWith_ReturnsOtherErrorsImmediately(t *testing.T) {
	wantErr := errors.New("not a sharing problem")
	calls := 0
	err := renameReplaceWith(func(string, string) error {
		calls++
		return wantErr
	}, "old", "new")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if calls != 1 {
		t.Fatalf("rename calls = %d, want 1; unrelated errors must not be retried", calls)
	}
}
