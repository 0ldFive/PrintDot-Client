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

	// Connection tracking
	clientCount   int
	countMu       sync.Mutex
	onCountChange func(int) // Callback to update frontend
	conns         map[*websocket.Conn]bool

	// Restart callback
	onRestart       func()
	onReload        func()
	onClientConnect func(string)
}

func NewBridge() *Bridge {
	return &Bridge{
		port:  "1122",
		key:   "",
		logs:  make([]string, 0),
		conns: make(map[*websocket.Conn]bool),
	}
}

func (b *Bridge) SetLogger(logger func(string)) {
	b.log = logger
}

func (b *Bridge) SetCountCallback(cb func(int)) {
	b.onCountChange = cb
}

func (b *Bridge) SetRestartCallback(cb func()) {
	b.onRestart = cb
}

func (b *Bridge) SetReloadCallback(cb func()) {
	b.onReload = cb
}

func (b *Bridge) SetClientConnectCallback(cb func(string)) {
	b.onClientConnect = cb
}

func (b *Bridge) updateClientCount(delta int) {
	b.countMu.Lock()
	b.clientCount += delta
	count := b.clientCount
	b.countMu.Unlock()

	if b.onCountChange != nil {
		b.onCountChange(count)
	}
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

	// Close all active connections first
	b.countMu.Lock()
	for conn := range b.conns {
		conn.Close()
	}
	// Clear the map
	b.conns = make(map[*websocket.Conn]bool)
	b.countMu.Unlock()

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
	Printer     string `json:"printer"`
	Content     string `json:"content"` // Raw string or base64? Let's assume string for now
	Key         string `json:"key"`
	JobName     string `json:"jobName"`
	Copies      int    `json:"copies"`      // Number of copies
	Orientation string `json:"orientation"` // "portrait" or "landscape" (requires driver support/GDI)
	DPI         int    `json:"dpi"`         // Print DPI (requires driver support/GDI)
	JobInterval int    `json:"jobInterval"` // Delay between copies in ms (for "interleaved" manual handling)
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (b *Bridge) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authentication check
	if b.key != "" {
		pass := r.URL.Query().Get("key")
		if pass == "" {
			pass = r.URL.Query().Get("password")
		}

		if pass != b.key {
			b.Log(fmt.Sprintf("Authentication failed for %s", r.RemoteAddr))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		b.Log(fmt.Sprintf("Upgrade error: %v", err))
		return
	}
	defer c.Close()

	b.countMu.Lock()
	b.conns[c] = true
	b.countMu.Unlock()
	b.updateClientCount(1)
	defer func() {
		b.countMu.Lock()
		delete(b.conns, c)
		b.countMu.Unlock()
		b.updateClientCount(-1)
	}()

	b.Log(fmt.Sprintf("Client connected from %s", c.RemoteAddr()))

	if b.onClientConnect != nil {
		b.onClientConnect(c.RemoteAddr().String())
	}

	for {
		var req PrintRequest
		err := c.ReadJSON(&req)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				b.Log(fmt.Sprintf("Client disconnected: %v", err))
			} else {
				// Normal closure or other error
			}
			break
		}

		// Handle print request
		// b.Log(fmt.Sprintf("Received print job for %s: %s...", req.Printer, req.JobName))

		// If copies > 1, loop and print
		if req.Copies <= 0 {
			req.Copies = 1
		}

		successCount := 0
		var lastErr error

		for i := 0; i < req.Copies; i++ {
			// If interval is set, wait before next copy (but not before first)
			if i > 0 && req.JobInterval > 0 {
				time.Sleep(time.Duration(req.JobInterval) * time.Millisecond)
			}

			// Note: Orientation and DPI are currently ignored for RAW printing as they usually
			// need to be embedded in the RAW commands (ZPL/EPL) or require GDI printing.
			// We expose them in the struct for future GDI implementation or driver manipulation.
			if req.Orientation != "" || req.DPI > 0 {
				// b.Log("Warning: Orientation/DPI settings are ignored in RAW mode. Please set them in your printer commands.")
			}

			err = b.printRaw(req.Printer, req.JobName, []byte(req.Content))
			if err != nil {
				lastErr = err
				b.Log(fmt.Sprintf("Print error (copy %d): %v", i+1, err))
				// Continue trying other copies? Or stop? Let's stop on error to avoid waste.
				break
			} else {
				successCount++
			}
		}

		if successCount == req.Copies {
			b.Log("Print success")
			c.WriteJSON(Response{Status: "success", Message: "Printed successfully"})
		} else {
			msg := fmt.Sprintf("Printed %d/%d copies. Error: %v", successCount, req.Copies, lastErr)
			b.Log(msg)
			c.WriteJSON(Response{Status: "error", Message: msg})
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
	mux.HandleFunc("/api/logs", b.handleLogsJSON)
	mux.HandleFunc("/api/logs/clear", b.handleClearLogs)
	mux.HandleFunc("/api/restart", b.handleRestartRequest)
	mux.HandleFunc("/api/reload", b.handleReloadRequest)

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

func (b *Bridge) StopLogServer() error {
	if b.logServer == nil {
		return nil
	}
	if err := b.logServer.Shutdown(context.Background()); err != nil {
		return err
	}
	b.logServer = nil
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
	<title>System Logs - PrintDot Client</title>
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

func (b *Bridge) handleLogsJSON(w http.ResponseWriter, r *http.Request) {
	b.logsMu.Lock()
	// Copy logs
	currentLogs := make([]string, len(b.logs))
	copy(currentLogs, b.logs)
	b.logsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // Allow child process to fetch
	json.NewEncoder(w).Encode(currentLogs)
}

func (b *Bridge) handleClearLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	b.logsMu.Lock()
	b.logs = []string{}
	b.logsMu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (b *Bridge) handleReloadRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Reload initiated"))

	// Trigger reload on main thread
	if b.onReload != nil {
		go func() {
			time.Sleep(100 * time.Millisecond)
			b.onReload()
		}()
	}
}
