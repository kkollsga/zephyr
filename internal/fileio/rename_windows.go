//go:build windows

package fileio

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// Windows refuses to replace a file that another handle holds open without
// FILE_SHARE_DELETE, so a save fails outright whenever some other program
// happens to be reading the target. The block is transient — the other reader
// closes the file in milliseconds — so retry briefly instead of losing the
// user's save. Roughly half a second total: long enough to ride out a reader
// that polls the file, short enough not to freeze the UI on a genuine lock.
const (
	renameRetryAttempts = 10
	renameRetryDelay    = 50 * time.Millisecond
)

// Windows error codes for a target another handle is holding:
// ERROR_ACCESS_DENIED and ERROR_SHARING_VIOLATION. golang.org/x/sys is only an
// indirect requirement of this module, so the values are matched numerically
// rather than promoting it to a direct dependency for two constants.
const (
	errorAccessDenied     = syscall.Errno(5)
	errorSharingViolation = syscall.Errno(32)
)

// renameReplace commits the freshly written temp file over the save target,
// retrying while the target is held open by another process.
func renameReplace(oldpath, newpath string) error {
	return renameReplaceWith(os.Rename, oldpath, newpath)
}

func renameReplaceWith(rename func(string, string) error, oldpath, newpath string) error {
	var err error
	for attempt := 0; attempt < renameRetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(renameRetryDelay)
		}
		err = rename(oldpath, newpath)
		if err == nil || !isRenameSharingError(err) {
			return err
		}
	}
	return err
}

func isRenameSharingError(err error) bool {
	return errors.Is(err, errorSharingViolation) || errors.Is(err, errorAccessDenied)
}
