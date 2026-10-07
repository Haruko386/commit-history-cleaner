package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"example.com/m/v2/internal/common"
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

	code, data, err := service.NewRepositoriesSvr().OpenRepository(repositoryPath)
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

	_, data, err := service.NewRepositoriesSvr().OpenRepository(repositoryPath)
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

func performRepositoryRequest(body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	router.POST("/api/v1/repositories/open", handler.NewRepositoriesHandler(service.NewRepositoriesSvr()).OpenRepository)

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
