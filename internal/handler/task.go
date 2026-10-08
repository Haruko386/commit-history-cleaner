package handler

import (
	"net/http"
	"strings"

	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/Haruko386/commit-history-cleaner/internal/middleware"
	"github.com/Haruko386/commit-history-cleaner/internal/service"
	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	taskSvr *service.TaskSvr
}

func NewTaskHandler(taskSvr *service.TaskSvr) *TaskHandler {
	return &TaskHandler{taskSvr: taskSvr}
}

func (h *TaskHandler) GetTask(c *gin.Context) {
	requestId := middleware.GetRequestID(c)

	taskId := strings.TrimSpace(c.Param("taskID"))
	if taskId == "" {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "taskId should not be empty", false, requestId))
		return
	}

	task, err := h.taskSvr.GetTask(taskId)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewErrorResponse(common.TaskNotFound, err.Error(), false, requestId))
		return
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(task, requestId))
}

func (h *TaskHandler) CancelTask(c *gin.Context) {
	requestId := middleware.GetRequestID(c)

	taskId := strings.TrimSpace(c.Param("taskID"))
	if taskId == "" {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(common.InvalidRequest, "taskId should not be empty", false, requestId))
		return
	}

	if err := h.taskSvr.CancelTask(taskId); err != nil {
		c.JSON(http.StatusNotFound, common.NewErrorResponse(common.TaskNotFound, err.Error(), false, requestId))
		return
	}
	c.Status(http.StatusNoContent)
}
