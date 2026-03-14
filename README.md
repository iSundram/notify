# Notify

A lightweight system notification service for Linux servers and applications written in Go.

## Overview

Notify is a simple, fast, and developer-friendly notification system designed for Linux environments (servers, panels, and developer workstations). It provides:

- **notifyd** — a centralized notification daemon
- **notify** — a CLI client for sending notifications
- **notifyctl** — a management tool for listing, counting, marking, deleting, and **live dashboarding**
- **Unix socket** for local IPC with real-time `watch` support
- **HTTP API** for remote access with **Server-Sent Events (SSE)** streaming
- **Interactive TUI Dashboard** for real-time notification management
- **Read/unread state** with persistent storage
- **Terminal startup message** showing unread count

## Installation

### Quick Install (recommended)

Download and install the latest release with a single command:

```bash
curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/install.sh | sudo sh
```

This auto-detects your architecture, downloads the correct binaries, and installs them to `/usr/local/bin`. When run as root it also installs:

- `systemd/notifyd.service`
- `scripts/notify.sh`
- `/etc/notify/config.yaml` (if not already present)

### Install a Specific Version

```bash
curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/install.sh | sudo sh -s -- --version 0.1.0
```

### Install to a Custom Directory

```bash
curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/install.sh | sh -s -- --prefix ~/.local/bin
```

### Install from Source

Requires Go 1.24+ and a C compiler (for SQLite via CGO).

On Debian/Ubuntu:

```bash
sudo apt-get update && sudo apt-get install -y gcc libsqlite3-dev
```

```bash
git clone https://github.com/iSundram/notify.git
cd notify
go build -o notifyd   ./cmd/notifyd
go build -o notify    ./cmd/notify
go build -o notifyctl ./cmd/notifyctl
sudo install -m 755 notifyd notify notifyctl /usr/local/bin/
```

### Post-Install Setup

After installing, start the daemon with systemd:

```bash
sudo cp systemd/notifyd.service /etc/systemd/system/  # only needed for source installs
sudo mkdir -p /etc/notify
sudo cp configs/notify.yaml /etc/notify/config.yaml
sudo systemctl daemon-reload
sudo systemctl enable --now notifyd
```

Optionally, enable the terminal startup message (shows unread count on new shells):

```bash
sudo cp scripts/notify.sh /etc/profile.d/  # only needed for source installs
```

### Verify Installation

```bash
notify --version
notifyctl --version
notifyd --version
```

### Uninstall

Remove installed binaries and service files:

```bash
curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/uninstall.sh | sudo sh
```

To fully purge config and data too:

```bash
curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/uninstall.sh | sudo sh -s -- --purge-config --purge-data --remove-user
```

Reinstall anytime with:

```bash
curl -fsSL https://raw.githubusercontent.com/iSundram/notify/main/install.sh | sudo sh
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
socket_path: /run/notify/notify.sock
http_addr: ":8008"
log_dir: /var/log/notify
cache_file: /run/notify/unread_count
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

# Interactive Dashboard (TUI)
notifyctl dashboard
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
| GET    | `/stream`                     | Real-time event stream (SSE) |

### Query Parameters for GET /notifications

- `status` — `unread`, `read`, or `all` (default: `all`)
- `limit` — max results (default: 50)
- `offset` — pagination offset
- `source` — filter by source
- `priority` — filter by priority

## Terminal Startup Message

The install script automatically places `scripts/notify.sh` into `/etc/profile.d/` when run as root. On every new shell session you'll see:

```
You have 3 unread notification(s). Run 'notifyctl list --status unread'
```

For source installs, copy it manually:

```bash
sudo cp scripts/notify.sh /etc/profile.d/
```

## Releasing a New Version

Releases are fully automated via [GoReleaser](https://goreleaser.com/) and GitHub Actions.

### How to Release

1. Edit the `VERSION` file with the new semver version:
   ```bash
   echo "0.2.0" > VERSION
   ```
2. Commit and push to `main`:
   ```bash
   git add VERSION
   git commit -m "release: v0.2.0"
   git push origin main
   ```
3. GitHub Actions will automatically:
   - Read the version from `VERSION`
   - Create a `v0.2.0` git tag
   - Build binaries for all supported platforms
   - Publish a GitHub Release with archives and checksums

### What Gets Built

| Binary     | Platforms        | Notes                          |
|------------|------------------|--------------------------------|
| `notifyd`  | linux/amd64, linux/arm64 | Requires CGO (sqlite3) |
| `notify`   | linux/amd64, linux/arm64 | Pure Go, no CGO        |
| `notifyctl`| linux/amd64, linux/arm64 | Pure Go, no CGO        |

Each release archive includes the binaries along with `README.md`, `LICENSE`, `systemd/notifyd.service`, `scripts/notify.sh`, and `configs/notify.yaml`.

### Version in Binaries

All binaries have the version, commit SHA, and build date embedded at compile time:

```bash
$ notify --version
notify 0.1.0 (commit: abc1234, built: 2026-03-14T05:00:00Z)
```

## Testing

```bash
go test ./...
```

## Project Structure

```
notify/
├── cmd/
│   ├── notify/       # CLI client for sending notifications
│   ├── notifyctl/    # Management tool (list, count, mark, delete, follow)
│   └── notifyd/      # Notification daemon (socket + HTTP server)
├── internal/
│   ├── config/       # YAML configuration loader
│   ├── model/        # Notification data model
│   ├── server/       # Socket and HTTP server implementations
│   └── store/        # SQLite storage backend
├── scripts/
│   └── notify.sh     # Shell startup script (profile.d)
├── configs/
│   └── notify.yaml   # Default daemon config used by installer/systemd
├── systemd/
│   └── notifyd.service
├── .goreleaser.yaml  # GoReleaser build configuration
├── VERSION           # Current version (edit to trigger a release)
├── install.sh        # One-line installer script
└── uninstall.sh      # Uninstall script (with optional purge flags)
```

## License

MIT
