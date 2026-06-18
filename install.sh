#!/bin/sh
# Arborist installer. Downloads a prebuilt `arb` binary from GitHub Releases and
# installs it onto your PATH. No Go toolchain required.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/jjacoblee/arborist/main/install.sh | sh
#
# Options (environment variables):
#   ARBORIST_VERSION       version to install, e.g. v0.1.0 (default: latest release)
#   ARBORIST_INSTALL_DIR   where to install arb (default: /usr/local/bin, falling
#                          back to $HOME/.local/bin if that is not writable)
set -eu

REPO="jjacoblee/arborist"
BINARY="arb"

info() { printf '%s\n' "$*"; }
err() { printf 'error: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || err "this installer needs '$1' but it was not found"; }

need curl
need tar
need uname

# --- Detect OS and architecture (matching GoReleaser's archive names) ---
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  darwin | linux) ;;
  *) err "unsupported OS '$os' (prebuilt binaries are provided for macOS and Linux; build from source instead)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) err "unsupported architecture '$arch'" ;;
esac

# --- Resolve the version to install ---
version="${ARBORIST_VERSION:-}"
if [ -z "$version" ]; then
  info "Looking up the latest release..."
  version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
    grep '"tag_name"' | head -n1 | cut -d '"' -f4)
  [ -n "$version" ] || err "could not determine the latest release; set ARBORIST_VERSION (e.g. v0.1.0)"
fi
ver_no_v="${version#v}"

archive="${BINARY}orist_${ver_no_v}_${os}_${arch}.tar.gz" # arborist_<ver>_<os>_<arch>.tar.gz
base_url="https://github.com/${REPO}/releases/download/${version}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

info "Downloading ${archive} (${version})..."
curl -fsSL -o "${tmp}/${archive}" "${base_url}/${archive}" ||
  err "download failed: ${base_url}/${archive}"

# --- Verify checksum when possible (best effort, but preferred) ---
if curl -fsSL -o "${tmp}/checksums.txt" "${base_url}/checksums.txt" 2>/dev/null; then
  sum=""
  if command -v sha256sum >/dev/null 2>&1; then
    sum=$(sha256sum "${tmp}/${archive}" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    sum=$(shasum -a 256 "${tmp}/${archive}" | awk '{print $1}')
  fi
  if [ -n "$sum" ]; then
    # Match the checksum for *this* archive specifically (field 2 == filename),
    # not just the hash appearing anywhere in the file.
    expected=$(awk -v f="$archive" '$2 == f { print $1 }' "${tmp}/checksums.txt")
    [ -n "$expected" ] || err "no checksum listed for ${archive}"
    [ "$expected" = "$sum" ] || err "checksum verification failed for ${archive}"
    info "Checksum verified."
  fi
fi

tar -xzf "${tmp}/${archive}" -C "$tmp"
[ -f "${tmp}/${BINARY}" ] || err "archive did not contain '${BINARY}'"
chmod +x "${tmp}/${BINARY}"

# --- Choose an install directory on PATH and install ---
install_dir="${ARBORIST_INSTALL_DIR:-/usr/local/bin}"

place() {
  dir="$1"
  if [ -w "$dir" ] || { [ ! -e "$dir" ] && mkdir -p "$dir" 2>/dev/null; }; then
    mv "${tmp}/${BINARY}" "${dir}/${BINARY}"
    return 0
  fi
  return 1
}

if place "$install_dir"; then
  :
elif command -v sudo >/dev/null 2>&1 && [ "$install_dir" = "/usr/local/bin" ]; then
  info "Installing to ${install_dir} (requires sudo)..."
  sudo mv "${tmp}/${BINARY}" "${install_dir}/${BINARY}"
else
  install_dir="${HOME}/.local/bin"
  mkdir -p "$install_dir"
  mv "${tmp}/${BINARY}" "${install_dir}/${BINARY}"
fi

info "Installed ${BINARY} to ${install_dir}/${BINARY}"

# --- PATH hint ---
case ":${PATH}:" in
  *":${install_dir}:"*)
    info "Run: ${BINARY} --version"
    ;;
  *)
    info "WARNING: ${install_dir} is not on your PATH."
    info "Add this to your shell profile (e.g. ~/.zshrc), then restart your shell:"
    info "    export PATH=\"\$PATH:${install_dir}\""
    ;;
esac

info ""
info "Next steps:"
info "  gh auth login                          # one-time GitHub CLI auth (needs gh + git)"
info "  mkdir -p ~/work/acme && cd ~/work/acme"
info "  ${BINARY} init --owner acme            # set up a workspace"
info "  ${BINARY} new my-feature"
