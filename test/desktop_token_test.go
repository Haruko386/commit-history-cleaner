package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appinternal "github.com/Haruko386/commit-history-cleaner/internal"
	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/Haruko386/commit-history-cleaner/internal/handler"
	"github.com/Haruko386/commit-history-cleaner/internal/middleware"
	"github.com/Haruko386/commit-history-cleaner/internal/service"
	"github.com/gin-gonic/gin"
)

// newDesktopEngine mirrors the desktop backend wiring: the request ID middleware
// runs first, then the desktop token guard, and only then are the API routes
// registered.
func newDesktopEngine(token string, checker handler.HealthChecker) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequestID())
	engine.Use(middleware.DesktopToken(token))

	taskService := service.NewTaskSvr()
	appinternal.NewRouter(
		handler.NewHealthHandler(checker),
		handler.NewRepositoriesHandler(service.NewRepositoriesSvr(taskService)),
		handler.NewTaskHandler(taskService),
	).Setup(engine)

	return engine
}

func TestDesktopTokenGuard(t *testing.T) {
	const token = "desktop-secret"

	tests := []struct {
		name      string
		expected  string
		header    string
		status    int
		errorCode string
	}{
		{name: "missing token header", expected: token, status: http.StatusUnauthorized, errorCode: common.Unauthorized},
		{name: "wrong token", expected: token, header: "not-the-token", status: http.StatusUnauthorized, errorCode: common.Unauthorized},
		{name: "valid token", expected: token, header: token, status: http.StatusOK},
		{name: "empty expected token disables the check", expected: "", status: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := newDesktopEngine(test.expected, stubHealthChecker{version: "2.47.1"})

			request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			if test.header != "" {
				request.Header.Set(common.DesktopTokenHeader, test.header)
			}

			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}
			if test.errorCode == "" {
				return
			}

			var body struct {
				Error struct {
					Code      string `json:"code"`
					Retryable bool   `json:"retryable"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error.Code != test.errorCode {
				t.Fatalf("code = %q, want %q", body.Error.Code, test.errorCode)
			}
			if body.Error.Retryable {
				t.Fatalf("retryable = true, want false")
			}
		})
	}
}

// TestDesktopTokenKeepsGithubAuthorizationSeparate verifies the desktop token is
// enforced through its own header and does not interfere with the GitHub
// Authorization header used by CheckGithubConnection.
func TestDesktopTokenKeepsGithubAuthorizationSeparate(t *testing.T) {
	const token = "desktop-secret"

	tests := []struct {
		name          string
		desktopToken  string
		authorization string
		status        int
		errorCode     string
	}{
		{name: "both headers present", desktopToken: token, authorization: "Bearer token", status: http.StatusOK},
		{name: "desktop token present, authorization missing", desktopToken: token, status: http.StatusBadRequest, errorCode: "INVALID_REQUEST"},
		{name: "authorization present, desktop token missing", authorization: "Bearer token", status: http.StatusUnauthorized, errorCode: common.Unauthorized},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine := newDesktopEngine(token, stubHealthChecker{githubConnected: true})

			request := httptest.NewRequest(http.MethodGet, "/api/v1/github/connection", nil)
			if test.desktopToken != "" {
				request.Header.Set(common.DesktopTokenHeader, test.desktopToken)
			}
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}

			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}
			if test.errorCode == "" {
				return
			}

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
		})
	}
}
