#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TMP=$(mktemp -d "${TMPDIR:-/tmp}/zephyr-install-test.XXXXXX")
trap 'rm -rf "$TMP"' EXIT

APP_SOURCE="$TMP/source/Zephyr.app"
INSTALL_DIR="$TMP/Applications"
BIN_DIR="$TMP/bin"
mkdir -p "$APP_SOURCE/Contents/MacOS"

write_fake_app() {
    local version=$1
    printf '#!/usr/bin/env bash\nprintf '\''Zephyr %s (test, built test, darwin/arm64)\\n'\''\n' "$version" \
        >"$APP_SOURCE/Contents/MacOS/zephyr"
    chmod +x "$APP_SOURCE/Contents/MacOS/zephyr"
}

run_installer() {
    ZEPHYR_APP_SOURCE="$APP_SOURCE" \
    ZEPHYR_INSTALL_DIR="$INSTALL_DIR" \
    ZEPHYR_BIN_DIR="$BIN_DIR" \
    ZEPHYR_NO_SUDO=1 \
    ZEPHYR_SKIP_REGISTER=1 \
        "$ROOT/install.sh" --version "$1"
}

assert_no_transaction_artifacts() {
    if find "$INSTALL_DIR" -maxdepth 1 \
        \( -name '.Zephyr.app.backup.*' -o -name '.Zephyr.app.install.*' \) \
        -print | grep -q .; then
        echo "installer left transaction artifacts" >&2
        exit 1
    fi
}

write_fake_app v9.9.1
run_installer v9.9.1
[[ -x "$INSTALL_DIR/Zephyr.app/Contents/MacOS/zephyr" ]]
[[ -L "$BIN_DIR/zephyr" ]]
"$BIN_DIR/zephyr" --version | grep -q 'v9.9.1'

# A failure after the bundle replacement must restore both the previous app and
# the existing CLI state, then remove staging and backup artifacts.
write_fake_app v9.9.2
if ZEPHYR_TEST_FAIL_AFTER_REPLACE=1 run_installer v9.9.2 >/dev/null 2>&1; then
    echo "post-replacement failure unexpectedly succeeded" >&2
    exit 1
fi
"$INSTALL_DIR/Zephyr.app/Contents/MacOS/zephyr" --version | grep -q 'v9.9.1'
"$BIN_DIR/zephyr" --version | grep -q 'v9.9.1'
assert_no_transaction_artifacts

run_installer v9.9.2
"$BIN_DIR/zephyr" --version | grep -q 'v9.9.2'
assert_no_transaction_artifacts

chmod -x "$APP_SOURCE/Contents/MacOS/zephyr"
if run_installer v9.9.3 >/dev/null 2>&1; then
    echo "broken app source unexpectedly installed" >&2
    exit 1
fi
"$BIN_DIR/zephyr" --version | grep -q 'v9.9.2'

write_fake_app v9.9.4
COLLISION_DIR="$TMP/collision-bin"
mkdir -p "$COLLISION_DIR"
printf 'unrelated command\n' >"$COLLISION_DIR/zephyr"
if ZEPHYR_APP_SOURCE="$APP_SOURCE" \
    ZEPHYR_INSTALL_DIR="$INSTALL_DIR" \
    ZEPHYR_BIN_DIR="$COLLISION_DIR" \
    ZEPHYR_NO_SUDO=1 ZEPHYR_SKIP_REGISTER=1 \
    "$ROOT/install.sh" --version v9.9.4 >/dev/null 2>&1; then
    echo "CLI collision unexpectedly overwritten" >&2
    exit 1
fi
grep -q 'unrelated command' "$COLLISION_DIR/zephyr"
"$BIN_DIR/zephyr" --version | grep -q 'v9.9.2'

if run_installer not-a-version >/dev/null 2>&1; then
    echo "invalid version unexpectedly accepted" >&2
    exit 1
fi

# Requiring elevation for the app must not run user-local CLI operations under
# sudo. A small sudo shim makes the protected test directory writable only for
# each privileged command and records every invocation.
write_fake_app v9.9.5
FAKE_BIN="$TMP/fake-bin"
FAKE_SUDO_LOG="$TMP/fake-sudo.log"
MIXED_PARENT="$TMP/protected"
MIXED_INSTALL="$MIXED_PARENT/Applications"
MIXED_BIN="$TMP/user-bin"
mkdir -p "$FAKE_BIN" "$MIXED_PARENT" "$MIXED_BIN"
printf '%s\n' \
    '#!/usr/bin/env bash' \
    'printf '\''%s\n'\'' "$*" >>"$FAKE_SUDO_LOG"' \
    'chmod u+w "$FAKE_SUDO_PARENT"' \
    '"$@"' \
    'rc=$?' \
    'chmod u-w "$FAKE_SUDO_PARENT"' \
    'exit "$rc"' >"$FAKE_BIN/sudo"
chmod +x "$FAKE_BIN/sudo"
chmod u-w "$MIXED_PARENT"
PATH="$FAKE_BIN:$PATH" \
FAKE_SUDO_LOG="$FAKE_SUDO_LOG" FAKE_SUDO_PARENT="$MIXED_PARENT" \
ZEPHYR_APP_SOURCE="$APP_SOURCE" ZEPHYR_INSTALL_DIR="$MIXED_INSTALL" \
ZEPHYR_BIN_DIR="$MIXED_BIN" ZEPHYR_SKIP_REGISTER=1 \
    "$ROOT/install.sh" --version v9.9.5 >/dev/null
[[ -L "$MIXED_BIN/zephyr" ]]
if grep -Fq "$MIXED_BIN" "$FAKE_SUDO_LOG"; then
    echo "user-local CLI operation unexpectedly used sudo" >&2
    exit 1
fi
chmod u+w "$MIXED_PARENT"

# Network installs must fail closed when checksums are missing or mismatched.
RELEASE_DIR="$TMP/release"
mkdir -p "$RELEASE_DIR"
CHECKSUM_VERSION=v8.8.8
CHECKSUM_ASSET="Zephyr-${CHECKSUM_VERSION}-macos.dmg"
printf 'not a real dmg\n' >"$RELEASE_DIR/$CHECKSUM_ASSET"
MISSING_CHECKSUM_LOG="$TMP/missing-checksum.log"
if ZEPHYR_RELEASE_BASE="file://$RELEASE_DIR" \
    ZEPHYR_INSTALL_DIR="$TMP/checksum-apps" ZEPHYR_BIN_DIR="$TMP/checksum-bin" \
    ZEPHYR_NO_SUDO=1 ZEPHYR_SKIP_REGISTER=1 \
    "$ROOT/install.sh" --version "$CHECKSUM_VERSION" >/dev/null 2>"$MISSING_CHECKSUM_LOG"; then
    echo "release without checksums unexpectedly accepted" >&2
    exit 1
fi
grep -q 'required SHA256SUMS' "$MISSING_CHECKSUM_LOG"
printf '%064d  %s\n' 0 "$CHECKSUM_ASSET" >"$RELEASE_DIR/SHA256SUMS"
MISMATCH_CHECKSUM_LOG="$TMP/mismatch-checksum.log"
if ZEPHYR_RELEASE_BASE="file://$RELEASE_DIR" \
    ZEPHYR_INSTALL_DIR="$TMP/checksum-apps" ZEPHYR_BIN_DIR="$TMP/checksum-bin" \
    ZEPHYR_NO_SUDO=1 ZEPHYR_SKIP_REGISTER=1 \
    "$ROOT/install.sh" --version "$CHECKSUM_VERSION" >/dev/null 2>"$MISMATCH_CHECKSUM_LOG"; then
    echo "release with mismatched checksum unexpectedly accepted" >&2
    exit 1
fi
grep -q 'checksum verification failed' "$MISMATCH_CHECKSUM_LOG"

# The first public release predates SHA256SUMS; its embedded digest must be
# selected (and reject non-matching content) instead of reporting it missing.
LEGACY_ASSET='Zephyr-v0.1.0-alpha-macos.dmg'
printf 'not the published legacy dmg\n' >"$RELEASE_DIR/$LEGACY_ASSET"
rm -f "$RELEASE_DIR/SHA256SUMS"
LEGACY_CHECKSUM_LOG="$TMP/legacy-checksum.log"
if ZEPHYR_RELEASE_BASE="file://$RELEASE_DIR" \
    ZEPHYR_INSTALL_DIR="$TMP/checksum-apps" ZEPHYR_BIN_DIR="$TMP/checksum-bin" \
    ZEPHYR_NO_SUDO=1 ZEPHYR_SKIP_REGISTER=1 \
    "$ROOT/install.sh" --version v0.1.0-alpha >/dev/null 2>"$LEGACY_CHECKSUM_LOG"; then
    echo "legacy release with mismatched content unexpectedly accepted" >&2
    exit 1
fi
grep -q 'checksum verification failed' "$LEGACY_CHECKSUM_LOG"

echo "macOS terminal install/upgrade test passed"
