---
name: add-todo
description: Capture work into the dev-docs backlog the right way — scope it, put the detail in a plans/ doc (reuse an existing one or create a new one), and add a lean backlink line to todos.md under the correct section. Handles a single one-off item (`/add-todo <free-text>`) and a deeper body of analysis (a review, an audit, inbox content) that decomposes into several items. The canonical authority on todo-entry shape — the other skills defer here rather than restating it.
---

# add-todo

**Adapted from doctrine reference 0.1.9** (oracle:
`/Volumes/EksternalHome/Koding/Rust/doctrine`; read it before this copy, `R14`).

Capture work into the backlog respecting the convention:

> **`dev-docs/todos.md` holds one lean backlink line per thread; the detail
> lives in a `dev-docs/plans/*.md` file.** Never put detail in `todos.md`; never
> leave a `plans/` doc unlinked.

The layout and its lifecycle tiers are `dev-docs/README.md` — that is the map,
this skill is the entry shape. Capture fast *and* scope well, so the item is
actionable later without rediscovery. Do the work directly; ask the user only
when a classification or scope decision is genuinely ambiguous.

**This skill is the single authority on *how a todo entry is shaped*** —
classification, the lean-backlink format, detail-in-`plans/`, fix-site
grounding. `read-inbox`, `dev-docs-cleanup`, `clean-comments`, `phased-plan` and
`release` all file todos by these rules rather than restating them.

## Two modes
- **One-off** — a single free-text item (`/add-todo <description>`). Run steps
  1–6 once.
- **Batch** — a body of findings (a review, an audit, a `clean-comments` report,
  inbox content) holding *several* actionable items. Decompose first (§0), then
  run the per-item logic for each.

## 0. Decompose (batch mode only)
Split the input into discrete, *independently actionable* items — each a single
change a future session could pick up alone. Across the whole set, before
filing:
- **Drop non-actionable material** — background, confirmations, "no action"
  conclusions. A todo is something to *do*, not a record of what was read.
- **Group by theme.** Items sharing a subsystem go in *one* `plans/` doc as
  sections (one backlink), not N scattered docs.
- **Dedup against the existing backlog** — read `todos.md` first; fold an item
  that extends an existing thread into that thread instead of a new line.
- **Order by priority/effort** so the backlink hooks read sensibly.
Then run steps 2–5 per item (step 1's index read happens once) and keep the
report to one line per filed entry.

## 1. Read the index + understand the ask
Read `dev-docs/todos.md` and `ls dev-docs/plans/`. Parse the ask into **type**,
**the concrete change**, and any **evidence** given.

**If `dev-docs/todos.md` does not exist yet, create it** with a one-line purpose
note and the section headings below — a missing index is not a reason to inline
detail. If it exists with *different* section headings, **the file wins**: match
what is there rather than imposing this list.

## 2. Classify → target `todos.md` section
- **Surfaced defect / wrong behaviour** → `## Bugs (surfaced, not yet fixed)`
- **Enhancement / optimization / code-health / refactor** → `## Engineering backlog (live)`
- **Release, CI, packaging or docs follow-up** → `## Release & process follow-ups`
- **Deliberately deferred scope-creep** → a section in
  `plans/consider-for-future.md` (the parking lot), backlinked from whichever
  section above fits.

A **bug is never filed as a plan for later** when it can be fixed now — that is
`phased-plan`'s "no bugs left behind" rule, and this skill is where the
temptation shows up. Filing a *missing capability* is correct; filing a known
defect to avoid fixing it is the anti-pattern.

## 3. Ground it (cheap, high-value)
For anything touching code, spend one or two `grep -rn` / `Read` calls to **pin
the fix site** (`file:line`) and confirm it is real. A scoped entry with a
concrete location is worth far more than a vague one. Zephyr specifics worth
one extra look:
- **Build-tag siblings.** A fix in `*_darwin.go` usually has `*_windows.go` and
  `*_other.go` counterparts; name all of them, or the entry sends the next
  session to fix one third of the bug.
- **Which net catches it.** Name the Go test that must go red
  (`go test ./internal/<pkg> -run <Name>`), or — for a rendering, pointer or
  window-chrome defect — the GUI-harness step that reproduces it
  (`./scripts/gui-test.sh <cmd>`, or `make gui-test-regression`). An entry that
  cannot name its catcher is under-scoped.
- For a claimed bug, read the surrounding code and tests and confirm it is a
  real defect, not deliberate behaviour, before filing it as one.
Convert any relative date to an absolute one.

## 4. Choose the detail home (reuse first)
- **Fits an existing `plans/` doc's theme** → append a section there. Prefer
  this.
- **Substantial new standalone thread** → create `plans/<kebab-title>.md`.
- **Small deferred item** → append to `plans/consider-for-future.md`.

Scope the detail with these bullets (adapt to the item):
- **What it is** — the concrete change.
- **Why it matters (long-run)** — the leverage, not just the symptom.
- **Evidence** — repro steps, a failing test, a capture path, a benchmark row.
- **Fix site + approach** — `file:line` (plus build-tag siblings) and the shape
  of the change.
- **Test / harness step** — the assertion that must go red first.
- **Effort** — rough size.

Do not cite a purge-tier path as durable evidence — copy the artifact into a
durable location per `dev-docs/README.md` and cite that.

## 5. Add the lean backlink
Append **one line** to the chosen section:

`- <short title> → [plans/<doc>.md](plans/<doc>.md) — <≤200-char hook with fix-site + effort>. Surfaced <YYYY-MM-DD>.`

Match the terse style of the existing lines. Do not duplicate the detail.

## 6. Report
State: the section it went under, the `plans/` doc (new vs appended), and the
one-line backlink — nothing more. Keep it under ~200 tokens.

## Notes
- This skill **adds**; it never prunes. Triage of stale entries is
  `dev-docs-cleanup`'s job.
- Don't bump `VERSION`, promote a `CHANGELOG.md` block, or touch code — this is
  backlog capture only.
- **Anything filed as a *finding* still meets `R15`'s bar**: a concrete failure
  — the input or state and the wrong outcome it produces. "Consider extracting
  this" is a preference wearing a label; a finding that cannot state its failure
  case is removed, not downgraded. Design input belongs in a plan (`phased-plan`
  Phase 1), not in the bugs section.
