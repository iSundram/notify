package store

import "github.com/iSundram/notify/internal/model"

// Store defines the interface for notification persistence.
type Store interface {
	// Create inserts a new notification and returns its ID.
	Create(n *model.Notification) (string, error)

	// Get retrieves a single notification by ID.
	Get(id string) (*model.Notification, error)

	// List returns notifications matching the given filter.
	List(filter model.ListFilter) ([]model.Notification, error)

	// Count returns the number of notifications matching the status filter.
	Count(status string) (int, error)

	// MarkRead marks a notification as read.
	MarkRead(id string, readBy string) error

	// MarkUnread marks a notification as unread.
	MarkUnread(id string) error

	// MarkAllRead marks all notifications as read.
	MarkAllRead(readBy string) error

	// Delete removes a notification by ID.
	Delete(id string) error

	// Close closes the store.
	Close() error
}
