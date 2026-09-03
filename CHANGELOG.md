# Changelog

All notable changes to Zephyr are documented here.

## Unreleased

### Files

- An external change to an open file is now noticed while the window sits
  idle. Previously the watcher's events were only read on the next frame, so
  a file changed while Zephyr was in the background went unreported until
  something else redrew the window.
- External-change detection now covers every open tab. Only the file the
  window started with was registered with the watcher, so a change to a file
  opened later — from the file finder, the navigator, or a tab dragged in from
  another window — was never reported, and a save over it was never questioned.
- A tab dragged out and dropped where no other Zephyr window claimed it no
  longer leaves a plaintext copy of the buffer behind in the temp directory.
- A file deleted or moved out from under an open tab is reported as deleted
  and the buffer is kept as unsaved work, instead of being announced as
  "Reloaded:" while the tab silently held the only remaining copy.
- A reload that fails now says so instead of reporting a successful reload.
- Saving no longer produces a spurious "File changed externally" warning for
  Zephyr's own write.
- Saving a file that changed on disk while the buffer had unsaved edits no
  longer overwrites that change without asking. Zephyr now offers Overwrite,
  Reload, or Cancel; Return and Escape both cancel, so neither key can be the
  one that loses a version. Reload is undoable — one Cmd+Z brings your text
  back.
- An unresolved external change is now a standing badge — an amber dot on the
  tab and "changed on disk" (or "deleted on disk") in the status bar — instead
  of a notification that expired after ten seconds and left no trace.
- Refusing a save during quit now leaves the tab open and the quit
  unfinished, rather than closing a tab whose file was never written.
- Windows: saving a file that another program has open no longer fails with
  "Access is denied". The replacement is retried for up to half a second while
  the other reader holds the file.
- Save As on an untitled buffer now opens in the folder your last save went
  to, remembered across launches. A buffer that already has a file still saves
  next to that file.
- The Save As overwrite prompt now answers to the keyboard: Return overwrites,
  Escape goes back to the filename with the existing file untouched. Return
  used to re-raise the same prompt and Escape closed the whole menu.

### Find

- The find bar now gives the keyboard back when you click in the text. The bar
  stays open with its matches highlighted, its field goes dim, and typing edits
  the file again. Cmd+F or a click in the bar's field takes focus back; Escape
  still closes it.
- Your text selection is drawn again while the find bar is open but not
  focused. It used to disappear the moment the bar appeared, so selecting text
  with matches on screen showed nothing.
- The current search match now carries an outline, and the dark theme's other
  matches are a deeper amber. Match, current match and selection are three
  different things and now look like it.

### Navigator

- Navigator keys with nothing behind them no longer report success. `go`,
  `gi`, `g?`, `<Space>f` and `<Space>b` used to be accepted and silently do
  nothing, so `go` in particular claimed to show HEAD content while you were
  still reading your own buffer. They are inert until each one lands.
- The vim tag text object `t` is no longer accepted: `dit` used to parse into
  a delete that could not be carried out and looked like an edit that had
  happened.
- Documentation: `<Space>e` (open the project root) was marked planned but has
  always worked; the keys that genuinely do not exist are now listed as
  unavailable rather than planned.
- `<Space>f` and `<Space>b` now open a fuzzy file finder: all files under the
  project root, or only the files git reports as changed. Type to filter,
  Up/Down or Ctrl-p/Ctrl-n to move, Enter or a click to open, Escape or a click
  outside to close. It never opens over the save menu, the language selector, a
  focused find bar or the root-folder dropdown. The overlay opens immediately
  and scans the project in the background, so a large tree no longer freezes
  the window, and every open rescans, so a deleted or renamed file is listed
  correctly.
- `g?` now works: it lists every navigator key binding in the finder overlay,
  filterable by typing, with `Enter` or `Escape` closing it. The list is
  generated from a table in the code that a test checks in both directions
  against the key handler, so a binding cannot change without the help
  changing with it.
- Documentation: `<Space>r` (toggle the markdown read view) has always worked
  but was not listed anywhere; `Ctrl-o` and `Ctrl-i` were listed as jumplist
  keys but no jumplist exists, and they are now recorded as unavailable
  alongside `gi`.
- `<Space>f` inside a git repository now lists the files git lists, so
  `.gitignore` is honoured and a build directory no longer fills the results.
  Tracked and untracked-but-not-ignored files are offered; a file deleted from
  the working tree but still in the index is not. Outside a repository the
  previous directory walk still applies.
- `go` now shows the file's committed content in place: same tab, same cursor
  line, ` (HEAD)` on the tab title and on the navigator breadcrumb, and the
  gutter's diff markers and the header's diff stats hidden because they
  describe the working file. The view is read-only — typing,
  pasting, vim edits, undo, redo, Save and Save As are all refused, so
  committed content can never be written to disk — and `go` again brings
  the buffer back exactly as it was, unsaved edits, cursor and undo history
  included. A file with nothing at HEAD is reported instead of swapped.
- Refreshing a directory listing or the git status buffer (toggling hidden
  files, staging, collapsing a section) now resets the state derived from the
  replaced listing, so folds and the viewport's cursor tracking describe the
  listing on screen.
- The `ih` hunk text object now works: `dih`, `yih` and `cih` operate on the
  run of changed lines under the cursor, linewise and as one undo step. It
  covers the changed lines only — a context line inside a hunk is not part of
  the object, and with the cursor on an unchanged line the keys do nothing
  rather than reaching for the whole file.
- `gf` inside a git repository now falls back to the open file's own directory
  when the name does not exist at the repo root. The root candidate was tested
  with a call that accepts any path, so it always won and `gf` on a sibling
  file did nothing.
- Lines deleted from the working file are now marked in the gutter. A red
  wedge sits on the surviving line at the boundary — the line after the
  deletion, or the line before it when the deletion runs to the end of the
  file — so a delete-only change (a formatter, a `git checkout`) no longer
  shows an empty gutter.
- Switching Navigator Mode on now fills in the git gutter for the tabs
  already open. Their diff was looked up before the navigator created the git
  cache, so the markers and the header's diff stats stayed blank until a save
  or an external change happened to refresh them.

### Markdown

- Markdown read mode now renders color emoji on macOS, including ZWJ
  sequences (👨‍👩‍👧‍👦), flags, and variation selectors, in body text, headings,
  and code blocks.

### Editing

- Clicking the right or middle mouse button during a drag no longer
  interrupts it. A secondary button pressed mid-drag collapsed a text
  selection to the pointer and restarted it there, or grabbed a different
  tab; releasing that button ended the drag outright, committing a tab move
  wherever the pointer happened to be rather than where the tab was dropped.
- Vim `r`, `>>`, `<<`, and `J` are now undoable. They edited the buffer
  behind the undo history's back, which also corrupted every older undo
  step, so an undo after one of them could damage unrelated text.
- Vim `J` on a line containing non-ASCII text no longer leaves a fragment of
  that line behind.
- A keyboard shortcut pressed while a vim key is waiting for its argument now
  reaches the app instead of being eaten as that argument. On Windows and Linux
  `r` followed by Ctrl+S replaced the character under the cursor with an "s"
  and never saved; on macOS Cmd+S went through but left the `r` pending, so it
  swallowed the next key typed. The same holds for `f`/`t`/`F`/`T`, a pending
  operator, `i`/`a` waiting for a text object, and the `g`/`z`/`[`/`]` and
  `<Space>` sequences.
- Replacing a selection — typing over it, pasting over it, Replace and
  Replace All in the find bar, or a visual-mode put — is now a single
  undoable step that restores the original text, instead of an undo that
  deleted the new text and left the old text lost. Replace All is one undo
  for the whole run.
- Replacing a selection with an empty replacement now deletes the selection
  instead of doing nothing.
- A multi-cursor insert or backspace, a bracket-pair delete, an indent
  change, and a markdown checkbox toggle are each one undo step.
- Undo and redo no longer desynchronise from the buffer when an edit is
  refused: the action stays on the stack it came from instead of moving
  across, so later undos keep applying at the right offsets.
- Undoing a format (or any whole-buffer replacement, such as a reload) now
  re-highlights, recomputes fold regions, and re-derives the find bar's match
  positions immediately, instead of leaving syntax highlighting, folds, and
  search matches describing the replaced text.
- Undo and redo now drop extra cursors, which pointed at positions that need
  not exist in the restored document.
- Vim `cc` now clears the line and opens insert on it, as vim does, instead of
  deleting the line and starting insert on the following one.
- Vim `yy` and `dd` on the last line of a file now fill the register linewise.
  The line break was recorded ahead of the text rather than after it, so `p`
  and `P` pasted the last line into the middle of the current one instead of
  onto a line of its own.
- Typing or deleting on a line containing non-ASCII characters no longer
  risks landing the edit at the top of the file: a cursor column past the end
  of such a line is clamped to the line's last character instead of resolving
  to offset 0.
- Soft-tab backspace measures the indentation before the cursor by character
  rather than by byte, and leaves the cursor in the right column afterwards on
  a line with non-ASCII text.
- An edit that changes nothing no longer leaves an undo step behind: typing or
  pasting exactly what is already selected, overwriting a character with
  itself, and an empty multi-cursor insert each used to add a Cmd+Z that
  restored nothing.
- Added syntax/format error detection with red gutter markers: JSON is
  validated with the standard parser and tree-sitter languages are checked
  for parse errors, on every Enter and after five seconds of typing
  inactivity, as well as on open and language switch.
- Added an Auto Indent mode (View menu, on by default): pressing Enter in
  JSON, JavaScript, and TypeScript files re-indents the line being left and
  indents the new line from token-aware bracket depth, leaving strings,
  template literals, and comments untouched.
- Added a configurable indentation level (View → Indentation: 2, 4, or
  8 spaces, default 2), persisted across sessions.
- Added a Compact/Expanded toggle in the status bar for JSON files that
  converts the buffer between minified and pretty-printed form in place, so
  saving stores the selected form; the conversion preserves key order and is
  a single undo step.
- Vim text objects now work in visual mode: `viw`, `va"`, `vi(`, `vih` and the
  rest select the object so you can see it before acting on it. The selection
  covers exactly what the operator form would take, so `viw` then `d` deletes
  what `diw` deletes; `vih` selects the changed lines of the hunk under the
  cursor line-wise. Previously the `i` was discarded and the delimiter ran as
  its own command — `vip` pasted over the selection and `vix` deleted it.
- Vim `gg` now works in visual mode. The second `g` re-armed the `g` prefix
  instead of completing the motion, so the selection never reached the start
  of the file.

### Cross-platform

- Vim mode on Windows and Linux now honours its Ctrl bindings. Gio reports the
  platform shortcut modifier as Ctrl off macOS, and every Ctrl key was being
  discarded as a host accelerator, so Ctrl+d, Ctrl+u, Ctrl+f, Ctrl+b, Ctrl+r
  and Ctrl+v did nothing.
- Application shortcuts (new tab, close tab, save, find, …) now work while vim
  mode is active on every platform. Vim swallowed them instead of passing them
  to the host as intended, so on macOS Cmd+T and friends were dead in vim mode.
- The navigator no longer mistakes the repository root for a different
  directory on Windows: git reports the root with forward slashes, which never
  compared equal to the paths Windows hands the editor.
- Alternate-file navigation now finds the sibling test or implementation file
  inside directories whose names contain `[`, `*` or `?` — Next.js route
  folders such as `[id]` were read as filename patterns and reported the
  existing sibling as missing.
- The file finder on Windows now lists and matches paths with forward slashes
  instead of backslashes, so typing `src/ed` finds `src\editor.go` the way it
  does everywhere else. The file still opens through the platform path.

### macOS

- The Zephyr application menu now shows the current version as its first row.

### Versioning

- The repository root `VERSION` file is now the version source of truth for
  local builds, with `scripts/bump-version.sh` to bump patch/minor/major.
- Pushing a `VERSION` change to `main` now automatically tags `vX.Y.Z` and
  dispatches the release pipeline, so installers pick up new versions
  without a manual tag.

### Reliability and testing

- Fixed primary/secondary/middle mouse isolation, pointer-gesture identity,
  fractional scrolling, tab-aware hit testing, and Markdown selection routing.
- Preserved symlink targets, permissions, and macOS extended attributes during
  atomic saves; added parent-directory watching and sync/failure coverage.
- Added 356 deterministic tests, two fuzzers, race/stress gates, headless
  controller coverage, and a real macOS input/screenshot regression harness.

### Performance

- Eliminated retained Tree-sitter trees across reparses.
- Reduced the combined large-document open/index/edit workload from roughly
  1.6 ms and 4.10 MiB to 0.85 ms and 0.77 MiB on the baseline Apple M4.
- Added repeatable first-frame, frame CPU, event-to-submit, RSS, and soak
  measurement tooling.

### Installation

- Added a terminal-first macOS installer/upgrader with checksum verification,
  transactional rollback, a `zephyr` command, and universal arm64/x86_64
  release builds.
- Added a dedicated GitHub Pages installation guide at `/install/` and CI
  checks that keep its primary command aligned with the README and installer.
- Replaced manual quarantine instructions with the supported one-command
  install/upgrade path and added copy buttons to the Pages install surfaces.

## [0.1.0-alpha] — 2026-03-25

First public pre-release.

### Cross-Platform Support
- Windows support: native window chrome, Win32 file dialogs (Save As, folder picker), clipboard via syscall
- Linux support: X11/Wayland via Gio, xclip/xsel clipboard, freedesktop .desktop entry
- Platform-specific code isolated via Go build tags (`_darwin.go`, `_windows.go`, `_other.go`)
- macOS: custom titlebar with traffic lights, Cocoa menus, Finder tags — all unchanged

### CI/CD & Installers
- GitHub Actions CI: vet, test, and build on macOS, Windows, and Linux
- Automated release workflow triggered by git tags
- macOS: DMG installer with drag-to-Applications layout
- Windows: Inno Setup installer (desktop icon, PATH, context menu) + portable zip
- Linux: .deb, .rpm (via nfpm), AppImage, and tarball
- Build-time version injection via ldflags (`--version` flag)

### Code Folding
- Collapse/expand bracket-delimited blocks (`{}`, `[]`, `()`) by clicking the gutter
- Recursive collapse/expand with Cmd+click
- Color-coded superscript line count indicator on collapsed blocks (green/orange/red)
- Hidden lines excluded from display, cursor clamped out of collapsed regions

### Markdown Preview
- Rendered markdown with headers, code blocks, tables, and blockquotes
- Task list checkboxes (interactive — click to toggle)
- Code block copy buttons
- Text selection in preview mode
- Edit/Read toggle in the status bar

### Themes & Appearance
- Theme bundle system with dark and light variants
- Configurable via YAML theme files
- Sun/moon toggle in the tab bar to switch between dark and light mode
- Native macOS View > Theme menu for switching theme bundles
- Window background color synced to theme

### Word Wrap
- Toggle via Option+Z or View > Word Wrap menu
- Visual line mapping preserves buffer line numbers in the gutter
- Persisted in settings

### Editor Core
- Piece table buffer for efficient text editing
- Undo/redo with intelligent operation coalescing
- Auto-pairing brackets and quotes
- Language-aware indentation and soft-tab backspace
- Find and replace with regex and case-sensitive modes
- Language selector in the status bar

### Syntax Highlighting
- Tree-sitter incremental parsing for 17+ languages
- Go, Python, JavaScript, TypeScript, Rust, C, C++, Java, Ruby, Lua, HTML, CSS, JSON, YAML, Markdown, Bash, SQL

### Tab Management
- Chrome-style tab bar with drag-to-reorder
- Overflow dropdown for many open tabs
- Unsaved-changes indicator (dot)
- Tab drag-out to spawn new window (macOS)
- Tab tooltip for clipped titles

### macOS Integration
- Custom titlebar with native traffic light buttons
- Native Save/Save As dialogs via AppleScript
- Finder tag support in save menu (7 color tags)
- "Open With" via Apple Events and drag-onto-dock
- macOS .app bundle with code signing

### Infrastructure
- GPU-accelerated rendering via Gio (Metal on macOS, Direct3D on Windows, OpenGL/Vulkan on Linux)
- Piece table data structure with comprehensive test suite
- Fuzzy matcher for future file finder/command palette
- Lua plugin API framework (foundation)
- IPC for cross-instance tab transfer (macOS)
- Scrollbar with proportional thumb

[0.1.0-alpha]: https://github.com/kkollsga/zephyr/releases/tag/v0.1.0-alpha
