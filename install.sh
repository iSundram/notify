#!/usr/bin/env sh
# install.sh — Download and install notify binaries from GitHub releases.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/install.sh | sh -s -- --version 0.1.0
#
# Options:
#   --version VERSION   Install a specific version (default: latest)
#   --prefix  DIR       Installation prefix (default: /usr/local/bin)
#   --help              Show this help message

set -e

REPO="iSundram/notify"
PREFIX="/usr/local/bin"
VERSION=""

usage() {
  echo "Usage: install.sh [--version VERSION] [--prefix DIR]"
  echo ""
  echo "Options:"
  echo "  --version VERSION   Install a specific version (default: latest)"
  echo "  --prefix  DIR       Installation prefix (default: /usr/local/bin)"
  echo "  --help              Show this help message"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --prefix)  PREFIX="$2"; shift 2 ;;
    --help)    usage; exit 0 ;;
    *)         echo "Unknown option: $1"; usage; exit 1 ;;
  esac
done

# Detect OS and architecture.
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  linux) ;;
  *)     echo "Unsupported OS: $OS (only linux is supported)"; exit 1 ;;
esac

# Resolve version.
if [ -z "$VERSION" ]; then
  echo "Fetching latest release..."
  if command -v jq >/dev/null 2>&1; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -r '.tag_name' | sed 's/^v//')
  else
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
  fi
  if [ -z "$VERSION" ]; then
    echo "Error: could not determine latest version"
    exit 1
  fi
fi

echo "Installing notify v${VERSION} (${OS}/${ARCH})..."

# Build download URL.
TARBALL="notify_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${TARBALL}"

# Download and extract.
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${URL}..."
curl -fsSL "$URL" -o "${TMPDIR}/${TARBALL}"

echo "Extracting..."
tar -xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"

# Install binaries.
for bin in notify notifyctl notifyd; do
  if [ -f "${TMPDIR}/${bin}" ]; then
    install -m 755 "${TMPDIR}/${bin}" "${PREFIX}/${bin}"
    echo "  Installed ${PREFIX}/${bin}"
  fi
done

# Install optional files if running as root.
if [ "$(id -u)" = "0" ]; then
  if [ -f "${TMPDIR}/systemd/notifyd.service" ]; then
    mkdir -p /etc/systemd/system
    cp "${TMPDIR}/systemd/notifyd.service" /etc/systemd/system/notifyd.service
    echo "  Installed /etc/systemd/system/notifyd.service"
  fi
  if [ -f "${TMPDIR}/scripts/notify.sh" ]; then
    mkdir -p /etc/profile.d
    cp "${TMPDIR}/scripts/notify.sh" /etc/profile.d/notify.sh
    echo "  Installed /etc/profile.d/notify.sh"
  fi
fi

echo ""
echo "notify v${VERSION} installed successfully!"
echo ""
echo "To start the daemon:"
echo "  sudo systemctl enable --now notifyd"
echo ""
echo "To send a notification:"
echo "  notify --title 'Hello' --message 'World'"
