---
name: clean-comments
description: Coordinator-run comment cleanup over a measured scope — the invoking agent measures comment density, briefs one sub-agent per dense file to delete zero-information comments, compress low-density ones and fix false claims (R17), then verifies the whole diff mechanically, never from worker self-reports. Carries Zephyr's R18 enumeration of what reads comments in this Go tree. Deliberately smaller than a phased plan — no branch ceremony, no plan doc.
---

# clean-comments

**Adapted from doctrine reference 0.1.9** (oracle:
`/Volumes/EksternalHome/Koding/Rust/doctrine`; read it before this copy, `R14`).

Make the comments in a measured scope **true and lean**: delete what carries no
information, compress the rest to what it carries, fix comments the code
contradicts (`R17`), and never touch what the tooling reads (`R18`).

**The steady state is `R17`'s same-change duty** — a change that falsifies a
nearby comment corrects it in the same change, and a change through commented
code applies the information test to what it touches. This skill is for the
residue. A heavy head right after a cleanup is itself the finding: the
same-change duty is being skipped, and the report says so.

## 0. Shape of the run
The invoking agent is the **coordinator**: it measures, briefs, dispatches,
verifies and reports — and edits no comments itself. Sub-agents (**workers**) do
the edits, one file each, because de-duplication needs the whole-file read and
self-reports need an independent checker. **One exception: if measurement
returns ≤ 2 files, skip the workers and apply the brief directly** — a
coordinator with one worker is ceremony.

Invocation authorizes the whole run (`R12`): it ends in the report or a named
blocker, never in "workers are running". This run **never pushes** — pushing
needs the user's explicit permission in this repo — and never touches `VERSION`
or a released `CHANGELOG.md` block.

## 1. Measure first, and be ready to stop
Count comment lines per file over the scope (default: the whole Go tree; a
subtree argument narrows it):

```bash
rg -c --no-messages '^\s*//' -g '*.go' <scope> | sort -t: -k2 -rn | head -40
rg -c --no-messages '^\s*//' -g '*.go' <scope> | awk -F: '{s+=$2} END {print s}'
```

Take the **head**: the files jointly holding ~half the scope's comment lines.
For calibration, the whole tree measured 2 419 comment lines across 169 Go files
on 2026-09-02, with the top twelve files holding ~44 % of them — a small
population, which is itself a reason the stop rule below usually fires.

**Stop rule, decided before counting (`R13`):** if the head is empty or
trivially small, report "already lean — nothing to do" and stop. A cleanup that
runs regardless of what the measurement says is a formality with a diff
attached. **Write the threshold down before you look at the numbers.**

**The naive count over-reports in this repo, and the correction is not
optional.** `cmd/zephyr/titlebar_darwin.go` ranks second on that list at 114
"comment" lines — and most of them are `//` comments *inside a compiled
Objective-C block*, not Go comments at all (see §2's reader list). Subtract the
cgo preamble files from the head before choosing workers, or the densest brief
you write is aimed at source code.

## 2. Assemble the worker brief (once, fixed)

**The two tests, per comment paragraph** — *does this add a fact the reader
cannot get from the code or from an earlier paragraph?*
- **Zero information → delete**: restates the next line or the signature,
  generic banner, self-referential bookkeeping, dead scaffolding, a commented-out
  experiment.
- **Low density → compress** to the information carried: repetition across
  paragraphs, throat-clearing, narration of the journey, over-explained
  mechanics, four variations of one example, hedging.

"Keep the fact, drop the label" is **not** the test — it preserves volume by
construction. The unit of value is information, not fact-count.

**The floor — never delete:** why-not-what; invariants, safety preconditions,
lock ordering (the piece table is read concurrently and CI stress-tests it — the
comments stating what may be read without the lock are the contract);
data-format lifecycle (how an older on-disk config, theme or session file is
detected, refused or migrated); regression rationale in tests — the reason the
test is not deletable; bail reasons in editor/render decision code, where
deleting one invites a wrong-behaviour regression no unit test catches; a
repeated comment that is a *local contract* rather than an accident; and
anything under the reader list below.

### What reads our comments in this repo (`R18`) — verified 2026-09-02
**A missing or unverified reader list stops the run. It is not improvised
mid-flight.** Each class below was confirmed by grep against this tree; re-run
the greps when the list is older than the code.

1. **Build constraints — `//go:build`.** 16 files across `cmd/zephyr/`
   (`platform_*.go`, `titlebar_*.go`), `internal/fileio/` (`syncdir_*.go`,
   `metadata_*.go`, and a `_test.go`), `internal/render/emoji_other.go` and
   `pkg/clipboard/`. The Go toolchain *parses* these: deleting or rewording one
   silently selects the wrong platform's file, and the local build compiles only
   the host variant, so the failure surfaces in CI's Windows or Linux leg, not
   here. `go vet`'s **`buildtag`** and **`directive`** analyzers read them too
   (`go tool vet help`), and `go vet ./...` runs in `make gate` and in CI.
   **Attachment matters:** a `//go:build` line must sit in the file's leading
   comment block with a blank line before `package`. Inserting or removing a
   line near it can detach it into an ordinary comment with no error anywhere.
   `grep -rn --include='*.go' '^//go:build' .`
2. **`//go:embed`.** Two sites: `internal/vim/tutor.go` (embeds
   `tutor_content.txt`) and `internal/config/defaults.go` (embeds
   `default_theme.yaml`). The directive must sit immediately above its `var`;
   deleting it, or inserting a line between it and the var, breaks the build or
   silently unbinds it. `grep -rn --include='*.go' '//go:embed' .`
3. **cgo preambles — the comment block *is* compiled source.** Three files:
   `cmd/zephyr/titlebar_darwin.go` (Objective-C: `#cgo CFLAGS: -x objective-c`,
   `#cgo LDFLAGS: -framework Cocoa -framework QuartzCore`, `#import` lines, and
   whole ObjC classes and C functions), `internal/fileio/metadata_darwin.go`
   (C: `#include <sys/xattr.h>` plus static wrapper functions), and
   `pkg/clipboard/clipboard_darwin.go`. **The `/* … */` block immediately above
   `import "C"` is code. Workers do not enter these blocks at all** — not to
   delete, not to compress, not to reflow — and a blank line inserted between
   the block and `import "C"` detaches the preamble entirely. Treat the whole
   file as out of scope unless the task is specifically about its *Go* comments,
   and then diff it by hand. `grep -rln --include='*.go' 'import "C"' .`
4. **`go doc` on exported identifiers.** `pkg/clipboard`'s `Get` and `Set` are
   the module's only `pkg/` surface; their doc comments are what `go doc
   ./pkg/clipboard` renders. Falsehood fixes only — do not compress a published
   doc comment for line count. Honest caveat: the module path is
   `github.com/kristianweb/zephyr` while the repo is hosted at
   `github.com/kkollsga/zephyr`, so **pkg.go.dev cannot fetch this module
   today** — the renderer that actually reads these is local `go doc`. If the
   module path is ever reconciled, this class becomes a published contract and
   answers to `R6`/`R9` discipline, not comment hygiene.
5. **Fixture files whose comments are test *input*, not documentation.**
   `testdata/gui/mouse_fixture.go` is the default fixture for **both**
   `scripts/gui-test.sh` (`DEFAULT_FIXTURE`) and `scripts/perf-test.sh`
   (default `FIXTURE`); its own header says it is "intentionally long and
   contains tabs, Unicode, and nested folds". Its comment lines set the file's
   length, fold structure and line layout that the harness's fixed-pixel clicks,
   drag coordinates and screenshot baselines land on. **`testdata/` is entirely
   out of scope for this skill.**
6. **staticcheck suppression directives — the class exists, the population is
   currently zero.** CI's quality job runs `staticcheck ./...`, which reads
   `//lint:ignore <check> <reason>` and `//lint:file-ignore`. A grep for
   `//lint:` and `//nolint` returns nothing today, so no comment is currently
   holding a suppression — **re-run the grep rather than trusting this
   sentence**, because the day one is added it becomes load-bearing with nothing
   at the site to say so.

Adjacent, and named so a worker does not mistake it for a comment: **`install.sh`'s
usage heredoc** is grepped verbatim by `scripts/docs-test.sh` (`make docs-test`,
also a CI step), as are exact strings *and counts* in `README.md`,
`docs/index.html` and `docs/install.md`. Those are published prose, not comments,
and they are out of scope here.

**De-duplication: within-file only.** Keep the fullest statement at the
most-read location and point the others at it. A fact repeated *across* files is
flagged to the coordinator, never collapsed by a worker — that decision needs
cross-file sight.

## 3. Calibrate — one worker, and this gate can fail (`R1`)
Dispatch one worker on one representative head file — pick a plain Go file, not
a cgo or fixture one. Read its **full diff** yourself against the brief. If it
holds, fan out. If not, fix the brief and calibrate again on a different file;
after two failed calibrations, stop and surface the diffs to the user. This gate
failed for real in the audit that bought this procedure: the first doctrine
passed 231 files while moving comment volume 0.2 %, and only a human read of the
calibration diff caught it.

## 4. Fan out
One worker per remaining head file, in bounded parallel batches. Each dispatch
is the brief plus the file path plus this contract:
- Read the entire file before editing anything.
- Comment and doc lines only. Apply the two tests per paragraph; respect the
  floor and the reader list.
- Fix false comments — claims the adjacent code contradicts, and expired
  predictions ("a later phase will…") that the work landing already retired.
- **Check doc-comment attachment.** In Go a doc comment binds by adjacency: a
  blank line between it and its declaration, or a declaration inserted into the
  middle of a block, silently re-points it at the *next* item and nothing
  complains. After any edit near a declaration, confirm the comment still lands
  on what it describes.
- Return a structured result: lines deleted / lines compressed (from → to),
  false comments fixed, cross-file duplicates flagged, code defects noticed (not
  fixed), anything left untouched and why.

A worker that fails is retried once, then its file is reported unprocessed
(`R12`). **Never hand a worker a bulk fixer script** — a fixer that matched
every fence opener rather than only malformed ones re-indented two well-formed
blocks in user-facing docs. Hand-fix or revert.

## 5. Verify mechanically — the diff, never the self-reports
Worker summaries have been wrong in exactly the reassuring direction. In this
order:

1. **Comment-only diff check, BEFORE the formatter.** Every changed line, on
   both sides of the diff, must start with a comment marker or be blank:
   ```bash
   git diff -U0 -- '*.go' | grep -E '^[+-]' | grep -vE '^(\+\+\+|---)' \
     | sed -E 's/^[+-][[:space:]]*//' | grep -vE '^(//|$)' | head
   ```
   Any output is a code change: revert that file and report it. **This check is
   not valid inside a cgo preamble** — those `//` lines are C comments in
   compiled source and would pass it while changing what the compiler sees.
   Preamble files are excluded from the sweep, not checked by this rule.
2. **`make fmt` for real** (`gofmt -w .`), not `-l`. Comment removal moves code:
   gofmt aligns trailing comments in a column, so deleting one re-aligns its
   neighbours, and removing a separator can reflow a block. Code motion
   introduced *by gofmt* is the only non-comment change allowed in the final
   diff — re-run check 1 before it, never after.
3. **Re-run the gates that read comments.** `make gate` (`check-dev-docs`,
   `go vet ./...` — which is where a broken `//go:build` or `//go:embed`
   surfaces — and `go test ./... -count=1`), plus `go build ./...`. If a
   build-tagged file was touched, the host build only proves one variant: say so
   and let CI's Windows and Linux legs be the check (`R10` corollary). If
   staticcheck suppressions existed in the touched files, re-run
   `go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...` — the same pin CI
   uses. An unexplained failure reverts the file that caused it.
4. **If any file under `cmd/zephyr/` or `internal/render/` was touched, the
   visual check still applies**: `make gui-test-build && make gui-test-launch`,
   `./scripts/gui-test.sh capture`. A comment-only change should move no pixels;
   if the capture cannot be taken, say "not run" rather than implying it passed.

## 6. Report
- **Deletion and compression separately, per file — never one percentage.** The
  rate splits hard by file character: mechanical emitters lose 25–38 %,
  specification-like files 2–11 %, and both are correct. A −2 % on a dense
  contract file is a success, not a shirk. An agent tuned on line count fails on
  exactly the files whose comments matter most.
- **Say what was excluded and why** — cgo preamble files, `testdata/`, published
  doc comments — so the next run does not re-litigate it.
- Findings fixed in-run are part of the diff. Anything larger — code defects
  workers noticed, cross-file de-duplication decisions, a gap in the reader list
  above — goes through **`add-todo`** under its entry rules. **Anything reported
  as a finding meets `R15`'s bar**: a concrete failure with its input, state and
  wrong outcome named, or it is not reported. Reading comments against code is
  effective static analysis — budget for findings, not just deletions.
- Offload the long form to the ephemeral tier `dev-docs/README.md` names and
  give the path; keep the inline summary lean.

## Relationship to phased-plan
Not part of one, on purpose — no branch ceremony, no plan doc. A first-ever
whole-tree audit at campaign scale would wrap this in a `phased-plan`; for
everything else this skill is complete on its own.
