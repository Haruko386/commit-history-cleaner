package main

import (
	"context"
	"log"
	"net/http"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx      context.Context
	shutdown context.CancelFunc
	backend  *http.Server
}

func NewApp() *App {
	return &App{}
}

// Start stores the Wails application context for later desktop operations.
func (a *App) Start(ctx context.Context) {
	a.ctx = ctx
}

// Focus brings the existing desktop window to the foreground.
func (a *App) Focus() {
	if a.ctx == nil {
		return
	}

	wailsRuntime.WindowShow(a.ctx)
	wailsRuntime.WindowUnminimise(a.ctx)

	wailsRuntime.WindowSetAlwaysOnTop(a.ctx, true)
	wailsRuntime.WindowSetAlwaysOnTop(a.ctx, false)
}

// Shutdown handles application shutdown cleanup.
func (a *App) Shutdown() {
	if a.backend != nil {
		toCtx, cancel := context.WithTimeout(a.ctx, time.Second*5)
		defer cancel()
		err := a.backend.Shutdown(toCtx)
		if err != nil {
			log.Printf("Error shutting down backend: %v", err)
		}
	}
	if a.shutdown != nil {
		a.shutdown()
	}
}
