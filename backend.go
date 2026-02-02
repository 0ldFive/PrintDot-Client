package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/alexbrainman/printer"
	"github.com/gorilla/websocket"
)

type Bridge struct {
	server *http.Server
	port   string
	key    string
	mu     sync.Mutex
	log    func(string) // Callback to log to frontend
}

func NewBridge() *Bridge {
	return &Bridge{
		port: "1122",
		key:  "",
	}
}

func (b *Bridge) SetLogger(logger func(string)) {
	b.log = logger
}

func (b *Bridge) Log(msg string) {
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
