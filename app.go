package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/gen2brain/beeep"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	AppMode  string
	LogPort  int
	bridge   *Bridge
	settings *SettingsManager

	logsMu      sync.Mutex
	settingsCmd *exec.Cmd
	logsCmd     *exec.Cmd
	helpCmd     *exec.Cmd
}

// NewApp creates a new App application struct
func NewApp(mode string, logPort int) *App {
	return &App{
		AppMode:  mode,
		LogPort:  logPort,
		bridge:   NewBridge(),
		settings: NewSettingsManager(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Bind logger
	a.bridge.SetLogger(func(msg string) {
		runtime.EventsEmit(ctx, "log", msg)
	})

	// Bind client count
	a.bridge.SetCountCallback(func(count int) {
		runtime.EventsEmit(ctx, "client_count", count)
	})

	// Bind reload callback
	a.bridge.SetReloadCallback(func() {
		a.Reload()
	})

	// Bind client connect
	a.bridge.SetClientConnectCallback(func(clientInfo string) {
		runtime.EventsEmit(ctx, "client_connected", clientInfo)

		// System Notification
		lang := a.settings.Get().Language
		title := "PrintDot Client"
		msg := fmt.Sprintf("Client connected: %s", clientInfo)
		if lang == "zh-CN" {
			title = "PrintDot 打印服务"
			msg = fmt.Sprintf("客户端已连接: %s", clientInfo)
		}

		// Use beeep
		go func() {
			err := beeep.Notify(title, msg, "")
			if err != nil {
				fmt.Printf("Failed to send notification: %v\n", err)
			}
		}()
	})

	if a.AppMode == "main" {
		// Start Log Server
		if err := a.bridge.StartLogServer(); err != nil {
			fmt.Printf("Failed to start log server: %v\n", err)
		} else {
			a.LogPort = a.bridge.logPort
			a.bridge.Log(fmt.Sprintf("Log server started on port %d", a.LogPort))
		}
	}
}

func (a *App) domReady(ctx context.Context) {
	// Add any dom ready logic
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

func (a *App) shutdown(ctx context.Context) {
	a.Cleanup()
}

func (a *App) Cleanup() {
	if a.AppMode == "main" {
		a.bridge.StopServer()
		a.bridge.StopLogServer()

		// Kill child windows
		if a.settingsCmd != nil && a.settingsCmd.Process != nil {
			a.settingsCmd.Process.Kill()
		}
		if a.logsCmd != nil && a.logsCmd.Process != nil {
			a.logsCmd.Process.Kill()
		}
		if a.helpCmd != nil && a.helpCmd.Process != nil {
			a.helpCmd.Process.Kill()
		}
	}
}

func (a *App) Quit() {
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	} else {
		os.Exit(0)
	}
}

// Wails Methods

func (a *App) GetUsageGuide() string {
	return `# PrintDot Client Usage Guide

## Introduction
PrintDot Client is a lightweight, cross-platform printing service bridge that exposes local printers via WebSocket and HTTP APIs.

## Features
- **Raw Printing**: Send ZPL, EPL, or other raw commands directly to printers.
- **WebSocket API**: Real-time printing communication.
- **HTTP API**: Simple REST endpoints for status and logging.
- **System Tray**: Minimized background operation.
- **Multi-language Support**: English and Simplified Chinese.

## Connection
- **Port**: 1122 (Default)
- **WebSocket URL**: ws://localhost:1122/ws
- **Authentication**: Optional security key configurable in Settings.

## Usage
1. **Start the Service**: Launch the application. The server starts automatically.
2. **Configure**: Open Settings to set port, security key, or auto-start preferences.
3. **Connect**: Use your web application to connect to the WebSocket URL.
4. **Print**: Send JSON print requests to the WebSocket.

## JSON Request Format
` + "```" + `json
{
  "printer": "ZDesigner GK888t",
  "content": "^XA^FO50,50^ADN,36,20^FDHello World^FS^XZ",
  "copies": 1,
  "jobName": "MyLabel"
}
` + "```" + `
`
}

func (a *App) GetSettings() AppSettings {
	return a.settings.Get()
}

func (a *App) SaveSettings(s AppSettings) error {
	return a.settings.Save(s)
}

func (a *App) GetPrinters() ([]string, error) {
	return a.bridge.GetPrinters()
}

func (a *App) StartServer(port, key string) error {
	return a.bridge.StartServer(port, key)
}

func (a *App) StopServer() error {
	return a.bridge.StopServer()
}

func (a *App) GetAppMode() string {
	return a.AppMode
}

func (a *App) Restart() {
	runtime.Quit(a.ctx)
}

func (a *App) ShowHelp() {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	a.spawnWindow("help", &a.helpCmd)
}

func (a *App) ShowLogs() {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	a.spawnWindow("logs", &a.logsCmd)
}

func (a *App) ShowSettings() {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	a.spawnWindow("settings", &a.settingsCmd)
}

func (a *App) spawnWindow(mode string, cmdStore **exec.Cmd) {
	exe, err := os.Executable()
	if err != nil {
		a.bridge.Log(fmt.Sprintf("Failed to get executable: %v", err))
		return
	}

	cmd := exec.Command(exe, mode, strconv.Itoa(a.LogPort))
	cmd.Start()
	*cmdStore = cmd
}

func (a *App) GetLogPort() int {
	return a.LogPort
}

func (a *App) CreateMenu(lang string) *menu.Menu {
	appMenu := menu.NewMenu()

	// Main App Configuration
	menuTitle := "Menu"
	settingsTitle := "Settings"
	logsTitle := "System Logs"
	helpTitle := "Help"
	quitTitle := "Quit"

	if lang == "zh-CN" {
		menuTitle = "菜单"
		settingsTitle = "设置"
		logsTitle = "系统日志"
		helpTitle = "帮助"
		quitTitle = "退出"
	}

	// Menu (菜单)
	MenuMenu := appMenu.AddSubmenu(menuTitle)
	MenuMenu.AddText(logsTitle, keys.CmdOrCtrl("l"), func(_ *menu.CallbackData) {
		a.ShowLogs()
	})
	MenuMenu.AddSeparator()
	MenuMenu.AddText(quitTitle, keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		a.Quit()
	})

	// Help (帮助)
	HelpMenu := appMenu.AddSubmenu(helpTitle)
	HelpMenu.AddText(helpTitle, keys.CmdOrCtrl("h"), func(_ *menu.CallbackData) {
		a.ShowHelp()
	})

	// Settings (设置)
	SettingsMenu := appMenu.AddSubmenu(settingsTitle)
	SettingsMenu.AddText(settingsTitle, keys.CmdOrCtrl(","), func(_ *menu.CallbackData) {
		a.ShowSettings()
	})

	return appMenu
}

func (a *App) UpdateUI(lang string) {
	if a.ctx != nil && a.AppMode == "main" {
		runtime.MenuSetApplicationMenu(a.ctx, a.CreateMenu(lang))
		// Emit event for frontend updates
		runtime.EventsEmit(a.ctx, "reload_settings")
	}
}

func (a *App) Reload() {
	// Reload settings from disk
	a.settings.Load()
	// Update menu and UI
	a.UpdateUI(a.settings.Get().Language)
	a.bridge.Log("Settings reloaded")
}
