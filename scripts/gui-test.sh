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
    : >"$LOG_FILE"
    : >"$STDOUT_LOG"
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
    : >"$LOG_FILE"
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

# trace_cursor prints LINE:COL from the most recent trace record that carries a
# cursor position.
trace_cursor() {
    grep 'ZEPHYR_GUI_TRACE' "$LOG_FILE" | tail -n 1 | \
        sed -E -n 's/.*"cursorLine":([0-9]+),"cursorCol":([0-9]+).*/\1:\2/p'
}

# trace_buffer_hash prints the buffer checksum from the most recent key or edit
# trace record, so a scenario can assert what the buffer holds without reading
# pixels. Empty when no such record has been emitted yet.
trace_buffer_hash() {
    grep -o '"bufferHash":"[0-9a-f]*"' "$LOG_FILE" | tail -n 1 | \
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

SCENARIOS=(smoke regression clipboard)

list_scenarios() {
    printf '%s\n' "${SCENARIOS[@]}"
}

run_scenario() {
    local name=$1
    local known
    for known in "${SCENARIOS[@]}"; do
        if [[ "$known" == "$name" ]]; then
            "scenario_${name//-/_}"
            return
        fi
    done
    echo "unknown scenario: $name" >&2
    echo "known scenarios: ${SCENARIOS[*]}" >&2
    return 2
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
    grep -q '"kind":"Press"' "$LOG_FILE" || { echo "no pointer press was recorded" >&2; exit 1; }
    grep -q '"selection":true' "$LOG_FILE" || { echo "no drag selection was recorded" >&2; exit 1; }
    grep -q '"kind":"Scroll"' "$LOG_FILE" || { echo "no scroll event was recorded" >&2; exit 1; }
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
    grep -q '"kind":"Drag".*"selection":true' "$LOG_FILE" || { echo "drag selection failed" >&2; exit 1; }
    "$DRIVER" scroll "$pid" 600 300 -7
    sleep 0.3
    grep -Eq '"kind":"Scroll".*"viewportOffset":[1-9][0-9]*' "$LOG_FILE" || { echo "fractional pixel scroll was not retained" >&2; exit 1; }
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
    md_drag=$(grep 'ZEPHYR_GUI_TRACE' "$LOG_FILE" | grep '"kind":"Drag"' | tail -n 1)
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
    grep 'ZEPHYR_GUI_TRACE' "$LOG_FILE" | tail -n 100
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
