# `bench/` — the committed benchmark record

## `bench/history/<version>.tsv`

One file per **released** version, named for the `VERSION` that release shipped
(`0.1.1.tsv`, not `v0.1.1.tsv`).

- **Owner: the release flow.** `.agents/skills/release/SKILL.md` §5 runs
  `make bench-capture` into `bench/history/<new version>.tsv` and stages it in
  the release commit. Nothing else writes here — not `make gate`, not CI, not a
  phase commit.
- **Never rewritten.** A file here is the capture that release actually shipped
  with. Recapturing on top of an existing version erases the only evidence of
  what that release measured, and it is exactly what makes a per-release gate
  blind: a gate that recaptures its own baseline every release cannot see 10 %
  of drift per release, because each single step passes the threshold
  (doctrine `R11`). `scripts/check-bench-anchor.sh` compares the newest capture
  against one roughly three releases back for that reason; **recapturing does
  not clear a failing anchor check — only recovering the performance does.**
- **Bound (`R4`).** One file per release, a few kilobytes each, added only by
  the release flow. At Zephyr's release cadence this tier grows by well under
  a megabyte a year, so it is bounded by the writer rather than by a purge:
  if it ever stops being, the fix is to thin *old* releases, never the newest.

Each file carries a `#` metadata header — host, arch, OS, Go version and
series, commit, `git describe`, UTC date, the load averages the capture ran
under, and the `-count`/`-benchtime` used. A longitudinal number carries the
conditions it was taken under (`R11`); the anchor check reads host, arch and Go
series back and returns VOID rather than a verdict when they do not match.

## Retry captures

A regression verdict is recaptured once before it is believed, and both files
are kept under `.artifacts/bench/` — never here, which is one file per released
version. That tier is bounded by its writer: `scripts/check-bench-anchor.sh`
keeps the five newest retry captures and deletes the rest on every write
(`R4`). Read that script's own exit code for the verdict: `make bench-anchor`
wraps it, and make collapses every non-zero status to `2`, so a FAIL and a VOID
are indistinguishable through the target.

Two of the rows are controls from `internal/benchcontrol`, which measure no
Zephyr code at all. Load moves every cell together and a real regression moves
one, so the controls are what makes the difference readable.

## Ad-hoc captures

A capture taken to answer a question — before/after a change, a noise-floor
measurement, a branch comparison — is **not** release history and does not
belong here. `scripts/bench-capture.sh` with no argument writes it into the
gitignored working folder's durable bench tier instead — that folder's own
`README.md` is the map of its tiers — and `scripts/bench-capture.sh <path>`
puts it wherever you ask.
