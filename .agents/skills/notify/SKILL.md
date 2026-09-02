---
name: notify
description: Send a coordination or feedback note to another local project's inbox. Resolves a target repo by name under the Koding/ workspace root, refuses an ambiguous match, composes a message file per the shared inbox schema, and drops it in that repo's inbox/unread/ (creating the folder if missing). Sends only; receiving is read-inbox.
---

# notify

**Adapted from doctrine reference 0.1.9** (oracle:
`/Volumes/EksternalHome/Koding/Rust/doctrine`; read it before this copy, `R14`).

Deliver a message to a sibling project's inbox so its maintainer or agent picks
it up on their next `read-inbox`. Input: a **target repo** (name or path) and
the **message** (topic + body; compose from the conversation if not given).

The filename schema and message anatomy are `inbox/README.md`'s — the send and
receive sides share them, which is why cross-project coordination needs no
per-pair wiring. This skill owns resolution and the send bar.

## 1. Resolve the target repo path
The workspace root is **`/Volumes/EksternalHome/Koding`** and projects sit two
levels down as `Koding/<Language>/<repo>` — `Go/zephyr`, `Rust/KGLite`,
`Rust/doctrine`, `Rust/codingest`, `HTML/flow-chart`, `Python/<repo>`, and so
on. Resolve by looking for the target's **inbox**, not merely a directory with
the right name:

```bash
KODING=/Volumes/EksternalHome/Koding
find "$KODING" -maxdepth 3 -type d -path "*/<name>/inbox" \
  -not -path '*-worktrees/*' \
  -not -path '*/node_modules/*' -not -path '*/.venv/*' \
  -not -path '*/target/*'     -not -path '*/.git/*'
```

- **Exactly one match** → use it.
- **Several matches** → **refuse and ask the user which path**, showing the
  candidates. Do not guess: sending writes into another project's working tree,
  and a note delivered to the wrong repo is invisible to both parties.
- **No match** → the target may run this system without a folder yet, or may not
  run it at all. Re-run the search for the repo directory itself
  (`-type d -name '<name>'` with the same exclusions). One match → create the
  inbox (step 2). Still nothing → ask the user for the path; never create a new
  top-level directory on a guess.
- If the caller gave an absolute path, skip the search and use it.

Two workspace facts that break a naive search, both verified:
- **`Koding/mcp-servers/` is one externally-managed project, not a tree of
  repos.** Its subdirectories (`code_review/`, `legal/`, `kglite/`,
  `open_source/`, …) are **not** notify targets; the project has a single
  `inbox/` at `Koding/mcp-servers/inbox/`. Target `mcp-servers` itself, and
  never resolve a name to `mcp-servers/<subdir>/`.
- **`*-worktrees/` directories are excluded** — an agent worktree is a checkout
  of a repo already in the list, and a note dropped into one is deleted when the
  release flow empties that directory.

The estate's rule-keeper is the `doctrine` repo, whose inbox is
`/Volumes/EksternalHome/Koding/Rust/doctrine/inbox/unread/`. That is where a
doctrine-level observation goes — a rule that misfired here, a procedure worth
upstreaming, a `[local-sweep]` that could not be executed as written.

Confirm the resolved path before writing if there was any ambiguity at all.

## 2. Ensure the inbox exists
```bash
mkdir -p "<target>/inbox/unread"
```
Expected for a first note to a project that runs this system but has no folder
yet. Creating an inbox in a repo that does *not* run the system means nobody
will ever read it — prefer telling the user.

## 3. Compose the message
Filename: **`<YYYY-MM-DD>-from-zephyr-<topic-slug>.md`** — session date,
kebab-case topic, sender `zephyr` (see `inbox/README.md`). Body:

```markdown
# <Short title>

- **From:** zephyr
- **To:** <target repo>
- **Date:** <YYYY-MM-DD>
- **Type:** feedback | bug | coordination | heads-up | request
- **Re:** <optional — version, file, PR, release tag, or prior message>

<1–3 paragraphs of context: what happened / what's needed and why.>

## Ask / action requested
- <concrete, actionable item(s) — or "FYI, no action needed">

## References
- <links, file paths, commit SHAs, versions, release tags — optional>
```

Cite an exact site — `internal/<pkg>/<file>.go:<line>`, a commit sha, a release
tag `vX.Y.Z` — rather than a description of where the thing roughly is. Never
cite a `dev-docs/` path in an outgoing note: that folder is gitignored, local
and unbacked, so the citation is a dangling instruction the moment it is read on
the other side.

## Send discipline
The outbound bar is **"changes what the recipient does"**, not "true and
relevant". Every note lands as triage work in someone's `read-inbox` — in this
estate, often the same person operating both ends.

- **Batch per target per session.** Collect everything for a given target and
  send one note. An immediate single-purpose note needs a **blocker** (work here
  cannot proceed), a **reply the target explicitly requested**, or a
  **time-sensitive coordination fact** — otherwise it waits for the batch.
- **No FYI-grade notes.** A bare acknowledgement, a "done on our side" or a
  progress report all fail the bar; the acknowledgement belongs in *our* copy's
  Status footer at archive time (`read-inbox` step 5), not in their `unread/`.
- **At most one ping per stalled thread**, and only carrying new evidence — a
  number, a repro, a commit. A bare "any update?" is noise.
- **Related items piggyback** on the next legitimate note instead of earning
  their own file.

**A note is not a publication and is not an approval.** Writing into a sibling's
gitignored `inbox/` is local file I/O, so it needs no release-grade
authorization — but anything outward-facing (a GitHub issue, a PR comment, an
email, anything attributed to the maintainer) does, in the turn it happens
(`R6`), and `notify` is not a route around that.

## 4. Write + report
Write the file to `<target>/inbox/unread/<filename>` and report the full path.
Do not move or touch anything in Zephyr's own inbox — this skill only *sends*.

## Notes
- Keep the response under 400 tokens.
- This is the send side; `read-inbox` is the receive side.
- If the resolved target was ambiguous, confirm with the user before writing.
