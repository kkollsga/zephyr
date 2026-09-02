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

### HEAD View

`go` swaps the buffer for the file's content at HEAD, in place — same tab, same
scroll position, same cursor line. The tab title gains ` (HEAD)` and the diff
markers in the gutter go away, because the diff describes the working file and
says nothing about the lines you are now looking at.

The HEAD view is read-only. Typing, pasting, cutting, vim edits, undo and redo
are all ignored, and a save is refused with a message rather than writing
committed content over your working file. Press `go` again to bring your buffer
back exactly as it was, including unsaved edits, the cursor position and the
undo history.

A file git cannot show at HEAD — untracked, newly added, or outside the
repository — is reported in the footer and the buffer is left alone.

### Hunk Text Object

`ih` is the run of changed lines under the cursor — the added and modified
lines only, never the unchanged context lines git prints around them. It is
linewise, so it takes whole lines with their newlines:

| Key | Action |
|-----|--------|
| `dih` | Delete the changed lines under the cursor |
| `yih` | Yank them (pastes back on its own line with `p`) |
| `cih` | Delete them and start typing on the empty line left behind |

Each is a single undo step. With the cursor on an unchanged line — including a
context line in the middle of a hunk — there is no hunk object and the keys do
nothing. Operator use only: `vih` in visual mode is not supported.

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

## Alternate & File Navigation

| Key | Action |
|-----|--------|
| `ga` | Alternate file — switches between implementation and test |
| `gf` | Go to file — opens the quoted path under cursor |

### Alternate File Patterns

| Language | Implementation | Test |
|----------|---------------|------|
| Go | `handler.go` | `handler_test.go` |
| JS/TS | `Button.tsx` | `Button.test.tsx` or `Button.spec.tsx` |
| Python | `handler.py` | `test_handler.py` or `handler_test.py` |

---

## File Finder

`<Space>f` opens a fuzzy finder over the files under the project root;
`<Space>b` lists only the files git reports as changed.

| Key | Action |
|-----|--------|
| any character | Filter |
| `Up` / `Down` | Move the selection |
| `Ctrl-p` / `Ctrl-n` | Move the selection |
| `Backspace` | Delete the last character of the query |
| `Enter` | Open the selected file in a tab |
| `Escape` | Close |

Clicking a row opens it; clicking outside closes the finder. Only one overlay
is up at a time — the finder does not open over the save menu, the language
selector or a focused find bar.

The file list skips hidden files and directories, `node_modules`, `vendor` and
`__pycache__`. It does not read `.gitignore`; use `<Space>b` when you want the
list git itself considers interesting. The scan is cached per root, so files
created after the first `<Space>f` appear once the root changes.

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
| `<Space>e` | Open project root directory |
| `<Space>f` | Find files under the project root |
| `<Space>b` | Find changed files only |

### Hunk Navigation (alternate keys)
| Key | Action |
|-----|--------|
| `]c` | Next hunk (same as `<Space>c`) |
| `[c` | Previous hunk (same as `<Space>C`) |
| `]C` | Next changed file (same as `<Space>n`) |
| `[C` | Previous changed file (same as `<Space>N`) |

### Text Objects
| Key | Action |
|-----|--------|
| `ih` | The changed lines under the cursor — works with `d`, `y` and `c` |

### g-prefix
| Key | Action |
|-----|--------|
| `ga` | Alternate file (test <-> implementation) |
| `gf` | Go to file under cursor |
| `go` | Toggle the HEAD view (read-only committed content) |

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

### Not available

`gi` (show imports) and `g?` (context help) are not implemented. The keys do
nothing — they are listed here only so a reader who expects them from the
design notes knows they are absent rather than broken.
