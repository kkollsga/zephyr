//go:build !windows

package fileio

import "os"

// renameReplace commits the freshly written temp file over the save target.
// Unix renames an open file without complaint, so no retry is needed here; the
// Windows sibling carries the sharing-violation retry.
func renameReplace(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
