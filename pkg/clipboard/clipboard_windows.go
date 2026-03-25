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

	return syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(ptr))[:])
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
	size := len(utf16) * 2 // uint16 = 2 bytes

	h, _, _ := globalAlloc.Call(gmemMoveable, uintptr(size))
	if h == 0 {
		return
	}

	ptr, _, _ := globalLock.Call(h)
	if ptr == 0 {
		globalFree.Call(h)
		return
	}

	src := unsafe.Pointer(&utf16[0])
	dst := unsafe.Pointer(ptr)
	copy((*[1 << 20]byte)(dst)[:size], (*[1 << 20]byte)(src)[:size])

	globalUnlock.Call(h)
	setClipData.Call(cfUnicodeText, h)
}
