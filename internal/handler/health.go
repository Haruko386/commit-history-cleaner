package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/Haruko386/commit-history-cleaner/internal/middleware"
	"github.com/Haruko386/commit-history-cleaner/internal/service"
	"github.com/gin-gonic/gin"
)

type HealthChecker interface {
	CheckHealth(ctx context.Context) (string, error)
	CheckGithubConnection(ctx context.Context, token string) (bool, error)
}

type HealthHandler struct {
	healthSvr HealthChecker
}

type HealthData struct {
	Status  string        `json:"status"`
	Version string        `json:"version"`
	Git     GitHealthData `json:"git"`
}

type GitHealthData struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
}

func NewHealthHandler(healthSvr HealthChecker) *HealthHandler {
	return &HealthHandler{healthSvr: healthSvr}
}

func (h *HealthHandler) CheckHealth(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	gitVersion, err := h.healthSvr.CheckHealth(c.Request.Context())
	if err != nil {
		status, code, message, retryable := healthError(err)
		c.JSON(status, common.NewErrorResponse(code, message, retryable, requestID))
		return
	}

	data := HealthData{
		Status:  "ok",
		Version: common.GetVersion(),
		Git: GitHealthData{
			Available: true,
			Version:   gitVersion,
		},
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(data, requestID))
}

func healthError(err error) (status int, code, message string, retryable bool) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusServiceUnavailable, common.HealthCheckTimeout, "The health check timed out.", true
	case errors.Is(err, context.Canceled):
		return http.StatusServiceUnavailable, common.HealthCheckCancelled, "The health check was cancelled.", true
	case errors.Is(err, service.ErrGitNotAvailable):
		return http.StatusServiceUnavailable, common.GitNotAvailable, "Git is not available. Install Git and ensure it is available on PATH.", false
	default:
		return http.StatusInternalServerError, common.HealthCheckFailed, "The health check failed.", false
	}
}

func (h *HealthHandler) CheckGithubConnection(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	authHeader := strings.TrimSpace(c.Request.Header.Get("Authorization"))
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(
			common.InvalidRequest,
			"auth header is required.",
			false,
			requestID,
		))
		return
	}

	scheme, accessToken, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(accessToken) == "" {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(
			common.InvalidRequest,
			"GitHub access token is required.",
			false,
			requestID,
		))
		return
	}

	ok, err := h.healthSvr.CheckGithubConnection(c.Request.Context(), accessToken)
	if err != nil {
		if errors.Is(err, service.ErrGithubUnauthorized) {
			c.JSON(http.StatusUnauthorized, common.NewErrorResponse(
				common.GithubUnauthorized,
				"The GitHub access token is invalid.",
				false,
				requestID,
			))
			return
		}
		if errors.Is(err, service.ErrGithubForbidden) {
			c.JSON(http.StatusForbidden, common.NewErrorResponse(
				common.GithubForbidden,
				"The GitHub access token does not have access.",
				false,
				requestID,
			))
			return
		}
		c.JSON(http.StatusServiceUnavailable, common.NewErrorResponse(
			common.GithubConnectionFailed,
			"The GitHub connection check failed.",
			true,
			requestID,
		))
		return
	}
	c.JSON(http.StatusOK, common.NewSuccessResponse(ok, requestID))
}
