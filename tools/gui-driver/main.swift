import ApplicationServices
import AppKit
import CoreGraphics
import Darwin
import Foundation
import ImageIO
import UniformTypeIdentifiers

struct WindowInfo {
    let id: CGWindowID
    let pid: pid_t
    let owner: String
    let bounds: CGRect
}

enum DriverFailure: Error, CustomStringConvertible {
    case usage(String)
    case runtime(String)

    var description: String {
        switch self {
        case .usage(let message), .runtime(let message):
            return message
        }
    }
}

func integer(_ value: String, name: String) throws -> Int {
    guard let result = Int(value) else {
        throw DriverFailure.usage("invalid \(name): \(value)")
    }
    return result
}

func number(_ value: String, name: String) throws -> Double {
    guard let result = Double(value) else {
        throw DriverFailure.usage("invalid \(name): \(value)")
    }
    return result
}

func windows(pid: pid_t? = nil) -> [WindowInfo] {
    let options: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
    let raw = CGWindowListCopyWindowInfo(options, kCGNullWindowID) as? [[String: Any]] ?? []
    return raw.compactMap { item in
        guard
            let ownerPID = (item[kCGWindowOwnerPID as String] as? NSNumber)?.int32Value,
            pid == nil || ownerPID == pid,
            let windowID = (item[kCGWindowNumber as String] as? NSNumber)?.uint32Value,
            let layer = (item[kCGWindowLayer as String] as? NSNumber)?.intValue,
            layer == 0,
            let boundsDictionary = item[kCGWindowBounds as String] as? NSDictionary,
            let bounds = CGRect(dictionaryRepresentation: boundsDictionary)
        else {
            return nil
        }
        return WindowInfo(
            id: windowID,
            pid: ownerPID,
            owner: item[kCGWindowOwnerName as String] as? String ?? "",
            bounds: bounds
        )
    }
}

func targetWindow(pid: pid_t) throws -> WindowInfo {
    guard let window = windows(pid: pid).first else {
        throw DriverFailure.runtime("no on-screen layer-0 window found for pid \(pid)")
    }
    return window
}

func json(_ object: Any) throws {
    let data = try JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys])
    FileHandle.standardOutput.write(data)
    FileHandle.standardOutput.write(Data("\n".utf8))
}

func localPoint(window: WindowInfo, x: Double, y: Double) -> CGPoint {
    CGPoint(x: window.bounds.minX + x, y: window.bounds.minY + y)
}

func activate(_ window: WindowInfo) throws {
    guard let application = NSRunningApplication(processIdentifier: window.pid) else {
        throw DriverFailure.runtime("application for pid \(window.pid) is not running")
    }
    application.activate(options: [.activateAllWindows])
    usleep(120_000)
}

func postMouse(kind: CGEventType, point: CGPoint, button: CGMouseButton) throws {
    guard let event = CGEvent(mouseEventSource: nil, mouseType: kind, mouseCursorPosition: point, mouseButton: button) else {
        throw DriverFailure.runtime("failed to create mouse event")
    }
    event.post(tap: .cghidEventTap)
}

func click(window: WindowInfo, x: Double, y: Double, buttonName: String) throws {
    try activate(window)
    let point = localPoint(window: window, x: x, y: y)
    let button: CGMouseButton
    let down: CGEventType
    let up: CGEventType
    switch buttonName {
    case "left", "primary":
        button = .left
        down = .leftMouseDown
        up = .leftMouseUp
    case "right", "secondary":
        button = .right
        down = .rightMouseDown
        up = .rightMouseUp
    case "middle", "center", "tertiary":
        button = .center
        down = .otherMouseDown
        up = .otherMouseUp
    default:
        throw DriverFailure.usage("unknown mouse button: \(buttonName)")
    }
    try postMouse(kind: .mouseMoved, point: point, button: button)
    usleep(20_000)
    try postMouse(kind: down, point: point, button: button)
    usleep(30_000)
    try postMouse(kind: up, point: point, button: button)
}

func drag(window: WindowInfo, fromX: Double, fromY: Double, toX: Double, toY: Double, duration: Double) throws {
    try activate(window)
    let start = localPoint(window: window, x: fromX, y: fromY)
    let end = localPoint(window: window, x: toX, y: toY)
    let steps = max(2, Int(duration * 60.0))
    try postMouse(kind: .mouseMoved, point: start, button: .left)
    try postMouse(kind: .leftMouseDown, point: start, button: .left)
    for index in 1...steps {
        let fraction = Double(index) / Double(steps)
        let point = CGPoint(
            x: start.x + (end.x - start.x) * fraction,
            y: start.y + (end.y - start.y) * fraction
        )
        try postMouse(kind: .leftMouseDragged, point: point, button: .left)
        usleep(useconds_t(max(1_000, duration * 1_000_000.0 / Double(steps))))
    }
    try postMouse(kind: .leftMouseUp, point: end, button: .left)
}

func scroll(window: WindowInfo, x: Double, y: Double, deltaY: Int32, units: CGScrollEventUnit = .pixel) throws {
    try activate(window)
    let point = localPoint(window: window, x: x, y: y)
    try postMouse(kind: .mouseMoved, point: point, button: .left)
    usleep(80_000)
    let source = CGEventSource(stateID: .hidSystemState)
    guard let event = CGEvent(
        scrollWheelEvent2Source: source,
        units: units,
        wheelCount: 1,
        wheel1: deltaY,
        wheel2: 0,
        wheel3: 0
    ) else {
        throw DriverFailure.runtime("failed to create scroll event")
    }
    event.post(tap: .cghidEventTap)
    usleep(80_000)
}

func typeText(_ text: String) throws {
    let units = Array(text.utf16)
    for chunkStart in stride(from: 0, to: units.count, by: 20) {
        let chunkEnd = min(chunkStart + 20, units.count)
        var chunk = Array(units[chunkStart..<chunkEnd])
        guard
            let down = CGEvent(keyboardEventSource: nil, virtualKey: 0, keyDown: true),
            let up = CGEvent(keyboardEventSource: nil, virtualKey: 0, keyDown: false)
        else {
            throw DriverFailure.runtime("failed to create keyboard event")
        }
        chunk.withUnsafeMutableBufferPointer { buffer in
            down.keyboardSetUnicodeString(stringLength: buffer.count, unicodeString: buffer.baseAddress!)
            up.keyboardSetUnicodeString(stringLength: buffer.count, unicodeString: buffer.baseAddress!)
        }
        down.post(tap: .cghidEventTap)
        up.post(tap: .cghidEventTap)
        usleep(10_000)
    }
}

let keyCodes: [String: CGKeyCode] = [
    "a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7,
    "c": 8, "v": 9, "b": 11, "q": 12, "w": 13, "e": 14, "r": 15,
    "y": 16, "t": 17, "1": 18, "2": 19, "3": 20, "4": 21, "6": 22,
    "5": 23, "=": 24, "9": 25, "7": 26, "-": 27, "8": 28, "0": 29,
    "o": 31, "u": 32, "i": 34, "p": 35, "return": 36, "enter": 36,
    "l": 37, "j": 38, "k": 40, "n": 45, "m": 46, "tab": 48, "space": 49,
    "delete": 51, "backspace": 51, "escape": 53, "left": 123, "right": 124,
    "down": 125, "up": 126
]

func pressKey(name: String, modifierNames: ArraySlice<String>) throws {
    guard let keyCode = keyCodes[name.lowercased()] else {
        throw DriverFailure.usage("unknown key: \(name)")
    }
    var flags: CGEventFlags = []
    for modifier in modifierNames {
        switch modifier.lowercased() {
        case "cmd", "command": flags.insert(.maskCommand)
        case "shift": flags.insert(.maskShift)
        case "alt", "option": flags.insert(.maskAlternate)
        case "ctrl", "control": flags.insert(.maskControl)
        default: throw DriverFailure.usage("unknown modifier: \(modifier)")
        }
    }
    guard
        let down = CGEvent(keyboardEventSource: nil, virtualKey: keyCode, keyDown: true),
        let up = CGEvent(keyboardEventSource: nil, virtualKey: keyCode, keyDown: false)
    else {
        throw DriverFailure.runtime("failed to create key event")
    }
    down.flags = flags
    up.flags = flags
    down.post(tap: .cghidEventTap)
    usleep(20_000)
    up.post(tap: .cghidEventTap)
}

func capture(window: WindowInfo, path: String) throws {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
    process.arguments = ["-x", "-l", String(window.id), path]
    try process.run()
    process.waitUntilExit()
    guard process.terminationStatus == 0 else {
        throw DriverFailure.runtime("screencapture failed with status \(process.terminationStatus)")
    }
}

func jpegPreview(inputPath: String, outputPath: String) throws {
    let inputURL = URL(fileURLWithPath: inputPath) as CFURL
    guard
        let source = CGImageSourceCreateWithURL(inputURL, nil),
        let sourceImage = CGImageSourceCreateImageAtIndex(source, 0, nil)
    else {
        throw DriverFailure.runtime("failed to read capture: \(inputPath)")
    }

    let width = sourceImage.width
    let height = sourceImage.height
    guard let context = CGContext(
        data: nil,
        width: width,
        height: height,
        bitsPerComponent: 8,
        bytesPerRow: width * 4,
        space: CGColorSpaceCreateDeviceRGB(),
        bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue
    ) else {
        throw DriverFailure.runtime("failed to create preview context")
    }
    context.setFillColor(CGColor(gray: 1, alpha: 1))
    context.fill(CGRect(x: 0, y: 0, width: width, height: height))
    context.draw(sourceImage, in: CGRect(x: 0, y: 0, width: width, height: height))

    guard
        let flattened = context.makeImage(),
        let destination = CGImageDestinationCreateWithURL(
            URL(fileURLWithPath: outputPath) as CFURL,
            UTType.jpeg.identifier as CFString,
            1,
            nil
        )
    else {
        throw DriverFailure.runtime("failed to create JPEG preview: \(outputPath)")
    }
    CGImageDestinationAddImage(destination, flattened, [
        kCGImageDestinationLossyCompressionQuality: 0.92
    ] as CFDictionary)
    guard CGImageDestinationFinalize(destination) else {
        throw DriverFailure.runtime("failed to write JPEG preview: \(outputPath)")
    }
}

func usage() -> String {
    """
    usage: zephyr-gui-driver COMMAND [ARGS]
      permissions [--request]
      windows [PID]
      click PID X Y [left|right|middle]
      drag PID FROM_X FROM_Y TO_X TO_Y [DURATION_SECONDS]
      scroll PID X Y DELTA_Y
      scroll-lines PID X Y DELTA_Y
      type PID TEXT
      key PID KEY [cmd|shift|option|control ...]
      capture PID PATH
      preview INPUT_PNG OUTPUT_JPEG
      time
    Coordinates are local to the target window.
    """
}

do {
    let arguments = Array(CommandLine.arguments.dropFirst())
    guard let command = arguments.first else {
        throw DriverFailure.usage(usage())
    }
    switch command {
    case "permissions":
        let request = arguments.dropFirst().contains("--request")
        let postEvents = request ? CGRequestPostEventAccess() : CGPreflightPostEventAccess()
        let screenCapture = request ? CGRequestScreenCaptureAccess() : CGPreflightScreenCaptureAccess()
        try json(["postEvents": postEvents, "screenCapture": screenCapture])

    case "windows":
        let pid = arguments.count > 1 ? pid_t(try integer(arguments[1], name: "pid")) : nil
        let result = windows(pid: pid).map { window in
            [
                "id": Int(window.id), "pid": Int(window.pid), "owner": window.owner,
                "x": Int(window.bounds.minX), "y": Int(window.bounds.minY),
                "width": Int(window.bounds.width), "height": Int(window.bounds.height)
            ] as [String: Any]
        }
        try json(result)

    case "click":
        guard arguments.count >= 4 else { throw DriverFailure.usage(usage()) }
        let pid = pid_t(try integer(arguments[1], name: "pid"))
        let window = try targetWindow(pid: pid)
        try click(
            window: window,
            x: try number(arguments[2], name: "x"),
            y: try number(arguments[3], name: "y"),
            buttonName: arguments.count > 4 ? arguments[4].lowercased() : "left"
        )

    case "drag":
        guard arguments.count >= 6 else { throw DriverFailure.usage(usage()) }
        let pid = pid_t(try integer(arguments[1], name: "pid"))
        try drag(
            window: targetWindow(pid: pid),
            fromX: try number(arguments[2], name: "from x"),
            fromY: try number(arguments[3], name: "from y"),
            toX: try number(arguments[4], name: "to x"),
            toY: try number(arguments[5], name: "to y"),
            duration: arguments.count > 6 ? try number(arguments[6], name: "duration") : 0.35
        )

    case "scroll", "scroll-lines":
        guard arguments.count >= 5 else { throw DriverFailure.usage(usage()) }
        let pid = pid_t(try integer(arguments[1], name: "pid"))
        try scroll(
            window: targetWindow(pid: pid),
            x: try number(arguments[2], name: "x"),
            y: try number(arguments[3], name: "y"),
            deltaY: Int32(try integer(arguments[4], name: "delta y")),
            units: command == "scroll-lines" ? .line : .pixel
        )

    case "type":
        guard arguments.count == 3 else { throw DriverFailure.usage(usage()) }
        let pid = pid_t(try integer(arguments[1], name: "pid"))
        try activate(targetWindow(pid: pid))
        try typeText(arguments[2])

    case "key":
        guard arguments.count >= 3 else { throw DriverFailure.usage(usage()) }
        let pid = pid_t(try integer(arguments[1], name: "pid"))
        try activate(targetWindow(pid: pid))
        try pressKey(name: arguments[2], modifierNames: arguments.dropFirst(3))

    case "capture":
        guard arguments.count == 3 else { throw DriverFailure.usage(usage()) }
        let pid = pid_t(try integer(arguments[1], name: "pid"))
        try capture(window: targetWindow(pid: pid), path: arguments[2])

    case "preview":
        guard arguments.count == 3 else { throw DriverFailure.usage(usage()) }
        try jpegPreview(inputPath: arguments[1], outputPath: arguments[2])

    case "time":
        guard arguments.count == 1 else { throw DriverFailure.usage(usage()) }
        print(DispatchTime.now().uptimeNanoseconds)

    default:
        throw DriverFailure.usage(usage())
    }
} catch {
    FileHandle.standardError.write(Data("error: \(error)\n".utf8))
    exit(2)
}
