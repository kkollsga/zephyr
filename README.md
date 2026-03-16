# Zephyr

A fast, GPU-accelerated text editor for macOS, written in Go.

<!-- ![Zephyr screenshot](screenshot.png) -->

## Features

- **GPU-accelerated rendering** via [Gio](https://gioui.org)
- **Tree-sitter syntax highlighting** for 17+ languages (Go, Python, JavaScript, TypeScript, Rust, C, C++, Java, Ruby, Lua, and more)
- **Tabbed editing** with a custom Chrome-style tab bar and macOS traffic light integration
- **Unsaved changes protection** — prompts before closing tabs, quitting, or clicking the close button
- **Native macOS integration** — app bundle, native Save/Save As dialogs, custom titlebar
- **Smart editing** — auto-pairing brackets/quotes, language-aware auto-indentation, soft-tab backspace
- **Undo/redo** with operation coalescing
- **Language selector** — switch syntax highlighting from the status bar
- **Configurable themes** — dark and light themes via JSON

### Coming soon

Architectural foundations exist for these features:

- Fuzzy file finder
- Command palette
- Find and replace
- Multiple cursors
- File tree sidebar
- File watching for external changes
- Lua plugin API

## Build

```
make build
```

### Run

```
make run ARGS=myfile.txt
```

### macOS App Bundle

```
make app
open Zephyr.app
```

### Test

```
make test
```

## Keyboard Shortcuts

| Shortcut | Action |
|---|---|
| Cmd+S | Save |
| Cmd+Shift+S | Save As |
| Cmd+T | New tab |
| Cmd+W | Close tab |
| Cmd+Q | Quit |
| Cmd+Z | Undo |
| Cmd+Shift+Z | Redo |
| Cmd+A | Select all |
| Cmd+C / Cmd+X / Cmd+V | Copy / Cut / Paste |

## License

[MIT](LICENSE)
