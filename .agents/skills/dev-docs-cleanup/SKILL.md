---
name: dev-docs-cleanup
description: Tidy the gitignored dev-docs/ working folder — purge the time-boxed tiers, then run a todos.md-driven tidy (read only todos.md; reconcile misplaced plans/ files and stale or completed actions, opening a backlinked doc only when a check points at it), soft-delete finished docs to dev-docs/bin/, process any agent worktrees, resync the instruction adapters, and finish on make check-dev-docs. Run before a new phased-plan or at the end of a release. Never reads design docs.
---

# dev-docs cleanup

**Adapted from doctrine reference 0.1.9** (oracle:
`/Volumes/EksternalHome/Koding/Rust/doctrine`; read it before this copy, `R14`).

`dev-docs/` accumulates plans, intermediates and scratch. This skill tidies it:
nothing is hard-deleted from the durable tiers — stale files are soft-deleted to
`dev-docs/bin/` and any open action is preserved as a `dev-docs/todos.md` entry.

**The layout and the lifecycle tiers are `dev-docs/README.md` — the canonical
map. Read it at the start of the run; it is the source of truth for which dirs
are durable and which are time-boxed, and this skill does not restate it.**

## 1. Purge the time-boxed tiers (always first)
Hard-delete only from the tiers the map marks as purged, with the windows the
map states. **If the map and the commands below disagree, the map wins** — fix
the commands, not the map.

```bash
mkdir -p dev-docs/temp dev-docs/bin dev-docs/bench/out
find dev-docs/temp      -type f -mtime +14 -print -delete   # ephemeral scratch + offloaded output
find dev-docs/bench/out -type f -mtime +14 -print -delete   # heavy generated measurement artifacts
find dev-docs/bin       -type f -mtime +7  -print -delete   # soft-deleted docs, grace expired
```

Never touch `plans/`, `designs/`, `bench/results/`, `todos.md`,
`learn-from-us.md` or `.doctrine-synced` here — the map marks them durable, and
**a durable tier is a promise** (`R4`): anything irreproducible that lands in a
purged tier is a scheduled data loss with a date on it. **Purge by the tier
marker, not by age alone** — an age-only sweep destroys whatever was filed in
the wrong tier. If something durable is sitting in `temp/` or `bench/out/`, move
it to its right tier *before* the sweep and say so.

Report what was purged (path list, or "nothing aged out").

## 2. Read `todos.md` — the only file read by default
`todos.md` is the index of open threads and its backlinks point at the durable
docs. **Read only `todos.md` to start.** Do not read through `plans/`, and
**never read `designs/`** — design reference is not todos-driven and is out of
scope for cleanup. Open a specific doc *only* when a check below points at it.
This keeps the run cheap: one file read plus the few docs the checks flag.

## 3. Two checks, both driven by `todos.md`

**a) Misplaced files in `plans/`.** `ls dev-docs/plans/`. Every file there
should be backlinked from `todos.md`. For any file with **no backlink**, read
*that file only* and decide:
- a live thread missing from the index → add a one-line backlink following
  `add-todo`'s entry shape; or
- finished / abandoned → soft-delete to `bin/`.
Genuinely unsure → surface it to the user. (`designs/`, `bench/`, `README.md`
and `todos.md` are exempt — this check is `plans/`-only.)

**b) Stale / completed actions in `todos.md`.** Scan the entries — especially at
end-of-release, where shipped work leaves completed items behind. For any entry
that reads as done or outdated, **read its backlinked doc to confirm**, then:
- shipped / doc fully complete → move the doc to `bin/` and **remove the entry**
  (code + `CHANGELOG.md` + git history are the record; there is no
  shipped-feature archive);
- partially done → trim the entry to only what is left;
- superseded / abandoned → drop it, keeping a one-line "closed" note only when
  it is worth not rediscovering.
Do not read a backlinked doc unless its entry looks stale.

**The prune test is one question: "would an agent picking this up act
differently for having read it?"** An entry whose action has shipped is dead
weight; one that would change what the next agent does stays, however long.

**When a claim is retracted, grep for every place it was written** (`R3`) — a
correction that lands in the plan doc but not in `todos.md` leaves the file that
claims to be sufficient to brief a fresh agent carrying the wrong thing.

## 4. Surface the plan
Report a short summary: what was purged, misplaced `plans/` files found and the
decision for each, stale `todos.md` entries to prune, docs to soft-delete.
- **Run standalone** (e.g. before a `phased-plan`): wait for the user's
  go-ahead before moving files or editing `todos.md`. A simple proceed is enough.
- **Run inside an authorized flow** (`/release`'s tidy step): perform the tidy
  directly — no prompt — then report what was done. The flow's invocation is the
  authorization.

## 5. Soft-delete processed files
On go-ahead, `mv` processed stale files into `dev-docs/bin/` (the map's grace
window keeps them recoverable). Never delete an active plan, `todos.md`, or
anything the user chose to keep.

## 6. Process `/Volumes/EksternalHome/Koding/Go/zephyr-worktrees/` (release-end)

Agent worktrees live in **`zephyr-worktrees/<name>`**, a sibling directory of
the repo — never loose in `Koding/Go/`, where they are indistinguishable at `ls`
from the real project repos. That directory exists **only while worktrees are in
progress**, so this step empties it. Run it **after** the tidy above: the entries
it writes are for the *next* sprint and must not be exposed to §3(b)'s staleness
sweep in the same run.

`git worktree list`, then for each worktree under `zephyr-worktrees/`:
1. **Capture state first** — `git -C <wt> status --porcelain`, its branch, and
   whether that branch is merged (`git merge-base --is-ancestor <branch> main`).
   **A branch whose commits landed by *rebase* reads as unmerged** to that
   check; `git cherry -v main <branch>` sees through it (`-` = already upstream).
2. **Dirty tree → save the work before anything else.** Write `git -C <wt> diff`
   and `status` to `dev-docs/worktree-harvest-<name>.diff`. **A worktree with
   uncommitted work is never removed without that diff saved and a `todos.md`
   entry pointing at it.**
3. **Migrate outstanding actions into `todos.md`** — branch name, what it
   contains, what remains, how to resume — following `add-todo`'s entry shape.
   Removing a worktree does *not* delete its branch: the ref lives in the main
   repo's `.git`, so unmerged work survives.
4. `git worktree remove <wt>` then `git worktree prune`.

Finally delete the emptied `zephyr-worktrees/` directory. Anything ambiguous —
or a worktree that looks like *active* user work (dirty **and** modified within
the week) — is left in place and reported. **Zephyr has no per-worktree build
cache to recreate**: `$GOCACHE` lives outside the tree and every worktree shares
it. Do not invent a symlink step.

## 7. Resync the agent-instruction adapters (`R7`)

Diff each adapter against its declared authority, rename-aware. The Authority
line at the top of `AGENTS.md` states which surface is the authority for the
conventions file and which for the skills — **that line is exempt from the
rename substitution and reads identically in every copy**, because a substituted
declaration inverts itself and tells the adapter's reader to edit the adapter.

- Identical → done.
- Divergent → **classify each hunk before touching either side** (`R14`'s
  adjudication): an **improvement** is merged into the *authority* first and the
  adapter regenerated from it; **staleness** is simply regenerated away. **Never
  run a blind sync on a divergent pair** — blind sync deletes improvements, and
  no sync preserves stale doctrine the other harness will follow.
- Where the authority is a tracked file this session may not commit, edit
  **neither** side and file the note for the repo's owner, so an editable
  adapter never receives a one-sided improvement that widens the very gap `R7`
  measures.

Mechanical check, the end state this step must leave green:

```bash
python3 /Volumes/EksternalHome/Koding/Rust/doctrine/rules/conform.py \
  /Volumes/EksternalHome/Koding/Go/zephyr
```

It must report no `R7` failure. Note that its mirror normaliser maps only
`AGENTS.md` → `CLAUDE.md`; **skill *paths* are deliberately not substituted**,
so a line naming a skills tree literally must read the same in both copies.

## 8. Finish on the bound (`R4`)

```bash
make check-dev-docs
```

It is also a prerequisite of `make gate`, so it runs at working cadence rather
than only at milestones — **a bound checked only at milestones is not a bound**.
If it fails, act on what it names: over-size in `dev-docs/` means something
belongs in a purge tier or was never worth keeping; a free-space failure means
the volume, not this folder, and `.artifacts/` (gitignored, no purge tier of its
own) plus `make clean` are the usual culprits. Do not raise the bound to get
green — that is `R10`'s decorative-gate failure applied to disk.

## Output discipline
Keep the response under 400 tokens. If the review is long, write the full report
under the map's ephemeral tier and report that path; surface only the new-todos
list and the keep/drop confirmations inline.

## Relationship to the other skills
`phased-plan` recommends running this first, so a project starts from a tidy
folder and a current `todos.md`; `release` runs §1–§8 as its tidy step. Entries
written here follow `add-todo`'s shape.
