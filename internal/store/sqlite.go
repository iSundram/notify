package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/iSundram/notify/internal/model"
)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at path and runs migrations.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			priority TEXT NOT NULL DEFAULT 'info',
			source TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			timestamp TIMESTAMP NOT NULL,
			read BOOLEAN NOT NULL DEFAULT 0,
			read_at TIMESTAMP NULL,
			read_by TEXT NULL,
			expires_at TIMESTAMP NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_read_timestamp ON notifications(read, timestamp)`,
		`CREATE TABLE IF NOT EXISTS unread_counts (
			key TEXT PRIMARY KEY,
			count INTEGER NOT NULL DEFAULT 0
		)`,
		`INSERT OR IGNORE INTO unread_counts (key, count) VALUES ('global', 0)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s[:40], err)
		}
	}
	return nil
}

// Create inserts a new notification.
func (s *SQLiteStore) Create(n *model.Notification) (string, error) {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.Timestamp.IsZero() {
		n.Timestamp = time.Now().UTC()
	}

	tagsJSON, err := json.Marshal(n.Tags)
	if err != nil {
		return "", fmt.Errorf("marshal tags: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO notifications (id, title, message, priority, source, tags, timestamp, read, read_at, read_by, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Title, n.Message, n.Priority, n.Source,
		string(tagsJSON), n.Timestamp,
		n.Read, nullTime(n.ReadAt), n.ReadBy, nullTime(n.ExpiresAt),
	)
	if err != nil {
		return "", fmt.Errorf("insert notification: %w", err)
	}

	if !n.Read {
		if _, err := tx.Exec(`UPDATE unread_counts SET count = count + 1 WHERE key = 'global'`); err != nil {
			return "", fmt.Errorf("increment unread: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return n.ID, nil
}

// Get retrieves a notification by ID.
func (s *SQLiteStore) Get(id string) (*model.Notification, error) {
	row := s.db.QueryRow(
		`SELECT id, title, message, priority, source, tags, timestamp, read, read_at, read_by, expires_at
		 FROM notifications WHERE id = ?`, id,
	)
	return scanNotification(row)
}

// List returns notifications matching the filter.
func (s *SQLiteStore) List(f model.ListFilter) ([]model.Notification, error) {
	var where []string
	var args []interface{}

	switch f.Status {
	case "unread":
		where = append(where, "read = 0")
	case "read":
		where = append(where, "read = 1")
	}
	if f.Source != "" {
		where = append(where, "source = ?")
		args = append(args, f.Source)
	}
	if f.Priority != "" {
		where = append(where, "priority = ?")
		args = append(args, f.Priority)
	}

	query := `SELECT id, title, message, priority, source, tags, timestamp, read, read_at, read_by, expires_at FROM notifications`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY timestamp DESC"

	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	if f.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", f.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list query: %w", err)
	}
	defer rows.Close()

	var results []model.Notification
	for rows.Next() {
		n, err := scanNotificationRows(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *n)
	}
	return results, rows.Err()
}

// Count returns the count of notifications by status.
func (s *SQLiteStore) Count(status string) (int, error) {
	if status == "unread" {
		var count int
		err := s.db.QueryRow(`SELECT count FROM unread_counts WHERE key = 'global'`).Scan(&count)
		return count, err
	}

	var query string
	switch status {
	case "read":
		query = `SELECT COUNT(*) FROM notifications WHERE read = 1`
	default:
		query = `SELECT COUNT(*) FROM notifications`
	}
	var count int
	err := s.db.QueryRow(query).Scan(&count)
	return count, err
}

// MarkRead marks a notification as read.
func (s *SQLiteStore) MarkRead(id, readBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check current state
	var isRead bool
	if err := tx.QueryRow(`SELECT read FROM notifications WHERE id = ?`, id).Scan(&isRead); err != nil {
		return fmt.Errorf("notification not found: %w", err)
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(
		`UPDATE notifications SET read = 1, read_at = ?, read_by = ? WHERE id = ?`,
		now, readBy, id,
	); err != nil {
		return err
	}

	if !isRead {
		if _, err := tx.Exec(`UPDATE unread_counts SET count = MAX(count - 1, 0) WHERE key = 'global'`); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// MarkUnread marks a notification as unread.
func (s *SQLiteStore) MarkUnread(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var isRead bool
	if err := tx.QueryRow(`SELECT read FROM notifications WHERE id = ?`, id).Scan(&isRead); err != nil {
		return fmt.Errorf("notification not found: %w", err)
	}

	if _, err := tx.Exec(
		`UPDATE notifications SET read = 0, read_at = NULL, read_by = NULL WHERE id = ?`, id,
	); err != nil {
		return err
	}

	if isRead {
		if _, err := tx.Exec(`UPDATE unread_counts SET count = count + 1 WHERE key = 'global'`); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// MarkAllRead marks all unread notifications as read.
func (s *SQLiteStore) MarkAllRead(readBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if _, err := tx.Exec(
		`UPDATE notifications SET read = 1, read_at = ?, read_by = ? WHERE read = 0`,
		now, readBy,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`UPDATE unread_counts SET count = 0 WHERE key = 'global'`); err != nil {
		return err
	}

	return tx.Commit()
}

// Delete removes a notification by ID.
func (s *SQLiteStore) Delete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var isRead bool
	if err := tx.QueryRow(`SELECT read FROM notifications WHERE id = ?`, id).Scan(&isRead); err != nil {
		return fmt.Errorf("notification not found: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM notifications WHERE id = ?`, id); err != nil {
		return err
	}

	if !isRead {
		if _, err := tx.Exec(`UPDATE unread_counts SET count = MAX(count - 1, 0) WHERE key = 'global'`); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Close closes the database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- helpers ---

func nullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanNotification(row *sql.Row) (*model.Notification, error) {
	var n model.Notification
	var tagsJSON string
	var readAt, expiresAt sql.NullTime
	var readBy sql.NullString

	err := row.Scan(
		&n.ID, &n.Title, &n.Message, &n.Priority, &n.Source,
		&tagsJSON, &n.Timestamp, &n.Read, &readAt, &readBy, &expiresAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(tagsJSON), &n.Tags); err != nil {
		n.Tags = nil
	}
	if readAt.Valid {
		n.ReadAt = &readAt.Time
	}
	if readBy.Valid {
		n.ReadBy = readBy.String
	}
	if expiresAt.Valid {
		n.ExpiresAt = &expiresAt.Time
	}
	return &n, nil
}

func scanNotificationRows(rows *sql.Rows) (*model.Notification, error) {
	var n model.Notification
	var tagsJSON string
	var readAt, expiresAt sql.NullTime
	var readBy sql.NullString

	err := rows.Scan(
		&n.ID, &n.Title, &n.Message, &n.Priority, &n.Source,
		&tagsJSON, &n.Timestamp, &n.Read, &readAt, &readBy, &expiresAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(tagsJSON), &n.Tags); err != nil {
		n.Tags = nil
	}
	if readAt.Valid {
		n.ReadAt = &readAt.Time
	}
	if readBy.Valid {
		n.ReadBy = readBy.String
	}
	if expiresAt.Valid {
		n.ExpiresAt = &expiresAt.Time
	}
	return &n, nil
}
