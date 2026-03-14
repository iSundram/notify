Notify

A lightweight system notification service for Linux servers and applications written in Go.


---

Overview

Notify is a simple, fast, and developer-friendly notification system designed for Linux environments (servers, panels, and developer workstations). It provides a centralized notification daemon (notifyd), a CLI client (notify / notifyctl), a Unix socket for local IPC, and an optional HTTP API. It supports read / unread state, persistent storage, and a terminal startup message that shows the current unread count (e.g. "You have 3 unread notifications").

Notify is ideal for hosting panels, automation scripts, monitoring tools, and background services that need a consistent, language-agnostic way to surface events to operators.


---

Key New Features (added)

Read / Unread state for notifications with timestamps and read_by metadata.

Unread counter on shell startup: prints unread count near the top of the terminal session (system-wide or per-user).

New CLI subcommands to list, filter, mark read/unread, count, and clear notifications.

Storage and indexing guidance to keep count queries fast and reliable.



---

Notification Data Model

type Notification struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    Message   string    `json:"message"`
    Priority  string    `json:"priority"` // info, success, warning, critical
    Source    string    `json:"source"`   // e.g. admini, backup, monitor
    Tags      []string  `json:"tags,omitempty"`
    Timestamp time.Time `json:"timestamp"`

    // Read/Unread fields
    Read      bool      `json:"read"`
    ReadAt    *time.Time `json:"read_at,omitempty"`
    ReadBy    string    `json:"read_by,omitempty"` // username / client id

    ExpiresAt *time.Time `json:"expires_at,omitempty"`
}


---

Storage & Indexing

Use SQLite, BoltDB, or BadgerDB for lightweight persistence. For high scale use Postgres.

Index on (read, timestamp) and (source, priority, tags) to support fast unread count and listing.

Maintain an unread counter per user (or global) in a small key/value store for O(1) reads.


Table example (SQL):

CREATE TABLE notifications (
  id TEXT PRIMARY KEY,
  title TEXT,
  message TEXT,
  priority TEXT,
  source TEXT,
  tags TEXT, -- JSON or CSV
  timestamp TIMESTAMP,
  read BOOLEAN DEFAULT FALSE,
  read_at TIMESTAMP NULL,
  read_by TEXT NULL,
  expires_at TIMESTAMP NULL
);
CREATE INDEX idx_notifications_read_timestamp ON notifications(read, timestamp);

And a simple key-value table for counts:

CREATE TABLE unread_counts (
  key TEXT PRIMARY KEY, -- e.g. "global" or "user:alice"
  count INTEGER NOT NULL
);


---

API (HTTP)

POST /notify

Send a new notification.

Request body:

{
  "title": "Backup",
  "message": "Backup completed successfully",
  "priority": "info",
  "source": "backup",
  "tags": ["daily","s3"],
  "expires_at": "2026-05-01T00:00:00Z"
}

Response: 201 Created with created id.

GET /notifications

List notifications with filters:

?status=unread|read|all

?limit=50&offset=0

?source=admini&priority=critical


GET /notifications/count?status=unread

Return JSON: { "count": 3 }.

POST /notifications/{id}/read

Mark notification as read. Body may include read_by.

POST /notifications/{id}/unread

Mark notification as unread.

DELETE /notifications/{id}

Delete (soft or hard depending on config).

Auth: API key or JWT (optional based on config). Use TLS when HTTP is enabled.


---

Socket Protocol (Unix socket)

Path: /var/run/notify.sock (or configured path)

Use a minimal text/JSON RPC protocol. Example request (newline-delimited JSON):

{"method":"notify","params":{"title":"Backup","message":"complete","priority":"info"}}

{"method":"count","params":{"status":"unread"}}

{"method":"list","params":{"status":"unread","limit":10}}

Responses are newline-delimited JSON objects.

Socket permissions should be restricted (see Security section).


---

CLI Tools

Two helpful binaries:

/usr/bin/notify — simple one-shot notifier (send only)

/usr/bin/notifyctl — management tool (list, count, mark, clear)


Examples:

Send a notification (simple)

notify --title "Admini" --message "WordPress installation completed" --priority success

Count unread notifications

notifyctl count --status unread
# prints: 3

List unread (paginated)

notifyctl list --status unread --limit 10 --offset 0

Mark read / unread

notifyctl mark --id <uuid> --read
notifyctl mark --id <uuid> --unread

Mark all read (use with care)

notifyctl mark --all --read

Clear / delete

notifyctl delete --id <uuid>

Follow new notifications (live)

notifyctl follow
# tail -f style streaming output


---

Terminal Startup Unread Message

Goal: On terminal startup (interactive login shell or new terminal window), display an unobtrusive message at the top like:

You have 3 unread notifications — run `notifyctl list --status unread` to view them

Implementation approaches

1. System-wide (global message)

Install a small shell script in /etc/profile.d/notify.sh (works for most sh, bash, zsh on login and interactive shells).

The script calls notifyctl count --status unread --format short and prints the message only if count > 0.

Use a short timeout for socket calls so shell startup is never blocked.



2. Per-user

Place ~/.config/notify/notify-shell.sh and ask the user to source it from their ~/.bashrc, ~/.zshrc, or ~/.config/fish/config.fish.



3. Non-blocking / performance

The startup script performs a non-blocking check: it launches the count request in the background and prints the message when available, or uses a cached unread count file (~/.cache/notify/unread_count) updated by the daemon every N seconds.



4. Placement / styling

Print the message at the top of the session output (stdout) before prompt appears.

Keep message short; avoid multi-line banners.




Example /etc/profile.d/notify.sh

#!/usr/bin/env sh
# print unread notifications count on interactive shells
[ -z "$PS1" ] && return 0
# try the cached file first for speed
if [ -r "/var/run/notify/unread_count" ]; then
  count=$(cat /var/run/notify/unread_count 2>/dev/null || echo 0)
else
  # fallback: quick socket query with timeout
  count=$(timeout 0.05 notifyctl count --status unread --format short 2>/dev/null || echo 0)
fi
if [ "$count" -gt 0 ]; then
  printf "You have %s unread notification(s). Run 'notifyctl list --status unread'
" "$count"
fi

Notes:

timeout helps ensure the shell prompt isn't delayed.

notifyd can maintain /var/run/notify/unread_count (updated on every notification and every mark/read change) for instant reads.



---

Access Control & Multi-User Considerations

Notifications may be global or per-user. Each notification can include a target field (e.g., global, user:alice, group:admins).

The unread counter and startup message can be per-user: ~/.cache/notify/unread_count:user:alice or /var/run/notify/unread_count:user:alice.

Use Unix groups and socket permissions to restrict who can write vs read.

notifyd should support running as root with a notify group; clients that need to write can be added to the notify group.



---

Security

Restrict Unix socket permissions: chown root:notify /var/run/notify.sock && chmod 660 /var/run/notify.sock.

If HTTP API is enabled, require HTTPS + API key or JWT.

Rate-limit endpoints and socket writes to avoid DoS.

Validate and sanitize all input text to avoid control characters that could manipulate terminal output.

Optional per-notification signing for high-integrity environments.



---

Output & Delivery Handlers

Terminal — immediate textual output in shells and in notifyctl follow.

Log — persistent logs at /var/log/notify/ with rotation.

Web Dashboard — view & mark read/unread.

Email / SMTP — optional; for critical notifications.

Webhooks — deliver JSON payloads to external services.

Chat integrations — Slack, Telegram, Discord.

Desktop bridge — forward to libnotify / D-Bus for desktop popups when a user desktop session exists.


Each handler can be enabled/disabled per notification priority.


---

CLI UX / TUI Ideas

notifyctl list supports --format json|table|short.

Interactive TUI (optional) using tview or bubbletea for browsing and marking notifications.

notify one-liner friendly output for scripts (exit codes, --json output).



---

Monitoring & Metrics

Expose /metrics for Prometheus with counters: notify_total, notify_unread_total, notify_sent_by_priority{priority}, notify_api_requests_total.

Health endpoints and readiness/liveness probes for containerized deployments.



---

Systemd Service Example (notifyd)

/etc/systemd/system/notifyd.service

[Unit]
Description=Notify Notification Service
After=network.target

[Service]
ExecStart=/usr/bin/notifyd --config /etc/notify/config.yaml
Restart=always
User=root
Group=notify

[Install]
WantedBy=multi-user.target

Daemon must ensure to atomically update unread count cache files after each write to keep shell startup messages accurate.


---

Performance Goals & Safety

Maintain an unread cache for O(1) reads at startup.

Keep socket/API queries non-blocking on shell startup (short timeouts, cached fallback).

Ensure the daemon can handle bursts — use a small in-memory queue with backpressure and persistent write-ahead logging if needed.



---

Example Workflows

New notification flow

1. Service calls notify CLI or HTTP API.


2. notifyd validates and stores notification, increments unread counter for target(s), appends to log.


3. notifyd updates /var/run/notify/unread_count[:user] or writes to cache DB.


4. If desktop user present: notifyd forwards to D-Bus/libnotify.


5. If configured: deliver to email / webhook / chat.



User reads notification

1. User runs notifyctl list --status unread and views messages.


2. User marks one (or all) as read with notifyctl mark --id <id> --read.


3. notifyd updates DB, decreases unread counter atomically, updates cache file.




---

Comprehensive Feature List (Full)

Core: centralized notifyd daemon, CLI notify and notifyctl, Unix socket + optional HTTP API.

Read / Unread state with read_at, read_by.

Startup unread message for terminals (per-user or global), non-blocking + cached.

Per-user, per-group, and global targeting.

Persistent storage (SQLite / Bolt / Postgres) with indexes.

Unread counters & cache for O(1) reads.

Filtering, pagination, sorting, and tags.

Priorities + TTL / expiry.

Delivery handlers: terminal, logs, web dashboard, email, webhooks, Slack/Telegram.

D-Bus / libnotify bridging for desktop popups.

Interactive TUI client.

Metrics (Prometheus), health endpoints.

RBAC / API keys / JWTs for HTTP API.

Rate limiting and input validation.

Audit logs and retention policies.

CLI-friendly outputs (--json, exit codes) for automation.

Pluggable plugin system for handlers.

High-availability options: clustered backend or leader election.



---


License

MIT


---

Vision

Notify should be the universal, lightweight notification layer for Linux infrastructure — small enough to run on a VM or container, powerful enough to integrate with large control panels like Admini, and friendly enough that any script or program can call it without friction.

