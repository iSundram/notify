package store

import (
	"os"
	"testing"

	"github.com/iSundram/notify/internal/model"
)

func tempStore(t *testing.T) *SQLiteStore {
	t.Helper()
	f, err := os.CreateTemp("", "notify-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	st, err := NewSQLiteStore(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestCreateAndGet(t *testing.T) {
	st := tempStore(t)

	n := &model.Notification{
		Title:    "Test",
		Message:  "Hello world",
		Priority: "info",
		Source:   "test",
		Tags:     []string{"a", "b"},
	}

	id, err := st.Create(n)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	got, err := st.Get(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.Title != "Test" {
		t.Errorf("title = %q, want %q", got.Title, "Test")
	}
	if got.Message != "Hello world" {
		t.Errorf("message = %q, want %q", got.Message, "Hello world")
	}
	if got.Read {
		t.Error("expected unread")
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags len = %d, want 2", len(got.Tags))
	}
}

func TestCountUnread(t *testing.T) {
	st := tempStore(t)

	// Initially 0 unread.
	count, err := st.Count("unread")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("initial unread count = %d, want 0", count)
	}

	// Create 3 notifications.
	for i := 0; i < 3; i++ {
		_, err := st.Create(&model.Notification{
			Title:    "Test",
			Message:  "msg",
			Priority: "info",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	count, err = st.Count("unread")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("unread count = %d, want 3", count)
	}
}

func TestMarkReadUnread(t *testing.T) {
	st := tempStore(t)

	id, _ := st.Create(&model.Notification{
		Title:    "Test",
		Message:  "msg",
		Priority: "info",
	})

	// Mark read.
	if err := st.MarkRead(id, "alice"); err != nil {
		t.Fatal(err)
	}

	got, _ := st.Get(id)
	if !got.Read {
		t.Error("expected read after MarkRead")
	}
	if got.ReadBy != "alice" {
		t.Errorf("read_by = %q, want alice", got.ReadBy)
	}
	if got.ReadAt == nil {
		t.Error("expected non-nil read_at")
	}

	// Unread count should be 0.
	count, _ := st.Count("unread")
	if count != 0 {
		t.Errorf("unread = %d, want 0", count)
	}

	// Mark unread.
	if err := st.MarkUnread(id); err != nil {
		t.Fatal(err)
	}

	got, _ = st.Get(id)
	if got.Read {
		t.Error("expected unread after MarkUnread")
	}

	count, _ = st.Count("unread")
	if count != 1 {
		t.Errorf("unread = %d, want 1", count)
	}
}

func TestMarkAllRead(t *testing.T) {
	st := tempStore(t)

	for i := 0; i < 5; i++ {
		st.Create(&model.Notification{
			Title:    "Test",
			Message:  "msg",
			Priority: "info",
		})
	}

	if err := st.MarkAllRead("admin"); err != nil {
		t.Fatal(err)
	}

	count, _ := st.Count("unread")
	if count != 0 {
		t.Errorf("unread after mark all = %d, want 0", count)
	}
}

func TestDelete(t *testing.T) {
	st := tempStore(t)

	id, _ := st.Create(&model.Notification{
		Title:    "Test",
		Message:  "msg",
		Priority: "info",
	})

	if err := st.Delete(id); err != nil {
		t.Fatal(err)
	}

	_, err := st.Get(id)
	if err == nil {
		t.Error("expected error after delete")
	}

	count, _ := st.Count("unread")
	if count != 0 {
		t.Errorf("unread after delete = %d, want 0", count)
	}
}

func TestListWithFilters(t *testing.T) {
	st := tempStore(t)

	st.Create(&model.Notification{Title: "A", Message: "m", Priority: "info", Source: "backup"})
	st.Create(&model.Notification{Title: "B", Message: "m", Priority: "critical", Source: "monitor"})
	id3, _ := st.Create(&model.Notification{Title: "C", Message: "m", Priority: "info", Source: "backup"})

	// Mark one as read.
	st.MarkRead(id3, "test")

	// List unread.
	results, err := st.List(model.ListFilter{Status: "unread"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("unread list = %d, want 2", len(results))
	}

	// List by source.
	results, err = st.List(model.ListFilter{Source: "backup"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("source backup list = %d, want 2", len(results))
	}

	// List by priority.
	results, err = st.List(model.ListFilter{Priority: "critical"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("critical list = %d, want 1", len(results))
	}

	// List with limit.
	results, err = st.List(model.ListFilter{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("limit 1 list = %d, want 1", len(results))
	}
}

func TestDoubleMarkRead(t *testing.T) {
	st := tempStore(t)

	id, _ := st.Create(&model.Notification{
		Title:    "Test",
		Message:  "msg",
		Priority: "info",
	})

	st.MarkRead(id, "alice")
	st.MarkRead(id, "bob") // Should not decrement unread below 0.

	count, _ := st.Count("unread")
	if count != 0 {
		t.Errorf("unread = %d, want 0", count)
	}
}
