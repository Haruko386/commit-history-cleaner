package handler

import (
	"errors"
	"net/http"

	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/Haruko386/commit-history-cleaner/internal/middleware"
	"github.com/Haruko386/commit-history-cleaner/internal/service"
	"github.com/gin-gonic/gin"
)

type RepositoriesHandler struct {
	repoSvr *service.RepositoriesSvr
}

func NewRepositoriesHandler(repoSvr *service.RepositoriesSvr) *RepositoriesHandler {
	return &RepositoriesHandler{repoSvr: repoSvr}
}

func (h *RepositoriesHandler) OpenRepository(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	type path struct {
		Path string `json:"path"`
	}
	repoPath := path{}

	if err := c.ShouldBindJSON(&repoPath); err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(
			common.InvalidRequest,
			"Request body must be valid JSON.",
			false,
			requestID,
		))
		return
	}

	code, data, err := h.repoSvr.OpenRepository(repoPath.Path)
	if err != nil {
		if code == "" {
			code = common.InternalError
		}
		status := http.StatusInternalServerError
		switch code {
		case common.InvalidRequest, common.PathRequired:
			status = http.StatusBadRequest
		case common.RepositoryNotFound:
			status = http.StatusNotFound
		case common.NotAGitRepository, common.PathNotReadable:
			status = http.StatusUnprocessableEntity
		}
		message := err.Error()
		if code == common.InternalError {
			message = "The repository could not be opened."
		}
		c.JSON(status, common.NewErrorResponse(code, message, false, requestID))
		return
	}

	output := make(map[string]any)
	output["id"] = common.GenerateRepoID(data.Path)
	output["name"] = data.Name
	output["path"] = data.Path
	output["branch"] = data.Branch
	output["detachedHead"] = data.DetachedHead
	output["head"] = data.Head
	output["commitCount"] = data.CommitCount
	output["workingTreeStatus"] = data.WorkingTreeStatus
	output["gitDirectoryBytes"] = data.GitDirectoryBytes
	output["analysisStatus"] = data.AnalysisStatus
	output["openedAt"] = data.OpenedAt

	c.JSON(http.StatusOK, common.NewSuccessResponse(output, requestID))
}

func (h *RepositoriesHandler) GetCurrentRepository(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	httpCode, data, code := h.repoSvr.GetCurrentRepository()
	if httpCode != http.StatusOK {
		c.JSON(httpCode, common.NewErrorResponse(code, "No repository opened", false, requestID))
		return
	}

	output := make(map[string]any)
	output["id"] = common.GenerateRepoID(data.Path)
	output["name"] = data.Name
	output["path"] = data.Path
	output["branch"] = data.Branch
	output["detachedHead"] = data.DetachedHead
	output["head"] = data.Head
	output["commitCount"] = data.CommitCount
	output["workingTreeStatus"] = data.WorkingTreeStatus
	output["gitDirectoryBytes"] = data.GitDirectoryBytes
	output["analysisStatus"] = data.AnalysisStatus
	output["openedAt"] = data.OpenedAt

	c.JSON(http.StatusOK, common.NewSuccessResponse(output, requestID))
}

func (h *RepositoriesHandler) ExitCurrentRepository(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	ok, err := h.repoSvr.ExitCurrentRepository()
	if !ok {
		c.JSON(http.StatusNotFound, common.NewErrorResponse(common.InternalError, "can not close repo", false, requestID))
		return
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, common.NewErrorResponse(common.InternalError, err.Error(), false, requestID))
	}

	c.Status(http.StatusNoContent)
}

func (h *RepositoriesHandler) ScanRepository(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	type Req struct {
		Force bool `json:"force"`
	}
	req := new(Req)

	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "Request body must contain a valid boolean force field.", false, requestID))
		return
	}

	data, err := h.repoSvr.ScanRepository(req.Force)
	if err != nil {
		switch {
		// repository not opened
		case errors.Is(err, service.ErrNoRepositoryOpen):
			c.JSON(http.StatusNotFound, common.NewErrorResponse(common.NoRepositoryOpen, err.Error(), false, requestID))
			return
		// scan already queued
		case errors.Is(err, service.ErrScanAlreadyQueued):
			c.JSON(http.StatusConflict, common.NewErrorResponseWithDetails(common.ScanAlreadyRunning, err.Error(), data.TaskID, false, requestID))
			return
		// scan already running
		case errors.Is(err, service.ErrScanAlreadyRunning):
			c.JSON(http.StatusConflict, common.NewErrorResponseWithDetails(common.ScanAlreadyRunning, err.Error(), data.TaskID, false, requestID))
			return
		// scan already completed
		case errors.Is(err, service.ErrScanAlreadyCompleted):
			c.JSON(http.StatusConflict, common.NewErrorResponseWithDetails(common.ScanAlreadyCompleted, err.Error(), data.TaskID, false, requestID))
			return
		// scan already canceled
		case errors.Is(err, service.ErrScanAlreadyCancelled):
			c.JSON(http.StatusConflict, common.NewErrorResponseWithDetails(common.ScanAlreadyCancelled, err.Error(), data.TaskID, false, requestID))
			return
		default:
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.InternalError, "The repository scan could not be started.", false, requestID))
			return
		}
	}

	c.JSON(http.StatusAccepted, common.NewSuccessResponse(data, requestID))
}
