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
  (doctrine `R11`). `make bench-anchor` compares the newest capture against one
  roughly three releases back for that reason; **recapturing does not clear a
  failing anchor check — only recovering the performance does.**
- **Bound (`R4`).** One file per release, a few kilobytes each, added only by
  the release flow. At Zephyr's release cadence this tier grows by well under
  a megabyte a year, so it is bounded by the writer rather than by a purge:
  if it ever stops being, the fix is to thin *old* releases, never the newest.

Each file carries a `#` metadata header — host, arch, OS, Go version and
series, commit, `git describe`, UTC date, the load averages the capture ran
under, and the `-count`/`-benchtime` used. A longitudinal number carries the
conditions it was taken under (`R11`); `make bench-anchor` reads host, arch and
Go series back and returns VOID rather than a verdict when they do not match.

Two of the rows are controls from `internal/benchcontrol`, which measure no
Zephyr code at all. Load moves every cell together and a real regression moves
one, so the controls are what makes the difference readable.

## Ad-hoc captures

A capture taken to answer a question — before/after a change, a noise-floor
measurement, a branch comparison — is **not** release history and does not
belong here. `scripts/bench-capture.sh` with no argument writes it into the
gitignored working folder's durable bench tier instead
(`dev-docs/bench/results/`, described by that folder's own `README.md`), and
`scripts/bench-capture.sh <path>` puts it wherever you ask.
