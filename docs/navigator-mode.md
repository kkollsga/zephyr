# Navigator Mode

Navigator Mode is a git-centric navigation system for reviewing changes. Toggle it with **Cmd+Shift+N** (macOS) or **Ctrl+Shift+N** (Windows/Linux).

When active, the tab bar is replaced by a breadcrumb and all navigator keybindings become available.

---

## Quick Start

```
Cmd+Shift+N       Toggle Navigator Mode on/off
<Space>c           Next changed hunk
<Space>C           Previous changed hunk
<Space>n           Next changed file (opens it, lands on first hunk)
<Space>N           Previous changed file
<Space>g           Open git status buffer
-                  Open parent directory as buffer
ga                 Jump to alternate file (test <-> implementation)
```

---

## Git Change Navigation

When Navigator Mode is active and you're in a git repository, the gutter shows colored markers on changed lines:

- **Green bar** — added line
- **Blue bar** — modified line (replaces a deleted line)

Changed lines also get a subtle background highlight.

### Hunk Navigation

| Key | Alt Key | Action |
|-----|---------|--------|
| `<Space>c` | `]c` | Next hunk in current file (wraps around) |
| `<Space>C` | `[c` | Previous hunk |
| `<Space>n` | `]C` | Next changed file, cursor on first hunk |
| `<Space>N` | `[C` | Previous changed file |

The `<Space>` leader keys are the recommended bindings — they work on all keyboard layouts. The `]`/`[` bracket variants are also available for US keyboard users.

Counts work: `3<Space>c` jumps forward 3 hunks.

---

## Status Buffer

Press `<Space>g` to open the git status buffer. This shows all changed files grouped by status:

```
Head:     main (abc1234)
Upstream: origin/main (ahead 2, behind 0)

Unstaged changes (2)  +15 -3
  M  internal/vim/normal.go           +10 -2
  M  cmd/zephyr/draw.go              +5 -1

Staged changes (1)  +20
  A  internal/navigator/git.go       +20

Untracked files (1)
  ?  scratch.go
```

### Status Buffer Keys

| Key | Action |
|-----|--------|
| `j`/`k` | Move between entries |
| `n`/`N` | Jump to next/previous section header |
| `Enter` | Open file under cursor |
| `s` | Stage file under cursor |
| `u` | Unstage file under cursor |
| `x` | Discard changes (destructive) |
| `=` | Toggle inline diff for file under cursor |
| `Tab` | Collapse/expand section |
| `R` | Refresh status |
| `q` | Close status buffer |

---

## Directory Buffer

Press `-` to open the parent directory as a navigable buffer:

```
internal/vim/
────────────────────────────────────────
  action.go
M normal.go                    +22 -4
A navigator.go                 +45
  operator.go
```

Directories are sorted first, then files alphabetically. Git status markers and diff stats appear for changed files.

### Directory Buffer Keys

| Key | Action |
|-----|--------|
| `j`/`k` | Move between entries |
| `Enter` or `l` | Open file or enter directory |
| `-` or `h` | Go to parent directory |
| `.` | Toggle hidden files |
| `q` | Close directory buffer |
| `/` | Search entries (normal vim search) |

Cursor positions are remembered per directory — navigate away and back, and you'll be on the same entry.

---

## Import & Alternate Navigation

| Key | Action |
|-----|--------|
| `ga` | Alternate file — switches between implementation and test |
| `gf` | Go to file — opens the quoted path under cursor |
| `gi` | Show imports (planned) |

### Alternate File Patterns

| Language | Implementation | Test |
|----------|---------------|------|
| Go | `handler.go` | `handler_test.go` |
| JS/TS | `Button.tsx` | `Button.test.tsx` or `Button.spec.tsx` |
| Python | `handler.py` | `test_handler.py` or `handler_test.py` |

---

## Header & Root Folder

When Navigator Mode is active, the tab bar is replaced by a header showing the project root folder name centered:

```
          [3M 1A]       zephyr/               :145
```

- **Folder name** — centered, clickable. Click to open the root folder dropdown.
- **Git badge** — `[3M 1A]` to the left, showing modified/added file counts.
- **Line number** — current cursor line on the right.

### Setting the Root Folder

When you toggle Navigator Mode on, the root is auto-detected:

1. **Git repository root** — if the open file is inside a git repo
2. **Open file's directory** — if no git repo is found
3. **Working directory** — fallback to where zephyr was launched

If no root can be detected (e.g., no file is open), the root folder dropdown opens automatically.

### Root Folder Dropdown

Click the centered folder name to open the dropdown at any time:

- **Recent roots** — your last 10 project folders, most recent first. The active root has a dot indicator.
- **Open Folder...** — opens the native folder picker to select a new root.

Clicking outside the dropdown closes it. Recent roots are persisted across sessions.

Toggling Navigator Mode off restores the tab bar with all tabs intact.

---

## Complete Keybinding Reference

### Toggle
| Key | Action |
|-----|--------|
| `Cmd+Shift+N` | Toggle Navigator Mode (Ctrl+Shift+N on Windows) |

### Leader Keys (require Navigator Mode)
| Key | Action |
|-----|--------|
| `<Space>c` | Next hunk |
| `<Space>C` | Previous hunk |
| `<Space>n` | Next changed file |
| `<Space>N` | Previous changed file |
| `<Space>g` | Open git status buffer |
| `<Space>f` | Find files (planned) |
| `<Space>b` | Find changed files only (planned) |
| `<Space>e` | Open project root directory (planned) |

### Hunk Navigation (alternate keys)
| Key | Action |
|-----|--------|
| `]c` | Next hunk (same as `<Space>c`) |
| `[c` | Previous hunk (same as `<Space>C`) |
| `]C` | Next changed file (same as `<Space>n`) |
| `[C` | Previous changed file (same as `<Space>N`) |
| `go` | Toggle original/modified view (planned) |
| `ih` | Hunk text object — works with `d`, `y`, `v`, `c` (planned execution) |

### g-prefix
| Key | Action |
|-----|--------|
| `ga` | Alternate file (test <-> implementation) |
| `gf` | Go to file under cursor |
| `gi` | Show imports (planned) |
| `go` | Toggle original content (planned) |
| `g?` | Context-sensitive help (planned) |

### File Tree
| Key | Action |
|-----|--------|
| `-` | Open parent directory |
| `q` | Close special buffer |

### Universal
| Key | Action |
|-----|--------|
| `Ctrl-o` | Jump back (vim jumplist) |
| `Ctrl-i` | Jump forward |
