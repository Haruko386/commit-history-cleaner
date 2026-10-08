package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/Haruko386/commit-history-cleaner/internal/entity"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type WorkerSvr struct {
	mu              sync.Mutex
	jobQueue        chan Job
	scannedRepoInfo map[string][]entity.CommitInfo
}

type Job struct {
	task *entity.Task
	repo *RepoData

	taskMu *sync.Mutex
	repoMu *sync.RWMutex
}

func NewWorker() *WorkerSvr {
	return &WorkerSvr{
		mu:              sync.Mutex{},
		jobQueue:        make(chan Job, 4),
		scannedRepoInfo: make(map[string][]entity.CommitInfo),
	}
}

func (s *WorkerSvr) ConsumeTask(ctx context.Context) {
	for {
		select {
		case job := <-s.jobQueue:
			err := s.Scan(job.task.Ctx, job.taskMu, job.repoMu, &job)
			if err != nil {
				log.Println(err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *WorkerSvr) Scan(ctx context.Context, taskMu *sync.Mutex, repoMu *sync.RWMutex, job *Job) error {
	task := job.task
	repo := job.repo

	taskMu.Lock()
	// check if it has been canceled
	if task.Status == entity.Cancelled {
		taskMu.Unlock()
		return ErrScanAlreadyCancelled
	}

	job.task.StartedAt = new(time.Now())
	job.task.Status = entity.Running
	taskMu.Unlock()

	repoMu.Lock()
	repo.AnalysisStatus = entity.RepoScanning
	repoMu.Unlock()

	path := filepath.Join(repo.Path, ".git")

	total := repo.CommitCount

	taskMu.Lock()
	task.Progress.Total = &total
	task.Progress.Current = 0
	task.Progress.Percent = new(0.0)
	task.Progress.Phase = entity.Running
	taskMu.Unlock()

	commitInfos, err := scanRepo(ctx, path, func(current int) {
		taskMu.Lock()
		defer taskMu.Unlock()

		task.Progress.Current = current
		if total > 0 {
			percent := float64(current) / float64(total) * 100
			task.Progress.Percent = &percent
		}
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) { // canceled
			taskMu.Lock()
			task.FinishedAt = new(time.Now())
			task.Status = entity.Cancelled
			taskMu.Unlock()

			repoMu.Lock()
			repo.AnalysisStatus = entity.RepoNotScanned
			repoMu.Unlock()
			return nil
		}
		// failed
		taskMu.Lock()
		task.FinishedAt = new(time.Now())
		task.Status = entity.Failed
		task.Error = new(err.Error())
		task.Progress.Phase = entity.Failed
		taskMu.Unlock()

		repoMu.Lock()
		repo.AnalysisStatus = entity.RepoFailed
		repoMu.Unlock()

		return err
	}

	// successes
	s.mu.Lock()
	s.scannedRepoInfo[common.GenerateRepoID(repo.Path)] = commitInfos
	s.mu.Unlock()

	taskMu.Lock()
	task.FinishedAt = new(time.Now())
	task.Status = entity.Completed
	task.Progress.Phase = entity.Completed
	task.Progress.Percent = new(100.0)
	taskMu.Unlock()

	repoMu.Lock()
	repo.AnalysisStatus = entity.RepoReady
	repoMu.Unlock()
	return nil
}

// scanRepo scan the repo's .git dir
func scanRepo(ctx context.Context, path string, onProgress func(current int)) ([]entity.CommitInfo, error) {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open git repo: %w", err)
	}

	headRef, err := repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return make([]entity.CommitInfo, 0), nil
		}
		return nil, fmt.Errorf("failed to read HEAD: %w", err)
	}

	startHash := headRef.Hash()
	commitIter, err := repo.Log(&git.LogOptions{From: startHash})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit iter: %w", err)
	}
	defer commitIter.Close()

	var commits []entity.CommitInfo

	err = commitIter.ForEach(func(c *object.Commit) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info := entity.CommitInfo{
			SHA:          c.Hash.String(),
			Message:      c.Message,
			AuthorName:   c.Author.Name,
			AuthorEmail:  c.Author.Email,
			AuthoredTime: c.Author.When,
		}

		for _, parentHash := range c.ParentHashes {
			info.ParentSHAs = append(info.ParentSHAs, parentHash.String())
		}
		commits = append(commits, info)
		onProgress(len(commits))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	return commits, nil
}
