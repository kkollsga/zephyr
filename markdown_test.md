# Zephyr

A fast, GPU-accelerated text editor for macOS, written in Go.

## Features

- [x] GPU-accelerated rendering via Gio
- [x] Tree-sitter syntax highlighting for 17+ languages
- [x] Tabbed editing with macOS integration
- [ ] Fuzzy file finder
- [ ] Lua plugin API

## Quick Start

```bash
# Build and run
make build
open Zephyr.app
```

```go
// fibonacci returns the nth Fibonacci number.
func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    a, b := 0, 1
    for i := 2; i <= n; i++ {
        a, b = b, a+b
    }
    return b
}
```

```
Plain code block without a language tag
should still render cleanly
```

## Design Philosophy

> Zephyr is designed to be fast, lightweight, and native to macOS.
> Built with Go and GPU-accelerated via the Gio framework.

Zephyr uses **GPU-accelerated rendering** via the *Gio* framework, ensuring smooth scrolling and responsive editing even with large files. Inline `code spans` are also styled.

## Architecture

| Package | Description |
|---------|-------------|
| `cmd/zephyr` | Main application entry point |
| `internal/render` | GPU rendering pipeline |
| `internal/config` | Theme and configuration |
| `internal/editor` | Buffer and cursor logic |

### Nested Lists

- First level item
  - Second level nested
  - Another nested item
- Back to first level

---

### Heading Level 3

#### Heading Level 4

##### Heading Level 5

###### Heading Level 6

Regular paragraph after various heading levels.
