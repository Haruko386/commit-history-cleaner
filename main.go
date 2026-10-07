package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"example.com/m/v2/internal"
	"example.com/m/v2/internal/handler"
	"example.com/m/v2/internal/middleware"
	"example.com/m/v2/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	healthHandler := handler.NewHealthHandler(service.NewHealthSvr())
	repoHandler := handler.NewRepositoriesHandler(service.NewRepositoriesSvr())

	r := gin.Default()
	r.Use(middleware.RequestID())

	router := internal.NewRouter(healthHandler, repoHandler)
	router.Setup(r)

	srv := &http.Server{
		Addr:              "127.0.0.1:9384",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("start backend: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}
