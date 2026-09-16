package main

import (
	"context"
	"embed"
	"os"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Enable WebView2 GPU hardware acceleration by default for smooth 60fps animations and hardware video decoding.
	// Can be disabled via --disable-gpu flag or DUCKE_DISABLE_GPU=1 environment variable if troubleshooting virtual display drivers.
	disableGPU := false
	for _, arg := range os.Args {
		if arg == "--disable-gpu" {
			disableGPU = true
			break
		}
		if arg == "--enable-gpu" {
			disableGPU = false
			break
		}
	}
	if os.Getenv("DUCKE_DISABLE_GPU") == "1" {
		disableGPU = true
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Ducke",
		Width:     1280,
		Height:    800,
		MinWidth:  1024,
		MinHeight: 640,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app,
		},
		BackgroundColour: &options.RGBA{R: 7, G: 8, B: 10, A: 255}, // #07080a
		OnStartup:        app.startup,
		OnDomReady: func(ctx context.Context) {
			wailsRuntime.WindowCenter(ctx)
			wailsRuntime.WindowShow(ctx)
			go func() {
				time.Sleep(100 * time.Millisecond)
				if app.cfgManager != nil {
					wailsRuntime.EventsEmit(ctx, "settings:updated", app.cfgManager.GetSettings())
				}
				wailsRuntime.EventsEmit(ctx, "torrents:updated", nil)
			}()
		},
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			Theme:                windows.Dark,
			CustomTheme:          nil,
			WebviewGpuIsDisabled: disableGPU,
		},
		Linux: &linux.Options{
			Icon:                icon,
			WindowIsTranslucent: false,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyAlways,
			ProgramName:         "Ducke",
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
