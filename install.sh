#!/usr/bin/env bash
# gitui installer.
#   curl -fsSL https://raw.githubusercontent.com/boutdecode/gitui/main/install.sh | bash
#
# Environment:
#   GITUI_VERSION      version to install (e.g. v0.1.0); default: latest release
#   GITUI_INSTALL_DIR  target directory; default: /usr/local/bin if writable
#                      (or sudo is available), else ~/.local/bin
set -euo pipefail

REPO="boutdecode/gitui"

say() { printf 'gitui-install: %s\n' "$*"; }
die() { printf 'gitui-install: error: %s\n' "$*" >&2; exit 1; }

have() { command -v "$1" >/dev/null 2>&1; }

fetch() { # fetch <url> <output file | ->
  if have curl; then
    curl -fsSL "$1" -o "$2"
  elif have wget; then
    wget -qO "$2" "$1"
  else
    die "curl or wget is required"
  fi
}

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported OS: $(uname -s) (Linux and macOS only)" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac

have tar || die "tar is required"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

version="${GITUI_VERSION:-}"
if [ -z "$version" ]; then
  fetch "https://api.github.com/repos/$REPO/releases/latest" "$tmp/latest.json"
  version="$(sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' "$tmp/latest.json" | head -n1)"
  [ -n "$version" ] || die "could not determine the latest release (set GITUI_VERSION)"
fi
case "$version" in v*) ;; *) version="v$version" ;; esac
plain="${version#v}"

archive="gitui_${plain}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$version"

say "installing gitui $version ($os/$arch)"
fetch "$base/$archive" "$tmp/$archive"
fetch "$base/checksums.txt" "$tmp/checksums.txt"

expected="$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")"
[ -n "$expected" ] || die "no checksum found for $archive"
if have sha256sum; then
  actual="$(sha256sum "$tmp/$archive" | awk '{ print $1 }')"
elif have shasum; then
  actual="$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')"
else
  die "sha256sum or shasum is required to verify the download"
fi
[ "$expected" = "$actual" ] || die "checksum mismatch for $archive"

tar -xzf "$tmp/$archive" -C "$tmp" gitui

sudo=""
dir="${GITUI_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ -w /usr/local/bin ]; then
    dir=/usr/local/bin
  elif have sudo; then
    dir=/usr/local/bin
    sudo=sudo
  else
    dir="$HOME/.local/bin"
  fi
fi

if [ -z "$sudo" ]; then
  mkdir -p "$dir" 2>/dev/null || true
  if [ ! -w "$dir" ]; then
    have sudo || die "$dir is not writable (set GITUI_INSTALL_DIR)"
    sudo=sudo
  fi
fi

$sudo mkdir -p "$dir"
$sudo install -m 0755 "$tmp/gitui" "$dir/gitui"
say "installed $dir/gitui"

case ":$PATH:" in
  *":$dir:"*) ;;
  *) say "warning: $dir is not in your PATH" ;;
esac
