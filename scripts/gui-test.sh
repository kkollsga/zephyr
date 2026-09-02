#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
STATE_DIR=${ZEPHYR_GUI_STATE_DIR:-"$ROOT/.artifacts/gui-test"}
APP="$STATE_DIR/Zephyr GUI Test.app"
CONTENTS="$APP/Contents"
BIN="$CONTENTS/MacOS/zephyr-gui-test"
DRIVER="$STATE_DIR/bin/zephyr-gui-driver"
DRIVER_SOURCE="$ROOT/tools/gui-driver/main.swift"
HOME_DIR="$STATE_DIR/home"
PID_FILE="$STATE_DIR/zephyr.pid"
LOG_FILE="${TMPDIR:-/tmp}/zephyr-gui-test-${UID}.log"
STDOUT_LOG="${TMPDIR:-/tmp}/zephyr-gui-test-${UID}.stdout.log"
LOG_LINK="$STATE_DIR/zephyr.log"
STDOUT_LOG_LINK="$STATE_DIR/zephyr.stdout.log"
DEFAULT_FIXTURE="$ROOT/testdata/gui/mouse_fixture.go"
MARKDOWN_FIXTURE="$ROOT/testdata/gui/markdown_fixture.md"

usage() {
    cat <<'EOF'
usage: ./scripts/gui-test.sh COMMAND [ARGS]
  build                              Build debug app and GUI driver
  launch [FIXTURE]                   Launch with isolated HOME and fixture
  run [FIXTURE]                      Run in foreground for managed tool sessions
  stop                               Stop the isolated Zephyr instance
  status                             Show process, window, and permissions
  permissions [--request]            Check/request macOS automation permissions
  window                             Print the Zephyr window geometry as JSON
  capture [PATH]                     Capture the Zephyr window
  click X Y [left|right|middle]      Click window-local coordinates
  drag X1 Y1 X2 Y2 [DURATION]        Drag between window-local coordinates
  scroll X Y DELTA_Y                 Send a pixel scroll event
  scroll-lines X Y DELTA_Y           Send a discrete wheel-line event
  type TEXT                          Type Unicode text into the focused field
  key KEY [MODIFIERS...]             Press a key with optional modifiers
  logs                               Tail the application log
  trace                              Show structured pointer-event telemetry
  scenarios                          List the named scenarios
  scenario NAME [NAME...]            Run named scenarios in order
  smoke                              Alias for: scenario smoke
  regression                         Alias for: scenario regression
EOF
}

ensure_directories() {
    mkdir -p "$STATE_DIR/bin" "$STATE_DIR/artifacts" "$HOME_DIR"
    ln -sfn "$LOG_FILE" "$LOG_LINK"
    ln -sfn "$STDOUT_LOG" "$STDOUT_LOG_LINK"
}

build_driver() {
    ensure_directories
    if [[ ! -x "$DRIVER" || "$DRIVER_SOURCE" -nt "$DRIVER" ]]; then
        swiftc -O -framework ApplicationServices -framework CoreGraphics \
            -o "$DRIVER" "$DRIVER_SOURCE"
    fi
}

build_app() {
    ensure_directories
    mkdir -p "$CONTENTS/MacOS" "$CONTENTS/Resources"
    go build -gcflags='all=-N -l' \
        -ldflags '-X main.version=gui-test -X main.commit=local -X main.date=debug' \
        -o "$BIN" ./cmd/zephyr
    cp "$ROOT/Info.plist" "$CONTENTS/Info.plist"
    cp "$ROOT/assets/icon.icns" "$CONTENTS/Resources/icon.icns"
    plutil -replace CFBundleExecutable -string zephyr-gui-test "$CONTENTS/Info.plist"
    plutil -replace CFBundleIdentifier -string com.kristianweb.zephyr.gui-test "$CONTENTS/Info.plist"
    plutil -replace CFBundleName -string 'Zephyr GUI Test' "$CONTENTS/Info.plist"
    plutil -replace CFBundleDisplayName -string 'Zephyr GUI Test' "$CONTENTS/Info.plist"
    codesign --force --sign - "$APP" >/dev/null
    build_driver
}

current_pid() {
    [[ -f "$PID_FILE" ]] || return 1
    local pid
    pid=$(<"$PID_FILE")
    [[ "$pid" =~ ^[0-9]+$ ]] || return 1
    kill -0 "$pid" 2>/dev/null || return 1
    printf '%s\n' "$pid"
}

stop_app() {
    local pid
    if ! pid=$(current_pid); then
        rm -f "$PID_FILE"
        return 0
    fi
    local command
    command=$(ps -p "$pid" -o command=)
    if [[ "$command" != *"$BIN"* ]]; then
        echo "refusing to stop pid $pid: it is not the GUI-test binary" >&2
        return 1
    fi
    kill "$pid"
    for _ in {1..30}; do
        if ! kill -0 "$pid" 2>/dev/null; then
            rm -f "$PID_FILE"
            return 0
        fi
        sleep 0.1
    done
    kill -9 "$pid" 2>/dev/null || true
    rm -f "$PID_FILE"
}

launch_app() {
    local fixture=${1:-"$DEFAULT_FIXTURE"}
    [[ -f "$fixture" ]] || { echo "fixture not found: $fixture" >&2; return 1; }
    fixture=$(cd "$(dirname "$fixture")" && pwd)/$(basename "$fixture")
    [[ -x "$BIN" ]] || build_app
    stop_app
    # Unlink rather than truncate, so the redirect below creates a new inode.
    # Truncating leaves any process still holding the old fd — an orphan from an
    # interrupted run, a torn-out second Zephyr — writing at its old offset, and
    # the gap it leaves ahead of the new log reads back as NUL bytes.
    rm -f "$LOG_FILE" "$STDOUT_LOG"
    # Launch the bundled executable directly. LaunchServices can acknowledge an
    # app launch while leaving a Gio window inactive in non-interactive agent
    # sessions; a detached direct launch reliably creates the native window.
    env HOME="$HOME_DIR" XDG_CONFIG_HOME="$HOME_DIR/.config" \
        ZEPHYR_GUI_STATE_DIR="$STATE_DIR" \
        GOTRACEBACK=all ZEPHYR_GUI_TRACE=1 \
        nohup "$BIN" "$fixture" >"$STDOUT_LOG" 2>"$LOG_FILE" </dev/null &
    local launcher_pid=$!
    local pid=""
    local window_info=""
    for _ in {1..600}; do
        pid=$launcher_pid
        if [[ -z "$pid" ]]; then
            if ! kill -0 "$launcher_pid" 2>/dev/null; then
                echo "Zephyr exited before its window appeared" >&2
                tail -n 100 "$LOG_FILE" >&2
                return 1
            fi
            sleep 0.1
            continue
        fi
        printf '%s\n' "$pid" >"$PID_FILE"
        window_info=$("$DRIVER" windows "$pid")
        if grep -q '"id"' <<<"$window_info"; then
            echo "Zephyr GUI Test ready (pid $pid)"
            printf '%s\n' "$window_info"
            return 0
        fi
        if ! kill -0 "$pid" 2>/dev/null; then
            echo "Zephyr exited during launch" >&2
            tail -n 100 "$LOG_FILE" >&2
            return 1
        fi
        sleep 0.1
    done
    echo "Zephyr launched but no window appeared" >&2
    return 1
}

run_app() {
    local fixture=${1:-"$DEFAULT_FIXTURE"}
    [[ -f "$fixture" ]] || { echo "fixture not found: $fixture" >&2; return 1; }
    fixture=$(cd "$(dirname "$fixture")" && pwd)/$(basename "$fixture")
    [[ -x "$BIN" ]] || build_app
    stop_app
    rm -f "$LOG_FILE"
    printf '%s\n' "$$" >"$PID_FILE"
    exec env HOME="$HOME_DIR" XDG_CONFIG_HOME="$HOME_DIR/.config" \
        ZEPHYR_GUI_STATE_DIR="$STATE_DIR" GOTRACEBACK=all ZEPHYR_GUI_TRACE=1 \
        "$BIN" "$fixture" >>"$LOG_FILE" 2>&1
}

require_pid() {
    local pid
    if ! pid=$(current_pid); then
        echo "Zephyr GUI Test is not running; use: $0 launch" >&2
        return 1
    fi
    printf '%s\n' "$pid"
}

require_permissions() {
    local permissions
    permissions=$("$DRIVER" permissions)
    if ! grep -q '"postEvents" : true' <<<"$permissions" || \
       ! grep -q '"screenCapture" : true' <<<"$permissions"; then
        echo "Accessibility and Screen Recording permissions are required." >&2
        echo "Run: $0 permissions --request" >&2
        echo "$permissions" >&2
        return 1
    fi
}

# --- scenario helpers -------------------------------------------------------

# trace_grep is the only way a scenario reads the trace log, and it passes -a so
# grep reads the log as text no matter what bytes are in it.
#
# The log is a plain file that more than one process can end up holding open. A
# writer whose file offset outlives a truncation goes on writing past the new
# end, and the gap between reads back as NUL bytes. grep then classifies the
# whole file as binary and reports no match — ugrep, this repo's usual grep on
# macOS, prints nothing and exits 1 for every pattern in the file, including the
# records that are plainly there. Every assertion below would then fail with a
# message blaming the app for a byte the app never wrote, which is the exact
# shape of a gate people learn to ignore.
trace_grep() {
    grep -a "$@" "$LOG_FILE"
}

# trace_cursor prints LINE:COL from the most recent trace record that carries a
# cursor position.
trace_cursor() {
    trace_grep 'ZEPHYR_GUI_TRACE' | tail -n 1 | \
        sed -E -n 's/.*"cursorLine":([0-9]+),"cursorCol":([0-9]+).*/\1:\2/p'
}

# trace_buffer_hash prints the buffer checksum from the most recent key or edit
# trace record, so a scenario can assert what the buffer holds without reading
# pixels. Empty when no such record has been emitted yet.
trace_buffer_hash() {
    trace_grep -o '"bufferHash":"[0-9a-f]*"' | tail -n 1 | \
        sed -E 's/.*:"([0-9a-f]*)"/\1/'
}

# capture_checked captures the window of $pid to a named PNG, refuses an empty
# or unreadable one, and writes the flattened JPEG preview beside it.
capture_checked() {
    local name=$1
    local png="$STATE_DIR/artifacts/$name.png"
    "$DRIVER" capture "$pid" "$png"
    [[ -s "$png" ]] || { echo "empty capture: $png" >&2; exit 1; }
    sips -g pixelWidth -g pixelHeight "$png" >/dev/null
    "$DRIVER" preview "$png" "$STATE_DIR/artifacts/$name.jpg"
}

# buffer_hash_of prints the checksum the app's trace would carry for a document
# with these bytes: the first 8 bytes of its SHA-256, hex-encoded.
buffer_hash_of() {
    shasum -a 256 "$1" | cut -c1-16
}

# expect_buffer fails the scenario unless the most recent trace record says the
# buffer holds exactly the bytes of the expected file.
expect_buffer() {
    local label=$1 expected=$2
    local want got
    want=$(buffer_hash_of "$expected")
    got=$(trace_buffer_hash)
    if [[ "$got" != "$want" ]]; then
        echo "buffer after $label = ${got:-<no trace record>}, want $want" >&2
        echo "  expected content: $expected" >&2
        exit 1
    fi
}

# --- scenarios --------------------------------------------------------------
#
# Every scenario is a function named scenario_<name with dashes as underscores>
# and listed in SCENARIOS. It launches its own app, traps stop_app, and states a
# pass condition read off the app or the filesystem — never off a screenshot.

SCENARIOS=(smoke regression clipboard save-as tear-out)

list_scenarios() {
    printf '%s\n' "${SCENARIOS[@]}"
}

# run_scenario runs one scenario in a subshell of its own. Scenarios install
# different EXIT traps — stop_app for most, tear_out_cleanup for tear-out — and
# a shell holds only one EXIT trap at a time: run in one shell, the next
# scenario's trap silently replaces the previous one and its cleanup never
# fires. That left an orphan Zephyr holding the global input stream after
# `scenario tear-out smoke`. A subshell per scenario ends where the scenario
# ends, so every trap runs exactly once, at its own scenario's end.
run_scenario() {
    local name=$1
    local known
    for known in "${SCENARIOS[@]}"; do
        if [[ "$known" == "$name" ]]; then
            ( "scenario_${name//-/_}" )
            return
        fi
    done
    echo "unknown scenario: $name" >&2
    echo "known scenarios: ${SCENARIOS[*]}" >&2
    return 2
}

# scenario_save_as drives Save As onto a file that already exists, and the
# overwrite prompt's keyboard confirm: Return overwrites, Escape goes back with
# the target untouched. Both halves are read off the target file's bytes.
scenario_save_as() {
    local work="$STATE_DIR/artifacts/save-as"
    rm -rf "$work"
    mkdir -p "$work"
    local src="$work/save_as_source.go"
    local target="$work/target.go"
    cp "$DEFAULT_FIXTURE" "$src"
    printf 'package target\n// this file must survive Escape\n' >"$target"

    # The bytes the target must still hold after Escape, and the document the
    # buffer must hold — and therefore what the target must hold after Return.
    local target_before="$work/expected-target-before.go"
    cp "$target" "$target_before"
    local marker='//zephyr-save-as '
    local expected_buffer="$work/expected-buffer.go"
    printf '%s' "$marker" >"$expected_buffer"
    cat "$DEFAULT_FIXTURE" >>"$expected_buffer"

    launch_app "$src"
    trap stop_app EXIT
    pid=$(require_pid)
    sleep 0.7

    "$DRIVER" click "$pid" 300 165 left
    sleep 0.2
    "$DRIVER" key "$pid" up cmd
    sleep 0.15
    "$DRIVER" type "$pid" "$marker"
    sleep 0.4
    expect_buffer "typing the marker" "$expected_buffer"

    "$DRIVER" key "$pid" s cmd shift
    sleep 0.5
    capture_checked 30-save-as-menu
    "$DRIVER" type "$pid" "$(basename "$target")"
    sleep 0.3
    "$DRIVER" key "$pid" return
    sleep 0.5
    capture_checked 31-save-as-overwrite-prompt

    "$DRIVER" key "$pid" escape
    sleep 0.5
    diff -q "$target" "$target_before" >/dev/null || {
        echo "Escape at the overwrite prompt wrote the target anyway" >&2
        diff "$target_before" "$target" | head -n 20 >&2
        exit 1
    }

    "$DRIVER" key "$pid" return
    sleep 0.5
    "$DRIVER" key "$pid" return
    sleep 0.8
    diff -q "$target" "$expected_buffer" >/dev/null || {
        echo "the confirmed overwrite did not write the buffer to the target" >&2
        diff "$expected_buffer" "$target" | head -n 20 >&2
        exit 1
    }
    capture_checked 32-save-as-saved

    echo "Save As scenario completed; artifacts: $STATE_DIR/artifacts"
    stop_app
    trap - EXIT
}

# --- tear-out ---------------------------------------------------------------
#
# Tearing a tab out spawns a second Zephyr, so this is the one scenario the
# harness's single-PID model does not cover: PID_FILE names the instance the
# harness launched, and stop_app refuses any other pid. Cleanup therefore works
# off the process list instead.

# tear_out_pids lists every process running the GUI-test binary. The path is
# inside the state dir and unique to the harness, so a Zephyr the developer is
# running themselves can never match it.
tear_out_pids() {
    pgrep -f "$BIN" 2>/dev/null || true
}

# tear_out_cleanup stops every process the scenario started, whether it reached
# the end or died in the middle, and fails loudly if one survives — an orphan
# Zephyr holds the global input stream and would poison every later run.
tear_out_cleanup() {
    local status=$? p attempt
    for attempt in 1 2 3; do
        local pids
        pids=$(tear_out_pids)
        [[ -n "$pids" ]] || break
        for p in $pids; do
            kill "$p" 2>/dev/null || true
        done
        sleep 0.5
    done
    for p in $(tear_out_pids); do
        kill -9 "$p" 2>/dev/null || true
    done
    sleep 0.3
    rm -f "$PID_FILE"
    local left
    left=$(tear_out_pids)
    if [[ -n "$left" ]]; then
        echo "tear-out cleanup left Zephyr processes running: $left" >&2
        exit 1
    fi
    exit "$status"
}

# scenario_tear_out drags a tab out of the window and asserts that a second
# Zephyr process exists with a window of its own, holding the file the tab was
# on. Both processes are killed on the way out, pass or fail.
scenario_tear_out() {
    local work="$STATE_DIR/artifacts/tear-out"
    rm -rf "$work"
    mkdir -p "$work"
    local src="$work/mouse_fixture.go"
    cp "$DEFAULT_FIXTURE" "$src"

    # Start from a known process count: a straggler from an earlier run would
    # make "a second process appeared" true without this scenario doing it.
    stop_app
    local strays
    strays=$(tear_out_pids)
    if [[ -n "$strays" ]]; then
        echo "a GUI-test Zephyr was already running: $strays" >&2
        return 1
    fi

    launch_app "$src"
    trap tear_out_cleanup EXIT
    pid=$(require_pid)
    sleep 0.7

    local geometry width height
    geometry=$("$DRIVER" windows "$pid")
    width=$(sed -E -n 's/.*"width" : ([0-9]+).*/\1/p' <<<"$geometry")
    height=$(sed -E -n 's/.*"height" : ([0-9]+).*/\1/p' <<<"$geometry")
    [[ -n "$width" && -n "$height" ]] || {
        echo "could not read the window geometry: $geometry" >&2
        exit 1
    }

    # A second tab: the drag-out is a no-op on the last remaining one.
    "$DRIVER" key "$pid" t cmd
    sleep 0.5
    capture_checked 40-tear-out-two-tabs

    # Drag the first tab from the tab bar to a point below the window, which is
    # what the app reads as "outside every window of this application".
    "$DRIVER" drag "$pid" 110 14 $((width / 2)) $((height + 200)) 0.8

    local torn="" waited
    for waited in {1..40}; do
        local pids
        pids=$(tear_out_pids)
        torn=$(grep -v "^$pid$" <<<"$pids" | head -n 1 || true)
        [[ -z "$torn" ]] || break
        sleep 0.25
    done
    [[ -n "$torn" ]] || {
        echo "the drag out of the window did not start a second Zephyr" >&2
        exit 1
    }

    local count
    count=$(tear_out_pids | wc -l | tr -d ' ')
    [[ "$count" == "2" ]] || {
        echo "expected two Zephyr processes after the tear-out, found $count" >&2
        tear_out_pids >&2
        exit 1
    }

    local torn_command
    torn_command=$(ps -p "$torn" -o command=)
    [[ "$torn_command" == *"$src"* ]] || {
        echo "the second process is not holding the torn-out file: $torn_command" >&2
        exit 1
    }

    local torn_window=""
    for waited in {1..60}; do
        torn_window=$("$DRIVER" windows "$torn")
        grep -q '"id"' <<<"$torn_window" && break
        torn_window=""
        sleep 0.25
    done
    [[ -n "$torn_window" ]] || {
        echo "the torn-out tab's process never opened a window" >&2
        exit 1
    }

    capture_checked 41-tear-out-source
    "$DRIVER" capture "$torn" "$STATE_DIR/artifacts/42-tear-out-detached.png"
    "$DRIVER" preview "$STATE_DIR/artifacts/42-tear-out-detached.png" \
        "$STATE_DIR/artifacts/42-tear-out-detached.jpg"

    echo "Tear-out scenario completed; artifacts: $STATE_DIR/artifacts"
}

# CLIPBOARD_BACKUP holds whatever was on the pasteboard before the clipboard
# scenario ran. A harness that keeps someone's clipboard is a harness they stop
# running, so it is restored from the EXIT trap whether the run passed or not.
CLIPBOARD_BACKUP=""

clipboard_cleanup() {
    if [[ -n "$CLIPBOARD_BACKUP" && -f "$CLIPBOARD_BACKUP" ]]; then
        pbcopy <"$CLIPBOARD_BACKUP" || true
    fi
    stop_app
}

# scenario_clipboard drives copy, paste, paste-over-selection, undo and save
# through real keystrokes, and reads its pass conditions off the buffer
# checksum in the trace and off the bytes Zephyr wrote to disk.
#
# It runs against a copy of the fixture inside the state dir. The scenario
# saves the file, and a scenario that writes to a tracked fixture would leave
# the working tree dirty on every run.
scenario_clipboard() {
    local work="$STATE_DIR/artifacts/clipboard"
    rm -rf "$work"
    mkdir -p "$work"
    local target="$work/clipboard_fixture.go"
    cp "$DEFAULT_FIXTURE" "$target"

    # The save and restore is text only: pbpaste flattens an image, a file
    # promise or rich text to whatever plain text it can produce, and the
    # restore puts that back. Anything else on the pasteboard is lost, so say
    # so before taking it rather than after.
    echo "clipboard scenario: your pasteboard is saved and restored as plain text; non-text content on it will be lost" >&2
    CLIPBOARD_BACKUP="$work/pasteboard.bak"
    pbpaste >"$CLIPBOARD_BACKUP" 2>/dev/null || : >"$CLIPBOARD_BACKUP"

    # The three documents the run must pass through, built here rather than
    # captured from the app, so the assertions are independent of it. The word
    # is the first seven characters of the fixture: "package".
    local word="package"
    local original="$work/expected-original.go"
    local pasted="$work/expected-pasted.go"
    local replaced="$work/expected-replaced.go"
    cp "$DEFAULT_FIXTURE" "$original"
    cat "$original" >"$pasted"
    printf '%s' "$word" >>"$pasted"
    printf '%s' "$word" >"$replaced"

    launch_app "$target"
    trap clipboard_cleanup EXIT
    pid=$(require_pid)
    sleep 0.7
    capture_checked 20-clipboard-launch

    "$DRIVER" click "$pid" 300 165 left
    sleep 0.2
    "$DRIVER" key "$pid" up cmd
    sleep 0.15
    local i
    for ((i = 0; i < ${#word}; i++)); do
        "$DRIVER" key "$pid" right shift
    done
    sleep 0.2
    # Empty the pasteboard first. With the copied word left on it from an
    # earlier run, the assertion below passes whether or not Cmd+C did
    # anything, which is a gate that cannot fail.
    pbcopy </dev/null
    "$DRIVER" key "$pid" c cmd
    sleep 0.4
    [[ "$(pbpaste)" == "$word" ]] || {
        echo "Cmd+C did not put the selected word on the pasteboard: $(pbpaste)" >&2
        exit 1
    }

    "$DRIVER" key "$pid" down cmd
    sleep 0.15
    "$DRIVER" key "$pid" v cmd
    sleep 0.4
    expect_buffer "the paste at end of file" "$pasted"

    "$DRIVER" key "$pid" a cmd
    sleep 0.2
    "$DRIVER" key "$pid" v cmd
    sleep 0.4
    expect_buffer "the paste over the whole selection" "$replaced"
    capture_checked 21-clipboard-pasted-over

    "$DRIVER" key "$pid" z cmd
    sleep 0.4
    expect_buffer "undoing the paste over the selection" "$pasted"
    "$DRIVER" key "$pid" z cmd
    sleep 0.4
    expect_buffer "undoing the paste at end of file" "$original"

    "$DRIVER" key "$pid" s cmd
    sleep 0.8
    diff -q "$target" "$original" >/dev/null || {
        echo "the saved file does not match the expected document" >&2
        diff "$original" "$target" | head -n 20 >&2
        exit 1
    }
    capture_checked 22-clipboard-saved

    echo "Clipboard scenario completed; artifacts: $STATE_DIR/artifacts"
    stop_app
    clipboard_cleanup
    trap - EXIT
}

scenario_smoke() {
    launch_app "$DEFAULT_FIXTURE"
    trap stop_app EXIT
    pid=$(require_pid)
    sleep 0.7
    "$DRIVER" capture "$pid" "$STATE_DIR/artifacts/00-launch.png"
    "$DRIVER" click "$pid" 360 165 left
    sleep 0.2
    "$DRIVER" click "$pid" 360 165 left
    "$DRIVER" type "$pid" '/* gui-smoke */'
    "$DRIVER" drag "$pid" 260 165 520 245 0.4
    sleep 0.2
    "$DRIVER" scroll-lines "$pid" 601 301 -4
    sleep 0.4
    trace_grep -q '"kind":"Press"' || { echo "no pointer press was recorded" >&2; exit 1; }
    trace_grep -q '"selection":true' || { echo "no drag selection was recorded" >&2; exit 1; }
    trace_grep -q '"kind":"Scroll"' || { echo "no scroll event was recorded" >&2; exit 1; }
    "$DRIVER" capture "$pid" "$STATE_DIR/artifacts/01-interacted.png"
    "$DRIVER" preview "$STATE_DIR/artifacts/00-launch.png" \
        "$STATE_DIR/artifacts/00-launch.jpg"
    "$DRIVER" preview "$STATE_DIR/artifacts/01-interacted.png" \
        "$STATE_DIR/artifacts/01-interacted.jpg"
    echo "Smoke test completed; artifacts: $STATE_DIR/artifacts"
    stop_app
    trap - EXIT
}

scenario_regression() {
    stop_app
    rm -rf "$HOME_DIR"
    mkdir -p "$HOME_DIR"
    launch_app "$DEFAULT_FIXTURE"
    trap stop_app EXIT
    pid=$(require_pid)
    sleep 0.7

    capture_checked 10-pointer-launch
    "$DRIVER" click "$pid" 260 165 left
    sleep 0.15
    primary_cursor=$(trace_cursor)
    [[ -n "$primary_cursor" ]] || { echo "primary click did not set a traceable cursor" >&2; exit 1; }
    "$DRIVER" click "$pid" 620 300 right
    sleep 0.15
    [[ "$(trace_cursor)" == "$primary_cursor" ]] || { echo "secondary click moved the cursor" >&2; exit 1; }
    "$DRIVER" click "$pid" 620 340 middle
    sleep 0.15
    [[ "$(trace_cursor)" == "$primary_cursor" ]] || { echo "middle click moved the cursor" >&2; exit 1; }

    "$DRIVER" drag "$pid" 260 165 520 245 0.4
    sleep 0.15
    trace_grep -q '"kind":"Drag".*"selection":true' || { echo "drag selection failed" >&2; exit 1; }
    "$DRIVER" scroll "$pid" 600 300 -7
    sleep 0.3
    trace_grep -Eq '"kind":"Scroll".*"viewportOffset":[1-9][0-9]*' || { echo "fractional pixel scroll was not retained" >&2; exit 1; }
    capture_checked 11-pointer-actions

    "$DRIVER" key "$pid" v cmd shift
    sleep 0.3
    capture_checked 12-vim-on
    "$DRIVER" key "$pid" v cmd shift
    "$DRIVER" key "$pid" n cmd shift
    sleep 0.4
    capture_checked 13-navigator-on
    "$DRIVER" key "$pid" n cmd shift
    "$DRIVER" key "$pid" v cmd shift
    sleep 0.2

    stop_app
    launch_app "$MARKDOWN_FIXTURE"
    pid=$(require_pid)
    sleep 0.7
    capture_checked 14-markdown-read
    "$DRIVER" drag "$pid" 180 150 560 300 0.4
    sleep 0.2
    md_drag=$(trace_grep 'ZEPHYR_GUI_TRACE' | grep -a '"kind":"Drag"' | tail -n 1)
    grep -q '"markdownSelect":true' <<<"$md_drag" || { echo "markdown drag selection was not active" >&2; exit 1; }
    if grep -q '"cursorLine"\|"cursorCol"' <<<"$md_drag"; then
        echo "markdown read-mode selection fell through to the hidden editor" >&2
        exit 1
    fi
    "$DRIVER" key "$pid" e cmd
    sleep 0.35
    capture_checked 15-markdown-edit
    "$DRIVER" key "$pid" e cmd
    sleep 0.35
    capture_checked 16-markdown-read-restored

    echo "GUI regression test completed; artifacts: $STATE_DIR/artifacts"
    stop_app
    trap - EXIT
}

command=${1:-}
shift || true

case "$command" in
build)
    build_app
    echo "$APP"
    ;;
launch)
    build_driver
    launch_app "${1:-$DEFAULT_FIXTURE}"
    ;;
run)
    build_driver
    run_app "${1:-$DEFAULT_FIXTURE}"
    ;;
stop)
    stop_app
    ;;
status)
    build_driver
    "$DRIVER" permissions
    if pid=$(current_pid); then
        ps -p "$pid" -o pid=,etime=,command=
        "$DRIVER" windows "$pid"
    else
        echo "Zephyr GUI Test is not running"
    fi
    ;;
permissions)
    build_driver
    "$DRIVER" permissions "$@"
    ;;
window)
    build_driver
    pid=$(require_pid)
    "$DRIVER" windows "$pid"
    ;;
capture)
    build_driver
    pid=$(require_pid)
    output=${1:-"$STATE_DIR/artifacts/window.png"}
    "$DRIVER" capture "$pid" "$output"
    echo "$output"
    ;;
click)
    build_driver
    pid=$(require_pid)
    "$DRIVER" click "$pid" "$@"
    ;;
drag)
    build_driver
    pid=$(require_pid)
    "$DRIVER" drag "$pid" "$@"
    ;;
scroll)
    build_driver
    pid=$(require_pid)
    "$DRIVER" scroll "$pid" "$@"
    ;;
scroll-lines)
    build_driver
    pid=$(require_pid)
    "$DRIVER" scroll-lines "$pid" "$@"
    ;;
type)
    build_driver
    pid=$(require_pid)
    "$DRIVER" type "$pid" "$@"
    ;;
key)
    build_driver
    pid=$(require_pid)
    "$DRIVER" key "$pid" "$@"
    ;;
logs)
    tail -n 100 "$LOG_FILE"
    ;;
trace)
    trace_grep 'ZEPHYR_GUI_TRACE' | tail -n 100
    ;;
scenarios)
    list_scenarios
    ;;
scenario)
    [[ $# -ge 1 ]] || { echo "usage: $0 scenario NAME [NAME...]" >&2; exit 2; }
    build_app
    require_permissions
    for name in "$@"; do
        run_scenario "$name"
    done
    ;;
smoke)
    build_app
    require_permissions
    scenario_smoke
    ;;
regression)
    build_app
    require_permissions
    scenario_regression
    ;;
*)
    usage
    exit 2
    ;;
esac
