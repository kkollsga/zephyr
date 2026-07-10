# Zephyr — agent instructions

## Versioning and releases

- `VERSION` at the repo root is the version source of truth (plain `X.Y.Z`, no `v` prefix). The Makefile injects it via ldflags; the app shows it in the macOS Zephyr menu.
- **Bump the patch version on commit+push only if the current `VERSION` has actually been released** (a published GitHub release for `vX.Y.Z` exists): run `./scripts/bump-version.sh` (defaults to patch) and include the `VERSION` change in the commit being pushed. If the current version is unreleased (e.g. its release pipeline failed), do NOT bump — push the fix with the same version and the auto-release workflow re-tags and retries it. Version numbers track published releases, not attempts. Pushing always requires explicit user permission.
- **Never bump the minor or major version without the user's explicit permission** for that specific release. Patch bumps are automatic; minor/major are user-initiated only.
- Releases are tag-driven (`v*` tags trigger `.github/workflows/release.yml`); creating tags/releases also requires explicit user permission.

## Testing

- `make test` for unit tests; `make vet` before committing.
- Full macOS GUI test harness: `make gui-test-build && make gui-test-launch`, then drive with `./scripts/gui-test.sh` (click/type/key/scroll/capture). See `docs/gui-testing.md`. `make gui-test-smoke` and `make gui-test-regression` for canned suites. Requires Accessibility + Screen Recording permission (already granted for the usual host).
- Verify UI changes visually with `./scripts/gui-test.sh capture` before declaring them done.
