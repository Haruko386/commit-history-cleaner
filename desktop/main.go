package main

import (
	"context"
	"embed"
	"log"

	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

const singleInstanceID = "com.haruko386.commit-history-cleaner.single-instance"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	backendCtx, cancelBackend := context.WithCancel(context.Background())
	app.shutdown = cancelBackend

	backend, backendAddr, desktopToken, err := startBackend(backendCtx)
	if err != nil {
		cancelBackend()
		log.Fatalf("start desktop backend: %v", err)
	}

	app.backend = backend

	err = wails.Run(&options.App{
		Title:  "FunPDF " + common.GetVersion(),
		Width:  1280,
		Height: 860,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: newAPIProxy(backendAddr, desktopToken),
		},
		SingleInstanceLock: newSingleInstanceLock(app),
		OnStartup:          app.Start,
		OnShutdown:         app.Shutdown,
	})

	if err != nil {
		log.Fatalf("start wails: %v", err)
	}
}

func newSingleInstanceLock(app *App) *options.SingleInstanceLock {
	return &options.SingleInstanceLock{
		UniqueId: singleInstanceID,
		OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
			log.Printf("second FunPDF instance ignored: args=%v", secondInstanceData.Args)
			app.Focus()
		},
	}
}
