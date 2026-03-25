# Changelog

All notable changes to Zephyr are documented here.

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
