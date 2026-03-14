#!/usr/bin/env sh
# /etc/profile.d/notify.sh
# Print unread notifications count on interactive shell startup.
[ -z "$PS1" ] && return 0

# Try the cached file first for speed.
if [ -r "/var/run/notify/unread_count" ]; then
  count=$(cat /var/run/notify/unread_count 2>/dev/null || echo 0)
else
  # Fallback: quick socket query with timeout.
  count=$(timeout 0.05 notifyctl count --status unread --format short 2>/dev/null || echo 0)
fi

if [ "$count" -gt 0 ] 2>/dev/null; then
  printf "You have %s unread notification(s). Run 'notifyctl list --status unread'\n" "$count"
fi
