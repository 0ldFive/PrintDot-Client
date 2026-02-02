package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/alexbrainman/printer"
	"github.com/gorilla/websocket"
)

type Bridge struct {
	server *http.Server
	port   string
	key    string
	mu     sync.Mutex
	log    func(string) // Callback to log to frontend

	// Log related
	logs      []string
	logsMu    sync.Mutex
	logServer *http.Server
	logPort   int
}

func NewBridge() *Bridge {
	return &Bridge{
		port: "1122",
		key:  "",
		logs: make([]string, 0),
	}
}

func (b *Bridge) SetLogger(logger func(string)) {
	b.log = logger
}

func (b *Bridge) Log(msg string) {
	// Store in memory
	b.logsMu.Lock()
	timestamp := time.Now().Format("15:04:05")
	entry := fmt.Sprintf("[%s] %s", timestamp, msg)
	b.logs = append(b.logs, entry)
	if len(b.logs) > 200 {
		b.logs = b.logs[1:]
	}
	b.logsMu.Unlock()

	if b.log != nil {
		b.log(msg)
	} else {
		log.Println(msg)
	}
}

func (b *Bridge) GetPrinters() ([]string, error) {
	names, err := printer.ReadNames()
	if err != nil {
		return nil, err
	}
	return names, nil
}

func (b *Bridge) StartServer(port string, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.server != nil {
		return fmt.Errorf("server already running")
	}

	b.port = port
	b.key = key

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", b.handleWebSocket)

	b.server = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		b.Log(fmt.Sprintf("Starting server on port %s...", port))
		if err := b.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			b.Log(fmt.Sprintf("Server error: %v", err))
		}
		b.Log("Server stopped")
	}()

	return nil
}

func (b *Bridge) StopServer() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.server == nil {
		return nil
	}

	if err := b.server.Shutdown(context.Background()); err != nil {
		return err
	}
	b.server = nil
	return nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

type PrintRequest struct {
	Printer string `json:"printer"`
	Content string `json:"content"` // Raw string or base64? Let's assume string for now
	Key     string `json:"key"`
	JobName string `json:"jobName"`
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (b *Bridge) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		b.Log(fmt.Sprintf("Upgrade error: %v", err))
		return
	}
	defer c.Close()

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			break
		}

		var req PrintRequest
		if err := json.Unmarshal(message, &req); err != nil {
			c.WriteJSON(Response{Status: "error", Message: "Invalid JSON"})
			continue
		}

		// Auth check
		if b.key != "" && req.Key != b.key {
			b.Log("Authentication failed")
			c.WriteJSON(Response{Status: "error", Message: "Invalid Key"})
			continue
		}

		b.Log(fmt.Sprintf("Printing job '%s' to '%s'", req.JobName, req.Printer))
		err = b.printRaw(req.Printer, req.JobName, []byte(req.Content))
		if err != nil {
			b.Log(fmt.Sprintf("Print error: %v", err))
			c.WriteJSON(Response{Status: "error", Message: err.Error()})
		} else {
			b.Log("Print success")
			c.WriteJSON(Response{Status: "success", Message: "Printed successfully"})
		}
	}
}

func (b *Bridge) printRaw(printerName string, jobName string, data []byte) error {
	p, err := printer.Open(printerName)
	if err != nil {
		return err
	}
	defer p.Close()

	if jobName == "" {
		jobName = "Raw Print Job"
	}

	err = p.StartDocument(jobName, "RAW")
	if err != nil {
		return err
	}
	defer p.EndDocument()

	err = p.StartPage()
	if err != nil {
		return err
	}
	defer p.EndPage()

	_, err = p.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func (b *Bridge) StartLogServer() error {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return err
	}
	b.logPort = listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/logs", b.handleLogs)

	b.logServer = &http.Server{
		Handler: mux,
	}

	go func() {
		if err := b.logServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Log server error: %v", err)
		}
	}()

	return nil
}

func (b *Bridge) handleLogs(w http.ResponseWriter, r *http.Request) {
	b.logsMu.Lock()
	// Copy logs to avoid holding lock during template execution
	currentLogs := make([]string, len(b.logs))
	copy(currentLogs, b.logs)
	b.logsMu.Unlock()

	// Reverse logs for display (newest top)
	for i, j := 0, len(currentLogs)-1; i < j; i, j = i+1, j-1 {
		currentLogs[i], currentLogs[j] = currentLogs[j], currentLogs[i]
	}

	html := `<!DOCTYPE html>
<html>
<head>
	<title>System Logs - Print Bridge</title>
	<meta http-equiv="refresh" content="2">
	<style>
		body { 
			font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; 
			background: #ffffff; 
			color: #333; 
			padding: 20px; 
			margin: 0;
		}
		h2 { border-bottom: 2px solid #eee; padding-bottom: 10px; margin-top: 0; color: #444; }
		.log-entry { 
			border-bottom: 1px solid #f0f0f0; 
			padding: 8px 0; 
			font-family: Consolas, 'Courier New', monospace;
			font-size: 13px;
		}
		.log-entry:hover { background-color: #f9f9f9; }
		.empty { color: #999; font-style: italic; padding: 20px 0; }
		.status { font-size: 12px; color: #888; margin-top: 5px; }
	</style>
</head>
<body>
	<h2>System Logs</h2>
	<div class="status">Auto-refreshing every 2 seconds</div>
	<div id="logs">
		{{range .}}
		<div class="log-entry">{{.}}</div>
		{{else}}
		<div class="empty">No logs available.</div>
		{{end}}
	</div>
</body>
</html>`

	t, err := template.New("logs").Parse(html)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	t.Execute(w, currentLogs)
}
