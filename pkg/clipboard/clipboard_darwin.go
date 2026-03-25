//go:build darwin

package clipboard

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#include <stdlib.h>

const char* getClipboard() {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSString *str = [pb stringForType:NSPasteboardTypeString];
    if (str == nil) return NULL;
    return [str UTF8String];
}

void setClipboard(const char* text) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    [pb setString:[NSString stringWithUTF8String:text] forType:NSPasteboardTypeString];
}
*/
import "C"
import "unsafe"

// Get returns the current clipboard text content.
func Get() string {
	cstr := C.getClipboard()
	if cstr == nil {
		return ""
	}
	return C.GoString(cstr)
}

// Set sets the clipboard text content.
func Set(text string) {
	cstr := C.CString(text)
	defer C.free(unsafe.Pointer(cstr))
	C.setClipboard(cstr)
}
