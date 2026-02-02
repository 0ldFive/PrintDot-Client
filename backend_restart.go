package main

import (
	"net/http"
	"time"
)

func (b *Bridge) handleRestartRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Restart initiated"))

	// Trigger restart on main thread
	if b.onRestart != nil {
		go func() {
			time.Sleep(500 * time.Millisecond) // Give time for response to be sent
			b.onRestart()
		}()
	}
}
