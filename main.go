package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	var isQuitting bool

	// Create application menu
	appMenu := menu.NewMenu()
	FileMenu := appMenu.AddSubmenu("File")
	FileMenu.AddText("System Logs", keys.CmdOrCtrl("l"), func(_ *menu.CallbackData) {
		app.ShowLogs()
	})
	FileMenu.AddSeparator()
	FileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		isQuitting = true
		app.Quit()
	})

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "print-dot-client",
		Width:     380,
		MinWidth:  380,
		Height:    600,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose: func(ctx context.Context) bool {
			if isQuitting {
				return false
			}
			runtime.WindowMinimise(ctx)
			return true
		},
		Menu: appMenu,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			BackdropType:         windows.Mica,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
