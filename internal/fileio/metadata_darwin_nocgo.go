//go:build darwin && !cgo

package fileio

import (
	"fmt"
	"os/exec"
	"strings"
	"unicode"
)

func preserveFileMetadata(source, destination string) error {
	output, err := exec.Command("/usr/bin/xattr", source).Output()
	if err != nil {
		return fmt.Errorf("list extended attributes: %w", err)
	}
	for _, name := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if name == "" {
			continue
		}
		hexValue, err := exec.Command("/usr/bin/xattr", "-px", name, source).Output()
		if err != nil {
			return fmt.Errorf("read extended attribute %q: %w", name, err)
		}
		compactHex := strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, string(hexValue))
		if output, err := exec.Command("/usr/bin/xattr", "-wx", name, compactHex, destination).CombinedOutput(); err != nil {
			return fmt.Errorf("write extended attribute %q: %w: %s", name, err, output)
		}
	}
	return nil
}
