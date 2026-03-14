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
HAS_NOTIFYD=0

normalize_version() {
  v="$1"
  v=$(printf "%s" "$v" | tr -d '[:space:]')
  v=${v#v}
  printf "%s" "$v"
}

is_valid_version() {
  printf "%s" "$1" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?([+][0-9A-Za-z.-]+)?$'
}

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
    TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -r '.tag_name // empty')
  else
    RESPONSE=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")
    TAG=$(printf "%s" "$RESPONSE" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
  fi
  VERSION=$(normalize_version "$TAG")
  if [ -z "$VERSION" ] || ! is_valid_version "$VERSION"; then
    echo "Error: could not determine latest version"
    exit 1
  fi
fi

VERSION=$(normalize_version "$VERSION")
if [ -z "$VERSION" ] || ! is_valid_version "$VERSION"; then
  echo "Error: invalid version '$VERSION' (expected semver like 1.2.3)"
  exit 1
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
    if [ "$bin" = "notifyd" ]; then
      HAS_NOTIFYD=1
    fi
  fi
done

if [ ! -f "${TMPDIR}/notify" ] || [ ! -f "${TMPDIR}/notifyctl" ]; then
  echo "Error: release archive is missing required binaries (notify, notifyctl)"
  exit 1
fi

if [ "$HAS_NOTIFYD" != "1" ]; then
  echo "Error: release archive for ${OS}/${ARCH} is missing notifyd"
  exit 1
fi

# Install optional files if running as root.
if [ "$(id -u)" = "0" ]; then
  if [ "$HAS_NOTIFYD" = "1" ] && [ -f "${TMPDIR}/systemd/notifyd.service" ]; then
    if ! getent group notify >/dev/null 2>&1; then
      if command -v groupadd >/dev/null 2>&1; then
        groupadd --system notify >/dev/null 2>&1 || {
          echo "Error: failed to create group 'notify'"
          exit 1
        }
      else
        echo "Error: group 'notify' is missing and groupadd is unavailable"
        exit 1
      fi
    fi
    if ! id -u notify >/dev/null 2>&1; then
      if command -v useradd >/dev/null 2>&1; then
        useradd --system --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin -g notify notify >/dev/null 2>&1 || {
          echo "Error: failed to create user 'notify'"
          exit 1
        }
      else
        echo "Error: user 'notify' is missing and useradd is unavailable"
        exit 1
      fi
    fi
    mkdir -p /etc/systemd/system
    sed "s|/usr/local/bin/notifyd|${PREFIX}/notifyd|g" "${TMPDIR}/systemd/notifyd.service" > /etc/systemd/system/notifyd.service
    echo "  Installed /etc/systemd/system/notifyd.service"
  fi
  if [ -f "${TMPDIR}/scripts/notify.sh" ]; then
    mkdir -p /etc/profile.d
    cp "${TMPDIR}/scripts/notify.sh" /etc/profile.d/notify.sh
    echo "  Installed /etc/profile.d/notify.sh"
  fi
  if [ "$HAS_NOTIFYD" = "1" ] && [ -f "${TMPDIR}/configs/notify.yaml" ] && [ ! -f /etc/notify/config.yaml ]; then
    mkdir -p /etc/notify
    cp "${TMPDIR}/configs/notify.yaml" /etc/notify/config.yaml
    echo "  Installed /etc/notify/config.yaml"
  fi
fi

echo ""
echo "notify v${VERSION} installed successfully!"
echo ""
echo "To start the daemon:"
echo "  sudo systemctl daemon-reload"
echo "  sudo systemctl enable --now notifyd"
echo ""
echo "To send a notification:"
echo "  notify --title 'Hello' --message 'World'"
