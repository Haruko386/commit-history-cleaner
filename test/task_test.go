package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/m/v2/internal/common"
	"example.com/m/v2/internal/entity"
	"example.com/m/v2/internal/handler"
	"example.com/m/v2/internal/middleware"
	"example.com/m/v2/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5"
)

func TestTaskLifecycle(t *testing.T) {
	repositoryPath := t.TempDir()
	if _, err := git.PlainInit(repositoryPath, false); err != nil {
		t.Fatalf("initialize repository: %v", err)
	}

	taskService := service.NewTaskSvr()
	repositoryService := service.NewRepositoriesSvr(taskService)
	if _, _, err := repositoryService.OpenRepository(repositoryPath); err != nil {
		t.Fatalf("open repository: %v", err)
	}
	task, err := repositoryService.ScanRepository(false)
	if err != nil {
		t.Fatalf("start scan: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	taskHandler := handler.NewTaskHandler(taskService)
	router.GET("/api/v1/tasks/:taskID", taskHandler.GetTask)
	router.DELETE("/api/v1/tasks/:taskID", taskHandler.CancelTask)

	request := func(method, taskID string) *httptest.ResponseRecorder {
		t.Helper()
		httpRequest := httptest.NewRequest(method, "/api/v1/tasks/"+taskID, nil)
		httpRequest.Header.Set("X-Request-ID", "req_task_test")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httpRequest)
		return response
	}

	response := request(http.MethodGet, task.TaskID)
	if response.Code != http.StatusOK {
		t.Fatalf("get task status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var found struct {
		Data entity.Task         `json:"data"`
		Meta common.ResponseMeta `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &found); err != nil {
		t.Fatalf("decode task response: %v", err)
	}
	if found.Data.TaskID != task.TaskID || found.Data.Status != entity.Queued || found.Meta.RequestID != "req_task_test" {
		t.Fatalf("unexpected task response: %+v", found)
	}

	response = request(http.MethodDelete, task.TaskID)
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("cancel task status = %d, body=%q", response.Code, response.Body.String())
	}

	response = request(http.MethodGet, task.TaskID)
	if response.Code != http.StatusOK {
		t.Fatalf("get cancelled task status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &found); err != nil {
		t.Fatalf("decode cancelled task response: %v", err)
	}
	if found.Data.Status != entity.Cancelled || found.Data.FinishedAt == nil {
		t.Fatalf("unexpected cancelled task: %+v", found.Data)
	}

	response = request(http.MethodGet, "task_missing")
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing task status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}
	var missing common.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &missing); err != nil {
		t.Fatalf("decode missing task response: %v", err)
	}
	if missing.Error.Code != common.TaskNotFound {
		t.Fatalf("missing task code = %q, want %q", missing.Error.Code, common.TaskNotFound)
	}
}
