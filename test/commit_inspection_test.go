package test

import (
	"bytes"
	"context"
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

func TestGetCurrentCommitsFiltersMinimumIntroducedBytes(t *testing.T) {
	repositoryService, _, updateHash, _ := newCommitInspectionFixture(t)
	router := newCommitInspectionRouter(repositoryService)

	response := performCommitInspectionRequest(router, "/api/v1/repositories/current/commits?minIntroducedBytes=32")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var body struct {
		Data []entity.CommitSummary `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].SHA != updateHash.String() {
		t.Errorf("filtered commits = %+v, want only %s", body.Data, updateHash)
	}

	assertCommitInspectionError(t, router, "/api/v1/repositories/current/commits?minIntroducedBytes=-1", http.StatusBadRequest, common.InvalidRequest)
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

	for sha, expected := range map[string]int64{
		rootHash.String():   15,
		updateHash.String(): 32,
		renameHash.String(): 41,
	} {
		commit, err := repositoryService.GetCommit(sha)
		if err != nil {
			t.Fatalf("get commit %s: %v", sha, err)
		}
		if commit.Stats.SnapshotBytes != expected {
			t.Errorf("snapshot bytes for %s = %d, want %d", sha, commit.Stats.SnapshotBytes, expected)
		}
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

func TestCleanupEndpointErrors(t *testing.T) {
	validSHA := "0123456789012345678901234567890123456789"
	withoutRepository := newCommitInspectionRouter(service.NewRepositoriesSvr(service.NewTaskSvr()))
	assertCleanupError(t, withoutRepository, map[string]any{
		"commitShas":   []string{validSHA},
		"expectedHead": validSHA,
	}, http.StatusNotFound, common.NoRepositoryOpen)

	repositoryService, rootHash, updateHash, headHash := newCommitInspectionFixture(t)
	router := newCommitInspectionRouter(repositoryService)
	assertCleanupError(t, router, map[string]any{
		"commitShas":   []string{"short"},
		"expectedHead": headHash.String(),
	}, http.StatusBadRequest, common.InvalidRequest)
	assertCleanupError(t, router, map[string]any{
		"commitShas":   []string{validSHA},
		"expectedHead": headHash.String(),
	}, http.StatusNotFound, common.CommitNotFound)
	assertCleanupError(t, router, map[string]any{
		"commitShas":   []string{rootHash.String()},
		"expectedHead": headHash.String(),
	}, http.StatusUnprocessableEntity, common.CleanupUnsupported)
	assertCleanupError(t, router, map[string]any{
		"commitShas":   []string{updateHash.String()},
		"expectedHead": validSHA,
	}, http.StatusConflict, common.RepositoryChanged)

	_, repositoryData, _ := repositoryService.GetCurrentRepository()
	if err := os.WriteFile(filepath.Join(repositoryData.Path, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}
	assertCleanupError(t, router, map[string]any{
		"commitShas":   []string{headHash.String()},
		"expectedHead": headHash.String(),
	}, http.StatusConflict, common.WorkingTreeDirty)
}

func TestCleanupRejectsUnreachableAndMergeCommits(t *testing.T) {
	repositoryService, rootHash, _, headHash := newCommitInspectionFixture(t)
	_, repositoryData, _ := repositoryService.GetCurrentRepository()
	repository, err := git.PlainOpen(repositoryData.Path)
	if err != nil {
		t.Fatalf("open fixture repository: %v", err)
	}
	rootCommit, err := repository.CommitObject(rootHash)
	if err != nil {
		t.Fatalf("get root commit: %v", err)
	}
	signature := object.Signature{Name: "Side Author", Email: "side@example.com", When: time.Now()}
	sideCommit := &object.Commit{
		Author:    signature,
		Committer: signature,
		Message:   "Independent history",
		TreeHash:  rootCommit.TreeHash,
	}
	sideObject := repository.Storer.NewEncodedObject()
	if err := sideCommit.Encode(sideObject); err != nil {
		t.Fatalf("encode side commit: %v", err)
	}
	sideHash, err := repository.Storer.SetEncodedObject(sideObject)
	if err != nil {
		t.Fatalf("store side commit: %v", err)
	}
	if err := repository.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("isolated"), sideHash)); err != nil {
		t.Fatalf("create isolated branch: %v", err)
	}
	reopenAndRescan(t, repositoryService, repositoryData.Path)
	if _, err := repositoryService.Cleanup(context.Background(), []string{sideHash.String()}, headHash.String(), false); !errors.Is(err, service.ErrCleanupUnsupported) {
		t.Errorf("unreachable commit error = %v, want %v", err, service.ErrCleanupUnsupported)
	}

	headCommit, err := repository.CommitObject(headHash)
	if err != nil {
		t.Fatalf("get head commit: %v", err)
	}
	mergeCommit := &object.Commit{
		Author:       signature,
		Committer:    signature,
		Message:      "Merge independent history",
		TreeHash:     headCommit.TreeHash,
		ParentHashes: []plumbing.Hash{headHash, sideHash},
	}
	mergeObject := repository.Storer.NewEncodedObject()
	if err := mergeCommit.Encode(mergeObject); err != nil {
		t.Fatalf("encode merge commit: %v", err)
	}
	mergeHash, err := repository.Storer.SetEncodedObject(mergeObject)
	if err != nil {
		t.Fatalf("store merge commit: %v", err)
	}
	headReference, err := repository.Head()
	if err != nil {
		t.Fatalf("get HEAD reference: %v", err)
	}
	if err := repository.Storer.SetReference(plumbing.NewHashReference(headReference.Name(), mergeHash)); err != nil {
		t.Fatalf("move current branch to merge commit: %v", err)
	}
	reopenAndRescan(t, repositoryService, repositoryData.Path)
	if _, err := repositoryService.Cleanup(context.Background(), []string{mergeHash.String()}, mergeHash.String(), false); !errors.Is(err, service.ErrCleanupUnsupported) {
		t.Errorf("merge commit error = %v, want %v", err, service.ErrCleanupUnsupported)
	}
}

func TestCleanupDropsSelectedCommitSynchronously(t *testing.T) {
	repositoryService, _, updateHash, headHash := newCommitInspectionFixture(t)
	newHead, err := repositoryService.Cleanup(context.Background(), []string{headHash.String()}, headHash.String(), false)
	if err != nil {
		t.Fatalf("cleanup selected commit: %v", err)
	}
	if newHead != updateHash.String() {
		t.Errorf("new HEAD = %s, want %s", newHead, updateHash)
	}
	_, repositoryData, _ := repositoryService.GetCurrentRepository()
	if repositoryData.Head == nil || *repositoryData.Head != newHead {
		t.Errorf("current repository HEAD = %v, want %s", repositoryData.Head, newHead)
	}
	if repositoryData.AnalysisStatus != entity.RepoNotScanned {
		t.Errorf("analysis status = %s, want %s", repositoryData.AnalysisStatus, entity.RepoNotScanned)
	}
	if _, err := repositoryService.GetCommit(updateHash.String()); !errors.Is(err, service.ErrRepositoryNotScanned) {
		t.Errorf("stale scan error = %v, want %v", err, service.ErrRepositoryNotScanned)
	}
}

func TestCleanupRejectsHeadChangedAfterScan(t *testing.T) {
	repositoryService, _, _, scannedHead := newCommitInspectionFixture(t)
	_, repositoryData, _ := repositoryService.GetCurrentRepository()
	repository, err := git.PlainOpen(repositoryData.Path)
	if err != nil {
		t.Fatalf("open fixture repository: %v", err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		t.Fatalf("open fixture worktree: %v", err)
	}
	changedPath := filepath.Join(repositoryData.Path, "after-scan.txt")
	if err := os.WriteFile(changedPath, []byte("changed after scan\n"), 0o644); err != nil {
		t.Fatalf("write post-scan file: %v", err)
	}
	if _, err := worktree.Add("after-scan.txt"); err != nil {
		t.Fatalf("stage post-scan file: %v", err)
	}
	if _, err := worktree.Commit("Change after scan", &git.CommitOptions{Author: &object.Signature{Name: "Alice Author", Email: "alice@example.com", When: time.Now()}}); err != nil {
		t.Fatalf("commit after scan: %v", err)
	}

	if _, err := repositoryService.Cleanup(context.Background(), []string{scannedHead.String()}, scannedHead.String(), false); !errors.Is(err, service.ErrRepositoryChanged) {
		t.Errorf("changed HEAD error = %v, want %v", err, service.ErrRepositoryChanged)
	}
}

func TestCleanupAutoStashRestoresWorkingTree(t *testing.T) {
	repositoryService, _, updateHash, headHash := newCommitInspectionFixture(t)
	_, repositoryData, _ := repositoryService.GetCurrentRepository()
	dirtyPath := filepath.Join(repositoryData.Path, "local-note.txt")
	if err := os.WriteFile(dirtyPath, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("write local change: %v", err)
	}

	newHead, err := repositoryService.Cleanup(context.Background(), []string{headHash.String()}, headHash.String(), true)
	if err != nil {
		t.Fatalf("cleanup with auto stash: %v", err)
	}
	if newHead != updateHash.String() {
		t.Errorf("new HEAD = %s, want %s", newHead, updateHash)
	}
	content, err := os.ReadFile(dirtyPath)
	if err != nil {
		t.Fatalf("read restored local change: %v", err)
	}
	if string(content) != "keep me" {
		t.Errorf("restored local change = %q, want %q", content, "keep me")
	}
}

func TestCleanupPreviewUsesCurrentHeadTree(t *testing.T) {
	repositoryService, _, updateHash, headHash := newCommitInspectionFixture(t)
	affectedFiles, err := repositoryService.CleanupPreview(context.Background(), []string{updateHash.String()}, headHash.String())
	if err != nil {
		t.Fatalf("preview cleanup: %v", err)
	}
	files := affectedFiles[updateHash.String()]
	if len(files) != 1 || files[0] != "new.txt" {
		t.Errorf("affected files = %v, want [new.txt]", files)
	}

	missingSHA := "0123456789012345678901234567890123456789"
	affectedFiles, err = repositoryService.CleanupPreview(context.Background(), []string{missingSHA}, headHash.String())
	if err != nil {
		t.Fatalf("preview missing commit: %v", err)
	}
	if len(affectedFiles) != 0 {
		t.Errorf("missing commit affected files = %v, want empty", affectedFiles)
	}
}

func TestCleanupPreviewEndpointContractAndErrors(t *testing.T) {
	validSHA := "0123456789012345678901234567890123456789"
	withoutRepository := newCommitInspectionRouter(service.NewRepositoriesSvr(service.NewTaskSvr()))
	assertCleanupPreviewError(t, withoutRepository, map[string]any{
		"commitShas":   []string{validSHA},
		"expectedHead": validSHA,
	}, http.StatusNotFound, common.NoRepositoryOpen)

	repositoryService, rootHash, updateHash, headHash := newCommitInspectionFixture(t)
	router := newCommitInspectionRouter(repositoryService)
	response := performCleanupPost(router, "/api/v1/repositories/current/cleanup/preview", map[string]any{
		"commitShas":   []string{updateHash.String()},
		"expectedHead": headHash.String(),
	})
	if response.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var body struct {
		Data struct {
			RequiresConfirmation bool                `json:"requiresConfirmation"`
			AffectedFiles        map[string][]string `json:"affectedFiles"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if !body.Data.RequiresConfirmation || len(body.Data.AffectedFiles[updateHash.String()]) != 1 {
		t.Errorf("unexpected preview response: %+v", body.Data)
	}

	assertCleanupPreviewError(t, router, map[string]any{
		"commitShas":   []string{"short"},
		"expectedHead": headHash.String(),
	}, http.StatusBadRequest, common.InvalidRequest)
	assertCleanupPreviewError(t, router, map[string]any{
		"commitShas":   []string{rootHash.String()},
		"expectedHead": headHash.String(),
	}, http.StatusUnprocessableEntity, common.CleanupUnsupported)
	assertCleanupPreviewError(t, router, map[string]any{
		"commitShas":   []string{updateHash.String()},
		"expectedHead": validSHA,
	}, http.StatusConflict, common.RepositoryChanged)
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
	router.GET("/api/v1/repositories/current/commits", repositoryHandler.GetCurrentCommits)
	router.GET("/api/v1/repositories/current/commits/:sha", repositoryHandler.GetCommit)
	router.GET("/api/v1/repositories/current/commits/:sha/files", repositoryHandler.GetFiles)
	router.POST("/api/v1/repositories/current/cleanup", repositoryHandler.Cleanup)
	router.POST("/api/v1/repositories/current/cleanup/preview", repositoryHandler.CleanupPreview)
	return router
}

func reopenAndRescan(t *testing.T, repositoryService *service.RepositoriesSvr, repositoryPath string) {
	t.Helper()
	if _, _, err := repositoryService.OpenRepository(repositoryPath); err != nil {
		t.Fatalf("reopen repository: %v", err)
	}
	_, err := repositoryService.ScanRepository(true)
	if err != nil {
		t.Fatalf("rescan repository: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, current, _ := repositoryService.GetCurrentRepository()
		if current.AnalysisStatus == entity.RepoReady {
			return
		}
		if current.AnalysisStatus == entity.RepoFailed {
			t.Fatal("rescan failed")
		}
		if time.Now().After(deadline) {
			t.Fatal("rescan did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertCleanupError(t *testing.T, router *gin.Engine, requestBody map[string]any, expectedStatus int, expectedCode string) {
	t.Helper()
	response := performCleanupPost(router, "/api/v1/repositories/current/cleanup", requestBody)
	if response.Code != expectedStatus {
		t.Fatalf("cleanup status = %d, want %d; body=%s", response.Code, expectedStatus, response.Body.String())
	}
	var errorBody common.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &errorBody); err != nil {
		t.Fatalf("decode cleanup error: %v", err)
	}
	if errorBody.Error.Code != expectedCode {
		t.Errorf("cleanup code = %q, want %q", errorBody.Error.Code, expectedCode)
	}
}

func assertCleanupPreviewError(t *testing.T, router *gin.Engine, requestBody map[string]any, expectedStatus int, expectedCode string) {
	t.Helper()
	response := performCleanupPost(router, "/api/v1/repositories/current/cleanup/preview", requestBody)
	if response.Code != expectedStatus {
		t.Fatalf("cleanup preview status = %d, want %d; body=%s", response.Code, expectedStatus, response.Body.String())
	}
	var errorBody common.ErrorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &errorBody); err != nil {
		t.Fatalf("decode cleanup preview error: %v", err)
	}
	if errorBody.Error.Code != expectedCode {
		t.Errorf("cleanup preview code = %q, want %q", errorBody.Error.Code, expectedCode)
	}
}

func performCleanupPost(router *gin.Engine, path string, requestBody map[string]any) *httptest.ResponseRecorder {
	body, err := json.Marshal(requestBody)
	if err != nil {
		panic(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
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
