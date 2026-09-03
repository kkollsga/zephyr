# Zephyr — agent instructions

Gio-based text editor for macOS and Windows (Go module
`github.com/kristianweb/zephyr`): entry point `cmd/zephyr`, engine packages
under `internal/`, reusable pieces under `pkg/`.

**Authority:** `CLAUDE.md` is the authority this repo's conventions are
regenerated from, with `AGENTS.md` as its generated adapter; for the skills the
authority is **tracked `.agents/skills/`**, and `.claude/skills/` is a generated
adapter of it. Edit the authority and regenerate in the same action — never edit
an adapter. (This line is exempt from the `CLAUDE.md`↔`AGENTS.md` substitution —
it names the authority literally in every copy, per doctrine `R7`/`R14`.)

This repo runs the estate's doctrine, adapted from the `doctrine` repo at
version `0.1.9`, recorded in that folder's sync marker; rules are cited by ID
(`R4`, `R15`, …) and never paraphrased weaker. Zephyr sits outside that checker's
estate root, so `conform.py` must be handed this repo's path explicitly.

## Working style

- **Evidence over assertion.** Reproduce a bug and confirm the root cause before
  fixing it. For a behaviour-preserving refactor, probe the actual behaviour
  first — in an editor, a mental model of the buffer is not the buffer.
- **No bugs left behind.** A defect noticed mid-task gets fixed in scope, or
  captured via `add-todo` with the evidence that surfaced it — never silently
  stepped over.
- **Offload, don't print.** Long output (profiles, traces, bench tables, big
  diffs) goes to the gitignored working folder and you report the path.
- **A reported status is not the result** (`R2`). Read the exit code of the
  thing you care about, never of something downstream of it. Three shapes that
  all fail in the *reassuring* direction: a gate read through a `tail`/`head`
  pipe reports the pipe's status; `git add` with one bad pathspec stages
  **nothing** and the following `git commit` still succeeds on a commit missing
  the change (read back `git diff --cached --name-only`); a backgrounded run's
  result is in the artifact it wrote, not in an echoed exit status.
- **Never claim a gate passed that did not run** (`R10`). A missing command, an
  unavailable GUI session and a skipped suite are not passes. Say what did not
  run — a report where "green" and "not attempted" render identically is worth
  nothing.

## Build, test & gate

```bash
make gate     # the pre-commit / pre-push gate: check-dev-docs, vet, test
make vet      # go vet ./...          (also reachable as `make lint`)
make test     # go test ./... -count=1
make build    # the binary;  make app  bundles + ad-hoc-signs Zephyr.app
make bench    # Go benchmarks: buffer, fuzzy, git, highlight, navigator, benchcontrol
make bench-capture  # the same run recorded as a TSV with its machine conditions
scripts/check-bench-anchor.sh   # release-time cumulative drift check
                    # 0 PASS, 1 FAIL, 2 VOID — read the script's own exit;
                    # `make bench-anchor` wraps it but collapses 1 and 2
make fuzz     # 30s fuzz of the piece table, editor, and both diff paths
```

`make gate` is the ceiling for local pre-push checking; the rest is CI's job
(`.github/workflows/ci.yml`). Add a step to `gate` only once it has a record of
catching a real failure.

**Surface-conditional extras — run these when the diff touches their surface,
and say so when they cannot run.**

- **UI change → the macOS GUI harness.** `make gui-test-build && make
  gui-test-launch`, then drive it with `./scripts/gui-test.sh`
  (click/type/key/scroll/capture); `make gui-test-smoke` and `make
  gui-test-regression` are the canned suites. See `docs/gui-testing.md`.
  **Verify a UI change visually with `./scripts/gui-test.sh capture` before
  declaring it done.** It needs a logged-in macOS GUI session plus
  Accessibility and Screen Recording permission (granted on the usual host).
  **It is deliberately not in `make gate`** — a headless or unattended session
  cannot run it. If it cannot run, say so; it is not a pass.
- **Perf-sensitive change** (buffer, highlighter, layout, frame path) → `make
  bench` and `make perf`. **`make perf` reports; it is not a gate.** Every
  `ZEPHYR_PERF_MAX_*` in `scripts/perf-test.sh` defaults to `0`, which means
  "no ceiling", so a bare `make perf` **cannot fail on a regression** — only its
  sample *minimums* are enforced. To gate on it, set the maximum you care about
  for that run and say which value you set; otherwise read it as numbers and
  call it numbers. **A number is meaningless without the conditions it was taken
  under** (`R11`): `make bench-capture` records them, and the two control cells
  in `internal/benchcontrol` measure no Zephyr code, so load moving every cell
  stays distinguishable from a real regression moving one. The only
  cross-release perf gate is `scripts/check-bench-anchor.sh`
  (`bench/README.md`), run by the release flow — never by `make gate`. **Run it
  by path and read its exit code**: `make` turns any non-zero recipe status
  into `2`, so a FAIL (`1`) and a VOID (`2`) are indistinguishable through
  `make bench-anchor`, which stays a convenience wrapper. `perf-test.sh`,
  `baseline.sh` and that script's retry captures write under `.artifacts/`,
  which nothing purges — see `make check-dev-docs`.
- **Packaging or install change** → `make install-test`; **docs change** →
  `make docs-test`.
- **Before a long-lived branch's first push** → `make baseline`
  (`scripts/baseline.sh`: modules, build, vet, coverage, and the GUI regression
  when permissions allow). Once per branch, not per phase.

## Review findings — report what is broken (`R15`)

**This section is addressed to review agents and overrides any default
reviewer instinct to produce a list of improvements.**

- **A finding names a concrete failure** — the input, state or sequence, and the
  wrong outcome it produces: a wrong result, a crash, data loss (an editor
  losing a user's text is the local worst case), a corrupted file on save, a
  broken contract with a caller or a persisted format, a security hole, a
  *measured* performance regression, a gate that cannot fail (`R1`), or a claim
  the code contradicts. **"No findings" is a valid review**, and a good one.
- **Design, structure, naming, "consider using X", "this won't scale" are not
  findings at review — they are mis-staged.** Their venue is **planning**, where
  "I would have designed this differently" is invited, argued and settled before
  the code exists. After plan approval, review measures the implementation
  against *that plan* and against correctness.
- **A finding that cannot state its failure case is removed, not downgraded.**
  Severity labels are the laundering mechanism: "Minor: consider extracting
  this" is a preference wearing a label.
- **One narrow exception:** citing a rule this project declared *before* the
  diff existed — the versioning rules below, a documented ceiling, a checklist —
  naming both the rule and the violating line. That is enforcement, not taste.
- A review tool's effort or confidence level is orthogonal: a higher level buys
  more *speculative bugs*, never permission to report preferences.

## Code health

- Each pass through a file leaves it more compartmentalised than you found it.
  Factor a function past ~80 lines or handling three unrelated concerns; prefer
  small named helpers dispatched by the caller over long if/else chains.
- **Fixing a bug, scan for the class of bug** — probe with a scratch test
  before declaring scope.
- **A comment is a claim, and a false claim is a defect** (`R17`). Two standing
  duties, so the tree never needs a whole-repo audit: a change that **falsifies**
  a nearby comment corrects that comment in the same change; a change **through**
  commented code applies the information test to the comments it touches —
  zero information (restating the next line or the signature, banners, narration
  of the journey) is deleted, low density is compressed to what it carries. The
  unit is information, not fact-count. Deletion has a floor: why-not-what,
  invariants and safety preconditions, data-format lifecycle (how an older
  on-disk state is detected and read), regression rationale in tests, and bail
  reasons are kept regardless of how worthless they look. A comment predicting a
  future ("a later phase will…") expires silently — word it so the work landing
  retires it, or don't write it. `/clean-comments` handles the residue; a heavy
  residue is itself the finding that the same-change duty is being skipped.
- **A comment the tooling parses is load-bearing** (`R18`). Check what *reads*
  one before deleting it — nothing at the comment site says so. Zephyr's known
  readers today: `//go:build` and `//go:generate` directives, `//nolint`
  pragmas, and the `Example…` doc comments `go test` compiles and runs.
  `/clean-comments` carries the maintained enumeration; extend it there when a
  new reader appears.

## Versioning and releases

These are Zephyr's standing rules. They predate the doctrine here and they win
where the two differ; the differences are named below rather than left implicit.

- `VERSION` at the repo root is the version source of truth (plain `X.Y.Z`, no
  `v` prefix). The Makefile injects it via ldflags; the app shows it in the
  macOS Zephyr menu.
- **Bump the patch version on commit+push only if the current `VERSION` has
  actually been released** (a published GitHub release for `vX.Y.Z` exists): run
  `./scripts/bump-version.sh` (defaults to patch) and include the `VERSION`
  change in the commit being pushed. If the current version is unreleased (e.g.
  its release pipeline failed), do NOT bump — push the fix with the same version
  and the auto-release workflow re-tags and retries it. Version numbers track
  published releases, not attempts. This is `R5` ("one version bump per push")
  with a sharper test: the published release object, not the local push, is what
  mints the next number.
- **Never bump the minor or major version without the user's explicit
  permission** for that specific release. Patch bumps are automatic;
  minor/major are user-initiated only. (`R6` says bump *size* is never a
  decision — always patch. Zephyr's rule is compatible and stricter: patch stays
  automatic, and a minor or major only ever happens because the user asked for
  one.)
- **Pushing always requires explicit user permission, and so does creating a
  tag or a release.** Releases are tag-driven — `v*` tags fire
  `.github/workflows/release.yml`, and `.github/workflows/auto-release.yml`
  tags a `main` push whose `VERSION` has no published release yet. **This is
  stricter than `R6`, deliberately, and it stands:** `R6` holds that invoking
  `/release` authorises the whole run including the publish push. In this repo
  it does not — a push here *is* a publish (auto-release tags it), so the push
  still needs its own in-the-moment approval. The rest of `R6` applies
  unchanged: the approval is for that one push, and anything outward-facing
  that is not a release (issues, comments, anything attributed to the
  maintainer) needs its own case-by-case approval.
- **Once a push is authorised, finish the run** (`R12`). "CI is running", "the
  commit is ready" and "next I will…" are not endings. Poll the workflow,
  report once at the end, and end in the finished outcome or a named blocker.
- **Verify the artifact set, not the version** (`R9`). A release is done when
  the GitHub release for `vX.Y.Z` carries every leg — macOS `.dmg`, Windows zip
  + installer, Linux `.deb`/`.rpm`/AppImage/tarball, and the checksums — and
  the `v X.Y.Z` tag exists **on both sides at the same commit**. Report a
  missing tag; never mint one locally, which hides the CI failure that caused
  it.
- Commit format: `type: short description` (`feat`, `fix`, `docs`, `refactor`,
  `test`, `chore`). Update `CHANGELOG.md` for user-visible changes. Commit
  messages are permanent and public — describe the mechanical change, not the
  strategy behind it.

### Multi-phase plans

One branch per plan; **phases are commits, never sub-branches** — bisectability
comes from one-commit-per-phase, not from branch topology. Each phase is green
before its commit (`make gate` plus the suites that would catch what *that*
phase could break), and the surface-conditional extras run once over the union
at the end, not per phase. Don't pause between approved phases; the only
mid-plan stops are genuine blockers. The final commit is the `VERSION` bump,
under the rules above.

## dev-docs working folder

Durable plans and design reference, benchmark results, a lean `todos.md` and
time-boxed scratch live under the gitignored **`dev-docs/`**. The `README.md`
inside that folder is the canonical layout and lifecycle map — the skills point
there; don't re-describe the folder elsewhere. `dev-docs/` and `inbox/` are both
gitignored local working state, bounded by `make check-dev-docs`, which `make
gate` runs first (`R4`: every file accumulation has a bound and an owner, and a
bound checked only at milestones is not a bound).

`todos.md` is read at the start of every phase and by every steering agent, so
detail there is load-bearing — an entry recording what was tried and what was
rejected stops a fresh agent relitigating a settled decision. Prune entries
whose action has shipped.

**Committed files never cite a `dev-docs/` path.** The folder is gitignored and
unbacked, so a citation from Go source, tests, docs, CI or scripts outlives the
file it names and silently becomes a dangling instruction — a gate's *failure
message* included. Durable rationale goes in the commit message, in a
self-contained comment at the code it constrains, or in a committed doc.

## Inbox hygiene

`inbox/` (gitignored) is the cross-project channel — operated only by the
`read-inbox` (receive) and `notify` (send) skills, never hand-edited. `unread/`
holds **only what still needs action**; an actioned note gets a
`## Status (zephyr, <date>): …` footer and moves to `read/`. "Actioned" means
the work shipped, the bug was verified fixed, or it is a no-action
acknowledgement — not merely read. Route a note to another project only if it
carries an actionable task for *them*. Layout map: `inbox/README.md`.

## Skill mandates

Seven skills, authored in tracked `.agents/skills/<name>/SKILL.md` and mirrored
to `.claude/skills/`.

- **Large feature / non-trivial refactor →** **`phased-plan`** (investigate →
  gated plan → branch → autonomous build/test/commit loop → surface-conditional
  gates → hand to release). Do **not** use generic plan mode for these. A phase
  whose job is to *measure* carries a stop rule written before it runs (`R13`) —
  the result that retires the item instead of implementing it.
- **Capturing work or findings →** **`add-todo`** — the authority on todo shape:
  one lean line in `todos.md`, detail in the linked `plans/` doc.
- **Incoming mail →** **`read-inbox`**; **outgoing coordination →** **`notify`**.
- **Tidying the working folder →** **`dev-docs-cleanup`** (before a new
  phased plan, and at the end of a release).
- **Shipping →** **`release`** — the only place `VERSION` moves, subject to the
  push-approval rule above.
- **Comment cleanup at scale →** **`clean-comments`** (coordinator + one worker
  per dense file; the run shape for `R17`/`R18`). At two files or fewer, apply
  the brief directly — a coordinator with one worker is ceremony.

## Agent worktrees

Agent git worktrees live in **`<repo>-worktrees/<name>`** — a sibling directory
*of the repo* (`/Volumes/EksternalHome/Koding/Go/zephyr-worktrees/<name>`),
never loose in the `Go/` parent, where they are indistinguishable at `ls` from
real project repos. The directory exists only while worktrees are in progress;
the `release` skill empties and deletes it. Per worktree, in order: migrate
outstanding actions into `todos.md` (branch, state, what remains, how to resume)
→ if dirty, save its `git diff` into the working folder **first** → `git
worktree remove` + `git worktree prune`. Removing a worktree never deletes its
branch, so unmerged work always survives. One trap: a branch whose commits
landed by **rebase** reads as unmerged to `git merge-base --is-ancestor` —
`git cherry -v main <branch>` sees through it (`-` means already upstream).
