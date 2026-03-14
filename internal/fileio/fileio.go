package fileio

import (
	"os"
	"path/filepath"

	"github.com/kristianweb/zephyr/internal/buffer"
)

// OpenFile reads a file and returns a PieceTable.
func OpenFile(path string) (*buffer.PieceTable, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return buffer.NewFromFile(absPath)
}

// SaveFile writes the piece table content to a file using a crash-safe
// write-to-temp-then-rename strategy.
func SaveFile(pt *buffer.PieceTable, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(absPath)
	tmp, err := os.CreateTemp(dir, ".zephyr-save-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	text := pt.Text()
	_, err = tmp.WriteString(text)
	if err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	// Preserve original file permissions if the file exists
	if info, err := os.Stat(absPath); err == nil {
		os.Chmod(tmpName, info.Mode())
	}

	return os.Rename(tmpName, absPath)
}

// FileExists returns true if the file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
