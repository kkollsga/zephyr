# Zephyr product improvement plan — 2026-07-09

## Current position

The expanded baseline now contains 356 deterministic tests, two fuzz targets,
20 benchmarks, native macOS input/capture scenarios, and 46.6% overall Go
statement coverage. Coverage is strong in the core paths that used to be most
risky: render 87.4%, fuzzy 100%, buffer 78.9%, editor 75.9%, navigator 73.0%,
and file I/O 62.4%. The application/controller layer is now 35.3%, up from
6.1% and above the 35% release target.

Additional testing found concrete product defects that the original baseline
missed: malformed diff input could panic, watcher shutdown could panic under
backpressure, failed Save As changed the editor path, failed saves leaked temp
files, Save As did not transfer file watching, inode-bound watches raced atomic
replacement, stale syntax highlighters could
survive a format change, Markdown selection omitted nested content, and reload
undo/redo could corrupt text. Each is now protected by a regression test.

The product is a credible beta candidate. Save integrity, application-layer
testability, large-edit scale, terminal installation, and measurable
responsiveness have completed implementation gates. Release confidence now
depends mainly on native workflow completeness, explicit external-change UX,
idle-memory reduction, accessibility, and crash recovery.

## Priority model

Work is ranked by user impact first, evidence/confidence second, and effort
third. A task is only complete when its acceptance gate is automated and runs
in CI or the reproducible macOS harness.

## Evidence-ranked remaining work

| Rank | Target | Why it is next | Release gate |
|---:|---|---|---|
| 1 | Native file lifecycle | Save/reload logic is covered headlessly, but platform dialogs and real filesystem outcomes remain the largest daily-workflow gap | Save As, overwrite, cancel, tag retention, clean reload, and dirty conflict pass 10 consecutive native runs |
| 2 | Idle and long-session memory | Small-file idle RSS remains around 160–200 MiB despite fixing the parse-count leak | Attribute retained memory by subsystem; no monotonic 30-minute soak growth; define and meet an idle-RSS budget |
| 3 | External-change conflict UX | A temporary notification does not give users a durable recovery choice | Persistent Reload / Keep Mine / Compare state with delete/recreate and rapid-write tests |
| 4 | Multi-window tabs and clipboard | Controller coverage cannot validate AppKit IPC, ownership transfer, or preservation of the real pasteboard | Reorder, overflow, tear-out, cross-window transfer, close-unsaved, and copy/cut/paste all assert observable native state and cleanup |
| 5 | Release artifact qualification | The terminal path is now testable locally; published DMGs still need clean-machine evidence | Remount DMG, verify both architectures/plist/signature/version, install/upgrade/rollback on a clean macOS account |
| 6 | Accessibility and recovery | These are release-quality requirements, not current crash/data-loss findings | VoiceOver/keyboard/reduced-motion review plus atomic unsaved-session recovery |

Do not spend the next cycle raising already-strong core package coverage merely
for a global percentage. New tests should close one of the platform-boundary
gaps above or enforce a measured performance/reliability budget.

## Now — release confidence (next 1–2 weeks)

### 1. Complete macOS save integrity

**Status: completed 2026-07-10.** Symlink targets, permissions, extended
attributes/Finder tags, temporary-file sync, directory sync, and injected
failure stages are covered. Residual filesystem semantics are documented in
the baseline.

Why it mattered: saving is irreversible user-data territory. The original
tests proved that atomic replacement lost extended attributes and replaced a
symlink instead of updating its target. The save path also lacked directory
sync and ignored permission-copy errors.

Delivered:

- Resolve symlink targets deliberately while retaining the editor's displayed
  path.
- Preserve relevant macOS metadata across atomic replacement, including Finder
  tags; define behavior for quarantine and other system attributes.
- Sync the parent directory after rename where supported.
- Propagate metadata-copy errors or surface an explicit warning.

Evidence:

- The two formerly skipped Darwin integrity regressions now run normally.
- Failure-injection tests cover write, sync, chmod/metadata, rename, and
  directory-sync stages.
- The focused save/symlink/metadata/atomic-reader/watcher suite passed 100
  repetitions (well over 1,000 filesystem cases) without leaked artifacts,
  metadata loss, watcher loss, or partial reader visibility.

### 2. Make the controller layer directly testable

**Status: completed 2026-07-10.** `cmd/zephyr` is at 35.3% coverage with
headless coverage for find/replace, input routing, save/overwrite, external
change, tabs, Markdown, Navigator, rendering state, and nil-window effects.

Why it mattered: `cmd/zephyr` began at 6.1% while the renderer was already at
87.4%. Most uncertainty was in command routing, overlays, tab lifecycle,
file-change decisions, and transitions tied to Gio objects.

Delivered:

- Extract pure controller operations for key commands, pointer gestures, save
  prompts, file-change decisions, and tab moves.
- Represent effects such as save, clipboard, dialog, invalidate, and open-file
  as injectable interfaces.
- Keep drawing and platform adapters thin; test state transitions without a
  window.

Evidence and remaining edge:

- `cmd/zephyr` coverage is 35.3%, above the 35% target.
- Tests cover command shortcuts, overlay priority, save-prompt outcomes,
  external-change branch, and tab-drag result.
- Focused repeated and race runs are clean; the full suite also passes ten
  shuffled repetitions. Native platform adapters remain the useful next edge.

### 3. Finish native macOS workflow automation

**Status: partially complete and now the highest-value remaining target.**
Pointer/mouse, fractional scroll, Vim, Navigator, and Markdown mode transitions
run against the real app. Save/external-change/tab decisions have headless
controller coverage, but the dialogs, clipboard, IPC, and multi-window outcomes
still need real native scenarios.

Why third: the current harness proves launch, primary/secondary/middle pointer
behavior, selection, fractional scrolling, Vim/Navigator transitions, and
Markdown Read/Edit/Read. The highest-value daily workflows remain observational
or unit-only.

Add serialized native scenarios for:

- Save As, overwrite confirmation, cancellation, and Finder-tag retention.
- Clean external reload and dirty-buffer conflict handling.
- Clipboard copy/cut/paste with exact restoration of the user's prior
  clipboard contents.
- Tab reorder, overflow-menu drag, tear-out, cross-window transfer, and window
  close with unsaved tabs.
- Find/replace, language changes, folding, word wrap, and theme changes in one
  long-lived session.

Acceptance gates:

- Each workflow asserts observable state, filesystem results, and a raw PNG;
  screenshots alone are not the oracle.
- Repeat the workflow suite 10 times from a clean isolated HOME.
- Guarantee process, temp-file, clipboard, and test-window cleanup on failure.

## Next — responsiveness and scale (weeks 3–5)

### 4. Redesign the large-file edit/index path

**Status: completed 2026-07-10 without a data-structure rewrite.** Exact line
index allocation, binary-searched edit bounds, and affected-suffix adjustment
met the target while retaining the simpler eager flat index.

Current evidence: the combined open/index/edit benchmarks are about 0.85 ms and
0.77 MiB, down from about 1.6 ms and 4.10 MiB. A 100 MiB edit plus immediate
undo is about 0.55 ms. Benchmark names explicitly distinguish combined setup
from edit-only measurements.

Implementation:

- Profile realistic edit sequences rather than one isolated giant operation.
- A chunked line index/balanced piece tree was evaluated but not justified by
  the measured result; the flat eager index remains simpler and fast enough.
- Add benchmarks for typing, paste, delete selection, undo, reload, and line
  lookup across 1 MB, 10 MB, and 100 MB documents.

Target status:

- Achieved: under 1 ms and 1 MiB for the combined large-edit benchmark.
- Achieved in the native interaction harness: steady frame P95 is below 8 ms;
  direct edit/undo on 100 MiB is about 0.55 ms.
- Achieved: index edits adjust the affected suffix and do not copy document
  text.

### 5. Add frame, input, launch, and memory telemetry

**Status: completed as a reproducible measurement facility; optimization
remains ongoing.** The harness records window visibility, first submitted
frame, startup and steady frame CPU time, Gio event-to-submit time, RSS, and
sustained soak samples. It rejects stale/empty evidence and supports regression
thresholds. The event metric deliberately does not claim OS/HID/display
input-to-photon latency.

Current evidence is recorded in the baseline and `.artifacts/perf/`: window
visibility and first-submit are measured separately, frame and Gio event timing
have P50/P95/P99 distributions, and RSS is sampled throughout native interaction
soaks. The completed 30-minute run produced 102,060 steady frames and 4,429
delivered-event samples: steady-frame P95 was 1.939 ms, Gio event-to-submit P95
was 0.675 ms, and RSS settled from a 203 MiB startup peak to 108–110 MiB. The
second-half RSS trend remained mildly positive at about 2.6 MiB/hour.

Delivered:

- Timestamp launch, Gio event receipt, event/draw CPU work, and frame submission
  in test builds. GPU/display completion remains outside this harness.
- Record P50/P95/P99 latency during typing, selection, scrolling, Markdown, and
  Navigator scenarios.
- Run 30-minute soak tests with file switching, parsing, edits, undo/redo, and
  mode changes; graph RSS and native allocations.

Targets:

- P95 Gio event-to-submit is below 16 ms; five isolated launches averaged
  258.1 ms to first submit (229.3 ms after the first run), meeting the 300 ms
  warm target.
- No runaway memory growth over a 30-minute soak; investigate the remaining
  ~2.6 MiB/hour fitted warm trend before calling it fully flat.
- Reduce idle RSS toward 100 MiB before making a lightweight-editor claim.

### 6. Design explicit external-change conflict UX

Today, clean buffers reload automatically and dirty buffers receive a temporary
notification. There is no persisted conflict state, comparison, merge, or
clear recovery workflow.

Deliverables:

- Persistent Reload / Keep Mine / Compare actions for dirty buffers.
- Detect file deletion, rename, recreation, and repeated external writes.
- Preserve undo history and cursor/selection across every accepted reload.

Acceptance gates:

- Table-driven controller tests for all disk/buffer state combinations.
- Native tests for clean reload, dirty conflict, delete/recreate, and rapid
  successive atomic saves.

## Later — release polish (weeks 6+)

- Cold-start and packaging tests on a clean macOS account and a second Mac
  architecture/configuration.
- Accessibility labels, keyboard-only navigation, reduced-motion behavior,
  high-contrast themes, and VoiceOver review.
- Crash recovery/session restoration with atomic journals for unsaved buffers.
- Project-scale Navigator tests and cancellation for slow git/file operations.
- Coverage for IPC and clipboard adapters, currently 0%.
- Fuzz targets for editor command sequences, Markdown selection coordinates,
  status parsing, and tab-layout calculations.

## Continuous test program

### Every pull request

- Full normal tests, race detector on macOS/Linux, vet, Staticcheck, build, and
  coverage profile.
- Deterministic property tests plus 10-second PieceTable and diff fuzz smoke.
- Focused race stress for PieceTable, watcher shutdown, reload history, and
  controller gesture state.
- Benchmark compile/run smoke with hard allocation ceilings for fuzzy matching
  and any newly optimized edit path.

### Nightly

- Five-minute fuzz runs per target with cached corpora.
- 20 shuffled race repetitions of high-risk packages.
- Ten native macOS workflow repetitions.
- 30-minute edit/parse/mode-change memory and frame-latency soak.
- Benchmark history with alerts at >10% time or allocation regression.

### Release candidate

- Clean-profile install, first launch, open/save/reload/conflict, multi-window,
  Finder metadata, clipboard, crash recovery, and uninstall residue review.
- 100 MB file open/edit/search/save test and a large project navigation test.
- Zero known user-data-loss defects, zero flaky required gates, and no skipped
  platform-integrity regressions.

## Milestones

| Milestone | Exit criteria |
|---|---|
| Beta-ready | Save integrity fixed; native file lifecycle green 10×; no required flaky tests |
| Performance beta | P95 Gio event-to-submit <16 ms; large edit <1 ms/<1 MiB; 30-minute memory soak has no runaway growth |
| Release candidate | `cmd/zephyr` ≥35%; full macOS workflow matrix; crash recovery; no skipped integrity tests |
