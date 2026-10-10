package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/Haruko386/commit-history-cleaner/internal/entity"
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

func (h *RepositoriesHandler) GetCurrentCommits(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	query := entity.CommitsQuery{}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "Query parameters must be valid.", false, requestID))
		return
	}

	if query.Since != nil && query.Until != nil && query.Since.After(*query.Until) {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "start time should smaller than end time", false, requestID))
		return
	}

	data, cursor, err := h.repoSvr.GetCurrentCommits(&query)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNoRepositoryOpen):
			c.JSON(http.StatusNotFound, common.NewErrorResponse(common.NoRepositoryOpen, "No repository opened.", false, requestID))
		case errors.Is(err, service.ErrRepositoryNotScanned):
			c.JSON(http.StatusConflict, common.NewErrorResponse(common.RepositoryNotScanned, "Scan the repository before requesting commits.", false, requestID))
		case errors.Is(err, service.ErrInvalidCommitCursor):
			c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "The commit cursor is invalid.", false, requestID))
		case errors.Is(err, service.ErrUnsupportedRef):
			c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "The ref does not exist or is ambiguous.", false, requestID))
		case errors.Is(err, service.ErrInvalidCommitQuery):
			c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "Minimum introduced bytes must be zero or greater.", false, requestID))
		default:
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.InternalError, "The commit list could not be loaded.", false, requestID))
		}
		return
	}

	c.JSON(http.StatusOK, common.NewSuccessResponseWithCursor(data, requestID, cursor, cursor != ""))
}

func (h *RepositoriesHandler) GetCommit(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	sha := strings.TrimSpace(c.Param("sha"))
	if sha == "" {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "Commit SHA is required.", false, requestID))
		return
	}

	commitInfo, err := h.repoSvr.GetCommit(sha)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCommitSHA):
			c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "Commit SHA must be a full 40-character hexadecimal value.", false, requestID))
		case errors.Is(err, service.ErrNoRepositoryOpen):
			c.JSON(http.StatusNotFound, common.NewErrorResponse(common.NoRepositoryOpen, "No repository opened.", false, requestID))
		case errors.Is(err, service.ErrRepositoryNotScanned):
			c.JSON(http.StatusConflict, common.NewErrorResponse(common.RepositoryNotScanned, "Scan the repository before requesting a commit.", false, requestID))
		case errors.Is(err, service.ErrCommitNotFound):
			c.JSON(http.StatusNotFound, common.NewErrorResponse(common.CommitNotFound, "The commit was not found in the current repository.", false, requestID))
		default:
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.InternalError, "The commit could not be loaded.", false, requestID))
		}
		return
	}

	data := map[string]any{
		"sha":      commitInfo.SHA,
		"shortSha": commitInfo.SHA[:7],
		"subject":  strings.Split(commitInfo.Message, "\n")[0],
		"author": map[string]any{
			"name":  commitInfo.AuthorName,
			"email": commitInfo.AuthorEmail,
		},
		"authoredAt":  commitInfo.AuthoredAt,
		"committedAt": commitInfo.CommittedAt,
		"parents":     commitInfo.ParentSHAs,
		"refs":        commitInfo.Refs,
		"stats":       commitInfo.Stats,
		"body":        commitInfo.Message,
		"committer": map[string]any{
			"name":  commitInfo.Commiter.Name,
			"email": commitInfo.Commiter.Email,
		},
	}
	c.JSON(http.StatusOK, common.NewSuccessResponse(data, requestID))
}

func (h *RepositoriesHandler) GetFiles(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	sha := strings.TrimSpace(c.Param("sha"))
	if sha == "" {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "Commit SHA is required.", false, requestID))
		return
	}

	query := entity.CommitFileQuery{}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "Query parameters must be valid.", false, requestID))
		return
	}

	data, cursor, err := h.repoSvr.GetFiles(sha, query)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCommitSHA), errors.Is(err, service.ErrInvalidCommitCursor), errors.Is(err, service.ErrInvalidCommitQuery):
			c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "The commit file query is invalid.", false, requestID))
		case errors.Is(err, service.ErrNoRepositoryOpen):
			c.JSON(http.StatusNotFound, common.NewErrorResponse(common.NoRepositoryOpen, "No repository opened.", false, requestID))
		case errors.Is(err, service.ErrRepositoryNotScanned):
			c.JSON(http.StatusConflict, common.NewErrorResponse(common.RepositoryNotScanned, "Scan the repository before requesting commit files.", false, requestID))
		case errors.Is(err, service.ErrCommitNotFound):
			c.JSON(http.StatusNotFound, common.NewErrorResponse(common.CommitNotFound, "The commit was not found in the current repository.", false, requestID))
		default:
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.InternalError, "The commit files could not be loaded.", false, requestID))
		}
		return
	}
	c.JSON(http.StatusOK, common.NewSuccessResponseWithCursor(data, requestID, cursor, cursor != ""))
}

func (h *RepositoriesHandler) Cleanup(c *gin.Context) {
	requestID := middleware.GetRequestID(c)

	type cleanupReq struct {
		CommitSHAs   []string `json:"commitShas"`
		ExpectedHead string   `json:"expectedHead"`
		AutoStash    bool     `json:"autoStash"`
	}

	req := cleanupReq{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "Request body could not be parsed.", false, requestID))
		return
	}

	newHead, err := h.repoSvr.Cleanup(c.Request.Context(), req.CommitSHAs, req.ExpectedHead, req.AutoStash)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCleanup), errors.Is(err, service.ErrInvalidCommitSHA):
			c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "The cleanup request is invalid.", false, requestID))
		case errors.Is(err, service.ErrNoRepositoryOpen):
			c.JSON(http.StatusNotFound, common.NewErrorResponse(common.NoRepositoryOpen, "No repository opened.", false, requestID))
		case errors.Is(err, service.ErrCommitNotFound):
			c.JSON(http.StatusNotFound, common.NewErrorResponse(common.CommitNotFound, "A selected commit was not found in the current repository.", false, requestID))
		case errors.Is(err, service.ErrRepositoryNotScanned):
			c.JSON(http.StatusConflict, common.NewErrorResponse(common.RepositoryNotScanned, "Scan the repository before cleaning its history.", false, requestID))
		case errors.Is(err, service.ErrRepositoryChanged):
			c.JSON(http.StatusConflict, common.NewErrorResponse(common.RepositoryChanged, "The repository HEAD changed. Refresh and confirm the cleanup again.", false, requestID))
		case errors.Is(err, service.ErrWorkingTreeDirty):
			c.JSON(http.StatusConflict, common.NewErrorResponse(common.WorkingTreeDirty, "Commit or stash working tree changes before cleanup, or enable auto stash.", false, requestID))
		case errors.Is(err, service.ErrCleanupUnsupported):
			c.JSON(http.StatusUnprocessableEntity, common.NewErrorResponse(common.CleanupUnsupported, "The selected commits cannot be cleaned automatically.", false, requestID))
		case errors.Is(err, service.ErrGitCommandFailed):
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.GitCommandFailed, "Git could not rewrite the selected history.", false, requestID))
		default:
			c.JSON(http.StatusInternalServerError, common.NewErrorResponse(common.InternalError, "The selected history could not be cleaned.", false, requestID))
		}
		return
	}

	data := map[string]any{
		"previousHead":      req.ExpectedHead,
		"newHead":           newHead,
		"droppedCommitShas": req.CommitSHAs,
	}
	c.JSON(http.StatusOK, common.NewSuccessResponse(data, requestID))
}
