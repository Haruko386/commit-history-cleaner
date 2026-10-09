package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/Haruko386/commit-history-cleaner/internal/entity"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

type WorkerSvr struct {
	mu              sync.Mutex
	jobQueue        chan Job
	scannedRepoInfo ScannedRepoInfo
}

type ScannedRepoInfo struct {
	RepoCommits    map[string][]entity.CommitInfo
	RepoCommitSHAs map[string]map[string]struct{}
	Commits        map[string]entity.CommitInfo
	CommitsFiles   map[string][]entity.CommitFile
}

type Job struct {
	task *entity.Task
	repo *RepoData

	taskMu *sync.Mutex
	repoMu *sync.RWMutex
}

func NewWorker() *WorkerSvr {
	return &WorkerSvr{
		mu:       sync.Mutex{},
		jobQueue: make(chan Job, 4),
		scannedRepoInfo: ScannedRepoInfo{
			RepoCommits:    make(map[string][]entity.CommitInfo),
			RepoCommitSHAs: make(map[string]map[string]struct{}),
			Commits:        make(map[string]entity.CommitInfo),
			CommitsFiles:   make(map[string][]entity.CommitFile),
		},
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
	repoID := common.GenerateRepoID(repo.Path)
	repoCommitSHAs := make(map[string]struct{}, len(commitInfos))
	s.scannedRepoInfo.RepoCommits[repoID] = commitInfos
	for _, commitInfo := range commitInfos {
		repoCommitSHAs[commitInfo.SHA] = struct{}{}
		s.scannedRepoInfo.Commits[commitInfo.SHA] = commitInfo
		s.scannedRepoInfo.CommitsFiles[commitInfo.SHA] = commitInfo.Files
	}
	s.scannedRepoInfo.RepoCommitSHAs[repoID] = repoCommitSHAs
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	branchesMap, tagsMap, err := buildRefMap(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to build ref map: %w", err)
	}

	_, err = repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return make([]entity.CommitInfo, 0), nil
		}
		return nil, fmt.Errorf("failed to read HEAD: %w", err)
	}

	commitIter, err := repo.Log(&git.LogOptions{All: true, Order: git.LogOrderCommitterTime})
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
			SHA:         c.Hash.String(),
			Message:     c.Message,
			AuthorName:  c.Author.Name,
			AuthorEmail: c.Author.Email,
			Commiter: entity.Commiter{
				Name:  c.Committer.Name,
				Email: c.Committer.Email,
			},
			AuthoredAt:  c.Author.When,
			CommittedAt: c.Committer.When,
		}

		for _, parentHash := range c.ParentHashes {
			info.ParentSHAs = append(info.ParentSHAs, parentHash.String())
		}

		info.Refs = entity.Refs{
			Branches: branchesMap[c.Hash],
			Tags:     tagsMap[c.Hash],
		}

		stats, files, err := calcStatsAndFiles(c)
		if err != nil {
			return fmt.Errorf("failed to calc stats for %s: %w", c.Hash, err)
		}
		info.Stats = stats
		info.Files = files

		commits = append(commits, info)
		onProgress(len(commits))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	commitIndexes := make(map[string]int, len(commits))
	for index := range commits {
		commitIndexes[commits[index].SHA] = index
	}
	snapshotByCommit := make(map[string]int64, len(commits))
	visiting := make(map[string]bool)
	var calculateSnapshot func(string) (int64, error)
	calculateSnapshot = func(sha string) (int64, error) {
		if snapshot, ok := snapshotByCommit[sha]; ok {
			return snapshot, nil
		}
		if visiting[sha] {
			return 0, fmt.Errorf("commit graph contains a cycle at %s", sha)
		}
		index, ok := commitIndexes[sha]
		if !ok {
			return 0, fmt.Errorf("commit %s is missing from scan results", sha)
		}
		visiting[sha] = true
		defer delete(visiting, sha)

		commit := &commits[index]
		snapshot := commit.Stats.SnapshotBytes
		if len(commit.ParentSHAs) > 0 {
			if _, ok := commitIndexes[commit.ParentSHAs[0]]; ok {
				parentSnapshot, err := calculateSnapshot(commit.ParentSHAs[0])
				if err != nil {
					return 0, err
				}
				snapshot = parentSnapshot
				for _, file := range commit.Files {
					switch file.Status {
					case entity.FileStatusAdded, entity.FileStatusCopied:
						if file.NewBytes != nil {
							snapshot += *file.NewBytes
						}
					case entity.FileStatusDeleted:
						if file.OldBytes != nil {
							snapshot -= *file.OldBytes
						}
					default:
						if file.OldBytes != nil {
							snapshot -= *file.OldBytes
						}
						if file.NewBytes != nil {
							snapshot += *file.NewBytes
						}
					}
				}
			} else {
				commitObject, err := repo.CommitObject(plumbing.NewHash(sha))
				if err != nil {
					return 0, err
				}
				tree, err := commitObject.Tree()
				if err != nil {
					return 0, err
				}
				fileIter := tree.Files()
				err = fileIter.ForEach(func(file *object.File) error {
					snapshot += file.Size
					return nil
				})
				if err != nil {
					return 0, err
				}
			}
		}
		commit.Stats.SnapshotBytes = snapshot
		snapshotByCommit[sha] = snapshot
		return snapshot, nil
	}
	for index := range commits {
		if _, err := calculateSnapshot(commits[index].SHA); err != nil {
			return nil, fmt.Errorf("failed to calculate snapshot for %s: %w", commits[index].SHA, err)
		}
	}

	return commits, nil
}

func calcStatsAndFiles(c *object.Commit) (entity.Stats, []entity.CommitFile, error) {
	var stats entity.Stats
	var files []entity.CommitFile

	currentTree, err := c.Tree()
	if err != nil {
		return stats, files, err
	}

	// init commit
	if c.NumParents() == 0 {
		fileIter := currentTree.Files()
		defer fileIter.Close()
		for {
			file, err := fileIter.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return stats, files, err
			}
			binary, err := file.IsBinary()
			if err != nil {
				return stats, files, err
			}
			size := file.Size
			blob := file.Hash.String()
			commitFile := entity.CommitFile{
				Path:            file.Name,
				Status:          entity.FileStatusAdded,
				NewBlob:         &blob,
				NewBytes:        &size,
				IntroducedBytes: size,
				Binary:          binary,
			}
			if !binary {
				lines, err := file.Lines()
				if err != nil {
					return stats, files, err
				}
				additions, deletions := len(lines), 0
				commitFile.Additions = &additions
				commitFile.Deletions = &deletions
			}
			files = append(files, commitFile)

			stats.AddedFiles++
			stats.SnapshotBytes += file.Size
		}
		stats.IntroducedBytes = stats.SnapshotBytes
		return stats, files, nil
	}

	parent, err := c.Parent(0)
	if err != nil {
		return stats, files, err
	}
	parentTree, err := parent.Tree()
	if err != nil {
		return stats, files, err
	}

	changes, err := parentTree.Diff(currentTree)
	if err != nil {
		return stats, files, err
	}

	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return stats, files, err
		}

		fromFile, toFile, err := change.Files()
		if err != nil {
			return stats, files, err
		}
		binary := fromFile == nil && toFile == nil
		if fromFile != nil {
			fromBinary, err := fromFile.IsBinary()
			if err != nil {
				return stats, files, err
			}
			binary = binary || fromBinary
		}
		if toFile != nil {
			toBinary, err := toFile.IsBinary()
			if err != nil {
				return stats, files, err
			}
			binary = binary || toBinary
		}

		commitFile := entity.CommitFile{Binary: binary}
		switch action {
		case merkletrie.Insert:
			commitFile.Status = entity.FileStatusAdded
			commitFile.Path = change.To.Name
			if toFile != nil {
				blob, size := toFile.Hash.String(), toFile.Size
				commitFile.NewBlob = &blob
				commitFile.NewBytes = &size
				commitFile.IntroducedBytes = size
			}
			stats.AddedFiles++
			stats.IntroducedBytes += commitFile.IntroducedBytes
		case merkletrie.Modify:
			commitFile.Status = entity.FileStatusModified
			if change.From.Name != change.To.Name {
				previousPath := change.From.Name
				commitFile.Status = entity.FileStatusRenamed
				commitFile.PreviousPath = &previousPath
			} else if isFileTypeChange(change.From.TreeEntry.Mode, change.To.TreeEntry.Mode) {
				commitFile.Status = entity.FileStatusTypeChanged
			}
			commitFile.Path = change.To.Name
			if fromFile != nil {
				blob, size := fromFile.Hash.String(), fromFile.Size
				commitFile.OldBlob = &blob
				commitFile.OldBytes = &size
			}
			if toFile != nil {
				blob, size := toFile.Hash.String(), toFile.Size
				commitFile.NewBlob = &blob
				commitFile.NewBytes = &size
				if commitFile.OldBlob == nil || *commitFile.OldBlob != blob {
					commitFile.IntroducedBytes = size
				}
			}
			stats.ModifiedFiles++
			stats.IntroducedBytes += commitFile.IntroducedBytes
		case merkletrie.Delete:
			commitFile.Status = entity.FileStatusDeleted
			commitFile.Path = change.From.Name
			if fromFile != nil {
				blob, size := fromFile.Hash.String(), fromFile.Size
				commitFile.OldBlob = &blob
				commitFile.OldBytes = &size
			}
			stats.DeletedFiles++
		}

		if !binary {
			patch, err := change.Patch()
			if err != nil {
				return stats, files, err
			}
			additions, deletions := 0, 0
			for _, fileStat := range patch.Stats() {
				additions += fileStat.Addition
				deletions += fileStat.Deletion
			}
			commitFile.Additions = &additions
			commitFile.Deletions = &deletions
		}

		files = append(files, commitFile)
	}

	deletedByBlob := make(map[string][]int)
	for index := range files {
		if files[index].Status == entity.FileStatusDeleted && files[index].OldBlob != nil {
			deletedByBlob[*files[index].OldBlob] = append(deletedByBlob[*files[index].OldBlob], index)
		}
	}
	removed := make(map[int]struct{})
	for index := range files {
		file := &files[index]
		if file.Status != entity.FileStatusAdded || file.NewBlob == nil {
			continue
		}
		if deleted := deletedByBlob[*file.NewBlob]; len(deleted) > 0 {
			deletedIndex := deleted[0]
			deletedByBlob[*file.NewBlob] = deleted[1:]
			previousPath := files[deletedIndex].Path
			file.Status = entity.FileStatusRenamed
			file.PreviousPath = &previousPath
			file.OldBlob = files[deletedIndex].OldBlob
			file.OldBytes = files[deletedIndex].OldBytes
			stats.IntroducedBytes -= file.IntroducedBytes
			file.IntroducedBytes = 0
			stats.AddedFiles--
			stats.DeletedFiles--
			stats.ModifiedFiles++
			if !file.Binary {
				additions, deletions := 0, 0
				file.Additions = &additions
				file.Deletions = &deletions
			}
			removed[deletedIndex] = struct{}{}
			continue
		}
	}

	addedBlobs := make(map[string]struct{})
	for index := range files {
		if files[index].Status == entity.FileStatusAdded && files[index].NewBlob != nil {
			addedBlobs[*files[index].NewBlob] = struct{}{}
		}
	}
	parentPathsByBlob := make(map[string][]string)
	if len(addedBlobs) > 0 {
		walker := object.NewTreeWalker(parentTree, true, nil)
		defer walker.Close()
		for {
			path, entry, err := walker.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return stats, files, err
			}
			if entry.Mode == filemode.Dir || entry.Mode == filemode.Submodule {
				continue
			}
			blob := entry.Hash.String()
			if _, ok := addedBlobs[blob]; ok {
				parentPathsByBlob[blob] = append(parentPathsByBlob[blob], path)
			}
		}
	}

	for index := range files {
		file := &files[index]
		if file.Status != entity.FileStatusAdded || file.NewBlob == nil {
			continue
		}
		for _, previousPath := range parentPathsByBlob[*file.NewBlob] {
			if previousPath == file.Path {
				continue
			}
			file.Status = entity.FileStatusCopied
			file.PreviousPath = new(previousPath)
			file.OldBlob = file.NewBlob
			file.OldBytes = file.NewBytes
			stats.IntroducedBytes -= file.IntroducedBytes
			file.IntroducedBytes = 0
			if !file.Binary {
				additions, deletions := 0, 0
				file.Additions = &additions
				file.Deletions = &deletions
			}
			break
		}
	}
	if len(removed) > 0 {
		filtered := make([]entity.CommitFile, 0, len(files)-len(removed))
		for index, file := range files {
			if _, ok := removed[index]; !ok {
				filtered = append(filtered, file)
			}
		}
		files = filtered
	}

	return stats, files, nil
}

func isFileTypeChange(from, to filemode.FileMode) bool {
	if from == to {
		return false
	}
	fromRegular := from == filemode.Regular || from == filemode.Deprecated || from == filemode.Executable
	toRegular := to == filemode.Regular || to == filemode.Deprecated || to == filemode.Executable
	return !fromRegular || !toRegular
}

func buildRefMap(repo *git.Repository) (map[plumbing.Hash][]string, map[plumbing.Hash][]string, error) {
	branchesMap := make(map[plumbing.Hash][]string)
	tagsMap := make(map[plumbing.Hash][]string)

	branchIter, err := repo.Branches()
	if err != nil {
		return nil, nil, err
	}
	defer branchIter.Close()

	err = branchIter.ForEach(func(ref *plumbing.Reference) error {
		branchesMap[ref.Hash()] = append(branchesMap[ref.Hash()], ref.Name().Short())
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	tagIter, err := repo.Tags()
	if err != nil {
		return nil, nil, err
	}
	defer tagIter.Close()

	err = tagIter.ForEach(func(ref *plumbing.Reference) error {
		target := ref.Hash()

		if tagObj, err := repo.TagObject(target); err == nil {
			target = tagObj.Target
		}

		tagsMap[target] = append(tagsMap[target], ref.Name().Short())
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return branchesMap, tagsMap, nil
}
