#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
COMMAND='curl -fsSL https://raw.githubusercontent.com/kkollsga/zephyr/main/install.sh | bash'

for file in README.md docs/index.html docs/install.md; do
    grep -Fq "$COMMAND" "$ROOT/$file" || {
        echo "$file does not contain the primary macOS install command" >&2
        exit 1
    }
done

[[ $(grep -Fc "$COMMAND" "$ROOT/README.md") -eq 1 ]]
[[ $(grep -Fc "$COMMAND" "$ROOT/docs/install.md") -eq 1 ]]
[[ $(grep -Fc "$COMMAND" "$ROOT/docs/index.html") -eq 2 ]]

grep -Fq 'permalink: /install/' "$ROOT/docs/install.md"
grep -Fq "{{ '/install/' | relative_url }}" "$ROOT/docs/index.html"
grep -Fq 'Running the installer again upgrades' <("$ROOT/install.sh" --help)
grep -Fq 'Requires macOS 12 or later' "$ROOT/README.md"
grep -Fq 'requires macOS 12 or later' "$ROOT/docs/install.md"
grep -Fq 'requires and verifies its release' "$ROOT/docs/install.md"
grep -Fq 'does not authenticate an Apple-verified publisher' "$ROOT/docs/install.md"
[[ $(grep -Fc 'onclick="copyInstallCommand(this)"' "$ROOT/docs/index.html") -eq 2 ]]
[[ $(grep -Fc 'onclick="copyInstallCommand(this)"' "$ROOT/docs/install.md") -eq 1 ]]
grep -Fq "{{ '/assets/js/site.js' | relative_url }}" "$ROOT/docs/_layouts/default.html"
node --check "$ROOT/docs/assets/js/site.js"
if grep -REq 'xattr -[a-z]*[cr]|com\.apple\.quarantine|After installing' \
    "$ROOT/README.md" "$ROOT/docs/index.html" "$ROOT/docs/install.md"; then
    echo "legacy manual quarantine instructions remain in user-facing docs" >&2
    exit 1
fi

include=$(mktemp)
trap 'rm -f "$include"' EXIT
sed -n '/^### Windows$/,/^## [^I]/{ /^## [^I]/d; p; }' "$ROOT/README.md" >"$include"
grep -q '^### Windows$' "$include"
if grep -q '^## Installation$\|^### macOS$' "$include" || grep -Fq "$COMMAND" "$include"; then
    echo "generated homepage include duplicates its macOS installation block" >&2
    exit 1
fi

echo "documentation install-path checks passed"
