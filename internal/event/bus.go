package event

import (
	"sync"

	"github.com/iSundram/notify/internal/model"
)

// EventType defines the type of event.
type EventType string

const (
	EventCreated      EventType = "created"
	EventMarkedRead   EventType = "marked_read"
	EventMarkedUnread EventType = "marked_unread"
	EventMarkedAllRead EventType = "marked_all_read"
	EventDeleted      EventType = "deleted"
)

// Event represents a notification lifecycle event.
type Event struct {
	Type         EventType           `json:"type"`
	Notification *model.Notification `json:"notification,omitempty"`
	ID           string              `json:"id,omitempty"`
}

// Bus manages event subscribers and broadcasts.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

// NewBus creates a new EventBus.
func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe adds a new subscriber and returns a channel for events.
func (b *Bus) Subscribe() chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, 10)
	b.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscriber.
func (b *Bus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subscribers, ch)
	close(ch)
}

// Broadcast sends an event to all subscribers.
func (b *Bus) Broadcast(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- e:
		default:
			// Subscriber is slow; skip or handle appropriately.
		}
	}
}
