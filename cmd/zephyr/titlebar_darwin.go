//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <objc/runtime.h>

static bool _titlebarDone = false;
static volatile bool _hasUnsavedChanges = false;
static volatile bool _closeRequested = false;

void setUnsavedFlag(bool val) { _hasUnsavedChanges = val; }
bool getUnsavedFlag(void)     { return _hasUnsavedChanges; }

// Check-and-reset: returns true once after the close button was clicked
// with unsaved changes, then clears the flag.
bool checkAndResetCloseRequested(void) {
	if (_closeRequested) {
		_closeRequested = false;
		return true;
	}
	return false;
}

static id _closeHelper = nil;  // forward decl for ensureButtonsVisible
static bool _mouseDownSwizzled = false;

// Override mouseDownCanMoveWindow on the Gio content view so that clicking
// in the titlebar area does NOT start a native window drag.  We handle
// window dragging ourselves for the empty tab-bar space.
static BOOL swizzled_mouseDownCanMoveWindow(id self, SEL _cmd) {
	return NO;
}

static void swizzleContentView(NSWindow *window) {
	if (_mouseDownSwizzled) return;
	NSView *cv = window.contentView;
	if (!cv) return;

	// Also override on the superview (theme frame) which is the actual
	// responder for titlebar drags.
	NSView *themeFrame = cv.superview;
	if (themeFrame) {
		Class themeClass = [themeFrame class];
		Method orig = class_getInstanceMethod(themeClass, @selector(mouseDownCanMoveWindow));
		if (orig) {
			method_setImplementation(orig, (IMP)swizzled_mouseDownCanMoveWindow);
		} else {
			class_addMethod(themeClass, @selector(mouseDownCanMoveWindow),
			                (IMP)swizzled_mouseDownCanMoveWindow, "B@:");
		}
	}

	// Override on the content view itself too.
	Class cvClass = [cv class];
	Method orig = class_getInstanceMethod(cvClass, @selector(mouseDownCanMoveWindow));
	if (orig) {
		method_setImplementation(orig, (IMP)swizzled_mouseDownCanMoveWindow);
	} else {
		class_addMethod(cvClass, @selector(mouseDownCanMoveWindow),
		                (IMP)swizzled_mouseDownCanMoveWindow, "B@:");
	}

	window.movableByWindowBackground = NO;
	_mouseDownSwizzled = true;
}

// Programmatically start a native window drag using the current event.
// Called from Go when the user drags on empty tab-bar space.
void performWindowDrag(void) {
	NSEvent *event = [NSApp currentEvent];
	if (!event) return;
	for (NSWindow *window in [NSApp windows]) {
		if ([window isVisible] && window.contentView) {
			[window performWindowDragWithEvent:event];
			return;
		}
	}
}

// Ensure the traffic light buttons are visible and composited above
// Gio's Metal layer.  Gio hides them whenever Configure() runs
// (on every title change, resize, etc.), so this is called repeatedly.
static void ensureButtonsVisible(NSWindow *window) {
	NSButton *close = [window standardWindowButton:NSWindowCloseButton];
	NSButton *mini  = [window standardWindowButton:NSWindowMiniaturizeButton];
	NSButton *zoom  = [window standardWindowButton:NSWindowZoomButton];
	if (!close) return;

	if (close.hidden) [close setHidden:NO];
	if (mini.hidden)  [mini  setHidden:NO];
	if (zoom.hidden)  [zoom  setHidden:NO];

	// Re-apply our close intercept if Gio reset the button's target.
	if (_closeHelper && [close target] != _closeHelper) {
		[close setTarget:_closeHelper];
		[close setAction:@selector(handleClose:)];
	}

	// Walk up to the titlebar container and keep its layer above Metal.
	NSView *themeFrame = window.contentView.superview;
	NSView *v = close;
	while (v.superview && v.superview != themeFrame) {
		v = v.superview;
	}
	if (v && v.superview == themeFrame) {
		if (!v.wantsLayer || v.layer.zPosition < 999) {
			v.wantsLayer = YES;
			v.layer.zPosition = 1000;
		}
	}
}

// Called from Go after every frame to immediately re-show traffic lights
// that Gio's Metal layer may have hidden during Configure().
// Must dispatch to the main thread since Go's run() goroutine is not on it.
void ensureTrafficLightsVisible(void) {
	if (!_titlebarDone) return;
	dispatch_async(dispatch_get_main_queue(), ^{
		for (NSWindow *window in [NSApp windows]) {
			if ([window isVisible] && window.contentView) {
				ensureButtonsVisible(window);
				return;
			}
		}
	});
}

// Intercept the close button directly by replacing its target/action.
// This is more robust than delegate swizzling because Gio frequently
// reconfigures the window and may reset delegate state.

static void handleCloseButtonClick(id self, SEL _cmd, id sender) {
	if (_hasUnsavedChanges) {
		_closeRequested = true;
		return;
	}
	// No unsaved changes — close normally.
	NSWindow *win = [sender window];
	if (win) [win close];
}

static void installCloseHandler(NSWindow *window) {
	NSButton *close = [window standardWindowButton:NSWindowCloseButton];
	if (!close) return;
	if (!_closeHelper) {
		Class cls = objc_allocateClassPair([NSObject class],
		                                   "ZephyrCloseHelper", 0);
		class_addMethod(cls, @selector(handleClose:),
		                (IMP)handleCloseButtonClick, "v@:@");
		objc_registerClassPair(cls);
		_closeHelper = [[cls alloc] init];
	}
	[close setTarget:_closeHelper];
	[close setAction:@selector(handleClose:)];
}

void registerTitlebarObserver() {
	// Poll until the window exists, then configure it and start the
	// button-visibility timer.
	__block int attempts = 0;
	[NSTimer scheduledTimerWithTimeInterval:0.05 repeats:YES block:^(NSTimer *t) {
		if (_titlebarDone || attempts > 60) {
			[t invalidate];
			return;
		}
		attempts++;
		for (NSWindow *window in [NSApp windows]) {
			if (!window.contentView) continue;

			// Extra configuration on top of what Gio sets with Decorated(false).
			window.tabbingMode = NSWindowTabbingModeDisallowed;
			window.backgroundColor = [NSColor colorWithRed:46.0/255.0
			                                        green:46.0/255.0
			                                         blue:46.0/255.0
			                                        alpha:1.0];
			if (@available(macOS 11.0, *)) {
				window.titlebarSeparatorStyle = NSTitlebarSeparatorStyleNone;
			}

			installCloseHandler(window);
			ensureButtonsVisible(window);
			swizzleContentView(window);
			_titlebarDone = true;
			[t invalidate];

			// Keep unhiding buttons — Gio re-hides them on every Configure().
			[NSTimer scheduledTimerWithTimeInterval:0.1 repeats:YES block:^(NSTimer *bt) {
				if (![window isVisible]) return;
				ensureButtonsVisible(window);
			}];
			return;
		}
	}];

	// Re-apply after exiting fullscreen.
	[[NSNotificationCenter defaultCenter]
		addObserverForName:NSWindowDidExitFullScreenNotification
		object:nil
		queue:[NSOperationQueue mainQueue]
		usingBlock:^(NSNotification *note) {
			dispatch_after(
				dispatch_time(DISPATCH_TIME_NOW, 200 * NSEC_PER_MSEC),
				dispatch_get_main_queue(), ^{
					ensureButtonsVisible(note.object);
				});
		}];
}

bool titlebarReady() {
	return _titlebarDone;
}

// Returns true if the global mouse cursor is outside all visible app windows.
bool isPointerOutsideWindow() {
	NSPoint mouseLocation = [NSEvent mouseLocation];
	for (NSWindow *window in [NSApp windows]) {
		if ([window isVisible] && NSPointInRect(mouseLocation, [window frame])) {
			return false;
		}
	}
	return true;
}

// Returns the global mouse position as (x, y) in screen coordinates.
void getGlobalMousePosition(double *outX, double *outY) {
	NSPoint mouseLocation = [NSEvent mouseLocation];
	*outX = mouseLocation.x;
	*outY = mouseLocation.y;
}

// Returns the window frame as (x, y, w, h) in screen coordinates.
void getWindowFrame(double *outX, double *outY, double *outW, double *outH) {
	for (NSWindow *window in [NSApp windows]) {
		if ([window isVisible] && window.contentView) {
			NSRect frame = [window frame];
			*outX = frame.origin.x;
			*outY = frame.origin.y;
			*outW = frame.size.width;
			*outH = frame.size.height;
			return;
		}
	}
	*outX = 0; *outY = 0; *outW = 0; *outH = 0;
}

// --- Word Wrap menu support ---

static volatile bool _wordWrapToggled = false;
static NSMenuItem *_wordWrapItem = nil;

bool checkAndResetWordWrapToggled(void) {
	if (_wordWrapToggled) {
		_wordWrapToggled = false;
		return true;
	}
	return false;
}

void updateWordWrapCheck(bool checked) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (_wordWrapItem) {
			[_wordWrapItem setState:checked ? NSControlStateValueOn : NSControlStateValueOff];
		}
	});
}

@interface ZephyrWordWrapHandler : NSObject
- (void)toggleWordWrap:(NSMenuItem *)sender;
@end

@implementation ZephyrWordWrapHandler
- (void)toggleWordWrap:(NSMenuItem *)sender {
	_wordWrapToggled = true;
}
@end

static ZephyrWordWrapHandler *_wordWrapHandler = nil;

void setupWordWrapMenuItem(bool checked) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (!_wordWrapHandler) {
			_wordWrapHandler = [[ZephyrWordWrapHandler alloc] init];
		}

		NSMenu *mainMenu = [NSApp mainMenu];
		if (!mainMenu) return;

		// Find or create the View menu
		NSMenuItem *viewMenuItem = nil;
		for (NSMenuItem *item in [mainMenu itemArray]) {
			if ([item.title isEqualToString:@"View"]) {
				viewMenuItem = item;
				break;
			}
		}
		if (!viewMenuItem) {
			viewMenuItem = [[NSMenuItem alloc] initWithTitle:@"View" action:nil keyEquivalent:@""];
			NSMenu *viewMenu = [[NSMenu alloc] initWithTitle:@"View"];
			[viewMenuItem setSubmenu:viewMenu];
			[mainMenu addItem:viewMenuItem];
		}

		NSMenu *viewMenu = [viewMenuItem submenu];

		// Check if Word Wrap item already exists
		if ([viewMenu indexOfItemWithTitle:@"Word Wrap"] >= 0) return;

		// Add Word Wrap item with Option+Z shortcut
		_wordWrapItem = [[NSMenuItem alloc] initWithTitle:@"Word Wrap"
		                                          action:@selector(toggleWordWrap:)
		                                   keyEquivalent:@"z"];
		[_wordWrapItem setKeyEquivalentModifierMask:NSEventModifierFlagOption];
		[_wordWrapItem setTarget:_wordWrapHandler];
		[_wordWrapItem setState:checked ? NSControlStateValueOn : NSControlStateValueOff];

		// Insert at the beginning of the View menu (before Theme)
		[viewMenu insertItem:_wordWrapItem atIndex:0];
		[viewMenu insertItem:[NSMenuItem separatorItem] atIndex:1];
	});
}

// --- Theme menu support ---

static char _selectedTheme[256] = {0};  // selected theme name (check-and-reset)
static NSMenu *_themeSubmenu = nil;
static NSString *_activeThemeName = nil;

// Returns the selected theme name and resets it. Caller must free() the result.
const char* checkAndResetSelectedTheme(void) {
	if (_selectedTheme[0] == '\0') return NULL;
	// Copy and reset
	static char buf[256];
	strncpy(buf, _selectedTheme, sizeof(buf)-1);
	buf[sizeof(buf)-1] = '\0';
	_selectedTheme[0] = '\0';
	return buf;
}

@interface ZephyrThemeHandler : NSObject
- (void)themeSelected:(NSMenuItem *)sender;
@end

@implementation ZephyrThemeHandler
- (void)themeSelected:(NSMenuItem *)sender {
	const char *name = [sender.title UTF8String];
	strncpy(_selectedTheme, name, sizeof(_selectedTheme)-1);
	_selectedTheme[sizeof(_selectedTheme)-1] = '\0';
}
@end

static ZephyrThemeHandler *_themeHandler = nil;

void setupThemeMenu(const char **themeNames, int count, const char *activeTheme) {
	// Copy strings before dispatch_async — the caller frees them immediately.
	NSMutableArray *names = [NSMutableArray arrayWithCapacity:count];
	for (int i = 0; i < count; i++) {
		[names addObject:[NSString stringWithUTF8String:themeNames[i]]];
	}
	NSString *active = [NSString stringWithUTF8String:activeTheme];

	dispatch_async(dispatch_get_main_queue(), ^{
		if (!_themeHandler) {
			_themeHandler = [[ZephyrThemeHandler alloc] init];
		}

		_activeThemeName = active;

		// Find or create the View menu
		NSMenu *mainMenu = [NSApp mainMenu];
		if (!mainMenu) {
			mainMenu = [[NSMenu alloc] initWithTitle:@""];
			[NSApp setMainMenu:mainMenu];
		}

		// Look for existing "View" menu
		NSMenuItem *viewMenuItem = nil;
		for (NSMenuItem *item in [mainMenu itemArray]) {
			if ([item.title isEqualToString:@"View"]) {
				viewMenuItem = item;
				break;
			}
		}
		if (!viewMenuItem) {
			viewMenuItem = [[NSMenuItem alloc] initWithTitle:@"View" action:nil keyEquivalent:@""];
			NSMenu *viewMenu = [[NSMenu alloc] initWithTitle:@"View"];
			[viewMenuItem setSubmenu:viewMenu];
			[mainMenu addItem:viewMenuItem];
		}

		NSMenu *viewMenu = [viewMenuItem submenu];

		// Remove old Theme submenu if exists
		NSInteger themeIdx = [viewMenu indexOfItemWithTitle:@"Theme"];
		if (themeIdx >= 0) {
			[viewMenu removeItemAtIndex:themeIdx];
		}

		// Create Theme submenu
		_themeSubmenu = [[NSMenu alloc] initWithTitle:@"Theme"];
		for (NSString *name in names) {
			NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:name
			                                             action:@selector(themeSelected:)
			                                      keyEquivalent:@""];
			[item setTarget:_themeHandler];
			if ([name isEqualToString:_activeThemeName]) {
				[item setState:NSControlStateValueOn];
			}
			[_themeSubmenu addItem:item];
		}

		NSMenuItem *themeItem = [[NSMenuItem alloc] initWithTitle:@"Theme" action:nil keyEquivalent:@""];
		[themeItem setSubmenu:_themeSubmenu];
		[viewMenu addItem:themeItem];
	});
}

void updateThemeMenuSelection(const char *activeTheme) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (!_themeSubmenu) return;
		NSString *active = [NSString stringWithUTF8String:activeTheme];
		for (NSMenuItem *item in [_themeSubmenu itemArray]) {
			[item setState:[item.title isEqualToString:active] ? NSControlStateValueOn : NSControlStateValueOff];
		}
	});
}

void setWindowBgColor(double r, double g, double b) {
	dispatch_async(dispatch_get_main_queue(), ^{
		for (NSWindow *window in [NSApp windows]) {
			if ([window isVisible] && window.contentView) {
				window.backgroundColor = [NSColor colorWithRed:r green:g blue:b alpha:1.0];
				break;
			}
		}
	});
}
*/
import "C"

import (
	"image/color"
	"unsafe"
)

func setupTitlebar() {
	C.registerTitlebarObserver()
}

func titlebarReady() bool {
	return bool(C.titlebarReady())
}

func setUnsavedFlag(unsaved bool) {
	C.setUnsavedFlag(C.bool(unsaved))
}

// ensureTrafficLights re-shows the macOS traffic light buttons after a frame
// render. Gio's Metal layer can hide them during Configure().
func ensureTrafficLights() {
	C.ensureTrafficLightsVisible()
}

// closeRequested returns true if the user clicked the red close button
// while there were unsaved changes. Resets the flag after reading.
func closeRequested() bool {
	return bool(C.checkAndResetCloseRequested())
}

// pointerOutsideWindow returns true if the mouse cursor is outside all
// visible application windows. Used for tab drag-out detection.
func pointerOutsideWindow() bool {
	return bool(C.isPointerOutsideWindow())
}

// globalMousePosition returns the global mouse position in screen coordinates.
func globalMousePosition() (x, y float64) {
	var cx, cy C.double
	C.getGlobalMousePosition(&cx, &cy)
	return float64(cx), float64(cy)
}

// windowFrame returns the current window frame in screen coordinates.
func windowFrame() (x, y, w, h float64) {
	var cx, cy, cw, ch C.double
	C.getWindowFrame(&cx, &cy, &cw, &ch)
	return float64(cx), float64(cy), float64(cw), float64(ch)
}

// startWindowDrag initiates a native macOS window drag from the current event.
// Called when the user drags on empty tab bar space (not on a tab).
func startWindowDrag() {
	C.performWindowDrag()
}

// trafficLightPaddingDp is the horizontal space (in Dp) reserved for the
// macOS close/minimize/zoom buttons plus a margin so tabs don't crowd them.
const trafficLightPaddingDp = 74

// updateWindowBackground sets the native macOS window background color
// to match the current theme's tab bar color.
func updateWindowBackground(c color.NRGBA) {
	C.setWindowBgColor(C.double(float64(c.R)/255.0), C.double(float64(c.G)/255.0), C.double(float64(c.B)/255.0))
}

// setupThemeMenu creates the View > Theme submenu in the macOS menu bar.
func setupThemeMenu(themeNames []string, activeTheme string) {
	if len(themeNames) == 0 {
		return
	}
	cNames := make([]*C.char, len(themeNames))
	for i, name := range themeNames {
		cNames[i] = C.CString(name)
	}
	cActive := C.CString(activeTheme)
	C.setupThemeMenu(&cNames[0], C.int(len(themeNames)), cActive)
	for _, cn := range cNames {
		C.free(unsafe.Pointer(cn))
	}
	C.free(unsafe.Pointer(cActive))
}

// checkThemeSelection returns the name of a theme the user selected from
// the menu, or "" if none. Resets after reading.
func checkThemeSelection() string {
	cs := C.checkAndResetSelectedTheme()
	if cs == nil {
		return ""
	}
	return C.GoString(cs)
}

// setupWordWrapMenu creates the View > Word Wrap menu item.
func setupWordWrapMenu(checked bool) {
	C.setupWordWrapMenuItem(C.bool(checked))
}

// wordWrapToggled returns true if the user clicked the Word Wrap menu item.
func wordWrapToggled() bool {
	return bool(C.checkAndResetWordWrapToggled())
}

// updateWordWrapMenuCheck syncs the Word Wrap menu checkmark.
func updateWordWrapMenuCheck(checked bool) {
	C.updateWordWrapCheck(C.bool(checked))
}

// updateThemeMenuCheck updates the checkmark in the Theme submenu.
func updateThemeMenuCheck(activeTheme string) {
	cs := C.CString(activeTheme)
	C.updateThemeMenuSelection(cs)
	C.free(unsafe.Pointer(cs))
}
