#!/bin/sh
# install.sh — one-line installer for lofi-player on macOS and Linux.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/iRootPro/lofi-player/main/scripts/install.sh | sh
#
# Environment variables:
#   VERSION       Pin to a specific tag (default: latest release).
#   INSTALL_DIR   Where to put the binary (default: $HOME/.local/bin).
#
# Examples:
#   VERSION=v0.1.0 sh -c "$(curl -fsSL https://...install.sh)"
#   INSTALL_DIR=/usr/local/bin sh -c "$(curl -fsSL https://...install.sh)"

set -eu

REPO="iRootPro/lofi-player"

err() {
	printf '%s\n' "lofi-player install: $*" >&2
	exit 1
}

# OS detection.
case "$(uname -s)" in
	Darwin) os=darwin ;;
	Linux)  os=linux ;;
	*) err "unsupported OS: $(uname -s) (only macOS and Linux are supported)" ;;
esac

# Architecture detection. uname -m varies by OS, normalize to goreleaser names.
case "$(uname -m)" in
	x86_64|amd64) arch=amd64 ;;
	arm64|aarch64) arch=arm64 ;;
	*) err "unsupported arch: $(uname -m) (only amd64 and arm64 are supported)" ;;
esac

# Version resolution. Without an explicit VERSION, query the GitHub API for
# the latest release tag. Without curl we have no way forward — we won't
# wget/fetch since adding fallbacks tends to silently mask the real cause.
command -v curl >/dev/null 2>&1 || err "curl is required and not on \$PATH"
command -v tar  >/dev/null 2>&1 || err "tar is required and not on \$PATH"

VERSION="${VERSION:-}"
if [ -z "$VERSION" ]; then
	VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
		| sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
		| head -n1)
	if [ -z "$VERSION" ]; then
		err "could not resolve the latest release; set VERSION=vX.Y.Z to override"
	fi
fi

# Goreleaser strips the leading "v" from archive names.
VERSION_NUMERIC="${VERSION#v}"

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

archive="lofi-player_${VERSION_NUMERIC}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$VERSION/$archive"

# Stage download in a tempdir so a failed install doesn't leave half-extracted
# files in the user's $PATH.
tmp=$(mktemp -d 2>/dev/null || mktemp -d -t lofi-player)
trap 'rm -rf "$tmp"' EXIT

printf 'lofi-player install: downloading %s\n' "$url"
if ! curl -fSL --progress-bar "$url" -o "$tmp/$archive"; then
	err "download failed (HTTP error from $url)"
fi

tar -C "$tmp" -xzf "$tmp/$archive"
if [ ! -f "$tmp/lofi-player" ]; then
	err "archive did not contain a lofi-player binary"
fi

mkdir -p "$INSTALL_DIR"
mv "$tmp/lofi-player" "$INSTALL_DIR/lofi-player"
chmod +x "$INSTALL_DIR/lofi-player"

printf 'lofi-player install: installed %s -> %s/lofi-player\n' "$VERSION" "$INSTALL_DIR"

case ":$PATH:" in
	*":$INSTALL_DIR:"*) ;;
	*) printf 'lofi-player install: NOTE: %s is not on $PATH\n' "$INSTALL_DIR" ;;
esac

# Surface dependency hints — install is fine on its own, but the app refuses
# to start without mpv. Cheap probe, no install attempted.
if ! command -v mpv >/dev/null 2>&1; then
	printf 'lofi-player install: mpv is not installed; install with `brew install mpv` (macOS) or `apt install mpv` / `pacman -S mpv` (Linux)\n'
fi
