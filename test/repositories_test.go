package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"example.com/m/v2/internal/common"
	"example.com/m/v2/internal/entity"
	"example.com/m/v2/internal/handler"
	"example.com/m/v2/internal/middleware"
	"example.com/m/v2/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestOpenEmptyRepository(t *testing.T) {
	repositoryPath := t.TempDir()
	if _, err := git.PlainInit(repositoryPath, false); err != nil {
		t.Fatalf("initialize repository: %v", err)
	}

	code, data, err := service.NewRepositoriesSvr(service.NewTaskSvr()).OpenRepository(repositoryPath)
	if err != nil {
		t.Fatalf("open repository: code=%q error=%v", code, err)
	}
	if code != "" {
		t.Fatalf("code = %q, want empty", code)
	}
	if data.Branch != nil || data.Head != nil || data.DetachedHead {
		t.Fatalf("unexpected empty repository HEAD data: %+v", data)
	}
	if data.CommitCount != 0 || data.WorkingTreeStatus != "unborn" {
		t.Fatalf("unexpected empty repository state: %+v", data)
	}
	if data.GitDirectoryBytes <= 0 {
		t.Fatalf("unexpected repository metadata: %+v", data)
	}
	if !filepath.IsAbs(data.Path) || data.AnalysisStatus != "not_scanned" {
		t.Fatalf("unexpected normalized data: %+v", data)
	}
}

func TestOpenDetachedRepository(t *testing.T) {
	repositoryPath := t.TempDir()
	repository, err := git.PlainInit(repositoryPath, false)
	if err != nil {
		t.Fatalf("initialize repository: %v", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		t.Fatalf("open worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repositoryPath, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := worktree.Add("README.md"); err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	hash, err := worktree.Commit("test", &git.CommitOptions{Author: &object.Signature{
		Name:  "Test",
		Email: "test@example.com",
		When:  time.Now(),
	}})
	if err != nil {
		t.Fatalf("commit fixture: %v", err)
	}
	if err := repository.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, hash)); err != nil {
		t.Fatalf("detach HEAD: %v", err)
	}

	_, data, err := service.NewRepositoriesSvr(service.NewTaskSvr()).OpenRepository(repositoryPath)
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	if !data.DetachedHead || data.Branch != nil {
		t.Fatalf("detached HEAD data = %+v", data)
	}
	if data.Head == nil || *data.Head != hash.String() || data.CommitCount != 1 {
		t.Fatalf("unexpected HEAD data: %+v", data)
	}
	if data.WorkingTreeStatus != "clean" {
		t.Fatalf("workingTreeStatus = %q, want clean", data.WorkingTreeStatus)
	}
}

func TestOpenRepositoryErrors(t *testing.T) {
	nonRepositoryPath := t.TempDir()
	missingPath := filepath.Join(t.TempDir(), "missing")

	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{name: "malformed JSON", body: `{`, status: http.StatusBadRequest, code: common.InvalidRequest},
		{name: "missing path", body: `{}`, status: http.StatusBadRequest, code: common.PathRequired},
		{name: "path not found", body: repositoryRequestBody(t, missingPath), status: http.StatusNotFound, code: common.RepositoryNotFound},
		{name: "not a repository", body: repositoryRequestBody(t, nonRepositoryPath), status: http.StatusUnprocessableEntity, code: common.NotAGitRepository},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performRepositoryRequest(test.body)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}

			var body common.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error.Code != test.code {
				t.Fatalf("code = %q, want %q", body.Error.Code, test.code)
			}
		})
	}
}

func TestOpenRepositoryResponseUsesSeparateIDs(t *testing.T) {
	repositoryPath := t.TempDir()
	if _, err := git.PlainInit(repositoryPath, false); err != nil {
		t.Fatalf("initialize repository: %v", err)
	}

	response := performRepositoryRequest(repositoryRequestBody(t, filepath.Join(repositoryPath, ".")))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	var body struct {
		Data struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"data"`
		Meta common.ResponseMeta `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.ID == "" || body.Data.ID == body.Meta.RequestID {
		t.Fatalf("repository id %q must differ from request id %q", body.Data.ID, body.Meta.RequestID)
	}
	if body.Data.Path != filepath.Clean(repositoryPath) {
		t.Fatalf("path = %q, want %q", body.Data.Path, filepath.Clean(repositoryPath))
	}
}

func TestCurrentRepositoryLifecycle(t *testing.T) {
	repositoryPath := t.TempDir()
	if _, err := git.PlainInit(repositoryPath, false); err != nil {
		t.Fatalf("initialize repository: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	repositoryHandler := handler.NewRepositoriesHandler(service.NewRepositoriesSvr(service.NewTaskSvr()))
	router.POST("/api/v1/repositories/open", repositoryHandler.OpenRepository)
	router.GET("/api/v1/repositories/current", repositoryHandler.GetCurrentRepository)
	router.DELETE("/api/v1/repositories/current", repositoryHandler.ExitCurrentRepository)

	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		httpRequest := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if body != "" {
			httpRequest.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httpRequest)
		return response
	}

	response := request(http.MethodGet, "/api/v1/repositories/current", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("initial current status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}
	var missing common.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &missing); err != nil {
		t.Fatalf("decode missing current response: %v", err)
	}
	if missing.Error.Code != common.NoRepositoryOpen || missing.Error.Retryable {
		t.Fatalf("unexpected missing current error: %+v", missing.Error)
	}

	response = request(http.MethodPost, "/api/v1/repositories/open", repositoryRequestBody(t, repositoryPath))
	if response.Code != http.StatusOK {
		t.Fatalf("open status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var opened struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &opened); err != nil {
		t.Fatalf("decode open response: %v", err)
	}

	response = request(http.MethodGet, "/api/v1/repositories/current", "")
	if response.Code != http.StatusOK {
		t.Fatalf("current status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var current struct {
		Data struct {
			ID                string  `json:"id"`
			Path              string  `json:"path"`
			Branch            *string `json:"branch"`
			Head              *string `json:"head"`
			CommitCount       int     `json:"commitCount"`
			WorkingTreeStatus string  `json:"workingTreeStatus"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil {
		t.Fatalf("decode current response: %v", err)
	}
	if current.Data.ID != opened.Data.ID || current.Data.Path != filepath.Clean(repositoryPath) {
		t.Fatalf("unexpected current repository: %+v", current.Data)
	}
	if current.Data.Branch != nil || current.Data.Head != nil || current.Data.CommitCount != 0 || current.Data.WorkingTreeStatus != "unborn" {
		t.Fatalf("unexpected empty repository state: %+v", current.Data)
	}

	response = request(http.MethodDelete, "/api/v1/repositories/current", "")
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("close status = %d, body=%q", response.Code, response.Body.String())
	}

	response = request(http.MethodGet, "/api/v1/repositories/current", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("current after close status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}

	response = request(http.MethodDelete, "/api/v1/repositories/current", "")
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("repeated close status = %d, body=%q", response.Code, response.Body.String())
	}
}

func TestScanRepositoryResponses(t *testing.T) {
	repositoryPath := t.TempDir()
	if _, err := git.PlainInit(repositoryPath, false); err != nil {
		t.Fatalf("initialize repository: %v", err)
	}

	taskService := service.NewTaskSvr()
	repositoryService := service.NewRepositoriesSvr(taskService)
	repositoryHandler := handler.NewRepositoriesHandler(repositoryService)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	router.POST("/api/v1/repositories/current/scans", repositoryHandler.ScanRepository)

	request := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		httpRequest := httptest.NewRequest(http.MethodPost, "/api/v1/repositories/current/scans", bytes.NewBufferString(body))
		httpRequest.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httpRequest)
		return response
	}

	response := request(`{"force":false}`)
	if response.Code != http.StatusNotFound {
		t.Fatalf("scan without repository status = %d, want %d; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}

	if _, _, err := repositoryService.OpenRepository(repositoryPath); err != nil {
		t.Fatalf("open repository: %v", err)
	}

	response = request(`{"force":false}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("start scan status = %d, want %d; body=%s", response.Code, http.StatusAccepted, response.Body.String())
	}
	var started struct {
		Data entity.Task         `json:"data"`
		Meta common.ResponseMeta `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode scan response: %v", err)
	}
	if started.Data.TaskID == "" || started.Data.Status != entity.Queued || started.Meta.RequestID == "" {
		t.Fatalf("unexpected scan response: %+v", started)
	}

	response = request(`{"force":false}`)
	if response.Code != http.StatusConflict {
		t.Fatalf("duplicate scan status = %d, want %d; body=%s", response.Code, http.StatusConflict, response.Body.String())
	}
	var duplicate common.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &duplicate); err != nil {
		t.Fatalf("decode duplicate scan response: %v", err)
	}
	if duplicate.Error.Code != common.ScanAlreadyRunning || duplicate.Error.Details == nil || duplicate.Error.Details.TaskID != started.Data.TaskID {
		t.Fatalf("unexpected duplicate scan error: %+v", duplicate)
	}
	if duplicate.Meta.RequestID == "" || duplicate.Meta.RequestID == started.Data.TaskID {
		t.Fatalf("request id = %q, task id = %q", duplicate.Meta.RequestID, started.Data.TaskID)
	}

	task, err := repositoryService.ScanRepository(false)
	if !errors.Is(err, service.ErrScanAlreadyQueued) {
		t.Fatalf("queued scan error = %v, want %v", err, service.ErrScanAlreadyQueued)
	}
	task.Status = entity.Completed
	forced, err := repositoryService.ScanRepository(true)
	if err != nil {
		t.Fatalf("force scan: %v", err)
	}
	if forced.Status != entity.Queued {
		t.Fatalf("forced scan status = %q, want %q", forced.Status, entity.Queued)
	}
}

func performRepositoryRequest(body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	router.POST("/api/v1/repositories/open", handler.NewRepositoriesHandler(service.NewRepositoriesSvr(service.NewTaskSvr())).OpenRepository)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/repositories/open", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func repositoryRequestBody(t *testing.T, path string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	return string(body)
}
