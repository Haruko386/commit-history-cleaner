package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appinternal "github.com/Haruko386/commit-history-cleaner/internal"
	"github.com/Haruko386/commit-history-cleaner/internal/handler"
	"github.com/Haruko386/commit-history-cleaner/internal/service"
	"github.com/gin-gonic/gin"
)

func TestHealthRouteUsesVersionedPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	taskService := service.NewTaskSvr()
	appinternal.NewRouter(
		handler.NewHealthHandler(stubHealthChecker{version: "2.47.1"}),
		handler.NewRepositoriesHandler(service.NewRepositoriesSvr(taskService)),
		handler.NewTaskHandler(taskService),
	).Setup(engine)

	versioned := httptest.NewRecorder()
	engine.ServeHTTP(versioned, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if versioned.Code != http.StatusOK {
		t.Fatalf("versioned route status = %d, want %d", versioned.Code, http.StatusOK)
	}

	legacy := httptest.NewRecorder()
	engine.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if legacy.Code != http.StatusNotFound {
		t.Fatalf("legacy route status = %d, want %d", legacy.Code, http.StatusNotFound)
	}
}
