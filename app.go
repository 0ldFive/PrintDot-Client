package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/gen2brain/beeep"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed docs/usage_guide.md
var usageGuide string

// App struct
type App struct {
	ctx         context.Context
	bridge      *Bridge
	settings    *SettingsManager
	AppMode     string
	LogPort     int
	logsCmd     *exec.Cmd
	logsMu      sync.Mutex
	helpCmd     *exec.Cmd
	settingsCmd *exec.Cmd
}

// NewApp creates a new App application struct
func NewApp(mode string, logPort int) *App {
	return &App{
		bridge:   NewBridge(),
		settings: NewSettingsManager(),
		AppMode:  mode,
		LogPort:  logPort,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	if a.AppMode != "main" {
		// In auxiliary modes (logs, help, settings), we don't start the bridge or log server
		return
	}

	// Setup logger to emit events
	a.bridge.SetLogger(func(msg string) {
		runtime.EventsEmit(a.ctx, "log", msg)
		fmt.Println(msg)
	})

	// Setup client count callback
	lastCount := 0
	a.bridge.SetCountCallback(func(count int) {
		runtime.EventsEmit(a.ctx, "client_count", count)
		if count > lastCount {
			// Notify system on new connection
			beeep.Notify("PrintDot Client", fmt.Sprintf("New client connected! Total: %d", count), "")
		}
		lastCount = count
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

func (a *App) Restart() {
	a.Cleanup()

	exe, err := os.Executable()
	if err != nil {
		return
	}

	cmd := exec.Command(exe)
	cmd.Start()

	runtime.Quit(a.ctx)
}

func (a *App) Quit() {
	runtime.Quit(a.ctx)
}

func (a *App) ShowLogs() {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()

	if a.logsCmd != nil {
		a.bridge.Log("Logs window already open, bringing to front")
		// Spawn a trigger process to activate SingleInstanceLock on the existing window
		exe, err := os.Executable()
		if err == nil {
			exec.Command(exe, "logs", fmt.Sprintf("%d", a.bridge.logPort)).Start()
		}
		return
	}

	if a.bridge.logPort > 0 {
		// Spawn a new process of ourselves with special flags
		exe, err := os.Executable()
		if err != nil {
			a.bridge.Log(fmt.Sprintf("Failed to get executable: %v", err))
			return
		}

		cmd := exec.Command(exe, "logs", fmt.Sprintf("%d", a.bridge.logPort))
		if err := cmd.Start(); err != nil {
			a.bridge.Log(fmt.Sprintf("Failed to spawn logs window: %v", err))
		} else {
			a.logsCmd = cmd
			go func() {
				cmd.Wait()
				a.logsMu.Lock()
				a.logsCmd = nil
				a.logsMu.Unlock()
			}()
		}
	} else {
		a.bridge.Log("Log server not running")
	}
}

func (a *App) Cleanup() {
	// Stop log server
	a.bridge.StopLogServer()

	a.logsMu.Lock()
	defer a.logsMu.Unlock()

	// Kill child processes if running
	if a.logsCmd != nil && a.logsCmd.Process != nil {
		a.logsCmd.Process.Kill()
		a.logsCmd = nil
	}
	if a.helpCmd != nil && a.helpCmd.Process != nil {
		a.helpCmd.Process.Kill()
		a.helpCmd = nil
	}
	if a.settingsCmd != nil && a.settingsCmd.Process != nil {
		a.settingsCmd.Process.Kill()
		a.settingsCmd = nil
	}
}

func (a *App) GetAppMode() string {
	return a.AppMode
}

func (a *App) GetUsageGuide() string {
	return usageGuide
}

func (a *App) GetSettings() AppSettings {
	return a.settings.Get()
}

func (a *App) SaveSettings(s AppSettings) error {
	return a.settings.Save(s)
}

func (a *App) spawnWindow(mode string, cmdStore **exec.Cmd) {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	if *cmdStore != nil {
		// If already open, trigger single instance logic
		exec.Command(exe, mode).Start()
		return
	}

	cmd := exec.Command(exe, mode)
	if err := cmd.Start(); err != nil {
		a.bridge.Log(fmt.Sprintf("Failed to spawn %s window: %v", mode, err))
	} else {
		*cmdStore = cmd
		go func() {
			cmd.Wait()
			a.logsMu.Lock()
			*cmdStore = nil
			a.logsMu.Unlock()
		}()
	}
}

func (a *App) ShowHelp() {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	a.spawnWindow("help", &a.helpCmd)
}

func (a *App) ShowSettings() {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	a.spawnWindow("settings", &a.settingsCmd)
}

func (a *App) GetLogPort() int {
	return a.LogPort
}
