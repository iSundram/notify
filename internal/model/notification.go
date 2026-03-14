package model

import (
	"time"
)

// Notification represents a system notification.
type Notification struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Priority  string     `json:"priority"`            // info, success, warning, critical
	Source    string     `json:"source"`               // e.g. admini, backup, monitor
	Tags      []string   `json:"tags,omitempty"`
	Timestamp time.Time  `json:"timestamp"`

	// Read/Unread fields
	Read   bool       `json:"read"`
	ReadAt *time.Time `json:"read_at,omitempty"`
	ReadBy string     `json:"read_by,omitempty"` // username / client id

	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// CreateRequest is the payload for creating a new notification.
type CreateRequest struct {
	Title     string   `json:"title"`
	Message   string   `json:"message"`
	Priority  string   `json:"priority"`
	Source    string   `json:"source"`
	Tags      []string `json:"tags,omitempty"`
	ExpiresAt *string  `json:"expires_at,omitempty"`
}

// ListFilter holds query parameters for listing notifications.
type ListFilter struct {
	Status   string // "unread", "read", "all"
	Limit    int
	Offset   int
	Source   string
	Priority string
}

// CountResult holds the count of notifications.
type CountResult struct {
	Count int `json:"count"`
}

// MarkRequest represents a request to mark a notification read/unread.
type MarkRequest struct {
	ReadBy string `json:"read_by,omitempty"`
}

// ValidPriorities lists the allowed priority values.
var ValidPriorities = map[string]bool{
	"info":     true,
	"success":  true,
	"warning":  true,
	"critical": true,
}

// ValidStatuses lists the allowed status filter values.
var ValidStatuses = map[string]bool{
	"unread": true,
	"read":   true,
	"all":    true,
}
