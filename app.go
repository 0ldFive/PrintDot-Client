package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx    context.Context
	bridge *Bridge
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		bridge: NewBridge(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Setup logger to emit events
	a.bridge.SetLogger(func(msg string) {
		runtime.EventsEmit(a.ctx, "log", msg)
		fmt.Println(msg)
	})

	a.bridge.StartLogServer()
	a.bridge.Log("Application started")
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) GetPrinters() ([]string, error) {
	return a.bridge.GetPrinters()
}

func (a *App) StartServer(port string, key string) error {
	return a.bridge.StartServer(port, key)
}

func (a *App) StopServer() error {
	return a.bridge.StopServer()
}

func (a *App) Quit() {
	runtime.Quit(a.ctx)
}

func (a *App) ShowLogs() {
	if a.bridge.logPort > 0 {
		url := fmt.Sprintf("http://localhost:%d/logs", a.bridge.logPort)
		runtime.BrowserOpenURL(a.ctx, url)
	} else {
		a.bridge.Log("Log server not running")
	}
}
