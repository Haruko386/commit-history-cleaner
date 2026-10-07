package handler

import (
	"net/http"

	"example.com/m/v2/internal/common"
	"example.com/m/v2/internal/middleware"
	"example.com/m/v2/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	output["id"] = "repo_" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(data.Path)).String()
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
	output["id"] = "repo_" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(data.Path)).String()
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

	ok := h.repoSvr.ExitCurrentRepository()
	if !ok {
		c.JSON(http.StatusNotFound, common.NewErrorResponse(common.InternalError, "can not close repo", false, requestID))
		return
	}

	c.Status(http.StatusNoContent)
}
