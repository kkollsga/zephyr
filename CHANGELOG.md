# Changelog

All notable changes to Zephyr are documented here.

## Unreleased

### Markdown

- Markdown read mode now renders color emoji on macOS, including ZWJ
  sequences (👨‍👩‍👧‍👦), flags, and variation selectors, in body text, headings,
  and code blocks.

### Editing

- Vim `r`, `>>`, `<<`, and `J` are now undoable. They edited the buffer
  behind the undo history's back, which also corrupted every older undo
  step, so an undo after one of them could damage unrelated text.
- Vim `J` on a line containing non-ASCII text no longer leaves a fragment of
  that line behind.
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
