# Navigator Mode — Design Document

## Context

Zephyr is a GPU-accelerated text editor in Go with vim mode, tree-sitter highlighting, and a tab-based file system. We're designing a **Navigator Mode** — a git-centric, modal navigation system that replaces the tab bar with a project-aware workflow.

**The key insight:** In a world where AI agents write code and humans review it, the editor's primary job shifts from *authoring* to *orientation and inspection*. The user needs to answer: "What changed? Where? Why? What does it connect to?" — as fast as possible. Navigator Mode makes these questions answerable with single keystrokes.

---

## 1. UX Philosophy: Review-First Design

### The Problem with Tabs

When an AI agent modifies 12 files across your project, opening them as tabs gives you a flat list with no structure. You can't see *what* changed, *how much* changed, or *how files relate*. You're left clicking tabs randomly, hoping to build a mental model.

### What Users Actually Need

When inspecting AI-generated changes (or any multi-file changeset), the workflow is:

1. **Overview** — "What files changed? How much?" (the status buffer)
2. **Triage** — "Which changes matter most?" (changed file list with diff stats)
3. **Deep inspection** — "What exactly changed here?" (hunk navigation with original toggle)
4. **Context** — "What does this file connect to?" (import graph, alternate files)
5. **Orientation** — "Where am I in the project?" (breadcrumb, directory navigation)

Navigator Mode is designed around this workflow, in this order.

### Core UX Principles

1. **Zero-setup orientation.** Opening a project should immediately show you what's different. No configuration, no manual marking, no "open the git panel." The diff IS your workspace.

2. **One keystroke per question.** "What changed?" → `<Space>g`. "Next change?" → `]c`. "What was here before?" → `go`. "What imports this?" → `gi`. Every navigation question has a single-motion answer.

3. **Never lose context.** The breadcrumb always tells you where you are. `Ctrl-o` always takes you back. Directory cursors are remembered. The jumplist is your undo for navigation.

4. **Progressive detail.** Status buffer shows files → `=` expands inline diff → `Enter` opens the file → `]c` jumps between hunks → `go` shows the original. Each step adds detail without losing the overview.

5. **Vim-native, not vim-adjacent.** Every navigation action is a vim motion, text object, or command. `ih` is a text object. `]c`/`[c` follow the bracket-motion convention. `-` follows the vinegar convention. Nothing feels grafted on.

### The AI Review Workflow

```
User runs AI agent → agent modifies files → user opens zephyr

<Space>g          → Status buffer: see all 12 changed files with +/- stats
j/k               → Scan the list, triage by change size
=                 → Expand inline diff on a suspicious file
Enter             → Open the file
]c                → Jump to first hunk
go                → "What was here before?" Toggle to see original
go                → Toggle back to modified
]c                → Next hunk
]C                → Done with this file, next changed file
gi                → "What does this file import?" Check for new dependencies
ga                → "Is there a test?" Jump to test file
<Space>g          → Back to status buffer for the next file
```

This entire workflow never leaves the keyboard, never opens a sidebar, never hunts through tabs.

---

## 2. Inspiration Analysis

### fugitive.vim (tpope)
**What's clever:** Git status as an interactive buffer, not a panel. `=` toggles inline diff per file. The `.` key pre-populates command line with the file path. Everything is a buffer — no special windows.

**What we take:** The status buffer concept. Interactive, actionable, buffer-based. `=` for inline diff toggle. The philosophy that git state IS the navigation state.

**What we adapt:** fugitive uses ex-commands (`:Git`, `:Gdiffsplit`). We use vim motions and leader keys instead since Navigator is a mode, not a plugin.

**What we skip:** Deep git plumbing (blame, log graph, rebase). Phase 1 focuses on status + diff.

### gitsigns.nvim (lewis6991)
**What's clever:** `ih` — the hunk as a vim text object. You can `dih`, `yih`, `vih`. Visual-mode partial hunk staging. Smart `]c`/`[c` that falls through to vim's native diff-mode `]c` when appropriate. Word-level diff via virtual text.

**What we take:** `]c`/`[c` hunk navigation. `ih` text object. Gutter signs (green/blue/red bars). The idea that hunks are first-class vim primitives.

**What we skip:** Quickfix integration (no quickfix list yet). Per-line staging from gutter (staging belongs in status buffer).

### diffview.nvim (sindrets)
**What's clever:** Accepts any git rev-parse expression (`HEAD~4..HEAD~2`, `origin/main...HEAD`). Index buffers are editable for staging. File panel toggles between tree and list view. Layout cycling with `g<C-x>`. Option panel (`g!`) for interactive git log filtering.

**What we take:** Direct git rev syntax for future comparisons. The file panel concept (our status buffer serves this role). `g?` context-sensitive help convention.

**What we skip:** Side-by-side layout (we use inline toggle). Index buffer editing (too complex for phase 1).

### oil.nvim (stevearc)
**What's clever:** The core insight — a directory listing is just text, file operations are text edits. Concealed metadata (hidden IDs) track which filesystem entry each line represents. `:w` commits all filesystem changes as a batch. Cross-directory move via yank/paste between directory buffers. `skip_confirm_for_simple_edits` — a calibrated heuristic for when confirmation is unnecessary.

**What we take:** `-` opens parent directory as buffer. Navigate with vim motions (`j/k/Enter/h/l`). Git status annotations per entry. The buffer-not-sidebar philosophy.

**What we adapt:** Oil allows renaming by editing text. We defer filesystem mutation — phase 1 is read-only navigation. The concealed metadata pattern is simplified since we don't need edit tracking.

**What we skip:** Batch-and-confirm on `:w`. SSH/S3 adapters. LSP rename integration.

### mini.files (echasnovski)
**What's clever:** Miller column layout with focused/unfocused widths (50 vs 15 chars) creates visual depth-of-field. `h`/`l` for spatial depth navigation. `=` to synchronize (batch apply). Bookmarks with `m`/`'` that mirror vim's mark system. Cursor positions tracked per directory — return to where you left off.

**What we take:** Per-directory cursor memory (critical for fluid navigation). `h`/`l` spatial vocabulary in directory buffers. The idea that navigation has spatial depth.

**What we skip:** Miller columns (we're single-buffer). The bookmark system (our "bookmarks" are git-changed files).

### Magit (Emacs)
**What's clever:** The transient prefix system — press `c` to see all commit commands + toggleable flags in a popup. Section-based actions: `s` stages whatever is under cursor (file, hunk, or partial hunk via selection). Collapsible sections with numeric depth shortcuts (`1`/`2`/`3`/`4`). `n`/`p` for section-aware movement. Auto-refresh after every git operation.

**What we take:** Section-based status buffer as the hub. `s`/`u` for stage/unstage with cursor-context sensitivity. Collapsible sections with `Tab`. `n`/`p` for section jumping. Auto-refresh after operations. `g?` for help.

**What we adapt:** Magit's transient system becomes simpler `g?` context help since we don't need the full popup framework.

**What we skip:** Commit authoring (use terminal). Rebase/merge UI. The full transient system.

### Harpoon (ThePrimeagen)
**What's clever:** The insight that your working set is 3-5 files, not 20. Numeric slot model with meaning (file 1 = main, file 2 = test). The list is editable as a buffer. Project-scoped by CWD.

**What we take:** The core insight reframed — **changed files ARE your working set automatically**. `]C`/`[C` cycles through them. No manual marking needed.

**What we skip:** The explicit mark list. Git diff replaces it.

### vim-projectionist (tpope)
**What's clever:** `:A` for alternate file with convention-based patterns. `"src/*.go": {"alternate": "src/*_test.go"}`. Heuristic project detection from file patterns. Template pre-population for new files.

**What we take:** `ga` for alternate file (implementation ↔ test). Convention-based patterns for Go (`_test.go`), JS (`.test.ts`), Python (`test_*.py`).

**What we skip:** Template population. Type-based navigation commands (`:Eplugin` etc.).

### telescope.nvim (nvim-telescope)
**What's clever:** Finder/Sorter/Previewer separation — one UI, any data source. Insert + normal mode in picker. `<Tab>` multi-select → send to quickfix. `resume` to reopen last picker. `builtin.builtin` — a meta-picker listing all pickers.

**What we take:** The multi-source fuzzy finder concept. `<Space>f` = files, `<Space>b` = changed files, `<Space>/` = grep. Git-changed files boosted in ranking.

**What we skip:** The full framework architecture (overkill for our needs). Multi-select to quickfix.

### Cross-Cutting Patterns Worth Adopting

| Pattern | Used By | Our Implementation |
|---------|---------|-------------------|
| `g?` context help | fugitive, diffview, oil, mini.files | Every special buffer shows its keybindings on `g?` |
| Data as editable buffer | oil, mini.files, harpoon | Directory buffer, status buffer |
| Batch-and-confirm | oil, mini.files | Stage/unstage with confirmation for destructive ops |
| `ih` hunk text object | gitsigns | Hunk as first-class vim primitive |
| `-` for go up/back | vinegar, oil | Parent directory navigation |
| Section movement | magit | `n`/`p` in status buffer |
| Progressive disclosure | magit, telescope | Overview → expand → inspect → toggle original |

---

## 3. Feature Design

### 3.1 Git Change Navigation

**The most important feature for AI review workflows.** This is what lets you answer "what changed?" in seconds.

#### Gutter Signs
Every line gets a colored marker at the left edge of the gutter:
- **Green bar** (2px): added line
- **Blue bar** (2px): modified line  
- **Red triangle**: deleted line (shown at the line after the deletion)

Plus subtle background highlight across the full line width for changed regions.

#### Hunk Navigation
| Key | Action |
|-----|--------|
| `]c` | Next hunk (current file, wraps to next changed file) |
| `[c` | Previous hunk |
| `]C` | Next changed file |
| `[C` | Previous changed file |

`]c` crosses file boundaries — when you reach the last hunk in a file, the next `]c` opens the next changed file and lands on its first hunk. This creates a continuous stream of changes to review.

#### Original Toggle
`go` — **Toggle between modified and original content for the current hunk.** This is the killer feature for code review. Instead of a side-by-side diff that splits your attention, the original content appears *in place*, with a distinct background color. Press `go` again to see the modified version. You're comparing in the same spatial location, which is cognitively easier.

When toggled to original:
- Lines show the HEAD version
- Background tint changes (subtle warm/amber tone)
- Gutter shows `~` instead of line numbers for the toggled region
- Cursor stays on the same logical position

#### Hunk Text Object
`ih` — inner hunk. Works with all operators:
- `vih` — visually select the hunk
- `yih` — yank the hunk
- `dih` — delete the hunk (revert to original)

#### Data Model
```go
// internal/git/diff.go
type Hunk struct {
    OldStart  int      // line in original (1-based)
    OldCount  int
    NewStart  int      // line in modified (1-based)  
    NewCount  int
    OldLines  []string // original text
    NewLines  []string // modified text
}

type FileDiff struct {
    Path    string
    Status  rune      // 'M', 'A', 'D', 'R'
    Hunks   []Hunk
}
```

### 3.2 The Status Buffer (The Hub)

**This is the landing page.** When you want to understand the state of the project, `<Space>g` opens the status buffer. It's the answer to "what happened while I was away?" or "what did the AI change?"

#### Layout
```
Head:     main (abc1234)
Upstream: origin/main (ahead 2)

Unstaged changes (3)                              +38 -12
  M  internal/vim/normal.go                       +22  -4
  M  cmd/zephyr/draw.go                           +12  -6
  D  old_file.go                                   -2

Staged changes (1)                                 +45
  A  internal/navigator/git.go                    +45

Untracked files (2)
  ?  internal/navigator/dirbuffer.go
  ?  internal/navigator/status.go

Recent commits
  abc1234  Fix cursor jump in visual mode          2h ago
  def5678  Add fold region computation             1d ago
```

**Key UX decisions:**
- **Diff stats on every line** (+N/-N). This is the triage signal — big numbers mean big changes, inspect those first.
- **Aggregate stats on section headers** so you can see total scope at a glance.
- **Recent commits** provide temporal context — "what was the last thing that happened?"

#### Status Buffer Keybindings
| Key | Action |
|-----|--------|
| `j`/`k` | Move between entries |
| `n`/`p` | Jump to next/previous section header |
| `Enter` | Open file at cursor, land on first hunk |
| `=` | Toggle inline diff for file at cursor |
| `s` | Stage file (or hunk if expanded) |
| `u` | Unstage file (or hunk if expanded) |
| `x` | Discard changes (with confirmation) |
| `Tab` | Collapse/expand section |
| `]c`/`[c` | Next/prev entry with changes |
| `q` | Close status buffer |
| `g?` | Show help |
| `R` | Refresh (re-run git status/diff) |

#### Inline Diff Expansion
Pressing `=` on a file entry expands the unified diff inline:
```
Unstaged changes (3)
  M  internal/vim/normal.go                       +22  -4
     @@ -145,6 +145,28 @@
     + func handleBracketSequence(s *State, ch rune) Action {
     +     switch ch {
     +     case 'c':
     ...
  M  cmd/zephyr/draw.go                           +12  -6
```

Press `=` again to collapse. When expanded, `s` on a hunk line stages just that hunk.

### 3.3 File Tree as Buffer (Oil-Style)

**Not a sidebar. A buffer.** You navigate the directory with vim motions, the same way you navigate a file. It's a buffer that happens to show directory contents instead of file contents.

#### Activation
| Key | Context | Action |
|-----|---------|--------|
| `-` | In a file | Open parent directory as buffer |
| `-` | In a directory buffer | Go to parent directory |
| `<Space>e` | Anywhere | Open project root as directory buffer |

#### Directory Buffer Format
```
internal/vim/
────────────────────────────
  action.go
  commandline.go
  keyhandler.go
M normal.go                    +22  -4
  mode.go
A navigator.go                 +45
  operator.go
  registers.go
  visual.go
```

**UX decisions:**
- Directory name as header, with a separator line
- Git status character at left margin (`M`, `A`, `D`, `?`)
- Diff stats at right margin for changed files
- Directories shown with trailing `/` and sorted first
- Hidden files (`.`, `node_modules`, etc.) filtered by default

#### Directory Buffer Keybindings
| Key | Action |
|-----|--------|
| `j`/`k` | Move between entries |
| `Enter` or `l` | Open file / enter directory |
| `-` or `h` | Go to parent directory |
| `q` | Close, return to previous file |
| `/` | Filter entries (search mode) |
| `.` | Toggle hidden files |
| `g?` | Show help |

#### Per-Directory Cursor Memory
When you navigate from `internal/vim/` to `internal/editor/` and back, the cursor returns to the same entry you left on. This is stored in `Navigator.DirCursors map[string]int`.

### 3.4 Import Graph Navigation

**The semantic axis.** Files are connected through imports. When reviewing AI-generated code, you need to quickly check: "What new dependencies were added? Does this file import something unexpected?"

| Key | Action |
|-----|--------|
| `gf` | Go to file under cursor (import-aware) |
| `gi` | Open import list for current file (fuzzy finder) |
| `ga` | Go to alternate file (test ↔ implementation) |

#### Import List (`gi`)
Opens the fuzzy finder pre-populated with the current file's imports:
```
Imports: internal/vim/normal.go
─────────────────────────────
  internal/vim/action.go         (Action, ActionKind)
  internal/vim/mode.go           (Mode, State)
  internal/vim/registers.go      (RegisterFile)
  internal/editor/editor.go      (Editor)
```

Select one to open it. This uses tree-sitter to parse import statements.

#### Alternate File (`ga`)
Convention-based patterns:
- `foo.go` ↔ `foo_test.go`
- `Component.tsx` ↔ `Component.test.tsx`
- `module.py` ↔ `test_module.py`

### 3.5 The Breadcrumb (Replacing Tabs)

When Navigator Mode is active, the tab bar is hidden. A breadcrumb takes its place:

```
zephyr > internal > vim > normal.go   [3M 1A]   :145
```

- **Path segments**: project name > directories > filename (filename is bright, rest is dim)
- **Git badge**: `[3M 1A]` — count of modified/added files in project
- **Position**: current line number

The breadcrumb is display-only. Navigation happens through keybindings, not clicks.

### 3.6 Enhanced Fuzzy Finder

The existing fuzzy finder gains git awareness:

| Key | Behavior |
|-----|----------|
| `<Space>f` | Find files (git-changed files boosted in ranking) |
| `<Space>b` | Find among changed files only ("buffer list") |
| `<Space>/` | Grep across project (future phase) |

For `<Space>b`, the file list is `Navigator.ChangedFiles()` — the git diff becomes your buffer list.

---

## 4. Complete Keybinding Table

### Navigator Mode Activation
| Key | Action |
|-----|--------|
| `<Space>n` | Toggle Navigator Mode on/off |

### Git Navigation (when Navigator Mode is active)
| Key | Action |
|-----|--------|
| `]c` | Next hunk (crosses file boundaries) |
| `[c` | Previous hunk |
| `]C` | Next changed file |
| `[C` | Previous changed file |
| `go` | Toggle original/modified for current hunk |
| `ih` | Hunk text object (works with d, y, v, c) |

### File Tree
| Key | Context | Action |
|-----|---------|--------|
| `-` | File buffer | Open parent directory |
| `-` or `h` | Directory buffer | Go to parent |
| `Enter` or `l` | Directory buffer | Open entry |
| `q` | Directory/status buffer | Close, return to file |
| `<Space>e` | Anywhere | Open project root directory |

### Import & Connections
| Key | Action |
|-----|--------|
| `gf` | Go to file under cursor |
| `gi` | Import list (fuzzy finder) |
| `ga` | Alternate file (test ↔ impl) |

### Status Buffer
| Key | Action |
|-----|--------|
| `<Space>g` | Open git status buffer |
| `n`/`p` | Next/previous section |
| `=` | Toggle inline diff |
| `s` | Stage file/hunk |
| `u` | Unstage file/hunk |
| `x` | Discard (with confirmation) |
| `Tab` | Collapse/expand section |
| `R` | Refresh |

### Fuzzy Finder
| Key | Action |
|-----|--------|
| `<Space>f` | Find files (git-boosted) |
| `<Space>b` | Find changed files only |

### Universal
| Key | Action |
|-----|--------|
| `g?` | Context-sensitive help |
| `Ctrl-o` | Jump back (vim jumplist) |
| `Ctrl-i` | Jump forward |

---

## 5. Architecture & Code Structure

### New Packages

#### `internal/git/` — Git integration foundation
| File | Purpose |
|------|---------|
| `exec.go` | Run git commands, handle errors |
| `repo.go` | Repository discovery, root detection |
| `status.go` | Parse `git status --porcelain=v1` |
| `diff.go` | Parse `git diff HEAD` unified output into `[]FileDiff` + `[]Hunk` |
| `cache.go` | Cache with invalidation on file save / fsnotify |

Key interfaces:
```go
type Repo struct {
    Root   string
    GitDir string
}

func Discover(path string) (*Repo, error)
func (r *Repo) Status() ([]FileStatus, error)
func (r *Repo) Diff(ref string) ([]FileDiff, error)
func (r *Repo) DiffFile(ref, path string) (*FileDiff, error)
func (r *Repo) Show(ref, path string) ([]byte, error)  // get original content
```

#### `internal/navigator/` — Navigator mode logic
| File | Purpose |
|------|---------|
| `navigator.go` | Core state: active flag, repo, history, cached data |
| `dirbuffer.go` | Directory-as-buffer: entries, rendering, entry lookup |
| `statusbuf.go` | Status buffer: sections, entries, collapse state |
| `imports.go` | Tree-sitter import extraction + resolution |
| `alternate.go` | Alternate file patterns (test ↔ impl) |
| `breadcrumb.go` | Breadcrumb data model |

Key types:
```go
type Navigator struct {
    Active     bool
    Repo       *git.Repo
    Root       string
    
    Status     []git.FileStatus
    FileDiffs  map[string]*git.FileDiff
    DirCursors map[string]int     // remembered cursor per directory
    
    DirBuf     *DirBuffer         // non-nil when viewing a directory
    StatusBuf  *StatusBuffer      // non-nil when viewing status
}
```

### Modified Existing Files

#### `internal/vim/action.go`
Add ~15 new ActionKind constants:
```
ActionNavNextHunk, ActionNavPrevHunk,
ActionNavNextFile, ActionNavPrevFile, 
ActionNavToggleOriginal,
ActionNavOpenParent, ActionNavOpenEntry, ActionNavCloseSpecial,
ActionNavOpenStatus, ActionNavStage, ActionNavUnstage,
ActionNavToggleDiff, ActionNavSectionNext, ActionNavSectionPrev,
ActionNavGoFile, ActionNavGoImports, ActionNavGoAlternate,
ActionNavHelp
```

#### `internal/vim/normal.go`
- Add `]`/`[` bracket prefix handling (new `handleBracketSequence()`)
- Add `<Space>` leader key handling (new `handleLeaderSequence()`)
- Add `-` key binding
- Extend `handleGSequence()` with `gf`, `gi`, `ga`, `go`

#### `internal/vim/operator.go`
- Add `ih` hunk text object to `handleTextObjDelimiter()`

#### `cmd/zephyr/main.go`
- Add `navigator *navigator.Navigator` and `navigatorActive bool` to `appState`
- Add `gitDiff *git.FileDiff` to `tabState`
- Add buffer type tracking (file vs directory vs status)

#### `cmd/zephyr/vim.go`
- Add cases in `executeVimAction()` for all `ActionNav*` kinds
- This is the bridge between vim actions and navigator operations

#### `cmd/zephyr/draw.go`
- Conditional: `drawBreadcrumb()` vs `drawTabBar()` based on `navigatorActive`
- Gutter sign rendering in `drawEditorNormal()` — colored bars for added/modified/deleted
- Background highlight bands for changed lines
- Original-toggle rendering (different background when showing HEAD content)

#### `internal/config/theme.go`
Add colors: `GitAdded`, `GitModified`, `GitDeleted`, `HunkAddedBg`, `HunkDeletedBg`, `HunkOriginalBg`, `BreadcrumbDim`, `BreadcrumbFile`, `StatusSection`

### How Special Buffers Work

Directory buffers and status buffers reuse the existing `Editor` + rendering pipeline. They are `Editor` instances with generated text content and a type flag. This means:
- Vim motions work naturally (j/k/gg/G/search)
- Viewport scrolling works
- The rendering pipeline doesn't need special cases
- Only the keybinding layer needs to know the buffer type (to map Enter/s/u differently)

### Integration with Existing Tab System

Navigator Mode does NOT remove tabs. `TabBar` continues to manage `Editor` instances internally. When Navigator Mode is active:
- Tab bar rendering is hidden (breadcrumb shown instead)
- File navigation (`]C`, `gf`, `Enter` in dir buffer) calls `tabBar.OpenFile()` internally
- `TabBar.Tabs` preserves all open editors
- Toggling Navigator Mode off restores the tab bar with all tabs intact

---

## 6. Implementation Phases

### Phase 1: Git Foundation
**Goal:** `internal/git/` package complete and tested.

1. `exec.go` — git command runner with error handling
2. `repo.go` — discover `.git`, resolve root
3. `status.go` — parse `git status --porcelain=v1`
4. `diff.go` — parse unified diff into `[]FileDiff{[]Hunk}`
5. `cache.go` — cache + invalidation
6. Tests for diff parsing (renames, binary, new/deleted files)

**Dependencies:** None. Pure Go.

### Phase 2: Gutter Signs & Hunk Navigation  
**Goal:** Changed lines visible in gutter. `]c`/`[c` work.

1. Git colors in theme
2. `gitDiff` field in `tabState`, populated on file open
3. Gutter sign rendering in `drawEditorNormal()`
4. `]`/`[` bracket prefix + `ActionNavNextHunk`/`ActionNavPrevHunk`
5. Hunk jump logic in `executeVimAction()`
6. Background highlight bands for changed lines

**Dependencies:** Phase 1.

### Phase 3: Navigator Toggle & Breadcrumb
**Goal:** Mode toggleable, breadcrumb replaces tab bar.

1. `navigatorActive` + `navigator` in `appState`
2. `<Space>` leader key in `handleNormal()`
3. `<Space>n` toggles mode
4. `drawBreadcrumb()` in draw.go
5. Conditional tab bar / breadcrumb rendering

**Dependencies:** Phase 1.

### Phase 4: Directory Buffer
**Goal:** `-` opens oil-style directory buffers.

1. `internal/navigator/dirbuffer.go` — model + line generation
2. Buffer type tracking in `tabState`
3. `-` key handling, Enter/l/h/q in directory context
4. Git status annotations per entry
5. Per-directory cursor memory

**Dependencies:** Phase 1, Phase 3.

### Phase 5: Status Buffer
**Goal:** `<Space>g` opens magit-inspired status hub.

1. `internal/navigator/statusbuf.go` — sections, entries, collapse
2. Section-aware navigation (n/p)
3. `=` inline diff expansion
4. `s`/`u`/`x` for stage/unstage/discard
5. Auto-refresh after operations
6. `g?` help overlay

**Dependencies:** Phases 1-4.

### Phase 6: Import Navigation & Alternate Files
**Goal:** `gf`, `gi`, `ga` work.

1. Tree-sitter import queries per language
2. Import resolution (relative paths, Go modules)
3. `gf`/`gi`/`ga` in `handleGSequence()`
4. Alternate file patterns

**Dependencies:** Phase 3, existing tree-sitter.

### Phase 7: Polish & Enhanced Fuzzy Finder
**Goal:** Git-aware fuzzy finder, `ih` text object, `go` toggle, cross-file `]C`/`[C`.

1. `<Space>f` with git boosting, `<Space>b` for changed files only
2. `ih` hunk text object
3. `go` original toggle with visual indicator
4. `]C`/`[C` cross-file navigation
5. Navigation history for Ctrl-o/Ctrl-i

**Dependencies:** Phases 2-6.

---

## 7. Open Questions

1. **Auto-activate on directory open?** If `zephyr .` is run, should Navigator Mode activate automatically with the status buffer?

2. **Performance on large repos.** `git diff HEAD` can be slow on monorepos. Options: run async in goroutine, diff current file only initially, use `--stat` for overview.

3. **`ih` without Navigator Mode?** The hunk text object is useful even in normal editing. Should git diff data always be loaded for files in repos?

4. **Partial hunk staging.** Visual-select lines within a hunk + `s` = stage only those lines. Extremely useful but complex to implement (requires constructing a valid patch). Defer to later phase?

5. **`go` key conflict.** `go` is not bound in standard vim (it's `g` then `o`). Verify no planned use in zephyr's existing `handleGSequence()`.

6. **File watcher for `.git/index`.** When staging happens externally (from terminal), the status buffer should refresh. Watch `.git/index` via fsnotify?

7. **Tab cleanup on mode-off.** Toggling Navigator Mode off might reveal 20+ tabs from navigation. Offer "close unmodified tabs" command?

---

## 8. Key Files Reference

| Purpose | Path | Notes |
|---------|------|-------|
| Vim normal mode handler | `internal/vim/normal.go` | Add `]`/`[`, `<Space>`, `-`, extend `g` prefix |
| Vim action definitions | `internal/vim/action.go` | Add ~15 ActionNav* constants |
| Vim operator/text obj | `internal/vim/operator.go` | Add `ih` hunk text object |
| Action executor | `cmd/zephyr/vim.go` | Bridge vim actions to navigator operations |
| App state | `cmd/zephyr/main.go` | Add navigator fields to appState/tabState |
| Rendering pipeline | `cmd/zephyr/draw.go` | Breadcrumb, gutter signs, hunk highlights |
| Event routing | `cmd/zephyr/events.go` | Key dispatch (mostly unchanged — vim handles it) |
| Theme colors | `internal/config/theme.go` | Git/navigator color definitions |
| File tree (reference) | `internal/ui/filetree.go` | Reuse patterns for directory buffer |
| Fuzzy finder | `internal/ui/fuzzyfinder.go` | Extend with git-aware file sources |
| Fuzzy matcher | `internal/fuzzy/matcher.go` | Reuse for all fuzzy filtering |
| Tree-sitter | `internal/highlight/treesitter.go` | Extend for import extraction |
| Command registry | `internal/command/commands.go` | Register navigator commands |
