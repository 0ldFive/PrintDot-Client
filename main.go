package main

import (
	"context"
	"embed"

	"os"
	"strconv"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/getlantern/systray"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	// Check command line args
	mode := "main"
	logPort := 0

	if len(os.Args) > 2 && os.Args[1] == "logs" {
		mode = "logs"
		if p, err := strconv.Atoi(os.Args[2]); err == nil {
			logPort = p
		}
	}

	// Create an instance of the app structure
	app := NewApp(mode, logPort)

	// Configure based on mode
	title := "print-dot-client"
	width := 380
	height := 600
	minWidth := 380
	minHeight := 600
	var onBeforeClose func(ctx context.Context) bool

	appMenu := menu.NewMenu()

	if mode == "main" {
		// Main App Configuration
		FileMenu := appMenu.AddSubmenu("Menu")
		FileMenu.AddText("System Logs", keys.CmdOrCtrl("l"), func(_ *menu.CallbackData) {
			app.ShowLogs()
		})
		FileMenu.AddSeparator()
		FileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
			app.Quit()
		})

		onBeforeClose = func(ctx context.Context) bool {
			app.Cleanup()
			return false
		}

		// Start system tray
		go systray.Run(func() {
			systray.SetIcon(icon)
			systray.SetTitle("PrintDot Client")
			systray.SetTooltip("PrintDot Client")

			mShow := systray.AddMenuItem("Show Main Window", "Show the application window")
			mQuit := systray.AddMenuItem("Quit", "Quit the application")

			go func() {
				for {
					select {
					case <-mShow.ClickedCh:
						if app.ctx != nil {
							runtime.WindowShow(app.ctx)
							// runtime.WindowSetFocus(app.ctx) // Not available in all versions
						}
					case <-mQuit.ClickedCh:
						app.Cleanup()
						if app.ctx != nil {
							runtime.Quit(app.ctx)
						} else {
							systray.Quit()
							os.Exit(0)
						}
					}
				}
			}()
		}, func() {
			// Cleanup if needed
		})
	} else {
		// Logs Window Configuration
		title = "System Logs"
		width = 700
		height = 500
		// No special menu or close behavior for logs window (it just closes)
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     title,
		Width:     width,
		Height:    height,
		MinWidth:  minWidth,
		MinHeight: minHeight,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    onBeforeClose,
		Menu:             appMenu,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			BackdropType:         windows.Mica,
		},
	})

	if mode == "main" {
		systray.Quit()
	}

	if err != nil {
		println("Error:", err.Error())
	}
}
