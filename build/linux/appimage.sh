#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-dev}"
APPIMAGE_NAME="Zephyr-${VERSION}-x86_64.AppImage"
APPDIR="Zephyr.AppDir"

rm -rf "$APPDIR" "$APPIMAGE_NAME"

# Download appimagetool if not present
if [ ! -f appimagetool ]; then
    wget -q "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage" \
        -O appimagetool
    chmod +x appimagetool
fi

# Build AppDir structure
mkdir -p "$APPDIR/usr/bin"
mkdir -p "$APPDIR/usr/share/applications"
mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"
mkdir -p "$APPDIR/usr/share/icons/hicolor/scalable/apps"

cp zephyr "$APPDIR/usr/bin/"
cp build/linux/zephyr.desktop "$APPDIR/"
cp build/linux/zephyr.desktop "$APPDIR/usr/share/applications/"
cp assets/icon.iconset/icon_256x256.png "$APPDIR/zephyr.png"
cp assets/icon.iconset/icon_256x256.png "$APPDIR/usr/share/icons/hicolor/256x256/apps/zephyr.png"
cp assets/icon.svg "$APPDIR/usr/share/icons/hicolor/scalable/apps/zephyr.svg"

# Create AppRun launcher
cat > "$APPDIR/AppRun" << 'APPRUN'
#!/bin/bash
HERE="$(dirname "$(readlink -f "${0}")")"
exec "$HERE/usr/bin/zephyr" "$@"
APPRUN
chmod +x "$APPDIR/AppRun"

# Build the AppImage (--appimage-extract-and-run avoids FUSE requirement in CI)
ARCH=x86_64 ./appimagetool --appimage-extract-and-run "$APPDIR" "$APPIMAGE_NAME"

echo "Created $APPIMAGE_NAME"
