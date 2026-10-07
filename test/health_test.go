package test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/m/v2/internal/handler"
	"example.com/m/v2/internal/middleware"
	"example.com/m/v2/internal/service"
	"github.com/gin-gonic/gin"
)

type stubHealthChecker struct {
	version         string
	err             error
	githubConnected bool
	githubErr       error
}

func (s stubHealthChecker) CheckHealth(context.Context) (string, error) {
	return s.version, s.err
}

func (s stubHealthChecker) CheckGithubConnection(context.Context, string) (bool, error) {
	return s.githubConnected, s.githubErr
}

func performHealthRequest(checker handler.HealthChecker, requestID string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/api/v1/health", handler.NewHealthHandler(checker).CheckHealth)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	if requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestCheckHealthSuccess(t *testing.T) {
	response := performHealthRequest(stubHealthChecker{version: "2.47.1"}, "req_test")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if value := response.Header().Get("X-Request-ID"); value != "req_test" {
		t.Fatalf("X-Request-ID = %q, want %q", value, "req_test")
	}

	var body struct {
		Data handler.HealthData `json:"data"`
		Meta struct {
			RequestID string `json:"requestId"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Status != "ok" || !body.Data.Git.Available || body.Data.Git.Version != "2.47.1" {
		t.Fatalf("unexpected health data: %+v", body.Data)
	}
	if body.Meta.RequestID != "req_test" {
		t.Fatalf("meta.requestId = %q, want %q", body.Meta.RequestID, "req_test")
	}
}

func TestCheckHealthErrors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		status    int
		code      string
		retryable bool
	}{
		{name: "git unavailable", err: service.ErrGitNotAvailable, status: http.StatusServiceUnavailable, code: "GIT_NOT_AVAILABLE", retryable: false},
		{name: "timeout", err: context.DeadlineExceeded, status: http.StatusServiceUnavailable, code: "HEALTH_CHECK_TIMEOUT", retryable: true},
		{name: "cancelled", err: context.Canceled, status: http.StatusServiceUnavailable, code: "HEALTH_CHECK_CANCELLED", retryable: true},
		{name: "unexpected", err: errors.New("unexpected"), status: http.StatusInternalServerError, code: "HEALTH_CHECK_FAILED", retryable: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performHealthRequest(stubHealthChecker{err: test.err}, "")
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}

			requestID := response.Header().Get("X-Request-ID")
			if !strings.HasPrefix(requestID, "req_") {
				t.Fatalf("X-Request-ID = %q, want generated req_ prefix", requestID)
			}

			var body struct {
				Error struct {
					Code      string `json:"code"`
					Retryable bool   `json:"retryable"`
				} `json:"error"`
				Meta struct {
					RequestID string `json:"requestId"`
				} `json:"meta"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error.Code != test.code || body.Error.Retryable != test.retryable {
				t.Fatalf("unexpected error response: %+v", body.Error)
			}
			if body.Meta.RequestID != requestID {
				t.Fatalf("body requestId = %q, header requestId = %q", body.Meta.RequestID, requestID)
			}
		})
	}
}

func TestCheckGithubConnection(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		checker   stubHealthChecker
		status    int
		errorCode string
	}{
		{name: "missing header", status: http.StatusBadRequest, errorCode: "INVALID_REQUEST"},
		{name: "missing scheme", header: "token", status: http.StatusBadRequest, errorCode: "INVALID_REQUEST"},
		{name: "empty token", header: "Bearer", status: http.StatusBadRequest, errorCode: "INVALID_REQUEST"},
		{name: "wrong scheme", header: "Basic token", status: http.StatusBadRequest, errorCode: "INVALID_REQUEST"},
		{name: "unauthorized", header: "Bearer token", checker: stubHealthChecker{githubErr: service.ErrGithubUnauthorized}, status: http.StatusUnauthorized, errorCode: "GITHUB_UNAUTHORIZED"},
		{name: "forbidden", header: "Bearer token", checker: stubHealthChecker{githubErr: service.ErrGithubForbidden}, status: http.StatusForbidden, errorCode: "GITHUB_FORBIDDEN"},
		{name: "connection failed", header: "Bearer token", checker: stubHealthChecker{githubErr: errors.New("failed")}, status: http.StatusServiceUnavailable, errorCode: "GITHUB_CONNECTION_FAILED"},
		{name: "connected", header: "Bearer token", checker: stubHealthChecker{githubConnected: true}, status: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(middleware.RequestID())
			router.GET("/api/v1/github/connection", handler.NewHealthHandler(test.checker).CheckGithubConnection)

			request := httptest.NewRequest(http.MethodGet, "/api/v1/github/connection", nil)
			if test.header != "" {
				request.Header.Set("Authorization", test.header)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}
			if test.errorCode != "" {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if body.Error.Code != test.errorCode {
					t.Fatalf("code = %q, want %q", body.Error.Code, test.errorCode)
				}
			}
		})
	}
}
