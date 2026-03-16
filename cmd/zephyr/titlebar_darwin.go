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

void setUnsavedFlag(bool val) { _hasUnsavedChanges = val; }
bool getUnsavedFlag(void)     { return _hasUnsavedChanges; }

// windowShouldClose: — always allow close; Go handles saving in DestroyEvent.
static BOOL interceptedWindowShouldClose(id self, SEL _cmd, NSWindow *sender) {
	return YES;
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

static void installCloseHandler(NSWindow *window) {
	id delegate = window.delegate;
	if (!delegate) return;
	Class cls = [delegate class];
	// Only add if the class doesn't already implement windowShouldClose:
	if (!class_getInstanceMethod(cls, @selector(windowShouldClose:))) {
		class_addMethod(cls, @selector(windowShouldClose:),
		                (IMP)interceptedWindowShouldClose, "B@:@");
	}
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

// trafficLightPadding is the horizontal space reserved for the macOS
// close/minimize/zoom buttons plus a margin so tabs don't crowd them.
const trafficLightPadding = 160
