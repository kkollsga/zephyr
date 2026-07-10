#!/usr/bin/env bash
set -euo pipefail

# Bump the version in the repo-root VERSION file.
# Usage: bump-version.sh [patch|minor|major]   (default: patch)

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
VERSION_FILE="$ROOT/VERSION"

component=${1:-patch}
case "$component" in
    patch|minor|major) ;;
    *)
        echo "usage: $(basename "$0") [patch|minor|major]" >&2
        exit 2
        ;;
esac

[[ -f "$VERSION_FILE" ]] || {
    echo "error: VERSION file not found: $VERSION_FILE" >&2
    exit 1
}

old=$(tr -d '[:space:]' <"$VERSION_FILE")
[[ "$old" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]] || {
    echo "error: VERSION must be X.Y.Z, got: $old" >&2
    exit 1
}

major=${BASH_REMATCH[1]}
minor=${BASH_REMATCH[2]}
patch=${BASH_REMATCH[3]}

case "$component" in
    major) major=$((major + 1)); minor=0; patch=0 ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    patch) patch=$((patch + 1)) ;;
esac

new="$major.$minor.$patch"
printf '%s\n' "$new" >"$VERSION_FILE"

echo "$old -> $new"
