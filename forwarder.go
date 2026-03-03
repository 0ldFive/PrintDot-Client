package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	remotePingInterval   = 20 * time.Second
	remoteReportInterval = 10 * time.Second
	remotePongWait       = 70 * time.Second
	remoteWriteWait      = 5 * time.Second
)

type RemoteConfig struct {
	Server     string
	AuthURL    string
	WsURL      string
	ClientID   string
	SecretKey  string
	ClientName string
}

type RemoteForwarderStatus struct {
	Connected     bool   `json:"connected"`
	LastError     string `json:"lastError"`
	LastChange    int64  `json:"lastChange"`
	AutoReconnect bool   `json:"autoReconnect"`
}

type remoteLoginResponse struct {
	Token string `json:"token"`
}

type remotePrintTask struct {
	Cmd    string `json:"cmd"`
	TaskID string `json:"task_id"`
	PrintRequest
}

func (b *Bridge) ConfigureRemoteForwarder(s AppSettings) {
	b.StartRemoteForwarderWithSettings(s, false)
}

func (b *Bridge) StartRemoteForwarderWithSettings(s AppSettings, force bool) {
	cfg := RemoteConfig{
		Server:     strings.TrimSpace(s.RemoteServer),
		AuthURL:    strings.TrimSpace(s.RemoteAuthURL),
		WsURL:      strings.TrimSpace(s.RemoteWsURL),
		ClientID:   strings.TrimSpace(firstNonEmpty(s.RemoteClientID, s.RemoteUser)),
		SecretKey:  strings.TrimSpace(firstNonEmpty(s.RemoteSecretKey, s.RemotePassword)),
		ClientName: strings.TrimSpace(s.RemoteClientName),
	}

	if !force && !s.RemoteAutoConnect {
		b.StopRemoteForwarder()
		return
	}

	if (cfg.AuthURL == "" && cfg.Server == "") ||
		(cfg.WsURL == "" && cfg.Server == "") ||
		cfg.ClientID == "" || cfg.SecretKey == "" {
		b.StopRemoteForwarder()
		return
	}

	b.remoteMu.Lock()
	same := cfg == b.remoteCfg && b.remoteStop != nil
	b.remoteMu.Unlock()
	if same {
		return
	}

	b.StopRemoteForwarder()

	b.remoteMu.Lock()
	b.remoteCfg = cfg
	b.remoteStop = make(chan struct{})
	stop := b.remoteStop
	b.remoteMu.Unlock()

	b.remoteWg.Add(1)
	go b.runRemoteForwarder(cfg, stop)
}

func (b *Bridge) StopRemoteForwarder() {
	b.remoteMu.Lock()
	stop := b.remoteStop
	conn := b.remoteConn
	b.remoteStop = nil
	b.remoteMu.Unlock()

	if stop != nil {
		close(stop)
	}
	if conn != nil {
		_ = conn.Close()
	}
	b.setRemoteConnected(false)
	b.remoteWg.Wait()
}

func (b *Bridge) runRemoteForwarder(cfg RemoteConfig, stop <-chan struct{}) {
	defer b.remoteWg.Done()

	for {
		if err := b.connectAndServeForwarder(cfg, stop); err != nil {
			b.setRemoteError(err)
			b.Log(fmt.Sprintf("Remote forwarder error: %v", err))
		}

		select {
		case <-stop:
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (b *Bridge) connectAndServeForwarder(cfg RemoteConfig, stop <-chan struct{}) error {
	loginURL, wsURL, err := buildRemoteURLsFromConfig(cfg)
	if err != nil {
		return err
	}

	token, err := b.remoteLogin(loginURL, cfg)
	if err != nil {
		return err
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	headers.Set("X-Client-Id", cfg.ClientID)
	if cfg.ClientName != "" {
		headers.Set("X-Client-Name", cfg.ClientName)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), headers)
	if err != nil {
		return fmt.Errorf("ws connect failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadLimit(8 * 1024 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(remotePongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(remotePongWait))
	})

	b.setRemoteConn(conn)
	b.setRemoteConnected(true)
	defer func() {
		b.clearRemoteConn(conn)
		b.setRemoteConnected(false)
	}()

	b.Log(fmt.Sprintf("Remote forwarder connected: %s", wsURL.String()))

	if err := b.reportPrinters(conn); err != nil {
		b.Log(fmt.Sprintf("Report printers failed: %v", err))
	}

	pingStop := make(chan struct{})
	go b.pingLoop(conn, pingStop)
	go b.reportLoop(conn, pingStop)
	defer close(pingStop)

	for {
		select {
		case <-stop:
			return nil
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(remotePongWait))
		var rawMsg map[string]interface{}
		if err := conn.ReadJSON(&rawMsg); err != nil {
			return err
		}

		cmd, _ := rawMsg["cmd"].(string)
		switch strings.ToLower(strings.TrimSpace(cmd)) {
		case "print_task":
			jsonBody, _ := json.Marshal(rawMsg)
			var task remotePrintTask
			if err := json.Unmarshal(jsonBody, &task); err != nil {
				b.Log(fmt.Sprintf("Invalid print task: %v", err))
				continue
			}

			message, err := b.processPrintRequest(task.PrintRequest)
			status := "success"
			if err != nil {
				status = "failed"
			}

			resp := map[string]interface{}{
				"cmd":     "report_result",
				"task_id": task.TaskID,
				"status":  status,
				"message": message,
			}
			if err := writeJSONWithDeadline(conn, resp); err != nil {
				b.Log(fmt.Sprintf("Report result failed: %v", err))
			}
		case "get_printers":
			if err := b.reportPrinters(conn); err != nil {
				b.Log(fmt.Sprintf("Report printers failed: %v", err))
			}
		case "auth_resp":
			b.Log("Remote forwarder auth ok")
		}
	}
}

func (b *Bridge) remoteLogin(loginURL *url.URL, cfg RemoteConfig) (string, error) {
	payload := map[string]string{
		"client_id":  cfg.ClientID,
		"secret_key": cfg.SecretKey,
	}
	if cfg.ClientName != "" {
		payload["client_name"] = cfg.ClientName
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", loginURL.String(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("login failed: status %d", resp.StatusCode)
	}

	var loginResp remoteLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", err
	}
	if strings.TrimSpace(loginResp.Token) == "" {
		return "", fmt.Errorf("login failed: empty token")
	}
	return loginResp.Token, nil
}

func (b *Bridge) reportPrinters(conn *websocket.Conn) error {
	printers, err := b.GetPrinters()
	if err != nil {
		return err
	}

	list := make([]map[string]interface{}, 0, len(printers))
	for _, p := range printers {
		caps, capsErr := b.GetPrinterCapabilities(p.Name)
		if capsErr != nil {
			b.Log(fmt.Sprintf("Get printer capabilities failed: %s: %v", p.Name, capsErr))
			caps = map[string]interface{}{}
		}
		list = append(list, map[string]interface{}{
			"printer_name":     p.Name,
			"printer_type":     "system",
			"paper_spec":       "",
			"is_ready":         true,
			"supported_format": "pdf",
			"capabilities":     caps,
		})
	}

	payload := map[string]interface{}{
		"cmd":      "report_printers",
		"printers": list,
	}
	return writeJSONWithDeadline(conn, payload)
}

func (b *Bridge) pingLoop(conn *websocket.Conn, stop <-chan struct{}) {
	ticker := time.NewTicker(remotePingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			deadline := time.Now().Add(remoteWriteWait)
			_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), deadline)
		}
	}
}

func (b *Bridge) reportLoop(conn *websocket.Conn, stop <-chan struct{}) {
	ticker := time.NewTicker(remoteReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := b.reportPrinters(conn); err != nil {
				b.Log(fmt.Sprintf("Report printers failed: %v", err))
			}
		}
	}
}

func writeJSONWithDeadline(conn *websocket.Conn, payload interface{}) error {
	_ = conn.SetWriteDeadline(time.Now().Add(remoteWriteWait))
	return conn.WriteJSON(payload)
}

func buildRemoteURLs(raw string) (*url.URL, *url.URL, error) {
	baseURL, err := normalizeRemoteBaseURL(raw)
	if err != nil {
		return nil, nil, err
	}

	loginBase := *baseURL
	switch loginBase.Scheme {
	case "ws":
		loginBase.Scheme = "http"
	case "wss":
		loginBase.Scheme = "https"
	}

	wsBase := *baseURL
	switch wsBase.Scheme {
	case "http":
		wsBase.Scheme = "ws"
	case "https":
		wsBase.Scheme = "wss"
	}

	loginURL := loginBase.ResolveReference(&url.URL{Path: "/api/client/login"})
	wsURL := wsBase.ResolveReference(&url.URL{Path: "/ws/client"})
	return loginURL, wsURL, nil
}

func buildRemoteURLsFromConfig(cfg RemoteConfig) (*url.URL, *url.URL, error) {
	var loginURL *url.URL
	var wsURL *url.URL

	if cfg.AuthURL != "" {
		parsed, err := normalizeAuthURL(cfg.AuthURL)
		if err != nil {
			return nil, nil, err
		}
		loginURL = parsed
	}

	if cfg.WsURL != "" {
		parsed, err := normalizeWsURL(cfg.WsURL)
		if err != nil {
			return nil, nil, err
		}
		wsURL = parsed
	}

	if (loginURL == nil || wsURL == nil) && cfg.Server != "" {
		baseLoginURL, baseWsURL, err := buildRemoteURLs(cfg.Server)
		if err != nil {
			return nil, nil, err
		}
		if loginURL == nil {
			loginURL = baseLoginURL
		}
		if wsURL == nil {
			wsURL = baseWsURL
		}
	}

	if loginURL == nil || wsURL == nil {
		return nil, nil, fmt.Errorf("auth or ws address is missing")
	}

	return loginURL, wsURL, nil
}

func normalizeAuthURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("auth address is empty")
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid auth address")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/api/client/login"
	}
	switch parsed.Scheme {
	case "ws":
		parsed.Scheme = "http"
	case "wss":
		parsed.Scheme = "https"
	}
	return parsed, nil
}

func normalizeWsURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("ws address is empty")
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "ws://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid ws address")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/ws/client"
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	}
	return parsed, nil
}

func normalizeRemoteBaseURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("remote server is empty")
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid remote server address")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed, nil
}

func (b *Bridge) setRemoteConn(conn *websocket.Conn) {
	b.remoteMu.Lock()
	b.remoteConn = conn
	b.remoteMu.Unlock()
}

func (b *Bridge) clearRemoteConn(conn *websocket.Conn) {
	b.remoteMu.Lock()
	if b.remoteConn == conn {
		b.remoteConn = nil
	}
	b.remoteMu.Unlock()
}

func (b *Bridge) setRemoteConnected(connected bool) {
	b.remoteMu.Lock()
	b.remoteStatus.Connected = connected
	if connected {
		b.remoteStatus.LastError = ""
	}
	b.remoteStatus.LastChange = time.Now().Unix()
	b.remoteMu.Unlock()
}

func (b *Bridge) setRemoteError(err error) {
	if err == nil {
		return
	}
	b.remoteMu.Lock()
	b.remoteStatus.LastError = err.Error()
	b.remoteStatus.LastChange = time.Now().Unix()
	b.remoteMu.Unlock()
}

func (b *Bridge) GetRemoteForwarderStatus() RemoteForwarderStatus {
	b.remoteMu.Lock()
	defer b.remoteMu.Unlock()
	return b.remoteStatus
}
