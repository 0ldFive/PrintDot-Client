package main

import (
	"context"
	"embed"
	sys_runtime "runtime"

	"os"
	"strconv"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/energye/systray"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed build/windows/icon.ico
var iconIco []byte

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
	LoadLocales(currentLang)

	// Configure based on mode
	title := T("window.main")
	width := 380
	height := 660
	minWidth := 380
	minHeight := 600
	var onBeforeClose func(ctx context.Context) bool

	var appMenu *menu.Menu

	if mode == "main" {
		appMenu = app.CreateMenu(currentLang)

		onBeforeClose = func(ctx context.Context) bool {
			if app.isQuitting {
				app.Cleanup()
				return false
			}
			runtime.WindowHide(ctx)
			return true
		}

		// Start system tray
		go systray.Run(func() {
			if sys_runtime.GOOS == "windows" {
				systray.SetIcon(iconIco)
			} else {
				systray.SetIcon(icon)
			}
			systray.SetTitle(T("tray.title"))
			systray.SetTooltip(T("tray.tooltip"))

			systray.SetOnClick(func(menu systray.IMenu) {
				if app.ctx != nil {
					runtime.WindowShow(app.ctx)
				}
			})
			systray.SetOnRClick(func(menu systray.IMenu) {
				menu.ShowMenu()
			})

			mShow := systray.AddMenuItem(T("tray.show"), T("tray.show"))
			mHelp := systray.AddMenuItem(T("menu.help"), T("menu.help"))
			mSettings := systray.AddMenuItem(T("menu.settings"), T("menu.settings"))
			systray.AddSeparator()
			mQuit := systray.AddMenuItem(T("tray.quit"), T("tray.quit"))

			mShow.Click(func() {
				if app.ctx != nil {
					runtime.WindowShow(app.ctx)
					// runtime.WindowSetFocus(app.ctx) // Not available in all versions
				}
			})
			mHelp.Click(func() {
				app.ShowHelp()
			})
			mSettings.Click(func() {
				app.ShowSettings()
			})
			mQuit.Click(func() {
				app.Quit()
			})
		}, func() {
			// Cleanup if needed
		})
	} else if mode == "logs" {
		// Logs Window Configuration
		title = T("window.logs")
		width = 700
		height = 500
		// No special menu or close behavior for logs window (it just closes)
	} else if mode == "help" {
		title = T("window.help")
		width = 800
		height = 600
		minWidth = 600
		minHeight = 400
	} else if mode == "settings" {
		title = T("window.settings")
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
