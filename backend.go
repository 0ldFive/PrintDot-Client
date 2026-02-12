package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	return b.getPrintersPlatform()
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
	Printer string `json:"printer"`
	Content string `json:"content"` // Base64 encoded PDF
	Key     string `json:"key"`

	Job struct {
		Name       string `json:"name"`
		Copies     int    `json:"copies"`
		IntervalMs int    `json:"intervalMs"`
	} `json:"job"`
	Pages struct {
		Range string `json:"range"`
		Set   string `json:"set"`
	} `json:"pages"`
	Layout struct {
		Scale       string `json:"scale"`
		Orientation string `json:"orientation"`
	} `json:"layout"`
	Color struct {
		Mode string `json:"mode"`
	} `json:"color"`
	Sides struct {
		Mode string `json:"mode"`
	} `json:"sides"`
	Paper PaperSpec `json:"paper"`
	Tray  struct {
		Bin string `json:"bin"`
	} `json:"tray"`
	Sumatra struct {
		Settings string `json:"settings"`
	} `json:"sumatra"`

	// Legacy fields ignored but kept for compatibility
	JobName       string `json:"jobName"`
	Copies        int    `json:"copies"`      // Number of copies
	JobInterval   int    `json:"jobInterval"` // Delay between copies in ms
	PageRange     string `json:"pageRange"`
	Duplex        string `json:"duplex"`
	ColorMode     string `json:"colorMode"`
	Scale         string `json:"scale"`
	PrintSettings string `json:"printSettings"`
	Orientation   string `json:"orientation"`
	DPI           int    `json:"dpi"`
}

type PaperSpec struct {
	Size string `json:"size"`
}

func (p *PaperSpec) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		p.Size = s
		return nil
	}

	var tmp struct {
		Size string `json:"size"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	p.Size = tmp.Size
	return nil
}

type PrintOptions struct {
	PageRange     string
	PageSet       string
	Duplex        string
	ColorMode     string
	Paper         string
	Scale         string
	Orientation   string
	TrayBin       string
	Copies        int
	PrintSettings string
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

	// Send printer list immediately upon connection
	printers, err := b.GetPrinters()
	if err == nil {
		msg := map[string]interface{}{
			"type": "printer_list",
			"data": printers,
		}
		if jsonBytes, err := json.Marshal(msg); err == nil {
			b.Log(fmt.Sprintf("Sent WS message: %s", string(jsonBytes)))
		}
		c.WriteJSON(msg)
	} else {
		b.Log(fmt.Sprintf("Failed to get printers on connect: %v", err))
	}

	for {
		// Read message as raw JSON map first to check type
		var rawMsg map[string]interface{}
		err := c.ReadJSON(&rawMsg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				b.Log(fmt.Sprintf("Client disconnected: %v", err))
			} else {
				// Normal closure or other error
			}
			break
		}

		if jsonBytes, err := json.Marshal(rawMsg); err == nil {
			b.Log(fmt.Sprintf("Received WS message: %s", string(jsonBytes)))
		}

		// Check message type
		if msgType, ok := rawMsg["type"].(string); ok && msgType == "get_printers" {
			printers, err := b.GetPrinters()
			if err == nil {
				msg := map[string]interface{}{
					"type": "printer_list",
					"data": printers,
				}
				if jsonBytes, err := json.Marshal(msg); err == nil {
					b.Log(fmt.Sprintf("Sent WS message: %s", string(jsonBytes)))
				}
				c.WriteJSON(msg)
			} else {
				resp := Response{Status: "error", Message: "Failed to get printer list"}
				if jsonBytes, err := json.Marshal(resp); err == nil {
					b.Log(fmt.Sprintf("Sent WS message: %s", string(jsonBytes)))
				}
				c.WriteJSON(resp)
			}
			continue
		}

		// Handle as PrintRequest (default)
		jsonBody, _ := json.Marshal(rawMsg)
		var req PrintRequest
		if err := json.Unmarshal(jsonBody, &req); err != nil {
			b.Log(fmt.Sprintf("Invalid print request: %v", err))
			resp := Response{Status: "error", Message: "Invalid request format"}
			c.WriteJSON(resp)
			continue
		}

		jobName := strings.TrimSpace(req.Job.Name)
		if jobName == "" {
			jobName = strings.TrimSpace(req.JobName)
		}

		copies := req.Job.Copies
		if copies <= 0 {
			copies = req.Copies
		}
		if copies <= 0 {
			copies = 1
		}

		intervalMs := req.Job.IntervalMs
		if intervalMs <= 0 {
			intervalMs = req.JobInterval
		}

		// Validate Content is PDF Base64
		contentToDecode := req.Content
		if strings.HasPrefix(contentToDecode, "data:") {
			if idx := strings.Index(contentToDecode, ","); idx != -1 {
				contentToDecode = contentToDecode[idx+1:]
			}
		}
		decoded, err := base64.StdEncoding.DecodeString(contentToDecode)
		if err != nil {
			b.Log("Error decoding Base64 content")
			c.WriteJSON(Response{Status: "error", Message: "Invalid Base64 content"})
			continue
		}

		// Strict PDF check
		if len(decoded) < 4 || string(decoded[0:4]) != "%PDF" {
			b.Log("Content is not a valid PDF (missing %PDF header)")
			c.WriteJSON(Response{Status: "error", Message: "Content must be a PDF file"})
			continue
		}

		options := PrintOptions{
			PageRange:     strings.TrimSpace(firstNonEmpty(req.Pages.Range, req.PageRange)),
			PageSet:       strings.TrimSpace(req.Pages.Set),
			Duplex:        strings.TrimSpace(firstNonEmpty(req.Sides.Mode, req.Duplex)),
			ColorMode:     strings.TrimSpace(firstNonEmpty(req.Color.Mode, req.ColorMode)),
			Paper:         strings.TrimSpace(req.Paper.Size),
			Scale:         strings.TrimSpace(firstNonEmpty(req.Layout.Scale, req.Scale)),
			Orientation:   strings.TrimSpace(firstNonEmpty(req.Layout.Orientation, req.Orientation)),
			TrayBin:       strings.TrimSpace(req.Tray.Bin),
			PrintSettings: strings.TrimSpace(firstNonEmpty(req.Sumatra.Settings, req.PrintSettings)),
		}

		runCount := 1
		perRunCopies := copies
		if intervalMs > 0 {
			runCount = copies
			perRunCopies = 1
		}
		options.Copies = perRunCopies

		successCount := 0
		var lastErr error

		for i := 0; i < runCount; i++ {
			if i > 0 && intervalMs > 0 {
				time.Sleep(time.Duration(intervalMs) * time.Millisecond)
			}

			err = b.printPDF(req.Printer, jobName, decoded, options)
			if err != nil {
				lastErr = err
				b.Log(fmt.Sprintf("Print error (copy %d): %v", i+1, err))
				break
			} else {
				successCount += perRunCopies
			}
		}

		if successCount == copies {
			b.Log("Print success")
			resp := Response{Status: "success", Message: "Printed successfully"}
			c.WriteJSON(resp)
		} else {
			msg := fmt.Sprintf("Printed %d/%d copies. Error: %v", successCount, copies, lastErr)
			b.Log(msg)
			c.WriteJSON(Response{Status: "error", Message: msg})
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (b *Bridge) printPDF(printerName string, jobName string, pdfData []byte, options PrintOptions) error {
	// 1. Write to temp file
	tmpFile, err := ioutil.TempFile("", "print-dot-*.pdf")
	if err != nil {
		return fmt.Errorf("create temp file failed: %v", err)
	}
	defer os.Remove(tmpFile.Name()) // Clean up on exit

	if _, err := tmpFile.Write(pdfData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file failed: %v", err)
	}
	tmpFile.Close()
	absPath, _ := filepath.Abs(tmpFile.Name())

	if jobName != "" {
		b.Log(fmt.Sprintf("Printing job '%s': %s to %s", jobName, absPath, printerName))
	} else {
		b.Log(fmt.Sprintf("Printing PDF file: %s to %s", absPath, printerName))
	}

	return b.printPDFPlatform(printerName, absPath, options)
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
