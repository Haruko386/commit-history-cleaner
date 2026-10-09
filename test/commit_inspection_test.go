package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/Haruko386/commit-history-cleaner/internal/entity"
	"github.com/Haruko386/commit-history-cleaner/internal/handler"
	"github.com/Haruko386/commit-history-cleaner/internal/middleware"
	"github.com/Haruko386/commit-history-cleaner/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestGetCommitEndpointContract(t *testing.T) {
	repositoryService, _, updateHash, _ := newCommitInspectionFixture(t)
	router := newCommitInspectionRouter(repositoryService)

	response := performCommitInspectionRequest(router, "/api/v1/repositories/current/commits/"+updateHash.String())
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	var body struct {
		Data struct {
			SHA       string        `json:"sha"`
			ShortSHA  string        `json:"shortSha"`
			Subject   string        `json:"subject"`
			Body      string        `json:"body"`
			Author    entity.Author `json:"author"`
			Committer entity.Author `json:"committer"`
		} `json:"data"`
		Meta common.ResponseMeta `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.SHA != updateHash.String() || body.Data.ShortSHA != updateHash.String()[:7] {
		t.Errorf("unexpected commit identity: %+v", body.Data)
	}
	if body.Data.Subject != "Update files" || body.Data.Body != "Update files\n\nDetailed body.\n" {
		t.Errorf("unexpected commit message: subject=%q body=%q", body.Data.Subject, body.Data.Body)
	}
	if body.Data.Author.Name != "Alice Author" || body.Data.Committer.Name != "Bob Committer" {
		t.Errorf("unexpected signatures: author=%+v committer=%+v", body.Data.Author, body.Data.Committer)
	}
	if body.Meta.RequestID == "" {
		t.Error("requestId is empty")
	}
}

func TestGetCommitEndpointErrors(t *testing.T) {
	withoutRepository := newCommitInspectionRouter(service.NewRepositoriesSvr(service.NewTaskSvr()))
	assertCommitInspectionError(t, withoutRepository, "/api/v1/repositories/current/commits/0123456789012345678901234567890123456789", http.StatusNotFound, common.NoRepositoryOpen)

	repositoryService, _, updateHash, _ := newCommitInspectionFixture(t)
	router := newCommitInspectionRouter(repositoryService)
	assertCommitInspectionError(t, router, "/api/v1/repositories/current/commits/short", http.StatusBadRequest, common.InvalidRequest)
	assertCommitInspectionError(t, router, "/api/v1/repositories/current/commits/0123456789012345678901234567890123456789", http.StatusNotFound, common.CommitNotFound)

	otherRepositoryPath := t.TempDir()
	if _, err := git.PlainInit(otherRepositoryPath, false); err != nil {
		t.Fatalf("initialize other repository: %v", err)
	}
	if _, _, err := repositoryService.OpenRepository(otherRepositoryPath); err != nil {
		t.Fatalf("open other repository: %v", err)
	}
	assertCommitInspectionError(t, router, "/api/v1/repositories/current/commits/"+updateHash.String(), http.StatusConflict, common.RepositoryNotScanned)
}

func TestGetFilesServiceContract(t *testing.T) {
	repositoryService, rootHash, updateHash, renameHash := newCommitInspectionFixture(t)
	sortName, order := "path", "desc"
	first, cursor, err := repositoryService.GetFiles(updateHash.String(), entity.CommitFileQuery{Limit: 2, Sort: &sortName, Order: &order})
	if err != nil {
		t.Fatalf("get first page: %v", err)
	}
	if len(first) != 2 || cursor == "" {
		t.Fatalf("unexpected first page: files=%d cursor=%q", len(first), cursor)
	}
	second, nextCursor, err := repositoryService.GetFiles(updateHash.String(), entity.CommitFileQuery{Limit: 2, Sort: &sortName, Order: &order, Cursor: &cursor})
	if err != nil {
		t.Fatalf("get second page: %v", err)
	}
	if nextCursor != "" {
		t.Errorf("last page cursor = %q, want empty", nextCursor)
	}

	files := append(append([]entity.CommitFile{}, first...), second...)
	if len(files) != 3 {
		t.Fatalf("changed files = %d, want 3", len(files))
	}
	if !sort.SliceIsSorted(files, func(i, j int) bool { return files[i].Path > files[j].Path }) {
		t.Errorf("files are not sorted by path descending: %+v", files)
	}

	byPath := make(map[string]entity.CommitFile, len(files))
	for _, file := range files {
		byPath[file.Path] = file
	}
	modified := byPath["README.md"]
	if modified.Status != entity.FileStatusModified || modified.NewBytes == nil || modified.IntroducedBytes != *modified.NewBytes {
		t.Errorf("unexpected modified file: %+v", modified)
	}
	if modified.Additions == nil || *modified.Additions != 1 || modified.Deletions == nil || *modified.Deletions != 0 || modified.Binary {
		t.Errorf("unexpected modified text stats: %+v", modified)
	}
	deleted := byPath["asset.bin"]
	if deleted.Status != entity.FileStatusDeleted || deleted.OldBlob == nil || deleted.NewBlob != nil || !deleted.Binary {
		t.Errorf("unexpected deleted binary file: %+v", deleted)
	}

	rootFiles, _, err := repositoryService.GetFiles(rootHash.String(), entity.CommitFileQuery{})
	if err != nil {
		t.Fatalf("get root commit files: %v", err)
	}
	if len(rootFiles) != 2 {
		t.Fatalf("root changed files = %d, want 2", len(rootFiles))
	}
	for _, file := range rootFiles {
		if file.Status != entity.FileStatusAdded || file.OldBlob != nil || file.NewBlob == nil {
			t.Errorf("unexpected root file: %+v", file)
		}
	}

	renameFiles, _, err := repositoryService.GetFiles(renameHash.String(), entity.CommitFileQuery{})
	if err != nil {
		t.Fatalf("get rename commit files: %v", err)
	}
	renameByPath := make(map[string]entity.CommitFile, len(renameFiles))
	for _, file := range renameFiles {
		renameByPath[file.Path] = file
	}
	renamed := renameByPath["GUIDE.md"]
	if renamed.Status != entity.FileStatusRenamed || renamed.PreviousPath == nil || *renamed.PreviousPath != "README.md" || renamed.IntroducedBytes != 0 {
		t.Errorf("unexpected renamed file: %+v", renamed)
	}
	copied := renameByPath["copy.txt"]
	if copied.Status != entity.FileStatusCopied || copied.PreviousPath == nil || *copied.PreviousPath != "new.txt" || copied.IntroducedBytes != 0 {
		t.Errorf("unexpected copied file: %+v", copied)
	}
}

func TestGetFilesEndpointAndValidation(t *testing.T) {
	repositoryService, _, updateHash, _ := newCommitInspectionFixture(t)
	router := newCommitInspectionRouter(repositoryService)
	path := "/api/v1/repositories/current/commits/" + updateHash.String() + "/files?limit=2&sort=path&order=asc"
	response := performCommitInspectionRequest(router, path)
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatalf("status = %d, want %d with JSON body; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var body struct {
		Data []entity.CommitFile     `json:"data"`
		Meta common.ListResponseMeta `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 2 || body.Meta.RequestID == "" || !body.Meta.HasMore || body.Meta.NextCursor == nil {
		t.Errorf("unexpected file response: %+v", body)
	}

	assertCommitInspectionError(t, router, "/api/v1/repositories/current/commits/"+updateHash.String()+"/files?sort=unknown", http.StatusBadRequest, common.InvalidRequest)
	assertCommitInspectionError(t, router, "/api/v1/repositories/current/commits/"+updateHash.String()+"/files?order=sideways", http.StatusBadRequest, common.InvalidRequest)
	assertCommitInspectionError(t, router, "/api/v1/repositories/current/commits/"+updateHash.String()+"/files?cursor=not-base64", http.StatusBadRequest, common.InvalidRequest)
}

func TestGetFilesNullableSortsAndCursorBinding(t *testing.T) {
	repositoryService, _, updateHash, _ := newCommitInspectionFixture(t)
	for _, sortName := range []string{"newBytes", "additions", "deletions"} {
		t.Run(sortName, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("sorting %s panicked: %v", sortName, recovered)
				}
			}()
			order := "asc"
			files, _, err := repositoryService.GetFiles(updateHash.String(), entity.CommitFileQuery{Sort: &sortName, Order: &order})
			if err != nil {
				t.Fatalf("sort %s: %v", sortName, err)
			}
			if len(files) == 0 {
				t.Fatal("sorted files are empty")
			}
			last := files[len(files)-1]
			isNull := (sortName == "newBytes" && last.NewBytes == nil) ||
				(sortName == "additions" && last.Additions == nil) ||
				(sortName == "deletions" && last.Deletions == nil)
			if !isNull {
				t.Errorf("null %s value must sort last: %+v", sortName, files)
			}
		})
	}

	pathSort, introducedSort, order := "path", "introducedBytes", "asc"
	_, cursor, err := repositoryService.GetFiles(updateHash.String(), entity.CommitFileQuery{Limit: 1, Sort: &pathSort, Order: &order})
	if err != nil || cursor == "" {
		t.Fatalf("create file cursor: cursor=%q err=%v", cursor, err)
	}
	if _, _, err := repositoryService.GetFiles(updateHash.String(), entity.CommitFileQuery{Limit: 1, Sort: &introducedSort, Order: &order, Cursor: &cursor}); !errors.Is(err, service.ErrInvalidCommitCursor) {
		t.Errorf("cursor reused with another sort error = %v, want %v", err, service.ErrInvalidCommitCursor)
	}
}

func TestCommitFileJSONKeepsNullableFields(t *testing.T) {
	encoded, err := json.Marshal(entity.CommitFile{Path: "deleted.txt", Status: entity.FileStatusDeleted})
	if err != nil {
		t.Fatalf("marshal commit file: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode commit file: %v", err)
	}
	for _, name := range []string{"previousPath", "oldBlob", "newBlob", "oldBytes", "newBytes", "additions", "deletions"} {
		if _, exists := fields[name]; !exists {
			t.Errorf("nullable field %q must be present; JSON=%s", name, encoded)
		}
	}
}

func newCommitInspectionFixture(t *testing.T) (*service.RepositoriesSvr, plumbing.Hash, plumbing.Hash, plumbing.Hash) {
	t.Helper()
	repositoryPath := t.TempDir()
	repository, err := git.PlainInit(repositoryPath, false)
	if err != nil {
		t.Fatalf("initialize repository: %v", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		t.Fatalf("open worktree: %v", err)
	}

	writeAndAdd := func(name string, content []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repositoryPath, name), content, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if _, err := worktree.Add(name); err != nil {
			t.Fatalf("stage %s: %v", name, err)
		}
	}
	when := time.Now().Add(-time.Hour)
	writeAndAdd("README.md", []byte("first line\n"))
	writeAndAdd("asset.bin", []byte{0x00, 0x01, 0x02, 0x03})
	rootHash, err := worktree.Commit("Initial commit", &git.CommitOptions{Author: &object.Signature{Name: "Alice Author", Email: "alice@example.com", When: when}})
	if err != nil {
		t.Fatalf("create root commit: %v", err)
	}

	writeAndAdd("README.md", []byte("first line\nsecond line\n"))
	writeAndAdd("new.txt", []byte("new file\n"))
	if _, err := worktree.Remove("asset.bin"); err != nil {
		t.Fatalf("remove binary fixture: %v", err)
	}
	updateHash, err := worktree.Commit("Update files\n\nDetailed body.\n", &git.CommitOptions{
		Author:    &object.Signature{Name: "Alice Author", Email: "alice@example.com", When: when.Add(time.Minute)},
		Committer: &object.Signature{Name: "Bob Committer", Email: "bob@example.com", When: when.Add(2 * time.Minute)},
	})
	if err != nil {
		t.Fatalf("create update commit: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(repositoryPath, "README.md"))
	if err != nil {
		t.Fatalf("read rename fixture: %v", err)
	}
	if _, err := worktree.Remove("README.md"); err != nil {
		t.Fatalf("remove renamed fixture: %v", err)
	}
	writeAndAdd("GUIDE.md", readme)
	newFile, err := os.ReadFile(filepath.Join(repositoryPath, "new.txt"))
	if err != nil {
		t.Fatalf("read copy fixture: %v", err)
	}
	writeAndAdd("copy.txt", newFile)
	renameHash, err := worktree.Commit("Rename and copy files", &git.CommitOptions{Author: &object.Signature{Name: "Alice Author", Email: "alice@example.com", When: when.Add(3 * time.Minute)}})
	if err != nil {
		t.Fatalf("create rename commit: %v", err)
	}

	taskService := service.NewTaskSvr()
	taskService.Start()
	repositoryService := service.NewRepositoriesSvr(taskService)
	if _, _, err := repositoryService.OpenRepository(repositoryPath); err != nil {
		t.Fatalf("open repository: %v", err)
	}
	task, err := repositoryService.ScanRepository(false)
	if err != nil {
		t.Fatalf("start scan: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		current, err := taskService.GetTask(task.TaskID)
		if err != nil {
			t.Fatalf("get scan task: %v", err)
		}
		if current.Status == entity.Completed {
			break
		}
		if current.Status == entity.Failed {
			t.Fatalf("scan failed: %v", current.Error)
		}
		if time.Now().After(deadline) {
			t.Fatal("scan did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return repositoryService, rootHash, updateHash, renameHash
}

func newCommitInspectionRouter(repositoryService *service.RepositoriesSvr) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	repositoryHandler := handler.NewRepositoriesHandler(repositoryService)
	router.GET("/api/v1/repositories/current/commits/:sha", repositoryHandler.GetCommit)
	router.GET("/api/v1/repositories/current/commits/:sha/files", repositoryHandler.GetFiles)
	return router
}

func performCommitInspectionRequest(router *gin.Engine, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, bytes.NewReader(nil))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func assertCommitInspectionError(t *testing.T, router *gin.Engine, path string, status int, code string) {
	t.Helper()
	response := performCommitInspectionRequest(router, path)
	if response.Code != status {
		t.Errorf("%s status = %d, want %d; body=%s", path, response.Code, status, response.Body.String())
		return
	}
	var body common.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Errorf("decode %s response: %v", path, err)
		return
	}
	if body.Error.Code != code {
		t.Errorf("%s code = %q, want %q", path, body.Error.Code, code)
	}
}
