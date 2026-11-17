#!/bin/sh
# install.sh: install c9s from a clone of this repository.
#
#   git clone https://github.com/JByns/c9s.git && cd c9s && sudo ./install.sh
#
# It installs the c9s version of this checkout:
#   - with Go (the version in go.mod or newer) it builds from this source;
#   - otherwise it downloads the prebuilt binary for this version from the
#     GitHub release, using the GitHub CLI (gh) when it is logged in, or curl.
#
# Options (environment):
#   PREFIX=/usr/local      install to $PREFIX/bin (default /usr/local)
#   C9S_METHOD=build|download   force one method
set -eu

cd "$(dirname "$0")"
PREFIX=${PREFIX:-/usr/local}
BIN_DIR="$PREFIX/bin"
METHOD=${C9S_METHOD:-auto}

say()  { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m==>\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# --- where and what ------------------------------------------------------

os=$(uname -s)
if [ "$os" != "Linux" ] && [ -z "${C9S_INSTALL_ANY_OS:-}" ]; then
	die "c9s runs on the ClusterControl controller (Linux). This is $os."
fi
case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) die "unsupported CPU architecture: $(uname -m) (amd64 and arm64 are supported)" ;;
esac

# Repository on GitHub, from the clone's origin (works after a transfer too).
repo=$(git config --get remote.origin.url 2>/dev/null |
	sed -E 's#^(https://|git@)github\.com[:/]##; s#\.git$##') || true
[ -n "$repo" ] || repo=JByns/c9s

# Version of this checkout: the tag it is on, else the newest tag before it.
tag=$(git describe --tags --exact-match 2>/dev/null || true)
[ -n "$tag" ] || tag=$(git describe --tags --abbrev=0 2>/dev/null || true)
version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)

# --- method 1: build from source -----------------------------------------

need_go=$(sed -n 's/^go //p' go.mod)
have_go() {
	command -v go >/dev/null 2>&1 || return 1
	have=$(go env GOVERSION 2>/dev/null | sed 's/^go//')
	# true when $have >= $need_go
	[ "$(printf '%s\n%s\n' "$need_go" "$have" | sort -V | head -1)" = "$need_go" ]
}

build() {
	say "building c9s $version from source with $(go version | cut -d' ' -f3)"
	tmp=$1
	CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$version" -o "$tmp/c9s" .
}

# --- method 2: download the release binary --------------------------------

download() {
	tmp=$1
	[ -n "$tag" ] || die "this checkout has no release tag; install Go to build it, or check out a tag (git checkout v0.1.1)"
	asset="c9s_linux_${arch}.tar.gz"
	say "downloading $asset for $tag from github.com/$repo"
	if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
		gh release download "$tag" -R "$repo" -p "$asset" -p checksums.txt -D "$tmp" ||
			die "download with gh failed"
	elif command -v curl >/dev/null 2>&1; then
		base="https://github.com/$repo/releases/download/$tag"
		if ! curl -fsSL -o "$tmp/$asset" "$base/$asset"; then
			die "could not download $base/$asset
  Check that this machine can reach github.com (set https_proxy if needed),
  or install Go $need_go+ and run this script again to build from source"
		fi
		curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || true
	else
		die "need gh or curl to download, or Go $need_go+ to build"
	fi
	if [ -s "$tmp/checksums.txt" ] && command -v sha256sum >/dev/null 2>&1; then
		(cd "$tmp" && grep " $asset\$" checksums.txt | sha256sum -c --quiet -) ||
			die "checksum mismatch for $asset"
		say "checksum OK"
	fi
	tar -xzf "$tmp/$asset" -C "$tmp" c9s
}

# --- run ------------------------------------------------------------------

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

case "$METHOD" in
	build) have_go || die "Go $need_go or newer is needed to build"; build "$tmp" ;;
	download) download "$tmp" ;;
	auto) if have_go; then build "$tmp"; else download "$tmp"; fi ;;
	*) die "C9S_METHOD must be build, download or auto" ;;
esac

if ! mkdir -p "$BIN_DIR" 2>/dev/null || [ ! -w "$BIN_DIR" ]; then
	die "cannot write to $BIN_DIR: run with sudo (sudo ./install.sh) or set PREFIX=\$HOME/.local"
fi
install -m 0755 "$tmp/c9s" "$BIN_DIR/c9s"
say "installed $BIN_DIR/c9s ($("$BIN_DIR/c9s" version 2>/dev/null || echo "$version"))"

case ":$PATH:" in
	*":$BIN_DIR:"*) ;;
	*) warn "$BIN_DIR is not in your PATH; add it, or run $BIN_DIR/c9s" ;;
esac
say "ready: run  c9s  (add --write to make changes)"
