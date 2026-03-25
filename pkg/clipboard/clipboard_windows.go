//go:build windows

package clipboard

import (
	"syscall"
	"unsafe"
)

var (
	user32         = syscall.NewLazyDLL("user32.dll")
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	openClipboard  = user32.NewProc("OpenClipboard")
	closeClipboard = user32.NewProc("CloseClipboard")
	getClipData    = user32.NewProc("GetClipboardData")
	setClipData    = user32.NewProc("SetClipboardData")
	emptyClipboard = user32.NewProc("EmptyClipboard")
	globalAlloc    = kernel32.NewProc("GlobalAlloc")
	globalFree     = kernel32.NewProc("GlobalFree")
	globalLock     = kernel32.NewProc("GlobalLock")
	globalUnlock   = kernel32.NewProc("GlobalUnlock")
	globalSize     = kernel32.NewProc("GlobalSize")
	rtlMoveMemory  = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// Get returns the current clipboard text content.
func Get() string {
	r, _, _ := openClipboard.Call(0)
	if r == 0 {
		return ""
	}
	defer closeClipboard.Call()

	h, _, _ := getClipData.Call(cfUnicodeText)
	if h == 0 {
		return ""
	}

	ptr, _, _ := globalLock.Call(h)
	if ptr == 0 {
		return ""
	}
	defer globalUnlock.Call(h)

	// Get the size of the global memory block in bytes.
	sz, _, _ := globalSize.Call(h)
	if sz == 0 || sz < 2 {
		return ""
	}

	// Copy the data into a Go-managed buffer.
	n := int(sz) / 2 // number of uint16s
	buf := make([]uint16, n)
	rtlMoveMemory.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		ptr,
		sz,
	)

	return syscall.UTF16ToString(buf)
}

// Set sets the clipboard text content.
func Set(text string) {
	r, _, _ := openClipboard.Call(0)
	if r == 0 {
		return
	}
	defer closeClipboard.Call()

	emptyClipboard.Call()

	utf16, _ := syscall.UTF16FromString(text)
	size := uintptr(len(utf16) * 2)

	h, _, _ := globalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return
	}

	ptr, _, _ := globalLock.Call(h)
	if ptr == 0 {
		globalFree.Call(h)
		return
	}

	// Copy Go slice into the global memory block via RtlMoveMemory.
	rtlMoveMemory.Call(
		ptr,
		uintptr(unsafe.Pointer(&utf16[0])),
		size,
	)

	globalUnlock.Call(h)
	setClipData.Call(cfUnicodeText, h)
}
