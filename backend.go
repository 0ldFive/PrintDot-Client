package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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
	logFileMu sync.Mutex
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

	// Remote forwarding
	remoteMu                sync.Mutex
	remoteStop              chan struct{}
	remoteWg                sync.WaitGroup
	remoteCfg               RemoteConfig
	remoteConn              *websocket.Conn
	remoteStatus            RemoteForwarderStatus
	forwarderStatusProvider func() RemoteForwarderStatus
	forwarderConnect        func()
	forwarderDisconnect     func()
}

func NewBridge() *Bridge {
	b := &Bridge{
		port:  "1122",
		key:   "",
		conns: make(map[*websocket.Conn]bool),
	}
	b.ensureLogDir()
	return b
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

func (b *Bridge) SetForwarderStatusProvider(cb func() RemoteForwarderStatus) {
	b.forwarderStatusProvider = cb
}

func (b *Bridge) SetForwarderConnectHandler(cb func()) {
	b.forwarderConnect = cb
}

func (b *Bridge) SetForwarderDisconnectHandler(cb func()) {
	b.forwarderDisconnect = cb
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
	timestamp := time.Now().Format("15:04:05")
	entry := fmt.Sprintf("[%s] %s", timestamp, msg)
	if err := b.appendLogLine(entry); err != nil {
		log.Println(entry)
	} else if b.log != nil {
		b.log(msg)
	}
}

func (b *Bridge) GetPrinters() ([]PrinterInfo, error) {
	return b.getPrintersPlatform()
}

type PrinterInfo struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

func (b *Bridge) GetPrinterCapabilities(printerName string) (map[string]interface{}, error) {
	return b.getPrinterCapabilitiesPlatform(printerName)
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
}

type PaperSpec struct {
	Size string `json:"size"`
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
		errMsg := fmt.Sprintf("Failed to get printer list: %v", err)
		c.WriteJSON(Response{Status: "error", Message: errMsg})
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
				resp := Response{Status: "error", Message: fmt.Sprintf("Failed to get printer list: %v", err)}
				if jsonBytes, err := json.Marshal(resp); err == nil {
					b.Log(fmt.Sprintf("Sent WS message: %s", string(jsonBytes)))
				}
				c.WriteJSON(resp)
			}
			continue
		}

		if msgType, ok := rawMsg["type"].(string); ok && msgType == "get_printer_caps" {
			printer, _ := rawMsg["printer"].(string)
			printer = strings.TrimSpace(printer)
			if printer == "" {
				resp := Response{Status: "error", Message: "printer is required"}
				c.WriteJSON(resp)
				continue
			}

			caps, err := b.GetPrinterCapabilities(printer)
			if err != nil {
				b.Log(fmt.Sprintf("Failed to get printer capabilities for '%s': %v", printer, err))
				resp := Response{Status: "error", Message: err.Error()}
				c.WriteJSON(resp)
				continue
			}

			msg := map[string]interface{}{
				"type":    "printer_caps",
				"printer": printer,
				"data":    caps,
			}
			c.WriteJSON(msg)
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

		msg, err := b.processPrintRequest(req)
		if err == nil {
			resp := Response{Status: "success", Message: msg}
			c.WriteJSON(resp)
		} else {
			c.WriteJSON(Response{Status: "error", Message: msg})
		}
	}
}

func (b *Bridge) processPrintRequest(req PrintRequest) (string, error) {
	jobName := strings.TrimSpace(req.Job.Name)

	copies := req.Job.Copies
	if copies <= 0 {
		copies = 1
	}

	intervalMs := req.Job.IntervalMs

	contentToDecode := req.Content
	if strings.HasPrefix(contentToDecode, "data:") {
		if idx := strings.Index(contentToDecode, ","); idx != -1 {
			contentToDecode = contentToDecode[idx+1:]
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(contentToDecode)
	if err != nil {
		b.Log("Error decoding Base64 content")
		return "Invalid Base64 content", fmt.Errorf("invalid base64 content")
	}

	if len(decoded) < 4 || string(decoded[0:4]) != "%PDF" {
		b.Log("Content is not a valid PDF (missing %PDF header)")
		return "Content must be a PDF file", fmt.Errorf("invalid pdf")
	}

	autoPaper := ""
	if strings.TrimSpace(req.Paper.Size) == "" {
		if name, ok := detectPaperFromPDF(decoded); ok {
			autoPaper = name
			b.Log(fmt.Sprintf("Auto paper size detected: %s", name))
		}
	}

	options := PrintOptions{
		PageRange:     strings.TrimSpace(req.Pages.Range),
		PageSet:       strings.TrimSpace(req.Pages.Set),
		Duplex:        strings.TrimSpace(req.Sides.Mode),
		ColorMode:     strings.TrimSpace(req.Color.Mode),
		Paper:         strings.TrimSpace(firstNonEmpty(req.Paper.Size, autoPaper)),
		Scale:         strings.TrimSpace(req.Layout.Scale),
		Orientation:   strings.TrimSpace(req.Layout.Orientation),
		TrayBin:       strings.TrimSpace(req.Tray.Bin),
		PrintSettings: strings.TrimSpace(req.Sumatra.Settings),
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
		return "Printed successfully", nil
	}

	msg := fmt.Sprintf("Printed %d/%d copies. Error: %v", successCount, copies, lastErr)
	b.Log(msg)
	return msg, fmt.Errorf("print failed")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

var (
	mediaBoxRegex = regexp.MustCompile(`/MediaBox\s*\[\s*([-0-9.]+)\s+([-0-9.]+)\s+([-0-9.]+)\s+([-0-9.]+)\s*\]`)
	cropBoxRegex  = regexp.MustCompile(`/CropBox\s*\[\s*([-0-9.]+)\s+([-0-9.]+)\s+([-0-9.]+)\s+([-0-9.]+)\s*\]`)
)

func detectPaperFromPDF(pdfData []byte) (string, bool) {
	limit := len(pdfData)
	if limit > 5*1024*1024 {
		limit = 5 * 1024 * 1024
	}
	chunk := string(pdfData[:limit])
	match := cropBoxRegex.FindStringSubmatch(chunk)
	if len(match) != 5 {
		match = mediaBoxRegex.FindStringSubmatch(chunk)
	}
	if len(match) != 5 {
		return "", false
	}

	llx, err1 := strconv.ParseFloat(match[1], 64)
	lly, err2 := strconv.ParseFloat(match[2], 64)
	urx, err3 := strconv.ParseFloat(match[3], 64)
	ury, err4 := strconv.ParseFloat(match[4], 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return "", false
	}

	widthPt := math.Abs(urx - llx)
	heightPt := math.Abs(ury - lly)
	return matchStandardPaper(widthPt, heightPt)
}

type paperSize struct {
	Name string
	Wmm  float64
	Hmm  float64
}

func matchStandardPaper(widthPt, heightPt float64) (string, bool) {
	mmPerPt := 25.4 / 72.0
	widthMm := widthPt * mmPerPt
	heightMm := heightPt * mmPerPt

	if widthMm <= 0 || heightMm <= 0 {
		return "", false
	}

	if widthMm > heightMm {
		widthMm, heightMm = heightMm, widthMm
	}

	standard := []paperSize{
		{Name: "A2", Wmm: 420, Hmm: 594},
		{Name: "A3", Wmm: 297, Hmm: 420},
		{Name: "A4", Wmm: 210, Hmm: 297},
		{Name: "A5", Wmm: 148, Hmm: 210},
		{Name: "A6", Wmm: 105, Hmm: 148},
		{Name: "letter", Wmm: 216, Hmm: 279},
		{Name: "legal", Wmm: 216, Hmm: 356},
		{Name: "tabloid", Wmm: 279, Hmm: 432},
		{Name: "statement", Wmm: 140, Hmm: 216},
	}

	const tolerance = 2.0
	for _, size := range standard {
		if math.Abs(widthMm-size.Wmm) <= tolerance && math.Abs(heightMm-size.Hmm) <= tolerance {
			return size.Name, true
		}
	}

	return "", false
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
	b.ensureLogDir()
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
	mux.HandleFunc("/api/forwarder/status", b.handleForwarderStatus)
	mux.HandleFunc("/api/forwarder/connect", b.handleForwarderConnect)
	mux.HandleFunc("/api/forwarder/disconnect", b.handleForwarderDisconnect)
	mux.HandleFunc("/api/forwarder/stream", b.handleForwarderStream)

	b.logServer = &http.Server{
		Handler: mux,
	}

	go func() {
		if err := b.logServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			b.Log(fmt.Sprintf("Log server error: %v", err))
		}
	}()

	return nil
}

func (b *Bridge) ensureLogDir() {
	logDir, err := b.logDirPath()
	if err != nil {
		return
	}
	_ = os.MkdirAll(logDir, 0755)
	path := filepath.Join(logDir, time.Now().Format("20060102")+".txt")
	if f, err := os.OpenFile(path, os.O_CREATE, 0644); err == nil {
		f.Close()
	}
}

func (b *Bridge) StopLogServer() error {
	if b.logServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := b.logServer.Shutdown(ctx); err != nil {
		_ = b.logServer.Close()
		return err
	}
	b.logServer = nil
	return nil
}

func (b *Bridge) handleLogs(w http.ResponseWriter, r *http.Request) {
	currentLogs, _ := b.readLogLines()

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
	currentLogs, _ := b.readLogLines()

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

	b.clearLogFile()

	w.WriteHeader(http.StatusOK)
}

func (b *Bridge) logFilePath() (string, error) {
	logDir, err := b.logDirPath()
	if err != nil {
		return "", err
	}
	fileName := time.Now().Format("20060102") + ".txt"
	return filepath.Join(logDir, fileName), nil
}

func (b *Bridge) logDirPath() (string, error) {
	baseDir, err := dataDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, "logs"), nil
}

func dataDirPath() (string, error) {
	if programData := strings.TrimSpace(os.Getenv("ProgramData")); programData != "" {
		return filepath.Join(programData, "PrintDot"), nil
	}
	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "PrintDot"), nil
		}
	}
	if runtime.GOOS == "linux" {
		if dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dataHome != "" {
			return filepath.Join(dataHome, "PrintDot"), nil
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "PrintDot"), nil
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, "PrintDot"), nil
	}
	return "", fmt.Errorf("failed to resolve data directory")
}

func (b *Bridge) appendLogLine(line string) error {
	b.ensureLogDir()
	path, err := b.logFilePath()
	if err != nil {
		return err
	}
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}

	b.logFileMu.Lock()
	defer b.logFileMu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(line)
	return err
}

func (b *Bridge) readLogLines() ([]string, error) {
	b.ensureLogDir()
	path, err := b.logFilePath()
	if err != nil {
		return nil, err
	}

	b.logFileMu.Lock()
	defer b.logFileMu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	lines := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		lines = append(lines, text)
	}

	if err := scanner.Err(); err != nil {
		return lines, err
	}

	// Reverse logs for display (newest top)
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return lines, nil
}

func (b *Bridge) clearLogFile() {
	path, err := b.logFilePath()
	if err != nil {
		return
	}

	b.logFileMu.Lock()
	defer b.logFileMu.Unlock()
	_ = os.Remove(path)
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

func (b *Bridge) handleForwarderStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := RemoteForwarderStatus{}
	if b.forwarderStatusProvider != nil {
		status = b.forwarderStatusProvider()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (b *Bridge) handleForwarderConnect(w http.ResponseWriter, r *http.Request) {
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

	if b.forwarderConnect != nil {
		b.forwarderConnect()
	}
	w.WriteHeader(http.StatusOK)
}

func (b *Bridge) handleForwarderDisconnect(w http.ResponseWriter, r *http.Request) {
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

	if b.forwarderDisconnect != nil {
		b.forwarderDisconnect()
	}
	w.WriteHeader(http.StatusOK)
}

func (b *Bridge) handleForwarderStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	status := RemoteForwarderStatus{}
	if b.forwarderStatusProvider != nil {
		status = b.forwarderStatusProvider()
	}
	if data, err := json.Marshal(status); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	last := status
	ctx := r.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := RemoteForwarderStatus{}
			if b.forwarderStatusProvider != nil {
				current = b.forwarderStatusProvider()
			}
			if current != last {
				if data, err := json.Marshal(current); err == nil {
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
					last = current
				}
			}
		}
	}
}
