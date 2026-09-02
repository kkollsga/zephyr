//go:build !windows

package fileio

// rename(2) never disturbs a concurrent reader, so the atomicity test accepts
// no read failure at all off Windows.
func isTransientReadError(error) bool { return false }
