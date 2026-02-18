#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Generate WinGet manifest files for yt-vod-manager.

Required:
  --version <version-no-v>             e.g. 0.1.0
  --release-tag <release-tag>          e.g. v0.1.0
  --installer-url <url>                Windows zip release asset URL
  --installer-sha <sha256>             SHA256 for the Windows zip asset

Optional:
  --release-date <YYYY-MM-DD>          Defaults to UTC today
  --out-root <path>                    Defaults to packaging/winget/output
EOF
}

VERSION=""
RELEASE_TAG=""
INSTALLER_URL=""
INSTALLER_SHA=""
RELEASE_DATE="$(date -u +%F)"
OUT_ROOT="packaging/winget/output"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --release-tag)
      RELEASE_TAG="${2:-}"
      shift 2
      ;;
    --installer-url)
      INSTALLER_URL="${2:-}"
      shift 2
      ;;
    --installer-sha)
      INSTALLER_SHA="${2:-}"
      shift 2
      ;;
    --release-date)
      RELEASE_DATE="${2:-}"
      shift 2
      ;;
    --out-root)
      OUT_ROOT="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "${VERSION}" || -z "${RELEASE_TAG}" || -z "${INSTALLER_URL}" || -z "${INSTALLER_SHA}" ]]; then
  echo "missing required arguments" >&2
  usage >&2
  exit 1
fi

if [[ ! "${RELEASE_DATE}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
  echo "release date must use YYYY-MM-DD: ${RELEASE_DATE}" >&2
  exit 1
fi

if [[ ! "${INSTALLER_SHA}" =~ ^[A-Fa-f0-9]{64}$ ]]; then
  echo "installer sha must be a 64-char SHA256 hex string" >&2
  exit 1
fi

PACKAGE_ID="MarcoHefti.YTVodManager"
PACKAGE_NAME="yt-vod-manager"
PUBLISHER="Marco Hefti"
REPOSITORY="marcohefti/yt-vod-manager"
MANIFEST_VERSION="1.10.0"
INSTALLER_FILE="$(basename "${INSTALLER_URL}")"

if [[ "${INSTALLER_FILE}" != *.zip ]]; then
  echo "installer URL must point to a .zip file: ${INSTALLER_URL}" >&2
  exit 1
fi

NESTED_DIR="${INSTALLER_FILE%.zip}"
NESTED_FILE="${NESTED_DIR}\\yt-vod-manager.exe"
LOWER_PARTITION="$(printf '%s' "${PACKAGE_ID:0:1}" | tr '[:upper:]' '[:lower:]')"
PACKAGE_PATH="${OUT_ROOT}/manifests/${LOWER_PARTITION}/${PACKAGE_ID//./\/}/${VERSION}"
REPO_URL="https://github.com/${REPOSITORY}"
RELEASE_NOTES_URL="${REPO_URL}/releases/tag/${RELEASE_TAG}"
LICENSE_URL="${REPO_URL}/blob/${RELEASE_TAG}/LICENSE"
INSTALLER_SHA_UPPER="$(printf '%s' "${INSTALLER_SHA}" | tr '[:lower:]' '[:upper:]')"

mkdir -p "${PACKAGE_PATH}"

cat > "${PACKAGE_PATH}/${PACKAGE_ID}.yaml" <<EOF
# Created by packaging/winget/generate-manifests.sh
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.version.${MANIFEST_VERSION}.schema.json

PackageIdentifier: ${PACKAGE_ID}
PackageVersion: ${VERSION}
DefaultLocale: en-US
ManifestType: version
ManifestVersion: ${MANIFEST_VERSION}
EOF

cat > "${PACKAGE_PATH}/${PACKAGE_ID}.installer.yaml" <<EOF
# Created by packaging/winget/generate-manifests.sh
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.installer.${MANIFEST_VERSION}.schema.json

PackageIdentifier: ${PACKAGE_ID}
PackageVersion: ${VERSION}
InstallerType: zip
NestedInstallerType: portable
NestedInstallerFiles:
- RelativeFilePath: '${NESTED_FILE}'
  PortableCommandAlias: yt-vod-manager
Installers:
- Architecture: x64
  InstallerUrl: ${INSTALLER_URL}
  InstallerSha256: ${INSTALLER_SHA_UPPER}
ReleaseDate: ${RELEASE_DATE}
ManifestType: installer
ManifestVersion: ${MANIFEST_VERSION}
EOF

cat > "${PACKAGE_PATH}/${PACKAGE_ID}.locale.en-US.yaml" <<EOF
# Created by packaging/winget/generate-manifests.sh
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.defaultLocale.${MANIFEST_VERSION}.schema.json

PackageIdentifier: ${PACKAGE_ID}
PackageVersion: ${VERSION}
PackageLocale: en-US
Publisher: ${PUBLISHER}
PublisherUrl: https://github.com/marcohefti
PublisherSupportUrl: ${REPO_URL}/issues
PackageName: ${PACKAGE_NAME}
PackageUrl: ${REPO_URL}
License: MIT
LicenseUrl: ${LICENSE_URL}
ShortDescription: CLI app to download YouTube channels and playlists and keep them up to date locally.
Moniker: yt-vod-manager
Tags:
- youtube
- yt-dlp
- vod
- cli
ReleaseNotesUrl: ${RELEASE_NOTES_URL}
ManifestType: defaultLocale
ManifestVersion: ${MANIFEST_VERSION}
EOF

echo "generated WinGet manifests in ${PACKAGE_PATH}"
