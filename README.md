# Notify

A lightweight system notification service for Linux servers and applications written in Go.

## Overview

Notify is a simple, fast, and developer-friendly notification system designed for Linux environments (servers, panels, and developer workstations). It provides:

- **notifyd** — a centralized notification daemon
- **notify** — a CLI client for sending notifications
- **notifyctl** — a management tool for listing, counting, marking, and deleting notifications
- **Unix socket** for local IPC
- **HTTP API** for remote access
- **Read/unread state** with persistent storage
- **Terminal startup message** showing unread count

## Building

```bash
go build -o notifyd   ./cmd/notifyd
go build -o notify    ./cmd/notify
go build -o notifyctl ./cmd/notifyctl
```

## Running the Daemon

```bash
# With defaults (SQLite at /var/lib/notify/notify.db)
notifyd

# With a config file
notifyd --config /etc/notify/config.yaml
```

### Configuration (YAML)

```yaml
db_path: /var/lib/notify/notify.db
socket_path: /var/run/notify.sock
http_addr: ":8008"
log_dir: /var/log/notify
cache_file: /var/run/notify/unread_count
```

## Sending Notifications

### CLI

```bash
notify --title "Backup" --message "Backup completed successfully" --priority success
```

### HTTP API

```bash
curl -X POST http://localhost:8008/notify \
  -H 'Content-Type: application/json' \
  -d '{"title":"Backup","message":"Backup completed","priority":"info","source":"backup","tags":["daily"]}'
```

## Managing Notifications

```bash
# Count unread
notifyctl count --status unread

# List unread
notifyctl list --status unread --limit 10

# Mark as read
notifyctl mark --id <uuid> --read

# Mark all as read
notifyctl mark --all --read

# Delete
notifyctl delete --id <uuid>

# Follow new notifications (live)
notifyctl follow
```

## HTTP API Endpoints

| Method | Endpoint                      | Description              |
|--------|-------------------------------|--------------------------|
| POST   | `/notify`                     | Create a notification    |
| GET    | `/notifications`              | List notifications       |
| GET    | `/notifications/count`        | Count notifications      |
| POST   | `/notifications/{id}/read`    | Mark as read             |
| POST   | `/notifications/{id}/unread`  | Mark as unread           |
| DELETE | `/notifications/{id}`         | Delete a notification    |

### Query Parameters for GET /notifications

- `status` — `unread`, `read`, or `all` (default: `all`)
- `limit` — max results (default: 50)
- `offset` — pagination offset
- `source` — filter by source
- `priority` — filter by priority

## Terminal Startup Message

Install `scripts/notify.sh` to `/etc/profile.d/` to display unread notification count on terminal startup:

```
You have 3 unread notification(s). Run 'notifyctl list --status unread'
```

## Systemd Service

```bash
cp systemd/notifyd.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now notifyd
```

## Testing

```bash
go test ./...
```

## License

MIT
