package fileio

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kristianweb/zephyr/internal/buffer"
)

type saveFileOps struct {
	createTemp       func(string, string) (*os.File, error)
	syncFile         func(*os.File) error
	closeFile        func(*os.File) error
	chmod            func(string, os.FileMode) error
	preserveMetadata func(string, string) error
	rename           func(string, string) error
	remove           func(string) error
	syncDir          func(string) error
	lstat            func(string) (os.FileInfo, error)
	evalSymlinks     func(string) (string, error)
}

var defaultSaveFileOps = saveFileOps{
	createTemp: os.CreateTemp,
	syncFile: func(file *os.File) error {
		return file.Sync()
	},
	closeFile: func(file *os.File) error {
		return file.Close()
	},
	chmod:            os.Chmod,
	preserveMetadata: preserveFileMetadata,
	rename:           os.Rename,
	remove:           os.Remove,
	syncDir:          syncParentDir,
	lstat:            os.Lstat,
	evalSymlinks:     filepath.EvalSymlinks,
}

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
	return saveFileWithOps(pt, path, defaultSaveFileOps)
}

func saveFileWithOps(pt *buffer.PieceTable, path string, ops saveFileOps) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	targetPath, err := resolveSavePath(absPath, ops)
	if err != nil {
		return err
	}

	dir := filepath.Dir(targetPath)
	tmp, err := ops.createTemp(dir, ".zephyr-save-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = ops.closeFile(tmp)
			_ = ops.remove(tmpName)
		}
	}()

	_, err = pt.WriteTo(tmp)
	if err != nil {
		return err
	}

	if err := ops.syncFile(tmp); err != nil {
		return err
	}

	if err := ops.closeFile(tmp); err != nil {
		return err
	}

	info, err := ops.lstat(targetPath)
	if err == nil {
		if err := ops.chmod(tmpName, info.Mode()); err != nil {
			return fmt.Errorf("preserve permissions: %w", err)
		}
		if err := ops.preserveMetadata(targetPath, tmpName); err != nil {
			return fmt.Errorf("preserve metadata: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect save target: %w", err)
	}

	if err := ops.rename(tmpName, targetPath); err != nil {
		return err
	}
	cleanup = false
	if err := ops.syncDir(dir); err != nil {
		return fmt.Errorf("sync save directory: %w", err)
	}
	return nil
}

func resolveSavePath(absPath string, ops saveFileOps) (string, error) {
	info, err := ops.lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return absPath, nil
		}
		return "", fmt.Errorf("inspect save path: %w", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return absPath, nil
	}
	target, err := ops.evalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("resolve save symlink: %w", err)
	}
	return target, nil
}

// FileExists returns true if the file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
