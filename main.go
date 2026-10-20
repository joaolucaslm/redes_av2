package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"bancada/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	sources, err := app.LoadSources()
	if err != nil {
		return err
	}
	rt := app.NewRuntime(sources)
	return wails.Run(&options.App{
		Title:            "Bancada · FSMs de transporte",
		Width:            1520,
		Height:           940,
		MinWidth:         1200,
		MinHeight:        760,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 22, G: 19, B: 16, A: 255},
		OnStartup:        rt.Startup,
		OnShutdown:       rt.Shutdown,
		Bind:             []any{rt.Bench()},
	})
}
