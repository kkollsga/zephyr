<p align="center">
  <img src="assets/icon.svg" width="128" height="128" alt="Zephyr icon"/>
</p>

<h1 align="center">Zephyr</h1>
<p align="center"><strong>The caffeinated editor</strong></p>

<p align="center">
  A fast, GPU-accelerated text editor written entirely in Go.<br/>
  Powered by <a href="https://gioui.org">Gio</a> for buttery-smooth rendering and <a href="https://tree-sitter.github.io/tree-sitter/">Tree-sitter</a> for precise syntax highlighting.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-blue?style=flat-square" alt="macOS | Windows | Linux"/>
  <img src="https://img.shields.io/badge/language-Go-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="MIT License"/>
  <img src="https://img.shields.io/badge/rendering-GPU-4ec9b0?style=flat-square" alt="GPU Accelerated"/>
  <a href="https://github.com/kkollsga/zephyr/releases/latest"><img src="https://img.shields.io/github/v/release/kkollsga/zephyr?include_prereleases&style=flat-square&label=download" alt="Latest Release"/></a>
</p>

---

<p align="center">
  <img src="assets/screenshots/hero-dark.svg" width="800" alt="Zephyr editor — dark theme"/>
</p>

## Why Zephyr?

Most editors are either fast and ugly, or pretty and slow. Zephyr is both — a native editor that renders every frame on the GPU while staying lightweight and responsive. No Electron, no web views, no compromises.

Written from scratch in Go, Zephyr starts instantly, uses minimal memory, and runs on macOS, Windows, and Linux with a native feel on each platform.

## Features

- **GPU-accelerated rendering** — every pixel drawn on the GPU via [Gio](https://gioui.org), delivering smooth scrolling and instant response
- **Tree-sitter syntax highlighting** — accurate, incremental parsing for 17+ languages including Go, Python, JavaScript, TypeScript, Rust, C, C++, Java, Ruby, Lua, and more
- **Cross-platform** — native experience on macOS (custom titlebar, traffic lights, Finder tags), Windows (native window chrome, file dialogs), and Linux
- **Tabbed editing** — Chrome-style tab bar with drag-to-reorder, overflow dropdown, and unsaved-changes indicators
- **Code folding** — collapse and expand blocks with a color-coded line count indicator
- **Markdown preview** — rendered markdown with code blocks, task list checkboxes, tables, and copy buttons
- **Smart editing** — auto-pairing brackets and quotes, language-aware indentation, soft-tab backspace
- **Undo/redo** — with intelligent operation coalescing so each undo step feels natural
- **Find and replace** — inline search with regex and case-sensitive modes
- **Dark and light themes** — configurable via YAML, toggled with a click in the tab bar
- **Word wrap** — toggle via Option+Z, with visual line mapping
- **Language selector** — switch syntax highlighting from the status bar

<details>
<summary><strong>Dark and light themes</strong></summary>
<br/>
<p align="center">
  <img src="assets/screenshots/hero-dark.svg" width="420" alt="Dark theme"/>
  &nbsp;&nbsp;
  <img src="assets/screenshots/hero-light.svg" width="420" alt="Light theme"/>
</p>
</details>

<details>
<summary><strong>Markdown preview</strong></summary>
<br/>
<p align="center">
  <img src="assets/screenshots/markdown-preview.svg" width="700" alt="Markdown preview mode"/>
</p>
</details>

### Roadmap

Architectural foundations exist for these features:

- Fuzzy file finder
- Command palette
- Multiple cursors
- File tree sidebar
- File watching for external changes
- Lua plugin API

## Installation

Download the latest release from the [releases page](https://github.com/kkollsga/zephyr/releases/latest).

### macOS

Download the `.dmg` file, open it, and drag **Zephyr.app** to your Applications folder.

> **Gatekeeper warning:** macOS blocks apps that aren't signed with an Apple Developer ID. After dragging Zephyr to Applications, open Terminal and run:
> ```
> xattr -cr /Applications/Zephyr.app
> ```
> Then open the app normally. You only need to do this once.

Or build from source:

```bash
make app
open Zephyr.app
```

### Windows

**Installer** — Download the `-setup.exe` and run it. Includes optional desktop shortcut, "Add to PATH", and "Open with Zephyr" context menu. Uninstaller included.

**Portable** — Download the `-windows-amd64.zip`, extract anywhere, and run `zephyr.exe`. No installation needed.

Or build from source (requires GCC/MinGW for tree-sitter):

```bash
go build -o zephyr.exe ./cmd/zephyr
```

### Linux

**Debian/Ubuntu:**

```bash
sudo dpkg -i zephyr_*_amd64.deb
```

**Fedora/RHEL:**

```bash
sudo rpm -i zephyr-*-1.x86_64.rpm
```

**AppImage** — no installation required:

```bash
chmod +x Zephyr-*.AppImage
./Zephyr-*.AppImage
```

**Tarball:**

```bash
tar xzf zephyr-*-linux-amd64.tar.gz
sudo cp zephyr-*/zephyr /usr/local/bin/
```

**Build from source** (requires gcc, pkg-config, and Gio dependencies):

```bash
sudo apt install gcc pkg-config libwayland-dev libx11-dev libx11-xcb-dev \
  libxkbcommon-x11-dev libgles2-mesa-dev libegl1-mesa-dev libffi-dev \
  libxcursor-dev libvulkan-dev
make build
```

## Building from Source

Requires Go 1.22+ and a C compiler (for tree-sitter).

```bash
make build          # native build
make test           # run tests
make bench          # run benchmarks
make app            # macOS .app bundle
./zephyr --version  # check version
```

## Keyboard Shortcuts

| Shortcut | Action |
|---|---|
| `Cmd+S` | Save |
| `Cmd+Shift+S` | Save As |
| `Cmd+T` | New tab |
| `Cmd+W` | Close tab |
| `Cmd+Q` | Quit |
| `Cmd+Z` | Undo |
| `Cmd+Shift+Z` | Redo |
| `Cmd+A` | Select all |
| `Cmd+C` / `Cmd+X` / `Cmd+V` | Copy / Cut / Paste |
| `Cmd+F` | Find |
| `Cmd+Shift+F` | Find and replace |
| `Option+Z` | Toggle word wrap |

## Architecture

Zephyr is built with a clean separation of concerns:

```
cmd/zephyr/     Main application, UI layout, and event loop
internal/
  buffer/       Piece table data structure for efficient text editing
  editor/       Core editor state — cursor, selection, undo history
  highlight/    Tree-sitter integration for syntax highlighting
  render/       GPU rendering — text, gutter, cursors, markdown, scrollbar
  ui/           UI components — tabs, find bar, language selector, status line
  config/       Themes, fonts, and configuration
  plugin/       Lua plugin API framework
```

Platform-specific code is isolated via Go build tags:

```
cmd/zephyr/
  titlebar_darwin.go      macOS traffic lights, native menus, Cocoa integration
  titlebar_windows.go     Windows native chrome (Decorated=true)
  titlebar_other.go       Linux/other fallback stubs
  platform_darwin.go      macOS file dialogs, Finder tags
  platform_windows.go     Win32 file dialogs via syscall
  platform_other.go       Fallback stubs
pkg/clipboard/
  clipboard_darwin.go     macOS pasteboard via AppKit
  clipboard_windows.go    Win32 clipboard via syscall
  clipboard_other.go      xclip/xsel fallback
```

## License

[MIT](LICENSE)
