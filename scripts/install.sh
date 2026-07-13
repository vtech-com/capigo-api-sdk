#!/bin/sh
# Install the capigo CLI on Linux or macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/vtech-com/capigo-api-sdk/main/scripts/install.sh | sh
#
# Re-run it to upgrade: the binary is replaced in place.
#
# Environment:
#   CAPIGO_VERSION   version to install, e.g. v0.16.0 (default: latest release)
#   CAPIGO_BIN_DIR   install directory (default: /usr/local/bin)
#
# The downloaded archive is verified against the release's checksums.txt before
# anything is installed. If verification fails, nothing is written.

set -eu

REPO="vtech-com/capigo-api-sdk"
BIN_DIR="${CAPIGO_BIN_DIR:-/usr/local/bin}"
TMP_DIR=""

die() {
	printf 'capigo install: %s\n' "$1" >&2
	exit 1
}

cleanup() {
	[ -n "$TMP_DIR" ] && rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

need() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

need curl
need tar

# --- platform -----------------------------------------------------------------

os=$(uname -s)
case "$os" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) die "unsupported operating system: $os (this script covers Linux and macOS)" ;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) die "unsupported architecture: $arch (released builds are amd64 and arm64)" ;;
esac

# --- version ------------------------------------------------------------------

version="${CAPIGO_VERSION:-}"
if [ -z "$version" ]; then
	version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
		sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
		head -1)
	[ -n "$version" ] || die "could not resolve the latest release tag; set CAPIGO_VERSION to pin one"
fi

# Release artifacts carry the version without the leading "v".
bare_version="${version#v}"
archive="capigo_${bare_version}_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$version"

# --- download -----------------------------------------------------------------

TMP_DIR=$(mktemp -d)

printf 'Downloading capigo %s (%s/%s)...\n' "$version" "$os" "$arch"
curl -fsSL -o "$TMP_DIR/$archive" "$base_url/$archive" ||
	die "download failed: $base_url/$archive (does release $version ship $os/$arch?)"
curl -fsSL -o "$TMP_DIR/checksums.txt" "$base_url/checksums.txt" ||
	die "could not download checksums.txt for $version; refusing to install an unverified binary"

# --- verify -------------------------------------------------------------------

expected=$(sed -n "s/^\([0-9a-f]\{64\}\)[[:space:]]*\*\{0,1\}${archive}$/\1/p" "$TMP_DIR/checksums.txt" | head -1)
[ -n "$expected" ] || die "$archive is not listed in checksums.txt; refusing to install"

if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$TMP_DIR/$archive" | cut -d' ' -f1)
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$TMP_DIR/$archive" | cut -d' ' -f1)
else
	die "need sha256sum or shasum to verify the download"
fi

[ "$actual" = "$expected" ] ||
	die "checksum mismatch for $archive (expected $expected, got $actual); nothing was installed"

# --- install ------------------------------------------------------------------

tar -xzf "$TMP_DIR/$archive" -C "$TMP_DIR" capigo || die "could not extract capigo from $archive"
chmod +x "$TMP_DIR/capigo"

# Write with sudo only when the target directory is not already writable.
if [ -w "$BIN_DIR" ] || { [ ! -e "$BIN_DIR" ] && mkdir -p "$BIN_DIR" 2>/dev/null; }; then
	install_cmd=""
elif command -v sudo >/dev/null 2>&1; then
	printf 'Installing to %s requires sudo.\n' "$BIN_DIR"
	install_cmd="sudo"
else
	die "$BIN_DIR is not writable and sudo is unavailable; set CAPIGO_BIN_DIR to a writable directory"
fi

$install_cmd mkdir -p "$BIN_DIR"
$install_cmd cp "$TMP_DIR/capigo" "$BIN_DIR/capigo"

printf 'Installed capigo %s to %s/capigo\n' "$version" "$BIN_DIR"

case ":$PATH:" in
*":$BIN_DIR:"*) "$BIN_DIR/capigo" version ;;
*) printf '%s is not on your PATH — add it, or run %s/capigo directly.\n' "$BIN_DIR" "$BIN_DIR" ;;
esac
