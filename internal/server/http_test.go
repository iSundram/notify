package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/iSundram/notify/internal/store"
)

func testStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	f, err := os.CreateTemp("", "notify-http-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	st, err := store.NewSQLiteStore(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestHTTPCreateAndList(t *testing.T) {
	st := testStore(t)
	srv := NewHTTPServer(st)

	// Create a notification.
	body := `{"title":"Backup","message":"Backup completed","priority":"info","source":"backup","tags":["daily"]}`
	req := httptest.NewRequest("POST", "/notify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("POST /notify = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var created map[string]string
	json.Unmarshal(w.Body.Bytes(), &created)
	if created["id"] == "" {
		t.Fatal("expected id in response")
	}

	// List notifications.
	req = httptest.NewRequest("GET", "/notifications?status=unread", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /notifications = %d, want 200", w.Code)
	}

	var list []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Errorf("list length = %d, want 1", len(list))
	}
}

func TestHTTPCount(t *testing.T) {
	st := testStore(t)
	srv := NewHTTPServer(st)

	// Initially 0 unread.
	req := httptest.NewRequest("GET", "/notifications/count?status=unread", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /notifications/count = %d", w.Code)
	}

	var result map[string]int
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["count"] != 0 {
		t.Errorf("count = %d, want 0", result["count"])
	}
}

func TestHTTPMarkReadUnread(t *testing.T) {
	st := testStore(t)
	srv := NewHTTPServer(st)

	// Create a notification.
	body := `{"title":"Test","message":"msg","priority":"info"}`
	req := httptest.NewRequest("POST", "/notify", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	var created map[string]string
	json.Unmarshal(w.Body.Bytes(), &created)
	id := created["id"]

	// Mark as read.
	req = httptest.NewRequest("POST", "/notifications/"+id+"/read", bytes.NewBufferString(`{"read_by":"alice"}`))
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST mark read = %d", w.Code)
	}

	// Verify count is 0.
	req = httptest.NewRequest("GET", "/notifications/count?status=unread", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	var result map[string]int
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["count"] != 0 {
		t.Errorf("unread count after mark = %d, want 0", result["count"])
	}

	// Mark as unread.
	req = httptest.NewRequest("POST", "/notifications/"+id+"/unread", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST mark unread = %d", w.Code)
	}

	// Verify count is 1 again.
	req = httptest.NewRequest("GET", "/notifications/count?status=unread", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["count"] != 1 {
		t.Errorf("unread count after unmark = %d, want 1", result["count"])
	}
}

func TestHTTPDelete(t *testing.T) {
	st := testStore(t)
	srv := NewHTTPServer(st)

	// Create a notification.
	body := `{"title":"ToDelete","message":"msg","priority":"info"}`
	req := httptest.NewRequest("POST", "/notify", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	var created map[string]string
	json.Unmarshal(w.Body.Bytes(), &created)
	id := created["id"]

	// Delete it.
	req = httptest.NewRequest("DELETE", "/notifications/"+id, nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d, want %d", w.Code, http.StatusNoContent)
	}

	// Verify not found.
	req = httptest.NewRequest("POST", "/notifications/"+id+"/read", nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("mark deleted = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHTTPValidation(t *testing.T) {
	st := testStore(t)
	srv := NewHTTPServer(st)

	tests := []struct {
		name string
		body string
		code int
	}{
		{"empty body", `{}`, http.StatusBadRequest},
		{"missing message", `{"title":"x"}`, http.StatusBadRequest},
		{"invalid priority", `{"title":"x","message":"y","priority":"extreme"}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/notify", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)
			if w.Code != tt.code {
				t.Errorf("%s: code = %d, want %d; body: %s", tt.name, w.Code, tt.code, w.Body.String())
			}
		})
	}
}
