package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	title := flag.String("title", "", "notification title")
	message := flag.String("message", "", "notification message")
	priority := flag.String("priority", "info", "priority: info, success, warning, critical")
	source := flag.String("source", "", "notification source")
	socketPath := flag.String("socket", "/var/run/notify.sock", "path to notifyd socket")
	flag.Parse()

	if *title == "" || *message == "" {
		fmt.Fprintln(os.Stderr, "usage: notify --title TITLE --message MESSAGE [--priority PRIORITY] [--source SOURCE]")
		os.Exit(1)
	}

	req := map[string]interface{}{
		"method": "notify",
		"params": map[string]interface{}{
			"title":    *title,
			"message":  *message,
			"priority": *priority,
			"source":   *source,
		},
	}

	resp, err := socketCall(*socketPath, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if errMsg, ok := resp["error"]; ok && errMsg != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", errMsg)
		os.Exit(1)
	}

	if result, ok := resp["result"]; ok {
		if m, ok := result.(map[string]interface{}); ok {
			if id, ok := m["id"]; ok {
				fmt.Println(id)
			}
		}
	}
}

func socketCall(path string, req interface{}) (map[string]interface{}, error) {
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", path, err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	buf := make([]byte, 64*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp, nil
}
