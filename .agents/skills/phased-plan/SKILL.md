---
name: phased-plan
description: Run a large Zephyr feature or non-trivial refactor as a gated, phased project. Starts with an investigation phase (read-only Explore agents over the Go tree) — NOT standard plan mode — then builds a custom gated plan with stop rules, opens one branch + draft PR for CI, and executes each phase autonomously (code → make gate → surface-conditional extras → commit) until done. Pushing asks; shipping is the release skill's job.
---

# Phased plan

**Adapted from doctrine reference 0.1.9** — the oracle is
`/Volumes/EksternalHome/Koding/Rust/doctrine` (`rules/RULES.md` +
`reference/skills/`). Read it before this copy (`R14`). Rules are cited by ID
and never paraphrased into something weaker.

For any large feature or non-trivial refactor. **Demand this skill** when the
user kicks off such work. Do **not** use standard plan mode (`EnterPlanMode` /
`ExitPlanMode`) — this skill builds its own gated plan instead.

## Working dir: `dev-docs/` (gitignored)
Plans, scratch and intermediates live under **`dev-docs/`**. The canonical
layout and its lifecycle tiers are **`dev-docs/README.md`** — read it; do not
re-derive it here. The only things this skill needs from it: this project's plan
is `dev-docs/plans/<slug>.md`, design trade-offs go to `dev-docs/designs/`, open
threads get a lean one-line backlink in `dev-docs/todos.md`, and **large output
is written to a purge tier and reported as a path, never printed**.

## Doctrine sync — first action of the run, before Phase −1
The estate's rules live in the sibling `doctrine` repo and are versioned. Pull
them forward before planning anything, so a plan is never built on doctrine this
repo has already been told is superseded.

1. Read `/Volumes/EksternalHome/Koding/Rust/doctrine/VERSION` and
   `dev-docs/.doctrine-synced`.
2. **Equal → done.** That is the normal case, it costs one file read, and
   "we're probably current" is not the check.
3. **Doctrine ahead → read
   `/Volumes/EksternalHome/Koding/Rust/doctrine/CHANGELOG.md` forward from the
   marker** and act on every entry newer than it. Each item carries exactly one
   action class:
   - **`[skills-update]`** — merge the change into this repo's declared
     **authority** (the Authority line at the top of `AGENTS.md` says which
     surface that is) and regenerate the adapter from it in the same action.
     Never hand-port into an adapter; that is what `R7` measures. If the
     authority is a tracked file this session may not touch, edit **neither**
     side — file the `R7` note and carry on, so the editable copy never receives
     a one-sided improvement that widens the gap.
   - **`[local-sweep]`** — run the check command the entry states. Clean → say
     so. **Failing → the sweep becomes Phase 0 work of *this* plan**, scoped and
     visible in the plan doc, never a silent side-task.
   - **`[info]`** — nothing to do.
4. **Write the new version into `dev-docs/.doctrine-synced` only after those
   actions completed.** A marker written first permanently hides the entry it
   skipped. If an item was deferred into Phase 0, the marker advances only once
   that item is in the plan — the plan is the record, not the marker.

**Zephyr is a consumer of doctrine, never its source** (`R14`'s one asymmetry
applies to KGLite, not here). So every divergence found is adjudicated as a
**local improvement** (upstream it) or **staleness** (fix it from the oracle),
and the verdict is named. If `/release` runs without a plan having run first,
that skill performs this same sync as the fallback.

## Phase −1 — Start fresh
Recommend the user run **`dev-docs-cleanup`** so the project starts from a tidy
`dev-docs/` and a current `todos.md`. Carried-over todos may then be folded into
this plan — **only with the user's go-ahead**. If they decline, proceed.

## Phase 0 — Investigation (feel the scale before committing to a plan)
- **Do not enter plan mode.** Investigate first, plan second.
- **Read-only until approval.** The main loop makes **zero edits** during
  Phase 0 and Phase 1 — no branch, no PR, no code, no file writes. All
  investigation runs through read-only `Explore` agents.
- Fan out investigators, **scaled to blast radius** (1–2 for a medium change,
  more only for a genuinely large one). Zephyr's subsystems are `internal/`
  (buffer, render, ui, editor, highlight, format, fileio, git, vim, navigator,
  plugin, config, command, fuzzy, ipc), `cmd/zephyr/` (the Gio app shell, menus,
  platform files) and `pkg/clipboard/`. Have each report: structure of the
  affected area, impacted paths and callers, hidden couplings, existing test
  coverage, a rough size estimate — **and any structural or design objection it
  holds**. Phase 0/1 is the only stage that will hear it (`R15`).
- **Build tags are a blast-radius multiplier here.** A change in
  `cmd/zephyr/platform_darwin.go`, `titlebar_*.go`, `internal/fileio/metadata_*`,
  `internal/render/emoji_*` or `pkg/clipboard/*` has `_darwin` / `_windows` /
  `_other` siblings that must move together; the local `go build` compiles only
  one of them. Enumerate the siblings in Phase 0 or CI's Windows and Linux legs
  will find them for you.
- **Bug-driven work: reproduce and confirm the root cause with evidence** before
  planning the fix. Confirm it is a real defect, not deliberate behaviour.
- **Behaviour-preserving refactor: probe the current behaviour first.** Write a
  throwaway probe (a `_test.go` you delete, or a scratch `main` under
  `dev-docs/temp/`) that exercises the paths you are about to move and capture
  their *actual* output. For a rendering or input change the probe is the GUI
  harness: `make gui-test-build && make gui-test-launch`, then
  `./scripts/gui-test.sh capture` — a before-image is worth more than a mental
  model of what the pixels do.
- **Decide the safety net in Phase 0, not after writing the wrong one.** Unit
  tests cannot see a Gio layout regression; the GUI harness cannot see a
  concurrency bug that only the `-race` legs in CI catch. Name which net catches
  *this class* of change.
- **Phase 0's cost attributions are hypotheses and the record must say so**
  (`R13`). "The cost is in X" written before measuring is a lead. When a later
  measurement falsifies one, the plan doc records the falsification *next to*
  the original claim — never re-word the claim to match the result.
- Synthesize into a scale read: small/medium/large, risk hot spots, what could
  invalidate a naive plan.

## Phase 1 — Build the gated phased plan
- Write the plan to **`dev-docs/plans/<slug>.md`**; the draft PR description in
  Phase 2 mirrors it as a checklist.
- Numbered phases, each independently **buildable, testable, committable**.
- For each phase spell out: the change, the tests that prove it, and the green
  gate — **the suites chosen to catch what *that* phase could break** (its
  touched surface plus that surface's direct consumers), not a fixed list and
  not everything. The full battery runs once at the Final branch gate.
- **A measurement phase carries a stop rule that can retire the work** (`R13`).
  Write, *before* measuring, the result that closes the item instead of
  implementing it ("if re-highlighting a 50 k-line buffer costs under 4 ms,
  drop the incremental-parse phase"). A measurement phase whose only possible
  outcome is "proceed" is a formality with a benchmark attached. **Checkable:
  the stop rule is in the approved plan, dated before the measuring phase ran.**
- **No phase touches `VERSION`, `scripts/bump-version.sh`, or promotes a
  `CHANGELOG.md` version block.** Shipping is the `release` skill's job. Phases
  *do* add entries under `## Unreleased`.
- **Challenge the plan once before presenting it.** (a) List the factual claims
  it rests on — paths, call sites, Gio/tree-sitter behaviour, cost attributions
  — and verify each against the code, recording the evidence in the plan doc.
  (b) Run one pre-mortem: "this shipped and failed — why?", 2–3 concrete
  scenarios. A scenario that names a real failure changes a phase, adds a test,
  or becomes a stop rule; one that cannot is a design preference — argue it
  here, unlabelled. **No severity tiers**: severity labels are how preferences
  get laundered (`R15`), and planning needs only *changes the plan* or *argued
  and settled*.
- Present the plan, then **invite revision and loop on the user's feedback until
  they approve.**
- **This is the stage where design critique belongs — raise it now or hold it.**
  "I would have designed this differently", "that boundary is wrong", "use X
  instead of Y" is **in scope here**, from the user, from an investigator, and
  from you. Argue it, settle it, write the outcome into the plan. It is in scope
  *only* here: after approval the diff is measured against **this plan** and
  against correctness (`R15`). A design objection arriving at review time is
  input to the *next* plan.
- **Hard stop — wait for an explicit go-ahead** ("proceed", "go ahead",
  "approved"). Until then stay read-only. Once approved, **do not pause between
  phases** — but note that approval to proceed is **not** permission to push
  (see Phase 2).

## Phase 2 — Branch + draft PR (the CI tracking handle)
- Create `feat/<slug>` or `refactor/<slug>`. Never work a project directly on
  `main`.
- **Exactly one branch and one draft PR per plan. Phases are commits, never
  sub-branches** — bisectability comes from one-commit-per-phase, not from
  branch topology. The `release` skill deletes the branch when the plan ships.
- **If a phase needs an isolated tree it goes under
  `/Volumes/EksternalHome/Koding/Go/zephyr-worktrees/<name>`** — one sibling
  directory holding every worktree, never a loose dir beside the real projects
  in `Koding/Go/`. That directory exists only while worktrees are in progress
  and the release flow empties and deletes it. **Zephyr has no build-cache
  symlink to recreate** (the Go build cache is `$GOCACHE`, outside the tree, and
  is shared by every worktree automatically) — do not invent one. A worktree
  carrying uncommitted work is never removed without its `git diff` saved under
  `dev-docs/` and a `todos.md` entry pointing at it; removing a worktree never
  deletes its branch, so unmerged work survives.
- **The first push of a long-lived branch runs the CI-only tier for the first
  time, and that is the point** — CI (`.github/workflows/ci.yml`) is the full
  tier and holds four checks the local gate structurally cannot see: the Windows
  and Linux build legs, `-race` (plus the 10× `TestPieceTable_ConcurrentReads`
  stress), `staticcheck` + `govulncheck` + `go mod tidy -diff`, and the fuzz
  smoke targets. A program branch that accumulates weeks of work reaches its
  first push and gets rejected on contact. Push early, once, rather than
  discovering four unrelated failures at release time.
- **Pushing requires explicit user permission in this repo** (`AGENTS.md` →
  "Versioning and releases"). This is stricter than the reference doctrine,
  deliberately, and it is the one place the autonomous loop stops: ask before
  the first push, and again at each checkpoint push. Everything *up to* the push
  — code, gate, commit — is autonomous.
- With permission: push the branch and **open a draft PR against `main`**.
  `ci.yml` triggers on `pull_request: [main]`, so every push to the branch now
  runs the full matrix, while **nothing publishes** — `auto-release.yml` and
  `pages.yml` only trigger on push to `main`, and `release.yml` only on a `v*`
  tag or a dispatch against one.
- Put the phased plan into the PR description as a checklist, one box per phase.

## Phase 3 — Execute each phase (the autonomous loop)
For every phase, in order:

1. Implement the phase's code and its tests.
2. **Local green gate before committing.** Run **`make gate`** — it runs
   `check-dev-docs`, `vet` and `test`, and it is the pre-push ceiling. Then run
   the targeted suites chosen to catch what *this* phase could break: the
   touched package and its direct consumers, e.g.
   `go test ./internal/buffer/ -run TestPieceTable -count=1`. Not a fixed list,
   and not the full battery — that runs once at the Final branch gate.
   - **`make gate`'s membership has no catch record yet.** It was assembled from
     what CI runs cheaply, not from a history of local runs catching CI
     failures. Doctrine's rule is "a catch record earns a slot" — so **when a CI
     failure lands that `make gate` could have caught locally, revise the
     membership and say so in the report**. Until that record exists the tier is
     provisional, and this paragraph is the honest label for it, not decoration.
   - **"Green" means you saw that command's own exit status, never a
     pipeline's** (`R2`). `go test ./... | tail` reports `tail`'s status.
     `grep -c` exits 1 on a zero count, so `grep -c … && next` breaks on exactly
     the empty result you needed to act on. `exit 1` inside `$( )` kills only
     the subshell.
   - **Go-specific vacuous passes, all of which render identically to green:**
     `go test -run NoSuchTest ./...` prints `ok … [no tests to run]` and exits
     0; a package with no test files prints `? … [no test files]` and exits 0; a
     `t.Skip` is not a pass; and `go test ./internal/foo` on a *build-tagged*
     file compiles only the current platform's variant. Assert the scan was
     non-empty before trusting its verdict (`R1`).
   - **A NEW GATE IS NOT TRUSTED UNTIL YOU HAVE SEEN IT FAIL** (`R1`). If the
     phase adds or changes a check — a test, a CI step, an assertion in a
     `scripts/*.sh` — break the thing it guards, confirm it goes red, then
     restore. Reading a gate cannot tell you whether it works. **Verify the
     probe, not just the result**: a mutation that edited the wrong text makes a
     working gate look broken.
   - **Never claim a gate passed that did not run** (`R10` corollary). A missing
     command, an unavailable GUI session and a skipped test are not passes. Say
     what did not run; "green" and "not attempted" must not render identically.
3. **Surface-conditional extras — they fire on the diff, not on ritual.**
   - **GUI harness** — when the diff touches `cmd/zephyr/` (the Gio app shell,
     menus, titlebar, draw/events), `internal/ui/` or `internal/render/`:
     `make gui-test-build && make gui-test-launch`, then
     `./scripts/gui-test.sh capture` and, for pointer/mode changes,
     `make gui-test-smoke` or `make gui-test-regression`. **Verify UI changes
     visually with a capture before declaring them done** (`AGENTS.md` →
     "Testing"). Two hard constraints from `docs/gui-testing.md`: it needs a
     **logged-in macOS GUI session with Accessibility + Screen Recording
     permission** (`make gui-test-permissions` checks and requests), and macOS
     has **one global synthetic-input stream** — run only one GUI automation
     session at a time and do not touch the physical mouse or keyboard while it
     runs. **If the session cannot run it — headless, SSH, permission denied,
     not macOS — the step reports "not run" and the UI change is not declared
     verified.** It never reports green.
   - **Perf** — when the diff touches a hot path (`internal/buffer`,
     `internal/highlight`, `internal/render`, `internal/fuzzy`, the Gio frame
     loop in `cmd/zephyr/draw.go`): `make bench` for the Go micro-benchmarks,
     `make perf` (`scripts/perf-test.sh`) for launch/frame/RSS numbers under the
     GUI harness. See Phase 4.
   - **Installer / docs** — `make install-test` when `install.sh` or its test
     changes; `make docs-test` when `README.md`, `docs/index.html` or
     `docs/install.md` changes. Both also run in CI, and `docs-test.sh` asserts
     *exact* strings and *counts* of the install command, so a wording edit in
     one file and not the others fails it.
   - **Workflows** — a change under `.github/workflows/` has no local runner.
     Say so, and let the branch's CI run be the check (`R10` corollary).
4. Update `CHANGELOG.md` under `## Unreleased` for user-visible changes, in the
   `### <Area>` subsection style the file already uses. Never touch a released
   `## [X.Y.Z]` block. Note that `pages.yml` publishes `CHANGELOG.md` to the
   site on every push to `main`, so this text is user-facing the moment it lands.
5. **Commit** the phase (`feat(...)` / `refactor(...)` / `fix(...)`), one commit
   per phase. `make gate` runs `check-dev-docs` (`R4`'s bound at working
   cadence, not only at milestones), so the phase commit is where the working
   folder's size is re-checked — a gated no-op costs nothing on a lean tree.
6. **Push at checkpoints — and ask first.** Batch 2–3 quick phases per push, or
   push at a risky milestone worth CI confirmation. Every push to a PR branch
   runs the full three-OS matrix plus the quality job. **`ci.yml` has no
   `concurrency` group**, so a superseded in-flight PR run is *not* cancelled
   and pushing three times in a minute burns three full matrices — batch
   deliberately. Tick completed phases' checkboxes when you push.
7. **Retire any `todos.md` action this phase completed**, at phase-commit time:
   fully done → remove the backlink and move its `plans/` doc to `dev-docs/bin/`;
   partially done → trim the entry to what is left; a shared doc → remove only
   the closed entry, and move the doc only once no live backlink points at it.
   `dev-docs/` is gitignored, so this is local bookkeeping alongside the commit.
   Note each retirement in the report-out.
8. Continue into the next phase. If a push's CI comes back red, fold the fix
   into the loop — do not leave the PR red.

Stop mid-plan only for a genuine blocker (an unfixable test, an architectural
surprise invalidating a later phase). Surface it; do not push through.

**Bugs that surface mid-plan — no bugs left behind. Fixing is the default; the
backlog is for missing capability, never for a known defect.**

First **classify**, because only one of them may be filed:
- A **bug** is a defect in behaviour that exists: a wrong result, a crash, data
  loss (this is a text editor — a save path that can lose a buffer is the top of
  this list), a broken contract with a caller or an on-disk file, a *measured*
  regression, a gate that cannot fail, a claim the code contradicts. A bug is
  **fixed**, never backlogged.
- A **missing capability** is a feature never built. *That* is what
  `plans/consider-for-future.md` is for.

Then fix, by where it lives:
- **In scope** — reproduce, confirm the root cause, fix it as its own bisectable
  phase (`Phase Nb`) with its own test and commit. Do not fold a behaviour
  change into a mechanical-refactor commit.
- **Out of scope** — still fix, still as its own `Phase Nb`. Out-of-scope
  changes the *commit boundary*, not the decision to fix. File to
  `plans/consider-for-future.md` only when fixing now is genuinely blocked, and
  then it is a *surfaced* bug with a `todos.md` backlink and a cheap regression
  test pinning it. Say **why** in the report-out; "out of scope" is a location,
  not a reason.
- **Suspected perf bug** — an unmeasured perf change is not a fix. Measure it
  *in this plan* (fold it into the perf phase, or add one).

## Phase 4 — Perf gate (only if the plan touched perf-sensitive paths)
Run `make bench` (Go benchmarks over `internal/buffer`, `internal/fuzzy`,
`internal/git`, `internal/highlight`, `internal/navigator` and the control cells
in `internal/benchcontrol`) and, for anything the user can feel, `make perf`
(`scripts/perf-test.sh` — launch time, first submit, steady frame p95, Gio
event-to-submit p95, RSS, optional soak) against `testdata/gui/mouse_fixture.go`.
`make baseline` (`scripts/baseline.sh`) captures the broad snapshot.

**State honestly what this instrument is and is not** (`R11`):
- `scripts/perf-test.sh`'s maximum gates are **disabled by default** — every
  `ZEPHYR_PERF_MAX_*` defaults to `0`, which means "no ceiling". Only the sample
  *minimums* are always enforced. So a bare `make perf` **cannot fail on a
  regression**; it produces numbers. Treating its exit code as a perf gate is
  exactly the decorative-gate failure `R1` names. To make it a gate, set the
  relevant `ZEPHYR_PERF_MAX_*` for the run and say which value you set.
- **Control cells exist; the anchor comparison does not yet.**
  `internal/benchcontrol` carries two benchmarks that measure no Zephyr code, so
  a capture can tell load moving every cell from a real regression moving one
  (`R11`). But nothing compares this release's capture against three releases
  back, so slow cumulative drift is still invisible here. Do not imply
  otherwise in a report.
- Outputs land under `.artifacts/perf/latest` and `.artifacts/baseline/latest`
  (`ZEPHYR_PERF_DIR` / `ZEPHYR_BASELINE_DIR` override). `.artifacts/` is
  gitignored and has no purge tier of its own; a number worth keeping is copied
  into `dev-docs/bench/results/` per `dev-docs/README.md`.
- **Run the capture under whatever load the machine has** — waiting for an idle
  machine stalls work and buys less than it costs. But a number compared *across
  sessions* **records the machine state it was taken under** (`R11` corollary):
  write it beside the numbers, as metadata, never as a gate on taking them.
- Judge a heavy-tailed cell by its median, not its min; measure a once-per-event
  cost (first paint, first parse) as the mean of first events. State which
  statistic a number is.

Fix regressions now, not in a follow-up. For plans that never touched
perf-sensitive code, skip this phase and say so.

## Final branch gate — required before Report out / release
After the last phase run, over the plan's union:
- `make gate` and `make all` (`vet`, `test`, `build`);
- every targeted suite the phases used;
- the surface-conditional extras the union requires — GUI harness capture for a
  UI plan, `make bench` / `make perf` for a perf plan, `make install-test` /
  `make docs-test` where the diff reached them;
- `go test -race ./... -count=1` once, here — CI runs it on macOS and Linux and
  it is the check most likely to reject a branch on contact.
Then commit any fixes, ask for permission, push once, and let the **full GitHub
PR CI on that exact HEAD** run the Windows and Linux legs, `staticcheck`,
`govulncheck`, `go mod tidy -diff`, coverage and the fuzz smoke in parallel. Do
not begin release work while CI is pending or red.

**Any review at this gate is failures-only, against the plan** (`R15`): did
every phase do what the plan said, and does anything now break — with the
concrete failing input, state or consequence named. Structure, naming, "consider
using X" and speculation about future scale are **not findings**, at any
confidence, and **a finding that cannot state its failure case is removed, not
downgraded**. "No findings" is a valid review. A design objection surfacing here
does not block the branch: record it in `plans/consider-for-future.md` with a
`todos.md` backlink, as input to the next plan, and ship. If you run
`/code-review` here, its effort level tunes confidence and breadth — it buys
more speculative *bugs*, never permission to report preferences.

## Report out (when the plan completes, before Ship)
Keep it under 400 tokens and link the plan doc for detail:
- **Phases** done, one line each, plus the PR link / final commit shas.
- **Bugs surfaced** and each one's disposition — *fixed in Phase Nb* by default;
  *filed* only for a bug fixing-now was genuinely blocked, stating **why**. This
  list is mandatory even when empty ("no bugs surfaced").
- **Gate results**, including **every step that did not run and why** — an
  unavailable GUI session, a workflow change with no local runner, a skipped
  suite. This is the report's most load-bearing line (`R10`).
- **Perf** result (numbers, which statistic, machine state, verdict) or "not
  perf-sensitive".
- **`todos.md` changes**: retired and added.
- **Plan deviations** and why. If a `[local-sweep]` from the doctrine sync
  landed as Phase 0 work, say so.

## Phase 5 — Ship (only on request)
When the user asks to ship, run the **`release`** skill. It performs the
version decision, the CHANGELOG promotion, the `main` push that fires
`auto-release.yml`, and verifies the published artifact set. **This skill never
touches `VERSION`, never tags, and never pushes `main`.**

## Notes
- Keep responses under 400 tokens; write long diffs and logs under `dev-docs/`
  per the layout map and report the path.
- Branch pushes are routine for CI but still ask (repo rule). The `main` push at
  release time is the publish-triggering one.
- **If context is genuinely running out, hand over at a clean boundary — never
  start a large phase tired.** A handover is: finish the phase in flight, gate
  it, commit, and write the state into `dev-docs/plans/<slug>.md` so the next
  agent resumes at a phase start. This is not licence to pause between phases;
  it is the rule for *where* an unavoidable break lands, and mid-phase is the
  one place it must not.
