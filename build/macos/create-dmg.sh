#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-dev}"
DMG_NAME="Zephyr-${VERSION}-macos.dmg"

# Install create-dmg if not available
if ! command -v create-dmg &>/dev/null; then
    brew install create-dmg
fi

rm -f "$DMG_NAME"

create-dmg \
    --volname "Zephyr" \
    --volicon "assets/icon.icns" \
    --window-pos 200 120 \
    --window-size 660 400 \
    --icon-size 80 \
    --icon "Zephyr.app" 180 190 \
    --hide-extension "Zephyr.app" \
    --app-drop-link 480 190 \
    --no-internet-enable \
    "$DMG_NAME" \
    "Zephyr.app"

echo "Created $DMG_NAME"
