package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/iSundram/notify/internal/event"
	"github.com/iSundram/notify/internal/model"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// Colors
	purple = lipgloss.Color("170")
	blue   = lipgloss.Color("39")
	green  = lipgloss.Color("76")
	amber  = lipgloss.Color("214")
	red    = lipgloss.Color("196")
	gray   = lipgloss.Color("240")

	// Styles
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(purple).
			Padding(0, 1).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(gray).
			Padding(0, 1).
			MarginLeft(1)

	docStyle = lipgloss.NewStyle().Margin(1, 2)

	listStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(1)

	detailStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(1, 2).
			MarginLeft(1)

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(purple)
	
	priorityStyles = map[string]lipgloss.Style{
		"info":     lipgloss.NewStyle().Foreground(blue),
		"success":  lipgloss.NewStyle().Foreground(green),
		"warning":  lipgloss.NewStyle().Foreground(amber),
		"critical": lipgloss.NewStyle().Foreground(red).Bold(true),
	}
)

// Keymap for custom actions
type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	MarkRead    key.Binding
	MarkAllRead key.Binding
	Delete      key.Binding
	Filter      key.Binding
	Quit        key.Binding
}
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.MarkRead, k.MarkAllRead, k.Delete, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.MarkRead, k.MarkAllRead},
		{k.Filter, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	MarkRead: key.NewBinding(
		key.WithKeys("r", " "),
		key.WithHelp("r/space", "read"),
	),
	MarkAllRead: key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "all read"),
	),
	Delete: key.NewBinding(
		key.WithKeys("x", "delete"),
		key.WithHelp("x", "delete"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

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
	case "dashboard":
		cmdDashboard(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", subcmd)
		printUsage()
		os.Exit(1)
	}
}

func defaultSocketPath() string {
	if socket := os.Getenv("NOTIFY_SOCKET"); socket != "" {
		return socket
	}
	return "/run/notify/notify.sock"
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `usage: notifyctl <command> [options]

commands:
  count     - count notifications
  list      - list notifications
  mark      - mark notification(s) read/unread
  delete    - delete a notification
  follow    - follow new notifications (live)
  dashboard - interactive terminal dashboard`)
}

func cmdCount(args []string) {
	fs := flag.NewFlagSet("count", flag.ExitOnError)
	status := fs.String("status", "unread", "filter: unread, read, all")
	format := fs.String("format", "text", "output format: text, short, json")
	socketPath := fs.String("socket", defaultSocketPath(), "socket path")
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
		data, err := json.Marshal(result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: marshal json: %v\n", err)
			os.Exit(1)
		}
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
	socketPath := fs.String("socket", defaultSocketPath(), "socket path")
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
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: marshal json: %v\n", err)
			os.Exit(1)
		}
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
	socketPath := fs.String("socket", defaultSocketPath(), "socket path")
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
	socketPath := fs.String("socket", defaultSocketPath(), "socket path")
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
	socketPath := fs.String("socket", defaultSocketPath(), "socket path")
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

// --- TUI Dashboard ---

func cmdDashboard(args []string) {
	fs := flag.NewFlagSet("dashboard", flag.ExitOnError)
	socketPath := fs.String("socket", defaultSocketPath(), "socket path")
	fs.Parse(args)

	l := list.New([]list.Item{}, itemDelegate{}, 0, 0)
	l.Title = "Notifications"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = headerStyle

	m := modelTUI{
		list:       l,
		socketPath: *socketPath,
		help:       help.New(),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	go watchEvents(*socketPath, p)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running dashboard: %v", err)
		os.Exit(1)
	}
}

type item struct {
	n model.Notification
}

func (i item) Title() string       { return i.n.Title }
func (i item) Description() string { return i.n.Message }
func (i item) FilterValue() string { return i.n.Title + " " + i.n.Source + " " + i.n.Message }

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	fn := lipgloss.NewStyle().PaddingLeft(4).Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(purple).
				Render("> " + strings.Join(s, " "))
		}
	}

	pSym := "●"
	pStyle, ok := priorityStyles[i.n.Priority]
	if !ok {
		pStyle = priorityStyles["info"]
	}

	readSym := " "
	title := i.n.Title
	if !i.n.Read {
		readSym = "•"
		title = lipgloss.NewStyle().Bold(true).Render(title)
	} else {
		title = lipgloss.NewStyle().Foreground(gray).Render(title)
	}

	fmt.Fprint(w, fn(fmt.Sprintf("%s %s %-35s %s", 
		readSym, 
		pStyle.Render(pSym), 
		truncate(title, 35), 
		lipgloss.NewStyle().Foreground(gray).Render(i.n.Source))))
}

type modelTUI struct {
	list         list.Model
	socketPath   string
	err          error
	lastSelected *model.Notification
	width        int
	height       int
	help         help.Model
	ready        bool
}

type eventMsg event.Event
type errMsg struct{ err error }

func (m modelTUI) Init() tea.Cmd {
	return m.fetchInitial()
}

func watchEvents(socketPath string, p *tea.Program) {
	for {
		conn, err := net.Dial("unix", socketPath)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		req := map[string]string{"method": "watch"}
		json.NewEncoder(conn).Encode(req)
		conn.Write([]byte("\n"))

		scanner := bufio.NewScanner(conn)
		scanner.Scan() // Skip "watching" response

		for scanner.Scan() {
			var resp struct {
				Result event.Event `json:"result"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				continue
			}
			p.Send(eventMsg(resp.Result))
		}
		conn.Close()
		time.Sleep(time.Second)
	}
}

func (m modelTUI) fetchInitial() tea.Cmd {
	return func() tea.Msg {
		resp, err := socketCall(m.socketPath, map[string]interface{}{
			"method": "list",
			"params": map[string]interface{}{"limit": 100},
		})
		if err != nil {
			return errMsg{err}
		}
		if result, ok := resp["result"].([]interface{}); ok {
			var items []list.Item
			for _, r := range result {
				data, _ := json.Marshal(r)
				var n model.Notification
				json.Unmarshal(data, &n)
				items = append(items, item{n: n})
			}
			return initialItemsMsg(items)
		}
		return nil
	}
}

type initialItemsMsg []list.Item

func (m modelTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, keys.MarkRead):
			if i, ok := m.list.SelectedItem().(item); ok {
				if i.n.Read {
					go m.markUnread(i.n.ID)
				} else {
					go m.markRead(i.n.ID)
				}
			}
		case key.Matches(msg, keys.MarkAllRead):
			go m.markAllRead()
		case key.Matches(msg, keys.Delete):
			if i, ok := m.list.SelectedItem().(item); ok {
				go m.deleteNotification(i.n.ID)
			}
		case msg.String() == "q" || msg.String() == "ctrl+c":
			return m, tea.Quit
		}

	case initialItemsMsg:
		m.list.SetItems(msg)

	case eventMsg:
		switch msg.Type {
		case event.EventCreated:
			m.list.InsertItem(0, item{n: *msg.Notification})
		case event.EventDeleted:
			for i, it := range m.list.Items() {
				if it.(item).n.ID == msg.ID {
					m.list.RemoveItem(i)
					break
				}
			}
		case event.EventMarkedRead, event.EventMarkedUnread:
			for i, it := range m.list.Items() {
				if it.(item).n.ID == msg.ID {
					n := it.(item).n
					n.Read = (msg.Type == event.EventMarkedRead)
					m.list.SetItem(i, item{n: n})
					// Update lastSelected if it's the one we just marked
					if m.lastSelected != nil && m.lastSelected.ID == n.ID {
						m.lastSelected = &n
					}
					break
				}
			}
		case event.EventMarkedAllRead:
			for i, it := range m.list.Items() {
				n := it.(item).n
				n.Read = true
				m.list.SetItem(i, item{n: n})
			}
		}

	case errMsg:
		m.err = msg.err

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	if i, ok := m.list.SelectedItem().(item); ok {
		m.lastSelected = &i.n
	} else {
		m.lastSelected = nil
	}

	return m, tea.Batch(cmds...)
}

func (m modelTUI) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.", m.err)
	}

	if !m.ready {
		return "\n  Initializing..."
	}

	// Header
	unreadCount := 0
	for _, it := range m.list.Items() {
		if !it.(item).n.Read {
			unreadCount++
		}
	}
	
	header := headerStyle.Render(" NOTIFY ")
	status := statusStyle.Render(fmt.Sprintf("%d Unread", unreadCount))
	topBar := lipgloss.JoinHorizontal(lipgloss.Center, header, status)

	// Two-pane Layout
	var mainView string
	
	// Determine if we have enough width for two panes
	useTwoPanes := m.width > 100

	if useTwoPanes {
		listWidth := (m.width * 6) / 10
		detailWidth := m.width - listWidth - 10

		m.list.SetSize(listWidth, m.height-10)
		lView := listStyle.Width(listWidth).Height(m.height - 8).Render(m.list.View())

		var rView string
		if m.lastSelected != nil {
			rView = m.renderDetailPane(detailWidth)
		} else {
			rView = detailStyle.Width(detailWidth).Height(m.height - 8).Render("\n  No notification selected.")
		}
		
		mainView = lipgloss.JoinHorizontal(lipgloss.Top, lView, rView)
	} else {
		m.list.SetSize(m.width-6, m.height-8)
		mainView = listStyle.Width(m.width - 4).Height(m.height - 6).Render(m.list.View())
	}

	// Footer
	helpView := m.help.View(keys)
	footer := lipgloss.NewStyle().MarginTop(1).Render(helpView)

	return docStyle.Render(lipgloss.JoinVertical(lipgloss.Left, topBar, mainView, footer))
}

func (m modelTUI) renderDetailPane(width int) string {
	n := m.lastSelected
	
	pStyle := priorityStyles[n.Priority]
	
	title := titleStyle.Render(n.Title)
	meta := fmt.Sprintf("%s • %s", 
		pStyle.Bold(true).Render(strings.ToUpper(n.Priority)),
		lipgloss.NewStyle().Foreground(gray).Render(n.Timestamp.Local().Format("Jan 02, 15:04:05")),
	)
	
	source := lipgloss.NewStyle().Italic(true).Foreground(gray).Render("Source: " + n.Source)
	
	body := lipgloss.NewStyle().
		Width(width - 4).
		MarginTop(1).
		Render(n.Message)

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		meta,
		source,
		"",
		body,
	)

	return detailStyle.Width(width).Height(m.height - 8).Render(content)
}

func (m modelTUI) markRead(id string) {
	socketCall(m.socketPath, map[string]interface{}{
		"method": "mark_read",
		"params": map[string]interface{}{"id": id, "read_by": "dashboard"},
	})
}

func (m modelTUI) markUnread(id string) {
	socketCall(m.socketPath, map[string]interface{}{
		"method": "mark_unread",
		"params": map[string]interface{}{"id": id},
	})
}

func (m modelTUI) markAllRead() {
	socketCall(m.socketPath, map[string]interface{}{
		"method": "mark_all_read",
		"params": map[string]interface{}{"read_by": "dashboard"},
	})
}

func (m modelTUI) deleteNotification(id string) {
	socketCall(m.socketPath, map[string]interface{}{
		"method": "delete",
		"params": map[string]interface{}{"id": id},
	})
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

func getError(resp map[string]interface{}) string {
	if errMsg, ok := resp["error"].(string); ok && errMsg != "" {
		return errMsg
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
