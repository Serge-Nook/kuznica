#!/usr/bin/env bash
# Builds KUZNICA-<version>-<arch>.AppImage in the dist/ directory.
#
# Requirements: go, curl, file, and the Fyne build dependencies
# (libgl1-mesa-dev, xorg-dev, libxkbcommon-dev on Debian/Ubuntu;
#  base-devel, libxkbcommon, mesa on Arch Linux).
set -euo pipefail

VERSION="${VERSION:-1.1.0}"
ARCH="$(uname -m)"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="${ROOT}/dist"
TOOLS="${WORK}/tools"

mkdir -p "${TOOLS}"
cd "${WORK}"

fetch_tool() {
  local name="$1" url="$2"
  if [ ! -d "${TOOLS}/${name}" ]; then
    curl -sSL -o "${TOOLS}/${name}.AppImage" "${url}"
    chmod +x "${TOOLS}/${name}.AppImage"
    # AppImages are extracted because FUSE is unavailable in most containers
    (cd "${TOOLS}" && "./${name}.AppImage" --appimage-extract >/dev/null && mv squashfs-root "${name}" && rm "${name}.AppImage")
  fi
}

fetch_tool linuxdeploy \
  "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-${ARCH}.AppImage"
fetch_tool linuxdeploy-plugin-appimage \
  "https://github.com/linuxdeploy/linuxdeploy-plugin-appimage/releases/download/continuous/linuxdeploy-plugin-appimage-${ARCH}.AppImage"

# linuxdeploy looks the plugin up on PATH by name
mkdir -p "${TOOLS}/bin"
printf '#!/bin/sh\nexec "%s/linuxdeploy-plugin-appimage/AppRun" "$@"\n' "${TOOLS}" > "${TOOLS}/bin/linuxdeploy-plugin-appimage"
chmod +x "${TOOLS}/bin/linuxdeploy-plugin-appimage"
export PATH="${TOOLS}/bin:${PATH}"

cd "${ROOT}"
go build -trimpath -ldflags '-s -w' -o "${WORK}/kuznica" ./cmd/kuznica

rm -rf "${WORK}/AppDir"
cd "${WORK}"
LDAI_OUTPUT="KUZNICA-${VERSION}-${ARCH}.AppImage" LINUXDEPLOY_OUTPUT_VERSION="${VERSION}" \
  "${TOOLS}/linuxdeploy/AppRun" \
  --appdir AppDir \
  -e "${WORK}/kuznica" \
  -d "${ROOT}/packaging/kuznica.desktop" \
  -i "${ROOT}/assets/icons/kuznica.svg" \
  --output appimage

echo "Built ${WORK}/KUZNICA-${VERSION}-${ARCH}.AppImage"
