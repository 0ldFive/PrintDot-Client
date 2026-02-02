package main

import (
	"context"
	"embed"

	"os"
	"strconv"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
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

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "logs":
			mode = "logs"
			if len(os.Args) > 2 {
				if p, err := strconv.Atoi(os.Args[2]); err == nil {
					logPort = p
				}
			}
		case "help":
			mode = "help"
		case "settings":
			mode = "settings"
			if len(os.Args) > 2 {
				if p, err := strconv.Atoi(os.Args[2]); err == nil {
					logPort = p
				}
			}
		}
	}

	// Create an instance of the app structure
	app := NewApp(mode, logPort)

	// Load settings to determine menu language
	sm := NewSettingsManager()
	currentLang := sm.Get().Language

	// Configure based on mode
	title := "PrintDot Client"
	if currentLang == "zh-CN" {
		title = "PrintDot 客户端"
	}

	width := 380
	height := 660
	minWidth := 380
	minHeight := 600
	var onBeforeClose func(ctx context.Context) bool

	var appMenu *menu.Menu

	if mode == "main" {
		appMenu = app.CreateMenu(currentLang)

		onBeforeClose = func(ctx context.Context) bool {
			app.Cleanup()
			return false
		}

		// Start system tray
		go systray.Run(func() {
			systray.SetIcon(icon)
			systray.SetTitle("PrintDot Client")
			systray.SetTooltip("PrintDot Client")

			showTitle := "Show Main Window"
			quitTitle := "Quit"
			if currentLang == "zh-CN" {
				showTitle = "显示主窗口"
				quitTitle = "退出"
			}

			mShow := systray.AddMenuItem(showTitle, showTitle)
			mQuit := systray.AddMenuItem(quitTitle, quitTitle)

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
	} else if mode == "logs" {
		// Logs Window Configuration
		title = "System Logs"
		if currentLang == "zh-CN" {
			title = "系统日志"
		}
		width = 700
		height = 500
		// No special menu or close behavior for logs window (it just closes)
	} else if mode == "help" {
		title = "Help - Usage Guide"
		if currentLang == "zh-CN" {
			title = "帮助 - 使用指南"
		}
		width = 800
		height = 600
		minWidth = 600
		minHeight = 400
	} else if mode == "settings" {
		title = "Settings"
		if currentLang == "zh-CN" {
			title = "设置"
		}
	}

	// Create application with options
	appOptions := &options.App{
		Title:     title,
		Width:     width,
		Height:    height,
		MinWidth:  minWidth,
		MinHeight: minHeight,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			// Restore position if valid
			if mode == "main" {
				s := sm.Get()
				if s.WindowX != 0 || s.WindowY != 0 {
					runtime.WindowSetPosition(ctx, s.WindowX, s.WindowY)
				}
				if s.Maximized {
					runtime.WindowMaximise(ctx)
				}
			}
		},
		OnBeforeClose: onBeforeClose,
		Menu:          appMenu,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			BackdropType:         windows.Mica,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
	}

	if mode == "main" {
		appOptions.SingleInstanceLock = &options.SingleInstanceLock{
			UniqueId: "56006c0a-0498-4228-a320-c2409044a14e",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				runtime.WindowShow(app.ctx)
			},
		}
	} else if mode == "logs" {
		appOptions.SingleInstanceLock = &options.SingleInstanceLock{
			UniqueId: "56006c0a-0498-4228-a320-c2409044a14e-logs",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				runtime.WindowShow(app.ctx)
			},
		}
	} else if mode == "help" {
		appOptions.SingleInstanceLock = &options.SingleInstanceLock{
			UniqueId: "56006c0a-0498-4228-a320-c2409044a14e-help",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				runtime.WindowShow(app.ctx)
			},
		}
	} else if mode == "settings" {
		appOptions.SingleInstanceLock = &options.SingleInstanceLock{
			UniqueId: "56006c0a-0498-4228-a320-c2409044a14e-settings",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				runtime.WindowShow(app.ctx)
			},
		}
	}

	err := wails.Run(appOptions)

	if mode == "main" {
		systray.Quit()
	}

	if err != nil {
		println("Error:", err.Error())
	}
}
