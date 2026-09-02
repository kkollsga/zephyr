---
name: release
description: Ship a Zephyr release — goal-check against the plan, gate, decide the version against what has actually been published, promote the CHANGELOG, commit, push main (which fires auto-release.yml → tag → release.yml), poll to green fixing failures as they come, verify the full published artifact set and the tag on both sides, then clean branches and tidy dev-docs. Runs to completion autonomously — invoking `/release` IS the push authorization this repo otherwise requires, and it authorizes nothing else.
---

# Release

**Adapted from doctrine reference 0.1.9** (oracle:
`/Volumes/EksternalHome/Koding/Rust/doctrine`; read it before this copy, `R14`).
Zephyr's pipeline is nothing like the reference's — no registries, no manifest
fan-out, a CI-minted tag — so most of this is Zephyr's own.

## THE AUTONOMY CONTRACT — read this before anything else

**The completion condition.** The run is done when **step 11 has verified that
the GitHub Release for `vX.Y.Z` exists, is not a draft, carries the full
artifact set, and that the tag exists on both sides at the same commit** — or
when you have surfaced a **specific blocker you cannot fix**. Nothing else is an
ending.

**The non-endings.** "CI is running", "the commit is ready", "the push is in",
"waiting for the Windows job", "next I will…" are **not endings**. Each is a
natural pause point that feels like a reasonable place to hand control back, and
each is indistinguishable from the inside from genuinely needing input (`R12`).
Three releases across the estate stalled exactly this way with every check
green, and the user noticed before the agent did — twice.

**Waiting is not a checkpoint.** Poll it, background it, or block on it. All
three continue the run; handing control back does not. Zephyr's slowest legs are
the Windows Inno Setup build and the Linux AppImage/nfpm packaging.

**A red pipeline is a task, not a verdict.** Diagnose, fix, push, re-poll,
repeat, within step 10's bound — then surface what remains as a named blocker.

**Report once, at the end** (plus the one pre-push report in step 9).
Intermediate narration must never substitute for the next action.

## Authorization — how `/release` reconciles with this repo's standing rules

`AGENTS.md` → "Versioning and releases" states that **pushing always requires
explicit user permission** and that **creating tags/releases also requires
explicit user permission**. That is stricter than doctrine's `R6`, which says
invoking `/release` authorizes the whole run. The reconciliation, and it is not
a loophole:

- **Invoking `/release` *is* that explicit permission, for this one release's
  `main` push, in the turn it was given.** It authorizes the version decision,
  the CHANGELOG promotion, the release commit, the `main` push that fires
  publication, and the fix-and-push loop that follows.
- **It authorizes nothing else.** Any other push — a branch push during
  `phased-plan`, a follow-up unrelated to this release, a push after the user
  has pivoted — still asks. So does any outward-facing publication that is not
  this release: an issue, a PR comment, an email, anything attributed to the
  maintainer (`R6`, first paragraph).
- **If this flow was reached without the user invoking `/release`** — you
  decided a release was due, or another skill handed off — **you do not have the
  push permission.** Prepare everything up to the commit, report, and ask.
- **The run never creates a tag.** `auto-release.yml` mints it. So the
  tags-need-permission rule is satisfied structurally, and a locally-minted tag
  is forbidden for a second reason: it hides the CI failure that would have
  caused its absence (`R9` corollary), and it collides with the workflow's
  retag path.

**Why the pre-push report is a report and not a gate.** By the time the release
commit exists, the version, the CHANGELOG and the gate results are settled — a
prompt there fires after the decision it claims to guard and can only delay it,
and it breaks unattended runs (the failure mode is "published nothing,
silently"). The safety on the irreversible act lives in checks that can fail,
all upstream of the push: green CI on the exact HEAD, the gate, the preflight
checklist, and the artifact-set verification afterwards. **A prompt cannot fail;
it can only wait** (`R1`).

**Stop only for:** an unfinished open PR at step 1, a genuine scope change (a
minor/major the user did not name, a removal of declared functionality), a
failure that survives ~3 fix attempts without progress, or a
destructive/irreversible action outside the release itself.

## 1. Land every open PR, or stop for the user's decision
A release ships `main` complete — no open PR rides past a release silently.
`gh pr list`, and for each:
- **Finished** (ready for review, CI green on its head, no conflicts) →
  fast-forward it into `main` per step 9's mechanic.
- **Not finished** (draft, red or incomplete CI, conflicts, visibly partial) →
  **stop and put it to the user as a decision before any release work begins.**
  Name the PR, its exact state, and the options: finish it in this run, merge it
  as-is, or defer it. Do not proceed until every open PR is merged or explicitly
  decided. This is a sanctioned stop under the autonomy contract — an unfinished
  PR is a release-scope decision only the user can make — and it sits first so
  the run never stalls on it later.

Do not open new PRs here. A deferred PR is named in the final report **with the
user's decision recorded**, not just "deferred".

## 2. Preconditions
- **No double-stage** (`R5` — one version bump per push):
  ```bash
  git log origin/main..HEAD --oneline | grep -E 'release\('
  ```
  If a `release(x.y.z)` commit sits unpushed, **keep that version** and fold this
  run's work into the same block. A version is not released until it is pushed.
- **Surgical staging.** If unrelated uncommitted work sits in the tree, do not
  block on it and do not sweep it in: stage every release file explicitly by
  path (`git add VERSION CHANGELOG.md …`, never `git add -A` or `.`), then
  **read back `git diff --cached --name-only`** and confirm it matches the
  intended list. This is not a formality: **`git add` is all-or-nothing across
  its pathspecs** — one typo'd or since-renamed path aborts the whole
  invocation, so none of the other files are staged either, and the following
  `git commit` still succeeds, on a release commit missing the bump.
- **On `main`, or on a branch that fast-forwards into it.**

## 3. Doctrine sync — the fallback host
`phased-plan` performs this at the start of a plan. If no plan ran, **this flow
performs it here instead**: compare `dev-docs/.doctrine-synced` against
`/Volumes/EksternalHome/Koding/Rust/doctrine/VERSION`; if doctrine is ahead,
read its `CHANGELOG.md` forward from the marker and act on every newer entry by
its class — `[skills-update]` merged into the declared authority and the adapter
regenerated from it (never hand-ported), `[local-sweep]` executed as the entry
states, `[info]` noted — and **write the new version into the marker only after
those actions completed**. A `[local-sweep]` that fails is surfaced as a release
blocker or carried into `todos.md` with the user's decision; it is never
silently skipped, and the marker never advances past an unactioned item.

## 4. Goal check — did we ship what we set out to ship?
If this release ships a `phased-plan` project, read its plan
(`dev-docs/plans/<slug>.md`) and the PR checklist and confirm every planned
phase actually landed. List any phase **dropped, deferred, or only partially
done** and surface the gaps **before** the version decision. Each gap is a
conscious choice: finish it now, or carry it into `todos.md`. Nothing vanishes
silently.

## 5. Gate — reuse the evidence, do not rebuild for ceremony
- If `phased-plan`'s Final branch gate passed on **this exact HEAD**, do not
  re-run it. If the HEAD moved or evidence is missing, run **`make gate`**
  (`check-dev-docs`, `vet`, `test`) and **`make all`**, plus the surface-
  conditional extras the release diff actually reaches:
  `make install-test` (touched `install.sh`), `make docs-test` (touched
  `README.md` / `docs/index.html` / `docs/install.md`), the GUI harness
  (`make gui-test-build && make gui-test-launch`, `./scripts/gui-test.sh
  capture`, `make gui-test-smoke` / `make gui-test-regression`) for a UI diff,
  `make bench` / `make perf` for a perf diff.
- **The GUI harness needs a logged-in macOS GUI session with Accessibility and
  Screen Recording permission.** If this session cannot run it, the step reports
  **"not run"** in the release report and the release does not claim the UI was
  verified (`R10` corollary). It never reports green.
- **Green branch CI on the exact HEAD is required before the release commit.**
  Check per-job, never the rollup — `gh` reports an empty-string `conclusion`
  for an in-progress job, which a naive filter reads as failure. Do not
  serialize CI's matrix locally to compensate; the three-OS legs, `-race`,
  `staticcheck`, `govulncheck` and `go mod tidy -diff` are CI's tier.
- **Read the command's own exit status, never a pipeline's** (`R2`).

**Test review before the push.** Walk the release diff (`origin/main..HEAD`)
test-first: for each substantive area the release changes, name the existing
test that goes red if that area regresses. Where nothing answers, add one now —
an undertested diff area does not ride the release. Anything added or promoted
here must have been **seen red first**; a test that cannot fail is not coverage
(`R1`).

## 6. The version decision — Zephyr's rule, which is stricter than the default
`VERSION` at the repo root is the single source of truth (plain `X.Y.Z`, no `v`
prefix). The Makefile injects it via ldflags and the app shows it in the macOS
Zephyr menu.

**The bump is conditional on what has actually been *published*, not on what
was attempted.** Check first:

```bash
gh release view "v$(tr -d '[:space:]' < VERSION)" --json tagName,isDraft
```

- **A published release exists for the current `VERSION`** → bump:
  `./scripts/bump-version.sh` (patch by default) and include the `VERSION`
  change in this release commit.
- **No published release exists** (the previous attempt's pipeline failed) →
  **do NOT bump.** Push the fix under the *same* version. `auto-release.yml`
  sees the tag without a release, moves the tag to the fix commit, and
  re-dispatches `release.yml`. **A failed release does not consume a version
  number** — version numbers track published releases, not attempts. Say in the
  report that this is a retry of `vX.Y.Z`, not a new version.
- **Minor or major only when the user named it in this invocation**
  (`/release minor`, `/release 0.2.0`). Absent that, patch, every time — a
  breaking change is not grounds to stop and ask (`R6`). **Escalation is
  one-way: user → agent, never agent → user.** The agent never suggests,
  recommends or announces a minor/major bump anywhere, including a readiness
  report; an agent-announced number the user did not repeat back in their own
  words is void, and proceeding past it adopts the patch default.
- **One bump per push** (`R5`). If a `release(x.y.z)` is already staged, fold
  into it rather than minting a version on top.

`scripts/bump-version.sh` is the only thing that writes `VERSION`; it validates
the current contents as `X.Y.Z` and refuses otherwise. `auto-release.yml`
re-validates the same regex and **fails the whole push** on a malformed value,
so a hand-edit with a stray `v` or a newline problem costs a red `main`.

## 7. Promote the CHANGELOG
Rename `## Unreleased` to `## [X.Y.Z] — <YYYY-MM-DD>` and leave a fresh, empty
`## Unreleased` on top. Keep the `### <Area>` subsection style the file uses,
and merge duplicate area headings that an integration produced.

**This step has been skipped before, and it is visible to users.** `CHANGELOG.md`
today holds `## Unreleased` and `## [0.1.0-alpha]` and nothing between — 0.1.2
and 0.1.3 shipped with their notes still sitting under `## Unreleased`. That is
not cosmetic: `pages.yml` runs on every push to `main` and copies
`tail -n +3 CHANGELOG.md` into `docs/_includes/changelog.md` for the published
site, so an unpromoted block is the public changelog claiming shipped work is
unreleased. Fix the historical entries **only if the user asks**; do not
retroactively invent dates.

## 8. Preflight checklist
**Zephyr has no `make release-preflight` target.** This list is the checklist,
run by hand; if someone adds the target later, this is its contents. All must
hold before the commit:

1. `VERSION` matches `^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$` —
   the same regex `auto-release.yml` enforces.
2. The top `## [X.Y.Z]` CHANGELOG heading equals `VERSION`, and a fresh empty
   `## Unreleased` sits above it.
3. The bump decision matches the `gh release view` result from step 6 (bumped
   iff the previous version is published).
4. `git status --porcelain` shows only the intended release files staged, read
   back from `git diff --cached --name-only`.
5. `git merge-base --is-ancestor origin/main HEAD` — the push will be a
   fast-forward.
6. `make gate` green on this exact HEAD, and the branch's CI green on the same
   sha.
7. No unpushed `release(...)` commit other than the one being made.
8. No `release.yml` run currently in flight for this tag — `auto-release.yml`
   refuses to act while one is (`gh run list --workflow=release.yml`).

A failed item is fixed, not waived. **Never relax a check to get green** — that
is `R10`'s decorative-gate failure.

## 9. Commit, report, push
Commit as the final phase: `release(x.y.z): <summary>` — the `VERSION` change
(when there is one) plus the CHANGELOG promotion plus any release fixes, only
the release files staged.

**Report immediately before pushing — then push, in the same turn. Do not block
on the report.** State: the exact version and whether this is a new version or a
retry of an unreleased one; the gate results **including every step that did not
run**; the goal-check gaps from step 4 and their disposition; and anything the
run turned up that the user did not know when they typed `/release`.

**ff mechanic — push the branch HEAD straight to `main`; do not `git checkout
main`.** With unrelated WIP in the tree a local checkout drags it across and
risks conflicts. Instead:

```bash
git merge-base --is-ancestor origin/main HEAD   # confirm fast-forward
git push origin HEAD:<branch>                   # update the PR, if releasing from one
git push origin HEAD:main                       # fires the pipeline
```

The working tree never moves.

## 10. Poll to green — this is where the run stalls if you let it
The `main` push fires **three** workflows: `ci.yml`, `pages.yml` and
`auto-release.yml`. `auto-release.yml` then reads `VERSION`, decides between
*skip / tag / retag / dispatch*, refuses to act while a `release.yml` run for
that tag is still in flight, creates and pushes `vX.Y.Z`, and **explicitly
dispatches** `release.yml` against the tag ref. The dispatch is not redundant: a
tag pushed by CI with `GITHUB_TOKEN` does not itself trigger another workflow,
so if that last step fails you get a **tag with no release** — which is exactly
the state step 6's retry path is designed for.

```bash
gh run list --workflow=auto-release.yml --branch main --limit 5
gh run list --workflow=release.yml --branch "v$(tr -d '[:space:]' < VERSION)" --limit 5
gh run watch <run-id>
```

Read **`conclusion`**, never `status`; an in-progress job reports an empty
conclusion. Check per-job, not the rollup.

`release.yml` runs `macos`, `windows` and `linux` in parallel and then a
`release` job that `needs:` all three, builds `SHA256SUMS`, **asserts every
expected artifact exists and is non-empty**, and creates the GitHub Release with
`--generate-notes`. Both upload steps carry `if-no-files-found: error`, so an
empty artifact from a green build is already guarded — do not re-derive those
checks by hand; read the job when one goes red.

**Fix-and-push loop — authorized, and it is a loop.** A shipped-code or infra
failure gets diagnosed, fixed, committed and pushed under the **same** VERSION;
`auto-release.yml` moves the tag to the fix commit and re-dispatches. No
re-approval at any iteration. Bounds: stop and surface after **~3 iterations
without progress** (the same failure recurring, or each fix revealing a deeper
one), or on any change to the release shape. An infra flake (a runner timeout, a
transient network failure) is re-run — `gh run rerun <id> --failed` — not
code-fixed. A poll timeout is a non-zero exit and means keep going, not pass.

## 11. Verify the artifact SET and the record (`R9`)
A version check answers "did something publish", never "did everything publish".

```bash
TAG="v$(tr -d '[:space:]' < VERSION)"
gh release view "$TAG" --json isDraft,assets --jq '.isDraft, (.assets[].name)'
```

The set `release.yml` is supposed to upload, with `APP=${TAG#v}` — **eight
assets**:

| Asset | From |
|---|---|
| `Zephyr-$TAG-macos.dmg` | macos job |
| `Zephyr-$TAG-windows-amd64.zip` | windows job |
| `Zephyr-$APP-setup.exe` | windows job (Inno Setup) |
| `Zephyr-$TAG-x86_64.AppImage` | linux job |
| `zephyr_${APP}_amd64.deb` | linux job (nfpm) |
| `zephyr-$APP-1.x86_64.rpm` | linux job (nfpm) |
| `zephyr-$TAG-linux-amd64.tar.gz` | linux job |
| `SHA256SUMS` | release job |

A missing platform is a **failed release**, not a partial success — say which
one and re-run its job. `isDraft` must be `false`.

**Verify the tag on both sides, at the same commit**, and never mint one
locally:

```bash
git fetch --tags
git rev-parse "$TAG^{}"
git ls-remote origin "refs/tags/$TAG^{}" | cut -f1
```

They must be equal, and equal to the release commit. The tag is created
*remotely*, and the release push never fetches, so the local clone has no
occasion to learn it exists unless you fetch. If it is missing remotely, report
that — creating it locally would hide the `auto-release.yml` failure that caused
it. Also confirm the released binary reports the right version: the macOS job
already asserts `zephyr --version` contains the tag, so a mismatch here means
the release job ran on the wrong ref.

Finally, confirm the site picked it up: `pages.yml` republishes `CHANGELOG.md`
on the same push, so the published changelog should now show the promoted
`[X.Y.Z]` section.

## 12. Clean released branches and worktrees — perform directly, no prompt
The `/release` invocation authorizes this. After verification:
- Prove containment before deleting anything: `git merge-base --is-ancestor
  <branch> "$TAG"`. **A branch whose commits landed by *rebase* reads as
  unmerged** — `git cherry -v main <branch>` sees through it (`-` = already
  upstream). Never infer safety from a similar commit message.
- Return the primary workspace to `main` without disturbing WIP:
  `git branch -f main origin/main`, then `git switch main` (a zero-diff switch
  when `main == HEAD`).
- `git branch -d <branch>` (it refuses if somehow unmerged — do not `-D` past
  that) and `git push origin --delete <branch>`. Confirm the PR shows `MERGED`;
  if it shows `OPEN` the commits did not land — investigate, do not force-close.
- **Never** delete `main`, `gh-pages`, a release tag, a branch with unique
  commits, or an open `dependabot/*` branch.
- Finish with `git fetch --prune` and report every retained branch with the
  reason it was not safe to remove.

Then process `/Volumes/EksternalHome/Koding/Go/zephyr-worktrees/` per
`dev-docs-cleanup` §6 — that directory exists only while worktrees are in
progress, so the release empties and deletes it: per worktree, migrate the
outstanding actions into `todos.md`, save a dirty tree's `git diff` under
`dev-docs/` **first**, then `git worktree remove` + `git worktree prune`.
Removing a worktree never deletes its branch.

## 13. Tidy dev-docs and resync the adapters — perform directly, no prompt
Follow **`dev-docs-cleanup`**, todos.md-driven: purge the time-boxed tiers, then
read **only `todos.md`** — archive the now-shipped plan to `dev-docs/bin/` and
prune its entry, trim or drop other completed entries (open a backlinked doc
only to confirm it shipped). Carry the step-4 gaps into `todos.md`. Do not read
`designs/` or sweep `plans/`.

Then perform that skill's **§7 adapter resync** — never blind-sync a divergent
pair; an improvement is merged into the authority first and the adapter
regenerated from it (`R7`). End state, which must be clean:

```bash
python3 /Volumes/EksternalHome/Koding/Rust/doctrine/rules/conform.py \
  /Volumes/EksternalHome/Koding/Go/zephyr
```

**Zephyr is a doctrine consumer, not its source** — there is no snapshot step
here and nothing regenerates the oracle from this repo. A practice worth
upstreaming goes to the doctrine repo's inbox via `notify`.

## 14. Prune the dev environment (`R4`)
Run `make check-dev-docs` (also a `make gate` prerequisite, so it runs at
working cadence, not only here). If it reports over-size or low free space,
act on what it names: `make clean` removes the built binary and `Zephyr.app`,
and **`.artifacts/` is gitignored with no purge tier of its own** — the GUI
harness, perf and baseline runs accumulate there
(`.artifacts/gui-test/`, `.artifacts/perf/`, `.artifacts/baseline/`), so it is
the usual culprit. Copy anything worth keeping into the durable tier
`dev-docs/README.md` names *before* deleting, and never raise the bound to get
green.

## 15. Downstreams
**Zephyr has no declared downstream repos today** — nothing in the estate
depends on `github.com/kristianweb/zephyr`, and there is no notifier script or
Makefile target for it. So this step is normally "none". If that changes, send
the note with **`notify`**, batched per target, and only to a project the
release actually changes something for: a "we released, nothing changes for
you" note trains people to ignore the inbox.

## Notes
- Keep responses under 400 tokens; write long logs under `dev-docs/` per the
  layout map and report the path.
- Version source of truth: `VERSION` at the repo root, one file, no `v` prefix.
  It is the *only* place the version is declared; the app version, the tag, the
  artifact names and the CHANGELOG heading are all derived from it.
- This skill is the only place `VERSION` changes. `phased-plan` never bumps.
- The `main` push is the publish-triggering, authorization-gated action. The
  tag is minted by CI and never by this run.
