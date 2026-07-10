#!/usr/bin/env bash
set -euo pipefail

REPO="${ZEPHYR_REPO:-kkollsga/zephyr}"
INSTALL_DIR="${ZEPHYR_INSTALL_DIR:-/Applications}"
BIN_DIR="${ZEPHYR_BIN_DIR:-/usr/local/bin}"
REQUESTED_VERSION="${ZEPHYR_VERSION:-latest}"
INSTALL_CLI=1
OPEN_AFTER=0

usage() {
    cat <<'EOF'
Install or upgrade Zephyr on macOS.

Usage: install.sh [options]
  --version TAG       Install a release tag instead of latest
  --install-dir DIR   Application directory (default: /Applications)
  --bin-dir DIR       CLI symlink directory (default: /usr/local/bin)
  --no-cli            Do not install the `zephyr` terminal command
  --open              Open Zephyr after installation
  -h, --help          Show this help

Running the installer again upgrades the existing installation in place.
EOF
}

while (($#)); do
    case "$1" in
    --version)
        [[ $# -ge 2 ]] || { echo "error: --version requires a tag" >&2; exit 2; }
        REQUESTED_VERSION=$2
        shift 2
        ;;
    --install-dir)
        [[ $# -ge 2 ]] || { echo "error: --install-dir requires a directory" >&2; exit 2; }
        INSTALL_DIR=$2
        shift 2
        ;;
    --bin-dir)
        [[ $# -ge 2 ]] || { echo "error: --bin-dir requires a directory" >&2; exit 2; }
        BIN_DIR=$2
        shift 2
        ;;
    --no-cli)
        INSTALL_CLI=0
        shift
        ;;
    --open)
        OPEN_AFTER=1
        shift
        ;;
    -h | --help)
        usage
        exit 0
        ;;
    *)
        echo "error: unknown option: $1" >&2
        usage >&2
        exit 2
        ;;
    esac
done

[[ "$(uname -s)" == "Darwin" ]] || {
    echo "error: this installer is for macOS" >&2
    exit 1
}
command -v curl >/dev/null || { echo "error: curl is required" >&2; exit 1; }
command -v ditto >/dev/null || { echo "error: ditto is required" >&2; exit 1; }
command -v xattr >/dev/null || { echo "error: xattr is required" >&2; exit 1; }

if [[ "$REQUESTED_VERSION" == "latest" ]]; then
    latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' \
        "https://github.com/$REPO/releases/latest")
    REQUESTED_VERSION=${latest_url##*/}
fi
[[ -n "$REQUESTED_VERSION" && "$REQUESTED_VERSION" != "latest" ]] || {
    echo "error: could not determine the latest Zephyr release" >&2
    exit 1
}
version_pattern='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?(\+[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$'
[[ "$REQUESTED_VERSION" =~ $version_pattern ]] || {
    echo "error: invalid Zephyr release tag: $REQUESTED_VERSION (expected vMAJOR.MINOR.PATCH)" >&2
    exit 2
}

asset="Zephyr-${REQUESTED_VERSION}-macos.dmg"
release_base="${ZEPHYR_RELEASE_BASE:-https://github.com/$REPO/releases/download/$REQUESTED_VERSION}"
target_app="$INSTALL_DIR/Zephyr.app"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/zephyr-install.XXXXXX")
mount_point="$tmp/mount"
mounted=0
stage=""
backup=""
cli_stage=""
transaction_active=0
original_present=0
backup_ready=0
new_app_installed=0
cli_created=0
rollback_failed=0

cleanup() {
    local status=$?
    trap - EXIT INT TERM
    set +e
    if [[ $transaction_active -eq 1 ]]; then
        if [[ $cli_created -eq 1 && -L "${cli_path:-}" &&
              "$(readlink "$cli_path" 2>/dev/null)" == "${target_executable:-}" ]]; then
            run_cli rm -f "$cli_path" || rollback_failed=1
        fi
        if [[ $backup_ready -eq 1 && ( -e "$backup" || -L "$backup" ) ]]; then
            target_removed=1
            if [[ $new_app_installed -eq 1 ]] && ! run_app rm -rf "$target_app"; then
                target_removed=0
                rollback_failed=1
                echo "error: could not remove failed replacement; backup remains at $backup" >&2
            fi
            if [[ $target_removed -eq 1 ]] && ! run_app mv "$backup" "$target_app"; then
                rollback_failed=1
                echo "error: could not restore the previous app; backup remains at $backup" >&2
            elif [[ $target_removed -eq 1 ]]; then
                echo "Previous Zephyr installation restored." >&2
            fi
        elif [[ $original_present -eq 0 && $new_app_installed -eq 1 ]]; then
            run_app rm -rf "$target_app" || rollback_failed=1
        fi
    elif [[ $backup_ready -eq 1 && ( -e "$backup" || -L "$backup" ) ]]; then
        if ! run_app rm -rf "$backup"; then
            rollback_failed=1
            echo "error: installed app is valid, but old backup remains at $backup" >&2
        fi
    fi
    [[ -z "$stage" ]] || run_app rm -rf "$stage" >/dev/null 2>&1 || true
    [[ -z "$cli_stage" ]] || run_cli rm -f "$cli_stage" >/dev/null 2>&1 || true
    if [[ $mounted -eq 1 ]]; then
        hdiutil detach "$mount_point" -quiet >/dev/null 2>&1 || true
    fi
    rm -rf "$tmp"
    if [[ $rollback_failed -eq 1 ]]; then
        status=1
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

nearest_existing_dir() {
    local path=$1
    while [[ ! -d "$path" && "$path" != "/" ]]; do
        path=$(dirname "$path")
    done
    printf '%s\n' "$path"
}

APP_PRIVILEGED=()
CLI_PRIVILEGED=()
if [[ "${ZEPHYR_NO_SUDO:-0}" != "1" ]]; then
    install_parent=$(nearest_existing_dir "$INSTALL_DIR")
    bin_parent=$(nearest_existing_dir "$BIN_DIR")
    if [[ ! -w "$install_parent" || ( $INSTALL_CLI -eq 1 && ! -w "$bin_parent" ) ]]; then
        command -v sudo >/dev/null || {
            echo "error: sudo is required to install into $INSTALL_DIR and $BIN_DIR" >&2
            exit 1
        }
    fi
    if [[ ! -w "$install_parent" ]]; then
        APP_PRIVILEGED=(sudo)
    fi
    if [[ $INSTALL_CLI -eq 1 && ! -w "$bin_parent" ]]; then
        CLI_PRIVILEGED=(sudo)
    fi
fi

run_app() {
    if ((${#APP_PRIVILEGED[@]})); then
        "${APP_PRIVILEGED[@]}" "$@"
    else
        "$@"
    fi
}

run_cli() {
    if ((${#CLI_PRIVILEGED[@]})); then
        "${CLI_PRIVILEGED[@]}" "$@"
    else
        "$@"
    fi
}

source_app="${ZEPHYR_APP_SOURCE:-}"
downloaded_release=0
if [[ -z "$source_app" ]]; then
    downloaded_release=1
    dmg="$tmp/$asset"
    echo "Downloading Zephyr $REQUESTED_VERSION..."
    curl -fL --retry 3 --progress-bar "$release_base/$asset" -o "$dmg"

    expected=""
    if curl -fsL --retry 3 "$release_base/SHA256SUMS" -o "$tmp/SHA256SUMS" 2>/dev/null; then
        expected=$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1; exit }' "$tmp/SHA256SUMS")
    elif [[ "$REPO" == "kkollsga/zephyr" && "$asset" == "Zephyr-v0.1.0-alpha-macos.dmg" ]]; then
        # The first public release predates SHA256SUMS. Preserve a verified
        # digest here so the terminal-first path also works for that release.
        expected="7c3838a6cc9d35a75e75e5847ad2b861ea5e4dd5177f6349720641f50984ff56"
    else
        echo "error: required SHA256SUMS could not be downloaded" >&2
        exit 1
    fi
    [[ "$expected" =~ ^[[:xdigit:]]{64}$ ]] || {
        echo "error: SHA256SUMS does not contain a valid checksum for $asset" >&2
        exit 1
    }
    actual=$(shasum -a 256 "$dmg" | awk '{print $1}')
    actual_lower=$(printf '%s' "$actual" | tr '[:upper:]' '[:lower:]')
    expected_lower=$(printf '%s' "$expected" | tr '[:upper:]' '[:lower:]')
    [[ "$actual_lower" == "$expected_lower" ]] || {
        echo "error: checksum verification failed for $asset" >&2
        exit 1
    }
    echo "Checksum verified."

    mkdir -p "$mount_point"
    hdiutil attach "$dmg" -nobrowse -readonly -mountpoint "$mount_point" -quiet
    mounted=1
    source_app="$mount_point/Zephyr.app"
fi

[[ -d "$source_app" && -x "$source_app/Contents/MacOS/zephyr" ]] || {
    echo "error: release does not contain a valid Zephyr.app" >&2
    exit 1
}
if [[ $downloaded_release -eq 1 ]]; then
    codesign --verify --deep --strict "$source_app" >/dev/null 2>&1 || {
        echo "error: downloaded Zephyr.app has an invalid code signature" >&2
        exit 1
    }
fi

# Ask a running copy to quit before replacing its bundle.
if pgrep -f '/Zephyr\.app/Contents/MacOS/zephyr' >/dev/null 2>&1; then
    osascript -e 'tell application "Zephyr" to quit' >/dev/null 2>&1 || true
    for _ in {1..50}; do
        pgrep -f '/Zephyr\.app/Contents/MacOS/zephyr' >/dev/null 2>&1 || break
        sleep 0.1
    done
    if pgrep -f '/Zephyr\.app/Contents/MacOS/zephyr' >/dev/null 2>&1; then
        echo "error: quit Zephyr before upgrading" >&2
        exit 1
    fi
fi

target_executable="$target_app/Contents/MacOS/zephyr"
cli_path="$BIN_DIR/zephyr"
cli_preexisting=0
if [[ $INSTALL_CLI -eq 1 ]]; then
    run_cli mkdir -p "$BIN_DIR"
    if [[ -e "$cli_path" || -L "$cli_path" ]]; then
        if [[ -L "$cli_path" && "$(readlink "$cli_path")" == "$target_executable" ]]; then
            cli_preexisting=1
        else
            echo "error: refusing to replace existing CLI path: $cli_path" >&2
            exit 1
        fi
    fi
fi

run_app mkdir -p "$INSTALL_DIR"
stage="$INSTALL_DIR/.Zephyr.app.install.$$"
backup="$INSTALL_DIR/.Zephyr.app.backup.$$"
run_app rm -rf "$stage" "$backup"
run_app ditto "$source_app" "$stage"
# Releases are currently ad-hoc signed; clear download quarantine so the
# terminal installation is immediately launchable.
run_app xattr -dr com.apple.quarantine "$stage"

transaction_active=1
if [[ -e "$target_app" || -L "$target_app" ]]; then
    original_present=1
    backup_ready=1
    run_app mv "$target_app" "$backup"
fi
new_app_installed=1
run_app mv "$stage" "$target_app"

if [[ "${ZEPHYR_TEST_FAIL_AFTER_REPLACE:-0}" == "1" ]]; then
    echo "error: injected post-replacement failure" >&2
    exit 1
fi

if [[ $downloaded_release -eq 1 ]]; then
    codesign --verify --deep --strict "$target_app" >/dev/null 2>&1 || {
        echo "error: installed Zephyr.app has an invalid code signature" >&2
        exit 1
    }
fi
installed_version=$($target_executable --version 2>/dev/null) || {
    echo "error: installed Zephyr executable failed validation" >&2
    exit 1
}
[[ "$installed_version" == "Zephyr $REQUESTED_VERSION ("* ]] || {
    echo "error: installed version does not match $REQUESTED_VERSION: $installed_version" >&2
    exit 1
}

if [[ $INSTALL_CLI -eq 1 && $cli_preexisting -eq 0 ]]; then
    cli_stage="$BIN_DIR/.zephyr.install.$$"
    run_cli rm -f "$cli_stage"
    run_cli ln -s "$target_executable" "$cli_stage"
    cli_created=1
    run_cli mv -n "$cli_stage" "$cli_path"
    if [[ -e "$cli_stage" || -L "$cli_stage" ]]; then
        echo "error: CLI path appeared during installation: $cli_path" >&2
        exit 1
    fi
fi
if [[ $INSTALL_CLI -eq 1 ]]; then
    cli_version=$($cli_path --version 2>/dev/null) || {
        echo "error: installed terminal command failed validation" >&2
        exit 1
    }
    [[ "$cli_version" == "Zephyr $REQUESTED_VERSION ("* ]] || {
        echo "error: terminal command version does not match $REQUESTED_VERSION: $cli_version" >&2
        exit 1
    }
fi

transaction_active=0
if [[ $backup_ready -eq 1 ]]; then
    run_app rm -rf "$backup"
    backup_ready=0
fi

if [[ "${ZEPHYR_SKIP_REGISTER:-0}" != "1" ]]; then
    lsregister="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
    [[ -x "$lsregister" ]] && "$lsregister" -f "$target_app" >/dev/null 2>&1 || true
fi

echo "Installed $installed_version at $target_app"
if [[ $INSTALL_CLI -eq 1 ]]; then
    echo "Terminal command: $BIN_DIR/zephyr"
fi

if [[ $OPEN_AFTER -eq 1 ]]; then
    open "$target_app"
else
    printf 'Open with: open %q\n' "$target_app"
fi
