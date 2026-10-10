package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/Haruko386/commit-history-cleaner/internal/entity"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type RepositoriesSvr struct {
	mu      sync.RWMutex
	current *RepoData
	taskSvr *TaskSvr
}

func NewRepositoriesSvr(taskSvr *TaskSvr) *RepositoriesSvr {
	return &RepositoriesSvr{taskSvr: taskSvr}
}

var (
	ErrNoRepositoryOpen     = errors.New("no repository has been opened")
	ErrScanAlreadyRunning   = errors.New("repository scan has already been running")
	ErrScanAlreadyQueued    = errors.New("repository scan has already been queued")
	ErrScanAlreadyCompleted = errors.New("repository scan has already been completed")
	ErrScanAlreadyCancelled = errors.New("repository scan has already been cancelled")
	ErrRepositoryNotScanned = errors.New("repository has not been scanned")
	ErrInvalidCommitCursor  = errors.New("invalid commit cursor")
	ErrUnsupportedRef       = errors.New("unsupported ref")
	ErrInvalidCommitSHA     = errors.New("invalid commit SHA")
	ErrCommitNotFound       = errors.New("commit not found")
	ErrInvalidCommitQuery   = errors.New("invalid commit file query")
	ErrInvalidCleanup       = errors.New("invalid cleanup request")
	ErrRepositoryChanged    = errors.New("repository HEAD has changed")
	ErrWorkingTreeDirty     = errors.New("working tree is dirty")
	ErrCleanupUnsupported   = errors.New("cleanup is unsupported for this commit history")
	ErrGitCommandFailed     = errors.New("Git command failed")
)

// TODO do a refactor for repo struct, create an entity for it
type RepoData struct {
	Name              string    `json:"name"`
	Path              string    `json:"path"`
	Branch            *string   `json:"branch"`
	DetachedHead      bool      `json:"detachedHead"`
	Head              *string   `json:"head"`
	CommitCount       int       `json:"commitCount"`
	WorkingTreeStatus string    `json:"workingTreeStatus"`
	GitDirectoryBytes int64     `json:"gitDirectoryBytes"`
	AnalysisStatus    string    `json:"analysisStatus"`
	OpenedAt          time.Time `json:"openedAt"`
}

func (s *RepositoriesSvr) OpenRepository(path string) (string, *RepoData, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return common.PathRequired, nil, fmt.Errorf("path should not be empty")
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return common.PathNotReadable, nil, fmt.Errorf("path %s is not readable", path)
	}
	path = filepath.Clean(absolutePath)

	// check repo info
	info, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return common.RepositoryNotFound, nil, fmt.Errorf("path %s does not exist", path)
		}
		return common.PathNotReadable, nil, fmt.Errorf("path %s is not readable", path)
	}
	if len(info) == 0 {
		return common.NotAGitRepository, nil, fmt.Errorf("path %s is not a Git repository", path)
	}

	// check git info
	r, err := git.PlainOpen(path)
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return common.NotAGitRepository, nil, fmt.Errorf(".git file not exists")
		}
		return common.PathNotReadable, nil, fmt.Errorf(".git file not readable")
	}

	now := time.Now()
	repoData := &RepoData{
		Name:     filepath.Base(path),
		Path:     path,
		OpenedAt: now,
	}

	ref, err := r.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			repoData.CommitCount = 0
			repoData.WorkingTreeStatus = "unborn"
			repoData.AnalysisStatus = entity.RepoNotScanned
			repoData.GitDirectoryBytes, err = dirSize(filepath.Join(path, ".git"))
			if err != nil {
				return common.InternalError, nil, fmt.Errorf("checking git repository size: %w", err)
			}

			s.mu.Lock()
			defer s.mu.Unlock()
			s.current = repoData

			return "", repoData, nil
		}
		return common.InternalError, nil, fmt.Errorf("reading git HEAD error: %w", err)
	}

	repoData.Head = new(ref.Hash().String())
	if ref.Name().IsBranch() {
		repoData.Branch = new(ref.Name().Short())
	} else {
		repoData.DetachedHead = true
	}

	// commit count
	iter, err := r.Log(&git.LogOptions{All: true})
	if err != nil {
		return common.InternalError, nil, fmt.Errorf("counting git history error: %w", err)
	}
	defer iter.Close()

	count := 0
	err = iter.ForEach(func(c *object.Commit) error {
		count++
		return nil
	})
	if err != nil {
		return common.InternalError, nil, fmt.Errorf("counting git history error: %w", err)
	}

	repoData.CommitCount = count

	// commit tree
	workTree, err := r.Worktree()
	if err != nil {
		return common.InternalError, nil, fmt.Errorf("open git worktree error: %w", err)
	}

	status, err := workTree.Status()
	if err != nil {
		return common.InternalError, nil, fmt.Errorf("checking git worktree status error: %w", err)
	}
	repoData.WorkingTreeStatus = "dirty"
	if status.IsClean() {
		repoData.WorkingTreeStatus = "clean"
	}

	//commit, err := r.CommitObject(ref.Hash())
	//if err != nil {
	//	return "", nil, fmt.Errorf("get commit history error: %v", err)
	//}

	// other info
	repoSize, err := dirSize(filepath.Join(path, ".git"))
	if err != nil {
		return common.InternalError, nil, fmt.Errorf("checking git repository size: %w", err)
	}
	repoData.GitDirectoryBytes = repoSize
	repoData.AnalysisStatus = entity.RepoNotScanned

	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = repoData

	return "", repoData, nil
}

func dirSize(path string) (int64, error) {
	var size int64

	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		size += info.Size()
		return nil
	})

	return size, err
}

func (s *RepositoriesSvr) GetCurrentRepository() (int, *RepoData, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.current != nil {
		cpRepoData := &RepoData{
			Name:              s.current.Name,
			Path:              s.current.Path,
			Branch:            s.current.Branch,
			DetachedHead:      s.current.DetachedHead,
			Head:              s.current.Head,
			CommitCount:       s.current.CommitCount,
			WorkingTreeStatus: s.current.WorkingTreeStatus,
			GitDirectoryBytes: s.current.GitDirectoryBytes,
			AnalysisStatus:    s.current.AnalysisStatus,
			OpenedAt:          s.current.OpenedAt,
		}
		return http.StatusOK, cpRepoData, ""
	}
	return http.StatusNotFound, nil, common.NoRepositoryOpen
}

func (s *RepositoriesSvr) ExitCurrentRepository() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil {
		return true, nil
	}

	if s.current.AnalysisStatus == entity.RepoNotScanned || s.current.AnalysisStatus == entity.RepoScanning {
		repoID := common.GenerateRepoID(s.current.Path)
		if taskID, ok := s.taskSvr.repoTaskList[repoID]; ok {
			if err := s.taskSvr.CancelTask(taskID); err != nil {
				return false, fmt.Errorf("cancel task err: %w", err)
			}
		}
		s.current = nil
		return true, nil
	}

	s.current = nil
	return true, nil
}

func (s *RepositoriesSvr) ScanRepository(force bool) (*entity.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// check if repo is opened
	if s.current == nil {
		return &entity.Task{}, ErrNoRepositoryOpen
	}

	s.taskSvr.mu.Lock()
	defer s.taskSvr.mu.Unlock()

	// if this repo has no task
	repoID := common.GenerateRepoID(s.current.Path) // every same repo should have the sam repoID
	existedTaskID, ok := s.taskSvr.repoTaskList[repoID]
	if !ok { // this repo has no task, create one task for the repo
		newTask := createNewTask(s.taskSvr.ctx)

		s.current.AnalysisStatus = entity.RepoNotScanned
		s.taskSvr.repoTaskList[repoID] = newTask.TaskID
		s.taskSvr.taskList[newTask.TaskID] = newTask

		s.taskSvr.AddTask(newTask, s.current, &s.taskSvr.mu, &s.mu)
		return newTask, nil
	}

	// taskID is existed, check task's status
	task, ok := s.taskSvr.taskList[existedTaskID]
	if !ok { // no task, create the task
		newTask := createNewTask(s.taskSvr.ctx)

		s.current.AnalysisStatus = entity.RepoNotScanned
		s.taskSvr.repoTaskList[repoID] = newTask.TaskID
		s.taskSvr.taskList[newTask.TaskID] = newTask

		s.taskSvr.AddTask(newTask, s.current, &s.taskSvr.mu, &s.mu)
		return newTask, nil
	}

	// if existed
	if task.Status == entity.Queued {
		return task, ErrScanAlreadyQueued
	} else if task.Status == entity.Running {
		return task, ErrScanAlreadyRunning
	}

	// if not running or queued and `force`(completed or canceled)
	if force {
		delete(s.taskSvr.taskList, s.taskSvr.repoTaskList[repoID])
		newTask := createNewTask(s.taskSvr.ctx)
		s.taskSvr.repoTaskList[repoID] = newTask.TaskID
		s.taskSvr.taskList[newTask.TaskID] = newTask
		s.current.AnalysisStatus = entity.RepoNotScanned
		s.taskSvr.AddTask(newTask, s.current, &s.taskSvr.mu, &s.mu)
		return newTask, nil
	}

	switch task.Status {
	case entity.Cancelled:
		return task, ErrScanAlreadyCancelled
	default:
		return task, ErrScanAlreadyCompleted
	}
}

// createNewTask create a new task
func createNewTask(parentCtx context.Context) *entity.Task {
	ctx, cancel := context.WithCancel(parentCtx)
	taskID := common.GenerateTaskID()
	task := &entity.Task{
		TaskID: taskID,
		Type:   entity.RepositoryScan,
		Status: entity.Queued,
		Progress: entity.Progress{
			Phase:   entity.Queued,
			Current: 0,
			Total:   nil,
			Percent: nil,
		},
		CreatedAt:  new(time.Now()),
		StartedAt:  nil,
		FinishedAt: nil,

		Ctx:    ctx,
		Cancel: cancel,
	}
	return task
}

func (s *RepositoriesSvr) GetCurrentCommits(commitQuery *entity.CommitsQuery) ([]entity.CommitSummary, string, error) {
	if commitQuery.MinIntroducedBytes < 0 {
		return nil, "", ErrInvalidCommitQuery
	}

	s.mu.RLock()
	if s.current == nil {
		s.mu.RUnlock()
		return []entity.CommitSummary{}, "", ErrNoRepositoryOpen
	}
	repoID := common.GenerateRepoID(s.current.Path)
	analysisStatus := s.current.AnalysisStatus
	headSHA := ""
	if s.current.Head != nil {
		headSHA = *s.current.Head
	}
	s.mu.RUnlock()

	s.taskSvr.worker.mu.Lock()
	commitInfos, ok := s.taskSvr.worker.scannedRepoInfo.RepoCommits[repoID]
	s.taskSvr.worker.mu.Unlock()
	if !ok {
		return nil, "", fmt.Errorf("%w for repo %s", ErrRepositoryNotScanned, repoID)
	}

	cursor, err := decodeIndexCursor(commitQuery.Cursor)
	if err != nil || cursor < 0 {
		return nil, "", fmt.Errorf("%w for repo %s: %v", ErrInvalidCommitCursor, repoID, err)
	}

	limit := commitQuery.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	ref := strings.TrimSpace(commitQuery.Ref)
	startSHA := headSHA
	if ref != "" && ref != "HEAD" {
		startSHA = ""
		for _, commitInfo := range commitInfos {
			for _, branch := range commitInfo.Refs.Branches {
				if ref == branch || ref == "refs/heads/"+branch {
					startSHA = commitInfo.SHA
					break
				}
			}
			for _, tag := range commitInfo.Refs.Tags {
				if ref == tag || ref == "refs/tags/"+tag {
					startSHA = commitInfo.SHA
					break
				}
			}
			if startSHA != "" {
				break
			}
		}

		if startSHA == "" {
			for _, commitInfo := range commitInfos {
				if strings.HasPrefix(commitInfo.SHA, ref) {
					if startSHA != "" && startSHA != commitInfo.SHA {
						return nil, "", fmt.Errorf("%w: %s is ambiguous", ErrUnsupportedRef, ref)
					}
					startSHA = commitInfo.SHA
				}
			}
		}
		if startSHA == "" {
			return nil, "", fmt.Errorf("%w: %s", ErrUnsupportedRef, ref)
		}
	}

	commitBySHA := make(map[string]entity.CommitInfo, len(commitInfos))
	for _, commitInfo := range commitInfos {
		commitBySHA[commitInfo.SHA] = commitInfo
	}
	reachable := make(map[string]struct{})
	stack := []string{startSHA}
	for len(stack) > 0 {
		sha := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, exists := reachable[sha]; exists {
			continue
		}
		commitInfo, exists := commitBySHA[sha]
		if !exists {
			continue
		}
		reachable[sha] = struct{}{}
		stack = append(stack, commitInfo.ParentSHAs...)
	}

	matched := 0
	var commitSummaries []entity.CommitSummary

	for _, commitInfo := range commitInfos {
		if _, exists := reachable[commitInfo.SHA]; !exists {
			continue
		}
		if !matchQuery(&commitInfo, commitQuery) {
			continue
		}

		if matched < cursor {
			matched++
			continue
		}
		if len(commitSummaries) == limit {
			return commitSummaries, encodeIndexCursor(cursor + len(commitSummaries)), nil
		}

		commitSummary := &entity.CommitSummary{
			SHA:      commitInfo.SHA,
			ShortSHA: commitInfo.SHA[:7],
			Subject:  strings.Split(commitInfo.Message, "\n")[0],
			Author: entity.Author{
				Name:  commitInfo.AuthorName,
				Email: commitInfo.AuthorEmail,
			},
			AuthoredAt:     commitInfo.AuthoredAt,
			CommittedAt:    commitInfo.CommittedAt,
			Parents:        commitInfo.ParentSHAs,
			Refs:           commitInfo.Refs,
			Stats:          commitInfo.Stats,
			AnalysisStatus: analysisStatus,
		}
		commitSummaries = append(commitSummaries, *commitSummary)
		matched++
	}

	return commitSummaries, "", nil
}

// encodeIndexCursor encode index to cursor
func encodeIndexCursor(index int) string {
	s := strconv.Itoa(index)
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// decodeIndexCursor decode cursor to index
func decodeIndexCursor(cursor *string) (int, error) {
	if cursor == nil || *cursor == "" {
		return 0, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(*cursor)
	if err != nil {
		return 0, err
	}
	index, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}
	return index, nil
}

// matchQuery match the query
func matchQuery(commitInfo *entity.CommitInfo, query *entity.CommitsQuery) bool {
	if query.Author != nil {
		author := strings.ToLower(*query.Author)
		if !strings.Contains(strings.ToLower(commitInfo.AuthorName), author) && !strings.Contains(strings.ToLower(commitInfo.AuthorEmail), author) {
			return false
		}
	}
	if query.Since != nil {
		if commitInfo.CommittedAt.Before(*query.Since) {
			return false
		}
	}
	if query.Until != nil {
		if commitInfo.CommittedAt.After(*query.Until) {
			return false
		}
	}
	if query.MinIntroducedBytes != 0 {
		if commitInfo.Stats.IntroducedBytes < query.MinIntroducedBytes {
			return false
		}
	}
	if query.Query != nil {
		keyword := strings.ToLower(*query.Query)
		if !strings.Contains(strings.ToLower(commitInfo.Message), keyword) && !strings.Contains(strings.ToLower(commitInfo.SHA), keyword) {
			return false
		}
	}
	return true
}

func (s *RepositoriesSvr) GetCommit(sha string) (*entity.CommitInfo, error) {
	sha = strings.ToLower(strings.TrimSpace(sha))
	if !isFullCommitSHA(sha) {
		return nil, ErrInvalidCommitSHA
	}

	s.mu.RLock()
	if s.current == nil {
		s.mu.RUnlock()
		return nil, ErrNoRepositoryOpen
	}
	repoID := common.GenerateRepoID(s.current.Path)
	s.mu.RUnlock()

	s.taskSvr.worker.mu.Lock()
	defer s.taskSvr.worker.mu.Unlock()

	repoCommitSHAs, ok := s.taskSvr.worker.scannedRepoInfo.RepoCommitSHAs[repoID]
	if !ok {
		return nil, fmt.Errorf("%w for repo %s", ErrRepositoryNotScanned, repoID)
	}
	if _, ok := repoCommitSHAs[sha]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrCommitNotFound, sha)
	}
	commitInfo, ok := s.taskSvr.worker.scannedRepoInfo.Commits[sha]
	if !ok {
		return nil, fmt.Errorf("commit cache is missing SHA %s", sha)
	}

	return &commitInfo, nil
}

func (s *RepositoriesSvr) GetFiles(sha string, fileQuery entity.CommitFileQuery) ([]entity.CommitFile, string, error) {
	sha = strings.ToLower(strings.TrimSpace(sha))
	if !isFullCommitSHA(sha) {
		return nil, "", ErrInvalidCommitSHA
	}

	s.mu.RLock()
	if s.current == nil {
		s.mu.RUnlock()
		return nil, "", ErrNoRepositoryOpen
	}
	repoID := common.GenerateRepoID(s.current.Path)
	s.mu.RUnlock()

	s.taskSvr.worker.mu.Lock()
	repoCommitSHAs, ok := s.taskSvr.worker.scannedRepoInfo.RepoCommitSHAs[repoID]
	if !ok {
		s.taskSvr.worker.mu.Unlock()
		return nil, "", fmt.Errorf("%w for repo %s", ErrRepositoryNotScanned, repoID)
	}
	if _, ok := repoCommitSHAs[sha]; !ok {
		s.taskSvr.worker.mu.Unlock()
		return nil, "", fmt.Errorf("%w: %s", ErrCommitNotFound, sha)
	}
	cachedFiles, ok := s.taskSvr.worker.scannedRepoInfo.CommitsFiles[sha]
	if !ok {
		s.taskSvr.worker.mu.Unlock()
		return nil, "", fmt.Errorf("commit file cache is missing SHA %s", sha)
	}
	commitFiles := append([]entity.CommitFile(nil), cachedFiles...)
	s.taskSvr.worker.mu.Unlock()

	limit := fileQuery.Limit
	if limit < 0 {
		return nil, "", ErrInvalidCommitQuery
	}
	if limit == 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	var sortName, order string

	if fileQuery.Order != nil {
		order = strings.ToLower(strings.TrimSpace(*fileQuery.Order))
	} else {
		order = "desc"
	}

	if fileQuery.Sort != nil {
		sortName = strings.TrimSpace(*fileQuery.Sort)
	} else {
		sortName = "introducedBytes"
	}
	if order != "asc" && order != "desc" {
		return nil, "", ErrInvalidCommitQuery
	}
	switch sortName {
	case "introducedBytes", "path", "newBytes", "additions", "deletions":
	default:
		return nil, "", ErrInvalidCommitQuery
	}

	cursor := 0
	if fileQuery.Cursor != nil && *fileQuery.Cursor != "" {
		data, err := base64.RawURLEncoding.DecodeString(*fileQuery.Cursor)
		if err != nil {
			return nil, "", ErrInvalidCommitCursor
		}
		parts := strings.Split(string(data), ":")
		if len(parts) != 4 || parts[1] != sha || parts[2] != sortName || parts[3] != order {
			return nil, "", ErrInvalidCommitCursor
		}
		cursor, err = strconv.Atoi(parts[0])
		if err != nil || cursor < 0 {
			return nil, "", ErrInvalidCommitCursor
		}
	}

	sortCommitFiles(commitFiles, sortName, order)

	start := cursor
	if start > len(commitFiles) {
		return nil, "", ErrInvalidCommitCursor
	}
	end := min(limit+cursor, len(commitFiles))
	var newCursor string
	if end == len(commitFiles) {
		newCursor = ""
	} else {
		cursorValue := fmt.Sprintf("%d:%s:%s:%s", end, sha, sortName, order)
		newCursor = base64.RawURLEncoding.EncodeToString([]byte(cursorValue))
	}

	return commitFiles[start:end], newCursor, nil
}

func isFullCommitSHA(sha string) bool {
	if len(sha) != 40 {
		return false
	}
	_, err := hex.DecodeString(sha)
	return err == nil
}

func sortCommitFiles(commitFiles []entity.CommitFile, sortName, order string) {
	if len(commitFiles) == 0 {
		return
	}
	switch sortName {
	case "introducedBytes":
		if order == "asc" {
			sort.Slice(commitFiles, func(i, j int) bool {
				return commitFiles[i].IntroducedBytes < commitFiles[j].IntroducedBytes
			})
		} else {
			sort.Slice(commitFiles, func(i, j int) bool {
				return commitFiles[i].IntroducedBytes > commitFiles[j].IntroducedBytes
			})
		}
	case "path":
		if order == "asc" {
			sort.Slice(commitFiles, func(i, j int) bool {
				return commitFiles[i].Path < commitFiles[j].Path
			})
		} else {
			sort.Slice(commitFiles, func(i, j int) bool {
				return commitFiles[i].Path > commitFiles[j].Path
			})
		}
	case "newBytes":
		sort.Slice(commitFiles, func(i, j int) bool {
			a, b := commitFiles[i].NewBytes, commitFiles[j].NewBytes
			if a == nil && b == nil {
				return commitFiles[i].Path < commitFiles[j].Path
			}
			if a == nil {
				return false
			}
			if b == nil {
				return true
			}
			if *a == *b {
				return commitFiles[i].Path < commitFiles[j].Path
			}
			if order == "asc" {
				return *a < *b
			}
			return *a > *b
		})
	case "additions":
		sort.Slice(commitFiles, func(i, j int) bool {
			a, b := commitFiles[i].Additions, commitFiles[j].Additions
			if a == nil && b == nil {
				return commitFiles[i].Path < commitFiles[j].Path
			}
			if a == nil {
				return false
			}
			if b == nil {
				return true
			}
			if *a == *b {
				return commitFiles[i].Path < commitFiles[j].Path
			}
			if order == "asc" {
				return *a < *b
			}
			return *a > *b
		})
	case "deletions":
		sort.Slice(commitFiles, func(i, j int) bool {
			a, b := commitFiles[i].Deletions, commitFiles[j].Deletions
			if a == nil && b == nil {
				return commitFiles[i].Path < commitFiles[j].Path
			}
			if a == nil {
				return false
			}
			if b == nil {
				return true
			}
			if *a == *b {
				return commitFiles[i].Path < commitFiles[j].Path
			}
			if order == "asc" {
				return *a < *b
			}
			return *a > *b
		})
	default:
		return
	}
}

func (s *RepositoriesSvr) Cleanup(ctx context.Context, commitShas []string, expectedHead string, autoStash bool) (newHead string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil {
		return "", ErrNoRepositoryOpen
	}
	if s.current.Head == nil || s.current.DetachedHead {
		return "", ErrCleanupUnsupported
	}
	if s.current.AnalysisStatus != entity.RepoReady {
		return "", ErrRepositoryNotScanned
	}
	if len(commitShas) == 0 {
		return "", ErrInvalidCleanup
	}

	selected := make(map[string]struct{}, len(commitShas))
	for index, sha := range commitShas {
		sha = strings.ToLower(strings.TrimSpace(sha))
		if !isFullCommitSHA(sha) {
			return "", ErrInvalidCommitSHA
		}
		if _, exists := selected[sha]; exists {
			return "", ErrInvalidCleanup
		}
		selected[sha] = struct{}{}
		commitShas[index] = sha
	}

	expectedHead = strings.ToLower(strings.TrimSpace(expectedHead))
	if !isFullCommitSHA(expectedHead) {
		return "", ErrInvalidCommitSHA
	}
	headCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	headCmd.Dir = s.current.Path
	headOutput, err := headCmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: read current HEAD: %v", ErrGitCommandFailed, err)
	}
	if strings.ToLower(*s.current.Head) != expectedHead || strings.ToLower(strings.TrimSpace(string(headOutput))) != expectedHead {
		return "", ErrRepositoryChanged
	}

	repoID := common.GenerateRepoID(s.current.Path)
	s.taskSvr.worker.mu.Lock()
	repoCommitSHAs, scanned := s.taskSvr.worker.scannedRepoInfo.RepoCommitSHAs[repoID]
	if !scanned {
		s.taskSvr.worker.mu.Unlock()
		return "", ErrRepositoryNotScanned
	}
	for sha := range selected {
		if _, exists := repoCommitSHAs[sha]; !exists {
			s.taskSvr.worker.mu.Unlock()
			return "", fmt.Errorf("%w: %s", ErrCommitNotFound, sha)
		}
	}

	reachable := make(map[string]struct{})
	stack := []string{expectedHead}
	for len(stack) > 0 {
		sha := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, visited := reachable[sha]; visited {
			continue
		}
		commit, exists := s.taskSvr.worker.scannedRepoInfo.Commits[sha]
		if !exists {
			continue
		}
		reachable[sha] = struct{}{}
		stack = append(stack, commit.ParentSHAs...)
	}
	for sha := range selected {
		if _, exists := reachable[sha]; !exists {
			s.taskSvr.worker.mu.Unlock()
			return "", fmt.Errorf("%w: commit %s is not reachable from HEAD", ErrCleanupUnsupported, sha)
		}
		commit := s.taskSvr.worker.scannedRepoInfo.Commits[sha]
		if len(commit.ParentSHAs) != 1 {
			s.taskSvr.worker.mu.Unlock()
			return "", fmt.Errorf("%w: commit %s is a root or merge commit", ErrCleanupUnsupported, sha)
		}
	}

	firstParentHistory := make([]entity.CommitInfo, 0)
	for sha := expectedHead; sha != ""; {
		commit, exists := s.taskSvr.worker.scannedRepoInfo.Commits[sha]
		if !exists {
			break
		}
		firstParentHistory = append(firstParentHistory, commit)
		if len(commit.ParentSHAs) == 0 {
			break
		}
		sha = commit.ParentSHAs[0]
	}

	oldestSelectedIndex := -1
	selectedOnFirstParent := 0
	for index, commit := range firstParentHistory {
		if _, exists := selected[commit.SHA]; exists {
			oldestSelectedIndex = index
			selectedOnFirstParent++
		}
	}
	if selectedOnFirstParent != len(selected) {
		s.taskSvr.worker.mu.Unlock()
		return "", fmt.Errorf("%w: selected commits must be on the current branch first-parent history", ErrCleanupUnsupported)
	}
	for index := 0; index < oldestSelectedIndex; index++ {
		if len(firstParentHistory[index].ParentSHAs) > 1 {
			s.taskSvr.worker.mu.Unlock()
			return "", fmt.Errorf("%w: the rewritten commit range contains a merge commit", ErrCleanupUnsupported)
		}
	}
	upstream := firstParentHistory[oldestSelectedIndex].ParentSHAs[0]
	s.taskSvr.worker.mu.Unlock()

	// exec command
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = s.current.Path
	statusOutput, err := statusCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: git status failed: %v", ErrGitCommandFailed, err)
	}
	dirty := len(bytes.TrimSpace(statusOutput)) > 0
	if dirty && !autoStash {
		return "", ErrWorkingTreeDirty
	}

	if dirty {
		stashCmd := exec.CommandContext(ctx, "git", "stash", "push", "--include-untracked", "-m", "auto-stash-before-rebase")
		stashCmd.Dir = s.current.Path
		if output, stashErr := stashCmd.CombinedOutput(); stashErr != nil {
			return "", fmt.Errorf("%w: git stash failed: %v: %s", ErrGitCommandFailed, stashErr, strings.TrimSpace(string(output)))
		}
		defer func() {
			popCmd := exec.CommandContext(context.Background(), "git", "stash", "pop")
			popCmd.Dir = s.current.Path
			if output, popErr := popCmd.CombinedOutput(); popErr != nil && err == nil {
				err = fmt.Errorf("%w: history was rewritten but git stash pop failed: %v: %s", ErrGitCommandFailed, popErr, strings.TrimSpace(string(output)))
			}
		}()
	}

	tempFile, err := os.CreateTemp("", "git-drop-editor-*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create rebase editor: %w", err)
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write([]byte(common.EditorScript)); err != nil {
		tempFile.Close()
		return "", err
	}
	if err := tempFile.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tempFile.Name(), 0o755); err != nil {
		return "", err
	}

	env := append(os.Environ(),
		"GIT_SEQUENCE_EDITOR=sh "+strconv.Quote(filepath.ToSlash(tempFile.Name())),
		"TARGET_SHAS="+strings.Join(commitShas, " "),
		"GIT_TERMINAL_PROMPT=0",
	)
	rebaseCmd := exec.CommandContext(ctx, "git", "-c", "core.abbrev=40", "rebase", "-i", upstream)
	rebaseCmd.Dir = s.current.Path
	rebaseCmd.Env = env
	if output, rebaseErr := rebaseCmd.CombinedOutput(); rebaseErr != nil {
		abortCmd := exec.CommandContext(context.Background(), "git", "rebase", "--abort")
		abortCmd.Dir = s.current.Path
		_ = abortCmd.Run()
		return "", fmt.Errorf("%w: git rebase failed: %v: %s", ErrGitCommandFailed, rebaseErr, strings.TrimSpace(string(output)))
	}

	headCmd = exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	headCmd.Dir = s.current.Path
	headOutput, err = headCmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: read rewritten HEAD: %v", ErrGitCommandFailed, err)
	}
	newHead = strings.TrimSpace(string(headOutput))
	s.current.Head = &newHead
	s.current.AnalysisStatus = entity.RepoNotScanned
	if dirty {
		s.current.WorkingTreeStatus = "dirty"
	} else {
		s.current.WorkingTreeStatus = "clean"
	}

	s.taskSvr.worker.mu.Lock()
	delete(s.taskSvr.worker.scannedRepoInfo.RepoCommits, repoID)
	delete(s.taskSvr.worker.scannedRepoInfo.RepoCommitSHAs, repoID)
	s.taskSvr.worker.mu.Unlock()

	return newHead, nil
}
