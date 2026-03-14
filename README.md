# Zephyr

The caffeinated editor. A fast, lightweight terminal text editor written in Go.

## Features

- GPU-accelerated rendering via [Gio](https://gioui.org)
- Tree-sitter syntax highlighting
- Fuzzy file finder
- Command palette
- Find and replace
- Multiple cursors
- File tree sidebar
- Tabs and split views
- Configurable keybindings and themes
- Lua plugin support
- File watching for external changes
- macOS app bundle support

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
```

### Test

```
make test
```

## Configuration

- Keymaps: `keymaps/default.json`
- Themes: `themes/dark.json`, `themes/light.json`

## License

[MIT](LICENSE)
