//go:build darwin && cgo

package fileio

/*
#include <errno.h>
#include <stdlib.h>
#include <sys/types.h>
#include <sys/xattr.h>

static ssize_t zephyr_listxattr(const char *path, char *names, size_t size) {
	return listxattr(path, names, size, 0);
}

static ssize_t zephyr_getxattr(const char *path, const char *name, void *value, size_t size) {
	return getxattr(path, name, value, size, 0, 0);
}

static int zephyr_setxattr(const char *path, const char *name, const void *value, size_t size) {
	return setxattr(path, name, value, size, 0, 0);
}

static int zephyr_errno(void) {
	return errno;
}
*/
import "C"

import (
	"bytes"
	"fmt"
	"syscall"
	"unsafe"
)

func preserveFileMetadata(source, destination string) error {
	cSource := C.CString(source)
	defer C.free(unsafe.Pointer(cSource))
	cDestination := C.CString(destination)
	defer C.free(unsafe.Pointer(cDestination))

	size := C.zephyr_listxattr(cSource, nil, 0)
	if size < 0 {
		return syscall.Errno(C.zephyr_errno())
	}
	if size == 0 {
		return nil
	}
	names := make([]byte, int(size))
	if C.zephyr_listxattr(cSource, (*C.char)(unsafe.Pointer(&names[0])), C.size_t(len(names))) < 0 {
		return syscall.Errno(C.zephyr_errno())
	}

	for _, name := range bytes.Split(bytes.TrimSuffix(names, []byte{0}), []byte{0}) {
		if len(name) == 0 {
			continue
		}
		cName := C.CString(string(name))
		valueSize := C.zephyr_getxattr(cSource, cName, nil, 0)
		if valueSize < 0 {
			err := syscall.Errno(C.zephyr_errno())
			C.free(unsafe.Pointer(cName))
			return fmt.Errorf("read extended attribute %q: %w", name, err)
		}
		value := make([]byte, int(valueSize))
		var valuePtr unsafe.Pointer
		if len(value) > 0 {
			valuePtr = unsafe.Pointer(&value[0])
			if C.zephyr_getxattr(cSource, cName, valuePtr, C.size_t(len(value))) < 0 {
				err := syscall.Errno(C.zephyr_errno())
				C.free(unsafe.Pointer(cName))
				return fmt.Errorf("read extended attribute %q: %w", name, err)
			}
		}
		if C.zephyr_setxattr(cDestination, cName, valuePtr, C.size_t(len(value))) != 0 {
			err := syscall.Errno(C.zephyr_errno())
			C.free(unsafe.Pointer(cName))
			return fmt.Errorf("write extended attribute %q: %w", name, err)
		}
		C.free(unsafe.Pointer(cName))
	}
	return nil
}
