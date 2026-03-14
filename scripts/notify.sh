#!/usr/bin/env sh
# /etc/profile.d/notify.sh
# Print unread notifications count on interactive shell startup.
[ -z "$PS1" ] && return 0

CACHE_FILE="${NOTIFY_CACHE_FILE:-/run/notify/unread_count}"
[ -r "$CACHE_FILE" ] || CACHE_FILE="/var/run/notify/unread_count"

# Try the cached file first for speed.
if [ -r "$CACHE_FILE" ]; then
  count=$(cat "$CACHE_FILE" 2>/dev/null || echo 0)
else
  # Fallback: quick socket query with timeout.
  if command -v notifyctl >/dev/null 2>&1; then
    count=$(timeout 0.05 notifyctl count --status unread --format short 2>/dev/null || echo 0)
  else
    count=0
  fi
fi

if [ "$count" -gt 0 ] 2>/dev/null; then
  printf "You have %s unread notification(s). Run 'notifyctl list --status unread'\n" "$count"
fi
