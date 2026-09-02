package fileio

import (
	"crypto/sha256"
	"io"
	"os"
	"time"
)

// Snapshot identifies a file's on-disk content at a point in time. It is
// deliberately portable: modification time, size and a content hash, with no
// inode or other platform-specific identity, so delete-and-recreate is
// detected the same way on macOS, Windows and Linux.
//
// The zero Snapshot means "no file there" and compares unequal to any
// existing file, so an absent path and an empty file stay distinguishable.
type Snapshot struct {
	Exists  bool
	ModTime time.Time
	Size    int64
	Hash    [sha256.Size]byte
}

// TakeSnapshot reads path and returns its snapshot. A missing file is not an
// error: it yields the zero Snapshot. Any other failure (a directory in the
// file's place, a permission change) is returned, since the caller must not
// treat an unreadable path as unchanged.
func TakeSnapshot(path string) (Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, nil
		}
		return Snapshot{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return Snapshot{}, err
	}
	if info.IsDir() {
		return Snapshot{}, &os.PathError{Op: "snapshot", Path: path, Err: os.ErrInvalid}
	}

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return Snapshot{}, err
	}

	snap := Snapshot{Exists: true, ModTime: info.ModTime(), Size: info.Size()}
	h.Sum(snap.Hash[:0])
	return snap, nil
}

// Equal reports whether two snapshots describe the same content. Modification
// time is carried for display but excluded from the comparison: a file
// rewritten with identical bytes is not a change the user needs to resolve,
// and coarse filesystem timestamps make mtime a poor equality test.
func (s Snapshot) Equal(other Snapshot) bool {
	return s.Exists == other.Exists && s.Size == other.Size && s.Hash == other.Hash
}

// SnapshotOfBytes returns the Snapshot an on-disk file holding b would have,
// so an in-memory buffer can be compared against disk with Equal without
// reading the file a second time. ModTime is left zero; Equal ignores it.
func SnapshotOfBytes(b []byte) Snapshot {
	return Snapshot{Exists: true, Size: int64(len(b)), Hash: sha256.Sum256(b)}
}
