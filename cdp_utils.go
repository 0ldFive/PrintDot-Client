package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// CDPPrintResult represents the structure of the response from Page.printToPDF
type CDPPrintResult struct {
	ID     int `json:"id"`
	Result struct {
		Data string `json:"data"` // Base64 encoded PDF
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// CDPCommand represents a generic CDP command
type CDPCommand struct {
	ID     int                    `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// CDPTarget represents a browser target (tab/window)
type CDPTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	Url                  string `json:"url"`
	WebSocketDebuggerUrl string `json:"webSocketDebuggerUrl"`
}

// GeneratePDFFromWebView uses Chrome DevTools Protocol to capture the current view as PDF.
// It connects to the local remote debugging port (9222).
func GeneratePDFFromWebView() ([]byte, error) {
	debugPort := 9222
	debugURL := fmt.Sprintf("http://localhost:%d/json/list", debugPort)

	// 1. Get the WebSocket URL for the active page
	// Retry a few times as the browser might be starting up
	var targets []CDPTarget
	var err error
	
	for i := 0; i < 5; i++ {
		resp, reqErr := http.Get(debugURL)
		if reqErr == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if json.Unmarshal(body, &targets) == nil && len(targets) > 0 {
				err = nil
				break
			}
		}
		err = reqErr
		time.Sleep(200 * time.Millisecond)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebView2 debugger: %v. Make sure the app is running with --remote-debugging-port=%d", err, debugPort)
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no active WebView targets found")
	}

	// Find the 'page' target (usually the main window)
	var wsURL string
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerUrl != "" {
			wsURL = t.WebSocketDebuggerUrl
			break
		}
	}

	if wsURL == "" {
		// Fallback to the first available target if no specific 'page' type found
		if targets[0].WebSocketDebuggerUrl != "" {
			wsURL = targets[0].WebSocketDebuggerUrl
		} else {
			return nil, fmt.Errorf("no WebSocket debugger URL found in targets")
		}
	}

	// 2. Connect via WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// 3. Send Page.printToPDF command
	// See: https://chromedevtools.github.io/devtools-protocol/tot/Page/#method-printToPDF
	cmd := CDPCommand{
		ID:     1,
		Method: "Page.printToPDF",
		Params: map[string]interface{}{
			"printBackground": true,
			"marginTop":       0,
			"marginBottom":    0,
			"marginLeft":      0,
			"marginRight":     0,
			// "paperWidth": 8.27, // A4 width in inches
			// "paperHeight": 11.69, // A4 height in inches
			// Leave dimensions empty to use page size or default
		},
	}

	if err := conn.WriteJSON(cmd); err != nil {
		return nil, fmt.Errorf("failed to send print command: %v", err)
	}

	// 4. Read response
	// We might receive other events, so we loop until we get our ID
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("failed to read WebSocket response: %v", err)
		}

		var res CDPCommand // Decode generic first to check ID
		if err := json.Unmarshal(message, &res); err != nil {
			continue
		}

		if res.ID == 1 {
			// This is our response
			var printRes CDPPrintResult
			if err := json.Unmarshal(message, &printRes); err != nil {
				return nil, fmt.Errorf("failed to decode print result: %v", err)
			}

			if printRes.Error != nil {
				return nil, fmt.Errorf("CDP error: %s", printRes.Error.Message)
			}

			// 5. Decode Base64 PDF
			pdfBytes, err := base64.StdEncoding.DecodeString(printRes.Result.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode PDF base64: %v", err)
			}

			return pdfBytes, nil
		}
	}
}
