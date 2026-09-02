# GUI testing on macOS

Zephyr includes a native GUI test harness for reproducing pointer, keyboard,
rendering, and window-management bugs. Generated applications, isolated user
configuration, logs, and screenshots are stored under `.artifacts/gui-test/`.
The harness never uses the normal `~/.config/zephyr` directory.

`ZEPHYR_GUI_STATE_DIR` overrides that directory. The harness also passes it to
the Zephyr it launches, where it namespaces the tab-transfer offer files that
otherwise live in the OS temp directory — so a test instance and a Zephyr you
are running yourself can never claim each other's dragged tabs.

## One-time permissions

Real input injection and window capture require macOS Accessibility and Screen
Recording permission for the terminal or agent hosting the test process.

```sh
make gui-test-permissions
```

If macOS opens Privacy & Security, enable the host application under both
**Accessibility** and **Screen & System Audio Recording**, then restart the host
application if macOS requests it. Confirm with:

```sh
./scripts/gui-test.sh permissions
```

Both values must be `true`.

macOS posts synthetic input into one global desktop event stream. Run only one
GUI automation session at a time, and avoid using the physical mouse or keyboard
while it is active. Separate HOME directories, PIDs, or agent workspaces do not
isolate native input events from each other.

## Reproducible debug session

```sh
make gui-test-build
make gui-test-launch
./scripts/gui-test.sh status
./scripts/gui-test.sh capture
./scripts/gui-test.sh logs
./scripts/gui-test.sh trace
make gui-test-stop
```

Automation agents or process supervisors that reap detached child processes can
keep the app attached to a managed foreground session instead:

```sh
./scripts/gui-test.sh run
```

The app is compiled with optimizations and inlining disabled (`-N -l`) and with
`GOTRACEBACK=all`. By default it opens `testdata/gui/mouse_fixture.go`, which has
tabs, Unicode text, folds, and enough lines for fractional scrolling tests. A
different fixture can be passed to `launch`.

## Input commands

Coordinates are local to the Zephyr window, so tests remain valid when the
window moves on screen.

```sh
./scripts/gui-test.sh click 300 160 left
./scripts/gui-test.sh click 40 160 right
./scripts/gui-test.sh drag 250 160 520 240 0.5
./scripts/gui-test.sh scroll 600 300 -180
./scripts/gui-test.sh scroll-lines 600 300 -3
./scripts/gui-test.sh type 'inserted by GUI test'
./scripts/gui-test.sh key s cmd
./scripts/gui-test.sh key escape
```

## Scenarios

The canned suites are named scenarios. List them, and run one or several by
name, with:

```sh
./scripts/gui-test.sh scenarios
./scripts/gui-test.sh scenario smoke
./scripts/gui-test.sh scenario regression smoke
```

Each scenario launches its own app, stops it on the way out, and states a pass
condition read off the app or the filesystem — the trace, a captured file's
bytes — never off a screenshot. `smoke` and `regression` remain as aliases for
`scenario smoke` and `scenario regression`.

`clipboard` drives copy, paste, paste over a full selection, two undos and a
save through real keystrokes. It runs against a copy of the fixture inside the
state dir, never the tracked one, and it saves and restores the pasteboard
around the run — including when the run fails part-way.

Run the end-to-end input and screenshot smoke test with:

```sh
make gui-test-smoke
```

It writes lossless PNG captures and easily previewed JPEG versions to
`.artifacts/gui-test/artifacts/`, and only changes the in-memory copy of the
fixture.

The broader regression program validates primary drag selection, ignored
secondary/middle clicks, retained fractional scroll offsets, Vim and Navigator
transitions, Markdown read-mode selection isolation, and Markdown Read/Edit
transitions:

```sh
make gui-test-regression
```

It produces lossless PNG evidence plus alpha-flattened JPEG previews under the
same artifact directory. The GUI driver composites captures onto an opaque
background before encoding JPEG; direct conversion can discard translucent Gio
layers and falsely resemble a partially rendered frame. `make baseline` runs
this regression program automatically when both required macOS permissions are
available.

Debug launches set `ZEPHYR_GUI_TRACE=1`. After every pointer event, Zephyr logs
a JSON record containing the event kind, button, local position, cursor and
selection state, viewport line/pixel offset, and text/tab drag state. Key
presses and committed text input log a second kind of record — `"kind":"Key"`
or `"kind":"Edit"` — carrying the active tab's path, the cursor position, the
line count, the modified flag, and `bufferHash`, a truncated SHA-256 of the
buffer's bytes. A scenario asserts what the document holds from that checksum
rather than from pixels; `trace_cursor` and `trace_buffer_hash` in
`scripts/gui-test.sh` read the latest of each. Normal application launches do
not emit this telemetry.

## Performance program

Run five isolated launch/first-frame samples with:

```sh
make perf
```

Enable native click/type/scroll/undo input, or a sustained edit/undo/scroll
soak, with:

```sh
ZEPHYR_PERF_INTERACT=1 make perf
ZEPHYR_PERF_RUNS=1 ZEPHYR_PERF_INTERACT=1 \
  ZEPHYR_PERF_SOAK_SECONDS=1800 make perf
```

Results are written below `.artifacts/perf/latest/`. Every invocation removes
old trace data before measuring. The summary separates window visibility,
Go-process start to first submitted frame, first-frame CPU time, steady-frame
CPU time, RSS, and soak growth. `gio_event_to_submit` starts when Gio dequeues
the earliest pending event and ends after frame submission; it does **not**
include device/OS queueing, compositor/display latency, or input-to-photon time.
Tracing itself performs one small JSONL write per submitted frame, so compare
results only with other traced runs on the same class of machine.

Interactive runs require Accessibility permission and fail if Zephyr does not
actually receive enough events. Optional maximums turn the report into a
regression gate:

```sh
ZEPHYR_PERF_INTERACT=1 \
ZEPHYR_PERF_MAX_FIRST_SUBMIT_MS=600 \
ZEPHYR_PERF_MAX_STEADY_FRAME_P95_MS=8 \
ZEPHYR_PERF_MAX_GIO_EVENT_TO_SUBMIT_P95_MS=16 \
ZEPHYR_PERF_MAX_RSS_MIB=260 make perf
```

All maximums default to zero (report only). Sample-count minimums remain
enforced whenever their scenario is enabled.
