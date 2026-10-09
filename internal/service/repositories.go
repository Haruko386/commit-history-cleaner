package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
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
	iter, err := r.Log(&git.LogOptions{
		From: ref.Hash(),
	})
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
