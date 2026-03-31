# Vim Mode

Toggle vim mode with **Cmd+Shift+V** (macOS) or **Ctrl+Shift+V** (Windows/Linux). A green **Vim** indicator appears in the status bar when active.

---

## Modes

| Mode | How to Enter | Purpose |
|------|-------------|---------|
| Normal | `Escape` or `Ctrl+c` from any mode | Navigate and issue commands |
| Insert | `i`, `a`, `I`, `A`, `o`, `O`, `s`, `S` | Type text |
| Visual | `v` | Select characters |
| Visual Line | `V` | Select whole lines |
| Visual Block | `Ctrl+v` | Select columns |
| Command | `:` | Enter ex-commands |
| Search | `/` or `?` | Search for text |
| Replace | `r` | Replace a single character |

---

## Normal Mode

### Basic Movement

| Key | Action | Count |
|-----|--------|-------|
| `h` | Left | Yes |
| `j` | Down | Yes |
| `k` | Up | Yes |
| `l` | Right | Yes |

### Word Movement

| Key | Action | Count |
|-----|--------|-------|
| `w` | Next word start | Yes |
| `b` | Previous word start | Yes |
| `e` | End of word | Yes |
| `W` | Next WORD start (whitespace-delimited) | Yes |
| `B` | Previous WORD start | Yes |
| `E` | End of WORD | Yes |

### Line Movement

| Key | Action |
|-----|--------|
| `0` | Line start |
| `^` | First non-blank character |
| `$` | Line end |

### File Movement

| Key | Action |
|-----|--------|
| `gg` | File start (or `{count}gg` to go to line N) |
| `G` | File end (or `{count}G` to go to line N) |
| `{` | Previous blank line |
| `}` | Next blank line |
| `%` | Matching bracket `()[]{}` |

### Scrolling

| Key | Action | Count |
|-----|--------|-------|
| `Ctrl+d` | Half-page down | Yes |
| `Ctrl+u` | Half-page up | Yes |
| `Ctrl+f` | Full page down | Yes |
| `Ctrl+b` | Full page up | Yes |
| `zz` | Center current line |
| `zt` | Scroll current line to top |
| `zb` | Scroll current line to bottom |

### Character Finding

| Key | Action | Count |
|-----|--------|-------|
| `f{char}` | Find character forward on line | Yes |
| `F{char}` | Find character backward on line | Yes |
| `t{char}` | Till character forward (stop before) | Yes |
| `T{char}` | Till character backward (stop before) | Yes |
| `;` | Repeat last f/t/F/T |
| `,` | Repeat last f/t/F/T reversed |

### Search

| Key | Action |
|-----|--------|
| `/` | Search forward |
| `?` | Search backward |
| `n` | Next match |
| `N` | Previous match |
| `*` | Search word under cursor |

Type the pattern and press `Enter` to search. Press `Escape` to cancel.

### Entering Insert Mode

| Key | Action |
|-----|--------|
| `i` | Insert before cursor |
| `a` | Insert after cursor |
| `I` | Insert at first non-blank |
| `A` | Insert at end of line |
| `o` | Open line below |
| `O` | Open line above |
| `s` | Delete character and insert |
| `S` | Delete line content and insert |

### Editing

| Key | Action | Count |
|-----|--------|-------|
| `x` | Delete character under cursor | Yes |
| `X` | Delete character before cursor | Yes |
| `r{char}` | Replace character under cursor | Yes |
| `J` | Join current line with next | Yes |
| `p` | Paste after cursor | Yes |
| `P` | Paste before cursor | Yes |
| `u` | Undo | Yes |
| `Ctrl+r` | Redo | Yes |
| `.` | Repeat last change | Yes |

---

## Operators

Operators wait for a motion or text object to define the range they act on.

| Operator | Action |
|----------|--------|
| `d` | Delete |
| `c` | Change (delete + enter insert) |
| `y` | Yank (copy) |
| `>` | Indent |
| `<` | Dedent |

### Operator + Motion Examples

| Keys | Action |
|------|--------|
| `dw` | Delete to next word |
| `d$` or `D` | Delete to end of line |
| `d3j` | Delete 3 lines down |
| `cw` | Change word |
| `C` | Change to end of line |
| `yw` | Yank word |
| `Y` | Yank entire line |
| `>>` | Indent current line |
| `<<` | Dedent current line |
| `dd` | Delete entire line |
| `cc` | Change entire line |
| `yy` | Yank entire line |

### Text Objects

Text objects define a range around or inside delimiters. Use `i` for **inner** (content only) or `a` for **around** (includes delimiters).

| Object | Inner (`i`) | Around (`a`) |
|--------|------------|--------------|
| `w` | Word | Word + surrounding whitespace |
| `W` | WORD | WORD + surrounding whitespace |
| `"` | Inside `"..."` | Including quotes |
| `'` | Inside `'...'` | Including quotes |
| `` ` `` | Inside `` `...` `` | Including backticks |
| `(` or `)` or `b` | Inside `(...)` | Including parens |
| `[` or `]` | Inside `[...]` | Including brackets |
| `{` or `}` or `B` | Inside `{...}` | Including braces |
| `<` or `>` | Inside `<...>` | Including angle brackets |
| `t` | Inside HTML/XML tag | Including tags |
| `h` | Git hunk (Navigator Mode) | Git hunk |

### Text Object Examples

| Keys | Action |
|------|--------|
| `diw` | Delete word |
| `ci"` | Change inside double quotes |
| `da{` | Delete around braces (including braces) |
| `yi)` | Yank inside parentheses |
| `vit` | Select inside HTML tag |

---

## Visual Mode

Enter visual mode with `v` (character), `V` (line), or `Ctrl+v` (block). All motions extend the selection.

| Key | Action |
|-----|--------|
| `d` or `x` | Delete selection |
| `c` or `s` | Change selection |
| `y` | Yank selection |
| `D` | Delete selection (line-wise) |
| `C` or `S` | Change selection (line-wise) |
| `Y` | Yank selection (line-wise) |
| `>` | Indent selection |
| `<` | Dedent selection |
| `p` | Replace selection with register |
| `J` | Join selected lines |
| `o` | Swap cursor and anchor |
| `v` | Exit visual mode |
| `Escape` | Exit visual mode |

---

## Command Mode

Press `:` to enter command mode. Type a command and press `Enter`.

| Command | Action |
|---------|--------|
| `:w` | Save file |
| `:w {name}` | Save as |
| `:q` | Quit (close tab) |
| `:q!` | Force quit (discard changes) |
| `:wq` or `:x` | Save and quit |
| `:{N}` | Go to line N |
| `:Tutor` | Open vim tutorial |

### Command Line Editing

| Key | Action |
|-----|--------|
| `Enter` | Execute command |
| `Escape` | Cancel |
| `Backspace` | Delete character (cancel if empty) |
| `Left`/`Right` | Move cursor |
| `Home`/`End` | Jump to start/end |

---

## Registers

Registers store yanked and deleted text. Specify a register with `"` before an operator.

| Register | Purpose |
|----------|---------|
| `"` | Unnamed (default for all yank/delete) |
| `0` | Last yank |
| `1`-`9` | Last 9 deletes (shifted on each new delete) |
| `-` | Small delete (less than a line) |
| `/` | Last search pattern |
| `a`-`z` | Named registers |
| `A`-`Z` | Append to named register |
| `+` or `*` | System clipboard |

### Register Examples

| Keys | Action |
|------|--------|
| `"ayy` | Yank line into register `a` |
| `"ap` | Paste from register `a` |
| `"Ayy` | Append line to register `a` |
| `"0p` | Paste last yank |
| `"+y` | Yank to system clipboard |
| `"+p` | Paste from system clipboard |

---

## Count Prefix

Most keys accept a numeric count. Type the number before the key.

| Example | Action |
|---------|--------|
| `5j` | Move down 5 lines |
| `3w` | Move forward 3 words |
| `2dd` | Delete 2 lines |
| `10x` | Delete 10 characters |
| `3p` | Paste 3 times |
| `5>>` | Indent 5 lines |

---

## g-Prefix Keys

| Key | Action |
|-----|--------|
| `gg` | Go to file start |
| `gd` | Go to definition (planned) |
| `gh` | Show hover info (planned) |
| `go` | Toggle original/modified (Navigator Mode) |
| `gf` | Go to file under cursor (Navigator Mode) |
| `gi` | Show imports (Navigator Mode) |
| `ga` | Alternate file — test/impl (Navigator Mode) |
| `g?` | Context help (Navigator Mode) |

---

## Tips

- Press `Escape` or `Ctrl+c` at any time to return to normal mode
- The pending key sequence is shown in the status bar (e.g., `d` while waiting for a motion)
- `:` commands and `/` searches share the same command line editing keys
- Use `u` to undo and `Ctrl+r` to redo — undo history is per-tab
- `*` is a quick way to search for the word under your cursor
- Combine counts with operators: `3dw` deletes 3 words, `5>>` indents 5 lines
