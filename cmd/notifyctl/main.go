package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcmd := os.Args[1]
	args := os.Args[2:]

	switch subcmd {
	case "version", "--version":
		fmt.Printf("notifyctl %s (commit: %s, built: %s)\n", version, commit, date)
		return
	case "count":
		cmdCount(args)
	case "list":
		cmdList(args)
	case "mark":
		cmdMark(args)
	case "delete":
		cmdDelete(args)
	case "follow":
		cmdFollow(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", subcmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `usage: notifyctl <command> [options]

commands:
  count   - count notifications
  list    - list notifications
  mark    - mark notification(s) read/unread
  delete  - delete a notification
  follow  - follow new notifications (live)`)
}

func cmdCount(args []string) {
	fs := flag.NewFlagSet("count", flag.ExitOnError)
	status := fs.String("status", "unread", "filter: unread, read, all")
	format := fs.String("format", "text", "output format: text, short, json")
	socketPath := fs.String("socket", "/var/run/notify.sock", "socket path")
	fs.Parse(args)

	resp, err := socketCall(*socketPath, map[string]interface{}{
		"method": "count",
		"params": map[string]interface{}{
			"status": *status,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if errMsg := getError(resp); errMsg != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", errMsg)
		os.Exit(1)
	}

	result := resp["result"]
	switch *format {
	case "json":
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
	case "short":
		if m, ok := result.(map[string]interface{}); ok {
			fmt.Printf("%.0f\n", m["count"])
		}
	default:
		if m, ok := result.(map[string]interface{}); ok {
			fmt.Printf("%.0f\n", m["count"])
		}
	}
}

func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	status := fs.String("status", "all", "filter: unread, read, all")
	limit := fs.Int("limit", 10, "max results")
	offset := fs.Int("offset", 0, "pagination offset")
	source := fs.String("source", "", "filter by source")
	priority := fs.String("priority", "", "filter by priority")
	format := fs.String("format", "table", "output format: json, table, short")
	socketPath := fs.String("socket", "/var/run/notify.sock", "socket path")
	fs.Parse(args)

	resp, err := socketCall(*socketPath, map[string]interface{}{
		"method": "list",
		"params": map[string]interface{}{
			"status":   *status,
			"limit":    *limit,
			"offset":   *offset,
			"source":   *source,
			"priority": *priority,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if errMsg := getError(resp); errMsg != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", errMsg)
		os.Exit(1)
	}

	result := resp["result"]
	switch *format {
	case "json":
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	case "short":
		if items, ok := result.([]interface{}); ok {
			for _, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					readStatus := "●"
					if read, ok := m["read"].(bool); ok && read {
						readStatus = "○"
					}
					fmt.Printf("%s [%s] %s: %s\n", readStatus, m["priority"], m["title"], m["message"])
				}
			}
		}
	default: // table
		if items, ok := result.([]interface{}); ok {
			if len(items) == 0 {
				fmt.Println("No notifications found.")
				return
			}
			fmt.Printf("%-8s %-36s %-10s %-20s %s\n", "STATUS", "ID", "PRIORITY", "TITLE", "MESSAGE")
			fmt.Println("-------- ------------------------------------ ---------- -------------------- --------------------")
			for _, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					readStatus := "unread"
					if read, ok := m["read"].(bool); ok && read {
						readStatus = "read"
					}
					title := truncate(fmt.Sprintf("%v", m["title"]), 20)
					msg := truncate(fmt.Sprintf("%v", m["message"]), 40)
					fmt.Printf("%-8s %-36s %-10s %-20s %s\n",
						readStatus, m["id"], m["priority"], title, msg)
				}
			}
		}
	}
}

func cmdMark(args []string) {
	fs := flag.NewFlagSet("mark", flag.ExitOnError)
	id := fs.String("id", "", "notification ID")
	all := fs.Bool("all", false, "mark all notifications")
	read := fs.Bool("read", false, "mark as read")
	unread := fs.Bool("unread", false, "mark as unread")
	readBy := fs.String("read-by", "", "who is marking (username)")
	socketPath := fs.String("socket", "/var/run/notify.sock", "socket path")
	fs.Parse(args)

	if !*read && !*unread {
		fmt.Fprintln(os.Stderr, "specify --read or --unread")
		os.Exit(1)
	}

	if *all && *read {
		resp, err := socketCall(*socketPath, map[string]interface{}{
			"method": "mark_all_read",
			"params": map[string]interface{}{
				"read_by": *readBy,
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if errMsg := getError(resp); errMsg != "" {
			fmt.Fprintf(os.Stderr, "error: %s\n", errMsg)
			os.Exit(1)
		}
		fmt.Println("All notifications marked as read.")
		return
	}

	if *id == "" {
		fmt.Fprintln(os.Stderr, "specify --id or --all")
		os.Exit(1)
	}

	var method string
	params := map[string]interface{}{"id": *id}
	if *read {
		method = "mark_read"
		params["read_by"] = *readBy
	} else {
		method = "mark_unread"
	}

	resp, err := socketCall(*socketPath, map[string]interface{}{
		"method": method,
		"params": params,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if errMsg := getError(resp); errMsg != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", errMsg)
		os.Exit(1)
	}
	fmt.Println("ok")
}

func cmdDelete(args []string) {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	id := fs.String("id", "", "notification ID")
	socketPath := fs.String("socket", "/var/run/notify.sock", "socket path")
	fs.Parse(args)

	if *id == "" {
		fmt.Fprintln(os.Stderr, "specify --id")
		os.Exit(1)
	}

	resp, err := socketCall(*socketPath, map[string]interface{}{
		"method": "delete",
		"params": map[string]interface{}{
			"id": *id,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if errMsg := getError(resp); errMsg != "" {
		fmt.Fprintf(os.Stderr, "error: %s\n", errMsg)
		os.Exit(1)
	}
	fmt.Println("deleted")
}

func cmdFollow(args []string) {
	fs := flag.NewFlagSet("follow", flag.ExitOnError)
	socketPath := fs.String("socket", "/var/run/notify.sock", "socket path")
	fs.Parse(args)

	fmt.Println("Following new notifications... (Ctrl+C to stop)")

	var lastCount float64 = -1
	for {
		resp, err := socketCall(*socketPath, map[string]interface{}{
			"method": "count",
			"params": map[string]interface{}{
				"status": "all",
			},
		})
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if result, ok := resp["result"].(map[string]interface{}); ok {
			count, _ := result["count"].(float64)
			if count != lastCount && lastCount >= 0 {
				// Fetch latest notification
				listResp, err := socketCall(*socketPath, map[string]interface{}{
					"method": "list",
					"params": map[string]interface{}{
						"status": "all",
						"limit":  1,
					},
				})
				if err == nil {
					if items, ok := listResp["result"].([]interface{}); ok && len(items) > 0 {
						if m, ok := items[0].(map[string]interface{}); ok {
							fmt.Printf("[%s] %s: %s\n", m["priority"], m["title"], m["message"])
						}
					}
				}
			}
			lastCount = count
		}
		time.Sleep(2 * time.Second)
	}
}

// --- helpers ---

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

	buf := make([]byte, 256*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return resp, nil
}

func getError(resp map[string]interface{}) string {
	if e, ok := resp["error"]; ok && e != nil {
		if s, ok := e.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
