package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/iSundram/notify/internal/model"
	"github.com/iSundram/notify/internal/store"
)

// SocketServer handles newline-delimited JSON RPC over a Unix socket.
type SocketServer struct {
	store    store.Store
	listener net.Listener
	path     string
}

type socketRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type socketResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// NewSocketServer creates a Unix domain socket server.
func NewSocketServer(s store.Store, path string) (*SocketServer, error) {
	// Remove any stale socket file.
	os.Remove(path)

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen unix %s: %w", path, err)
	}

	// Set socket permissions: owner + group read/write.
	if err := os.Chmod(path, 0660); err != nil {
		ln.Close()
		return nil, fmt.Errorf("chmod socket: %w", err)
	}

	return &SocketServer{store: s, listener: ln, path: path}, nil
}

// Serve starts accepting connections. Blocks until listener is closed.
func (ss *SocketServer) Serve() error {
	for {
		conn, err := ss.listener.Accept()
		if err != nil {
			return err // listener closed
		}
		go ss.handleConn(conn)
	}
}

// Close shuts down the socket server and removes the socket file.
func (ss *SocketServer) Close() error {
	err := ss.listener.Close()
	os.Remove(ss.path)
	return err
}

func (ss *SocketServer) handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req socketRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeSocketResponse(conn, socketResponse{Error: "invalid JSON"})
			continue
		}

		resp := ss.dispatch(req)
		writeSocketResponse(conn, resp)
	}
}

func (ss *SocketServer) dispatch(req socketRequest) socketResponse {
	switch req.Method {
	case "notify":
		return ss.handleNotify(req.Params)
	case "count":
		return ss.handleSocketCount(req.Params)
	case "list":
		return ss.handleSocketList(req.Params)
	case "mark_read":
		return ss.handleSocketMarkRead(req.Params)
	case "mark_unread":
		return ss.handleSocketMarkUnread(req.Params)
	case "mark_all_read":
		return ss.handleSocketMarkAllRead(req.Params)
	case "delete":
		return ss.handleSocketDelete(req.Params)
	default:
		return socketResponse{Error: "unknown method: " + req.Method}
	}
}

func (ss *SocketServer) handleNotify(params json.RawMessage) socketResponse {
	var p struct {
		Title     string   `json:"title"`
		Message   string   `json:"message"`
		Priority  string   `json:"priority"`
		Source    string   `json:"source"`
		Tags      []string `json:"tags"`
		ExpiresAt *string  `json:"expires_at"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return socketResponse{Error: "invalid params: " + err.Error()}
	}

	if p.Priority == "" {
		p.Priority = "info"
	}

	n := &model.Notification{
		Title:     sanitize(p.Title),
		Message:   sanitize(p.Message),
		Priority:  p.Priority,
		Source:    sanitize(p.Source),
		Tags:      p.Tags,
		Timestamp: time.Now().UTC(),
	}

	if p.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *p.ExpiresAt)
		if err != nil {
			return socketResponse{Error: "invalid expires_at"}
		}
		n.ExpiresAt = &t
	}

	id, err := ss.store.Create(n)
	if err != nil {
		log.Printf("ERROR socket notify: %v", err)
		return socketResponse{Error: "create failed"}
	}

	return socketResponse{Result: map[string]string{"id": id}}
}

func (ss *SocketServer) handleSocketCount(params json.RawMessage) socketResponse {
	var p struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return socketResponse{Error: "invalid params"}
	}
	if p.Status == "" {
		p.Status = "unread"
	}

	count, err := ss.store.Count(p.Status)
	if err != nil {
		return socketResponse{Error: "count failed"}
	}

	return socketResponse{Result: map[string]int{"count": count}}
}

func (ss *SocketServer) handleSocketList(params json.RawMessage) socketResponse {
	var p struct {
		Status   string `json:"status"`
		Limit    int    `json:"limit"`
		Offset   int    `json:"offset"`
		Source   string `json:"source"`
		Priority string `json:"priority"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return socketResponse{Error: "invalid params"}
	}
	if p.Status == "" {
		p.Status = "all"
	}
	if p.Limit <= 0 {
		p.Limit = 50
	}

	results, err := ss.store.List(model.ListFilter{
		Status:   p.Status,
		Limit:    p.Limit,
		Offset:   p.Offset,
		Source:   p.Source,
		Priority: p.Priority,
	})
	if err != nil {
		return socketResponse{Error: "list failed"}
	}
	if results == nil {
		results = []model.Notification{}
	}

	return socketResponse{Result: results}
}

func (ss *SocketServer) handleSocketMarkRead(params json.RawMessage) socketResponse {
	var p struct {
		ID     string `json:"id"`
		ReadBy string `json:"read_by"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return socketResponse{Error: "invalid params"}
	}

	if err := ss.store.MarkRead(p.ID, p.ReadBy); err != nil {
		return socketResponse{Error: "mark read failed: " + err.Error()}
	}

	return socketResponse{Result: map[string]string{"status": "ok"}}
}

func (ss *SocketServer) handleSocketMarkUnread(params json.RawMessage) socketResponse {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return socketResponse{Error: "invalid params"}
	}

	if err := ss.store.MarkUnread(p.ID); err != nil {
		return socketResponse{Error: "mark unread failed: " + err.Error()}
	}

	return socketResponse{Result: map[string]string{"status": "ok"}}
}

func (ss *SocketServer) handleSocketMarkAllRead(params json.RawMessage) socketResponse {
	var p struct {
		ReadBy string `json:"read_by"`
	}
	// Params are optional for mark_all_read; ignore parse errors.
	_ = json.Unmarshal(params, &p)

	if err := ss.store.MarkAllRead(p.ReadBy); err != nil {
		return socketResponse{Error: "mark all read failed"}
	}

	return socketResponse{Result: map[string]string{"status": "ok"}}
}

func (ss *SocketServer) handleSocketDelete(params json.RawMessage) socketResponse {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return socketResponse{Error: "invalid params"}
	}

	if err := ss.store.Delete(p.ID); err != nil {
		return socketResponse{Error: "delete failed: " + err.Error()}
	}

	return socketResponse{Result: map[string]string{"status": "ok"}}
}

func writeSocketResponse(conn net.Conn, resp socketResponse) {
	data, _ := json.Marshal(resp)
	data = append(data, '\n')
	conn.Write(data)
}
