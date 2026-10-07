package service

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"example.com/m/v2/internal/common"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type RepositoriesSvr struct {
	mu      sync.RWMutex
	current *RepoData
}

func NewRepositoriesSvr() *RepositoriesSvr {
	return &RepositoriesSvr{}
}

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
			repoData.AnalysisStatus = "not_scanned"
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
	repoData.AnalysisStatus = "not_scanned"

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
		return http.StatusOK, s.current, ""
	}
	return http.StatusNotFound, nil, common.NoRepositoryOpen
}

func (s *RepositoriesSvr) ExitCurrentRepository() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil {
		return true
	}
	// TODO: waiting for `scan` API to be done
	s.current = nil
	return true
}
