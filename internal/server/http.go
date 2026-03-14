package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iSundram/notify/internal/model"
	"github.com/iSundram/notify/internal/store"
)

// HTTPServer wraps the notification store with HTTP handlers.
type HTTPServer struct {
	store store.Store
	mux   *http.ServeMux
}

// NewHTTPServer creates a new HTTP server with routes.
func NewHTTPServer(s store.Store) *HTTPServer {
	srv := &HTTPServer{store: s, mux: http.NewServeMux()}
	srv.routes()
	return srv
}

// Handler returns the http.Handler for use with http.ListenAndServe.
func (s *HTTPServer) Handler() http.Handler {
	return s.mux
}

func (s *HTTPServer) routes() {
	s.mux.HandleFunc("POST /notify", s.handleCreate)
	s.mux.HandleFunc("GET /notifications", s.handleList)
	s.mux.HandleFunc("GET /notifications/count", s.handleCount)
	s.mux.HandleFunc("POST /notifications/{id}/read", s.handleMarkRead)
	s.mux.HandleFunc("POST /notifications/{id}/unread", s.handleMarkUnread)
	s.mux.HandleFunc("DELETE /notifications/{id}", s.handleDelete)
}

func (s *HTTPServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req model.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	if req.Priority == "" {
		req.Priority = "info"
	}
	if !model.ValidPriorities[req.Priority] {
		writeError(w, http.StatusBadRequest, "invalid priority: "+req.Priority)
		return
	}

	n := &model.Notification{
		Title:     sanitize(req.Title),
		Message:   sanitize(req.Message),
		Priority:  req.Priority,
		Source:    sanitize(req.Source),
		Tags:      req.Tags,
		Timestamp: time.Now().UTC(),
	}

	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid expires_at: "+err.Error())
			return
		}
		n.ExpiresAt = &t
	}

	id, err := s.store.Create(n)
	if err != nil {
		log.Printf("ERROR create notification: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create notification")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (s *HTTPServer) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	if status == "" {
		status = "all"
	}
	if !model.ValidStatuses[status] {
		writeError(w, http.StatusBadRequest, "invalid status: "+status)
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	filter := model.ListFilter{
		Status:   status,
		Limit:    limit,
		Offset:   offset,
		Source:   q.Get("source"),
		Priority: q.Get("priority"),
	}

	if filter.Priority != "" && !model.ValidPriorities[filter.Priority] {
		writeError(w, http.StatusBadRequest, "invalid priority: "+filter.Priority)
		return
	}

	results, err := s.store.List(filter)
	if err != nil {
		log.Printf("ERROR list notifications: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list notifications")
		return
	}

	if results == nil {
		results = []model.Notification{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *HTTPServer) handleCount(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "unread"
	}
	if !model.ValidStatuses[status] {
		writeError(w, http.StatusBadRequest, "invalid status: "+status)
		return
	}

	count, err := s.store.Count(status)
	if err != nil {
		log.Printf("ERROR count notifications: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to count notifications")
		return
	}

	writeJSON(w, http.StatusOK, model.CountResult{Count: count})
}

func (s *HTTPServer) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var req model.MarkRequest
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}

	if err := s.store.MarkRead(id, req.ReadBy); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "notification not found")
			return
		}
		log.Printf("ERROR mark read: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to mark as read")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *HTTPServer) handleMarkUnread(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := s.store.MarkUnread(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "notification not found")
			return
		}
		log.Printf("ERROR mark unread: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to mark as unread")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *HTTPServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := s.store.Delete(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "notification not found")
			return
		}
		log.Printf("ERROR delete: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete notification")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// sanitize strips control characters from input strings.
func sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if len(result) > 10000 {
		return result[:10000]
	}
	return result
}
