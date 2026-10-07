package handler

import (
	"context"
	"errors"
	"net/http"

	"example.com/m/v2/internal/common"
	"example.com/m/v2/internal/middleware"
	"example.com/m/v2/internal/service"
	"github.com/gin-gonic/gin"
)

type HealthChecker interface {
	CheckHealth(ctx context.Context) (string, error)
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
		return http.StatusServiceUnavailable, "HEALTH_CHECK_TIMEOUT", "The health check timed out.", true
	case errors.Is(err, context.Canceled):
		return http.StatusServiceUnavailable, "HEALTH_CHECK_CANCELLED", "The health check was cancelled.", true
	case errors.Is(err, service.ErrGitNotAvailable):
		return http.StatusServiceUnavailable, "GIT_NOT_AVAILABLE", "Git is not available. Install Git and ensure it is available on PATH.", false
	default:
		return http.StatusInternalServerError, "HEALTH_CHECK_FAILED", "The health check failed.", false
	}
}
