#!/usr/bin/env sh
# uninstall.sh — Remove notify binaries and optional runtime data.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/uninstall.sh | sudo sh
#   curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/uninstall.sh | sudo sh -s -- --purge-data --purge-config
#
# Options:
#   --prefix DIR      Installation prefix (default: /usr/local/bin)
#   --purge-data      Remove runtime/state data (/var/lib/notify, /var/log/notify, /run/notify)
#   --purge-config    Remove /etc/notify directory
#   --remove-user     Remove notify system user/group (if present)
#   --help            Show this help message

set -e

PREFIX="/usr/local/bin"
PURGE_DATA=0
PURGE_CONFIG=0
REMOVE_USER=0

usage() {
  echo "Usage: uninstall.sh [--prefix DIR] [--purge-data] [--purge-config] [--remove-user]"
  echo ""
  echo "Options:"
  echo "  --prefix DIR      Installation prefix (default: /usr/local/bin)"
  echo "  --purge-data      Remove runtime/state data (/var/lib/notify, /var/log/notify, /run/notify)"
  echo "  --purge-config    Remove /etc/notify directory"
  echo "  --remove-user     Remove notify system user/group (if present)"
  echo "  --help            Show this help message"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --prefix) PREFIX="$2"; shift 2 ;;
    --purge-data) PURGE_DATA=1; shift ;;
    --purge-config) PURGE_CONFIG=1; shift ;;
    --remove-user) REMOVE_USER=1; shift ;;
    --help) usage; exit 0 ;;
    *) echo "Unknown option: $1"; usage; exit 1 ;;
  esac
done

echo "Uninstalling notify..."

# Stop and disable service if systemd is available.
if command -v systemctl >/dev/null 2>&1; then
  if systemctl list-unit-files notifyd.service >/dev/null 2>&1; then
    systemctl disable --now notifyd >/dev/null 2>&1 || true
  fi
fi

# Remove binaries.
for bin in notify notifyctl notifyd; do
  if [ -f "${PREFIX}/${bin}" ]; then
    rm -f "${PREFIX}/${bin}"
    echo "  Removed ${PREFIX}/${bin}"
  fi
done

# Remove system-level files.
if [ -f /etc/systemd/system/notifyd.service ]; then
  rm -f /etc/systemd/system/notifyd.service
  echo "  Removed /etc/systemd/system/notifyd.service"
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload >/dev/null 2>&1 || true
fi

if [ -f /etc/profile.d/notify.sh ]; then
  rm -f /etc/profile.d/notify.sh
  echo "  Removed /etc/profile.d/notify.sh"
fi

# Always clean runtime files.
rm -f /run/notify/notify.sock /run/notify/unread_count /var/run/notify.sock /var/run/notify/unread_count 2>/dev/null || true
rmdir /run/notify 2>/dev/null || true
rmdir /var/run/notify 2>/dev/null || true

if [ "$PURGE_CONFIG" = "1" ]; then
  if [ -d /etc/notify ]; then
    rm -rf /etc/notify
    echo "  Removed /etc/notify"
  fi
fi

if [ "$PURGE_DATA" = "1" ]; then
  if [ -d /var/lib/notify ]; then
    rm -rf /var/lib/notify
    echo "  Removed /var/lib/notify"
  fi
  if [ -d /var/log/notify ]; then
    rm -rf /var/log/notify
    echo "  Removed /var/log/notify"
  fi
  if [ -d /run/notify ]; then
    rm -rf /run/notify
    echo "  Removed /run/notify"
  fi
fi

if [ "$REMOVE_USER" = "1" ]; then
  if id -u notify >/dev/null 2>&1; then
    if command -v userdel >/dev/null 2>&1; then
      userdel notify >/dev/null 2>&1 || true
      echo "  Removed user notify"
    fi
  fi
  if getent group notify >/dev/null 2>&1; then
    if command -v groupdel >/dev/null 2>&1; then
      groupdel notify >/dev/null 2>&1 || true
      echo "  Removed group notify"
    fi
  fi
fi

echo ""
echo "notify uninstall complete."
echo "To reinstall: curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/install.sh | sudo sh"
