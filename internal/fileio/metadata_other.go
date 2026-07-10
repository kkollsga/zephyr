//go:build !darwin

package fileio

func preserveFileMetadata(_, _ string) error {
	return nil
}
