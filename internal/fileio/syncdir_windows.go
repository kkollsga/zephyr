//go:build windows

package fileio

// Windows does not expose a portable directory handle that os.File.Sync can
// flush. The file itself is synced before Replace/Rename, which is the strongest
// durability guarantee available through the standard library on Windows.
func syncParentDir(string) error {
	return nil
}
