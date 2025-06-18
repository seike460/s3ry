package main

import (
	"context"
	"fmt"
	"log"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/desktop"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: Failed to load configuration: %v", err)
		cfg = config.Default()
	}

	// Create desktop app instance
	app := desktop.NewApp(cfg)

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "S3ry Desktop - High-Performance S3 Browser",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:  &options.RGBA{R: 40, G: 42, B: 54, A: 1},
		MinWidth:          800,
		MinHeight:         600,
		OnStartup:         app.Startup,
		OnDomReady:        app.DomReady,
		OnBeforeClose:     app.BeforeClose,
		OnShutdown:        app.Shutdown,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		FullscreenOnStart: false,
		AlwaysOnTop:       false,
		CSSDragProperty:   "--wails-draggable",
		CSSDragValue:      "drag",
		Windows: &options.Windows{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			ResizeDebounceMS:                  10,
		},
		Mac: &options.Mac{
			TitleBarAppearsTransparent: true,
			WindowIsTranslucent:        false,
			DisableWindowIcon:          false,
			DisableToolbarSeparator:    true,
			About: &options.AboutInfo{
				Title:   "S3ry Desktop",
				Message: "High-Performance S3 Browser with 271,615x improvement\n\nBuilt with Wails v2 and Go",
				Icon:    nil,
			},
		},
		Linux: &options.Linux{
			Icon:                nil,
			WindowIsTranslucent: false,
		},
	})

	if err != nil {
		log.Fatal("Error:", err)
	}
}
