#!/bin/sh
# everything-cli installer: curl -fsSL https://oskarhane.github.io/google-cli/install.sh | sh
# (pages URL still uses the google-cli repo slug until the GitHub repo is renamed)
set -eu

REPO=oskarhane/google-cli
ASSET_PREFIX=everything-cli
BIN_NAME=everything-cli
INSTALL_DIR="$HOME/.local/bin"

die() {
	echo "install.sh: $1" >&2
	exit 1
}

case "$(uname -s)" in
Darwin) os=darwin ;;
Linux) os=linux ;;
*)
	die "unsupported OS: $(uname -s). Supported: Darwin, Linux."
	;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*)
	die "unsupported architecture: $(uname -m). Supported: amd64, arm64."
	;;
esac

command -v curl >/dev/null 2>&1 ||
	die "curl is required. Install it and retry."

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# curl|sh: no script on disk, so all state lives in $tmp. Never reference $0.
fetch() {
	if [ -n "${GITHUB_TOKEN:-}" ]; then
		curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" "$1"
	else
		curl -fsSL "$1"
	fi
}

asset="${ASSET_PREFIX}_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download"

# Fragile-but-accepted sed parse of the JSON API (dependency-free shell).
tag=$(fetch "https://api.github.com/repos/$REPO/releases/latest" |
	sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
	head -n 1)
[ -n "$tag" ] || die "could not resolve latest release for $REPO"

echo "Installing everything-cli $tag ($os/$arch) to $INSTALL_DIR"

download_url="$base_url/$tag"
fetch "$download_url/$asset" >"$tmp/$asset" ||
	die "failed to download $asset from $download_url"
fetch "$download_url/checksums.txt" >"$tmp/checksums.txt" ||
	die "failed to download checksums.txt from $download_url"

# Expected hash line forms: "<hex>  <name>" and "<hex> *<name>" (binary mode).
expected=$(grep -E "^[0-9a-fA-F]{64}[[:space:]]+\*?${asset}\$" "$tmp/checksums.txt" |
	head -n 1 | awk '{print $1}')
[ -n "$expected" ] ||
	die "checksums.txt from $tag has no entry for $asset"

printf '%s  %s\n' "$expected" "$asset" | (
	cd "$tmp" &&
		if command -v sha256sum >/dev/null 2>&1; then
			sha256sum -c -
		else
			shasum -a 256 -c -
		fi
) || die "checksum verification failed for $asset; installing nothing"

# Extract ONLY the named binary member: any other member (e.g. a crafted
# "../path" traversal name in a compromised release) is never written to disk,
# and a tarball missing "$BIN_NAME" makes tar fail, aborting the install.
tar -xzf "$tmp/$asset" -C "$tmp" "$BIN_NAME"

mkdir -p "$INSTALL_DIR"
mv "$tmp/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
chmod +x "$INSTALL_DIR/$BIN_NAME"

case ":$PATH:" in
*":$INSTALL_DIR:"*) ;;
*)
	echo "NOTE: $INSTALL_DIR is not in your PATH." >&2
	echo "Add it to your shell profile, e.g.:" >&2
	echo "  export PATH=\"$INSTALL_DIR:\$PATH\"" >&2
	;;
esac

echo "Installed everything-cli $tag at $INSTALL_DIR/$BIN_NAME"
echo "run: everything-cli skill install"
