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

void setUnsavedFlag(bool val) {
	if (val != _hasUnsavedChanges) NSLog(@"[Zephyr] unsaved flag: %d -> %d", _hasUnsavedChanges, val);
	_hasUnsavedChanges = val;
}
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

// Intercept the close button directly by replacing its target/action.
// This is more robust than delegate swizzling because Gio frequently
// reconfigures the window and may reset delegate state.

static void handleCloseButtonClick(id self, SEL _cmd, id sender) {
	NSLog(@"[Zephyr] close button clicked, unsaved=%d", _hasUnsavedChanges);
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
	if (!close) {
		NSLog(@"[Zephyr] installCloseHandler: no close button");
		return;
	}
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
	NSLog(@"[Zephyr] close handler installed on button %@", close);
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
*/
import "C"

func setupTitlebar() {
	C.registerTitlebarObserver()
}

func titlebarReady() bool {
	return bool(C.titlebarReady())
}

func setUnsavedFlag(unsaved bool) {
	C.setUnsavedFlag(C.bool(unsaved))
}

// closeRequested returns true if the user clicked the red close button
// while there were unsaved changes. Resets the flag after reading.
func closeRequested() bool {
	return bool(C.checkAndResetCloseRequested())
}

// trafficLightPaddingDp is the horizontal space (in Dp) reserved for the
// macOS close/minimize/zoom buttons plus a margin so tabs don't crowd them.
// This is converted to pixels via gtx.Dp() at each call site so it scales
// correctly across Retina (2×) and non-Retina (1×) displays.
// The three buttons span ~54pt; we add a small margin.
const trafficLightPaddingDp = 76
