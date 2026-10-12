package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/Haruko386/commit-history-cleaner/internal"
	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/Haruko386/commit-history-cleaner/internal/handler"
	"github.com/Haruko386/commit-history-cleaner/internal/middleware"
	"github.com/Haruko386/commit-history-cleaner/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	backendHost      = "127.0.0.1"
	backendStartPort = 38600
	backendMaxPort   = 38800
)

func newAPIProxy(backendAddr, desktopToken string) http.Handler {
	target, err := url.Parse("http://" + backendAddr)
	if err != nil {
		panic(fmt.Errorf("parse backend address %q: %w", backendAddr, err))
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Commit-History-Cleaner-Host", req.Host)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

func listenBackend() (net.Listener, string, error) {
	var lastErr error

	for p := backendStartPort; p <= backendMaxPort; p++ {
		addr := fmt.Sprintf("%s:%d", backendHost, p)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			return listener, addr, nil
		}
		lastErr = err
		log.Printf("failed to listen on port %d: %v", p, err)
	}
	return nil, "", fmt.Errorf("no available desktop backend port in %s:%d-%d: last error: %w",
		backendHost,
		backendStartPort,
		backendMaxPort,
		lastErr,
	)
}

func startBackend(ctx context.Context) (*http.Server, string, string, error) {
	desktopToken := common.GenerateUUID()
	taskSvr := service.NewTaskSvr()

	taskHandler := handler.NewTaskHandler(taskSvr)
	healthHandler := handler.NewHealthHandler(service.NewHealthSvr())
	repoHandler := handler.NewRepositoriesHandler(service.NewRepositoriesSvr(taskSvr))

	r := gin.Default()
	r.Use(middleware.RequestID())

	router := internal.NewRouter(healthHandler, repoHandler, taskHandler)
	router.Setup(r)

	listener, backendAddr, err := listenBackend()
	if err != nil {
		return nil, "", "", err
	}

	httpSvr := &http.Server{
		Addr:              backendAddr,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       10 * time.Second,
	}

	go func() {
		if err := httpSvr.Serve(listener); err != nil {
			log.Fatalf("failed to serve backend: %v", err)
		}
	}()
	log.Printf("desktop backend listening on http://%s", backendAddr)

	return httpSvr, backendAddr, desktopToken, nil
}
