//go:build windows

package fileio

import "errors"

// MoveFileEx(REPLACE_EXISTING) is not atomic against concurrent openers, so a
// reader's open during a save can be refused with ERROR_SHARING_VIOLATION.
// SaveFile's doc comment states why the writer cannot close that window; the
// atomicity test therefore tolerates a bounded number of these and nothing
// else. Access-denied is deliberately not tolerated: on the reader's side it
// would be a real permission fault, not the replace window.
func isTransientReadError(err error) bool {
	return errors.Is(err, errorSharingViolation)
}
