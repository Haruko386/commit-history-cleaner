package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Haruko386/commit-history-cleaner/internal/entity"
)

type TaskSvr struct {
	mu           sync.Mutex
	ctx          context.Context
	taskList     map[string]*entity.Task
	repoTaskList map[string]string

	worker *WorkerSvr
}

func NewTaskSvr() *TaskSvr {
	return &TaskSvr{
		mu:           sync.Mutex{},
		ctx:          context.Background(),
		taskList:     make(map[string]*entity.Task),
		repoTaskList: make(map[string]string),
		worker:       NewWorker(),
	}
}

func (s *TaskSvr) Start() {
	go func() {
		s.worker.ConsumeTask(s.ctx)
	}()
}

func (s *TaskSvr) AddTask(task *entity.Task, repo *RepoData, taskMu *sync.Mutex, repoMu *sync.RWMutex) {
	s.worker.jobQueue <- Job{task: task, repo: repo, taskMu: taskMu, repoMu: repoMu}
}

func (s *TaskSvr) GetTask(taskId string) (*entity.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.taskList[taskId]
	if !ok {
		return nil, fmt.Errorf("task %s not exist", taskId)
	}

	cpTask := &entity.Task{
		TaskID: task.TaskID,
		Type:   task.Type,
		Status: task.Status,
		Progress: entity.Progress{
			Phase:   task.Progress.Phase,
			Current: task.Progress.Current,
			Total:   task.Progress.Total,
			Percent: task.Progress.Percent,
		},
		CreatedAt:  task.CreatedAt,
		StartedAt:  task.StartedAt,
		FinishedAt: task.FinishedAt,
	}

	if task.Error != nil {
		cpTask.Error = task.Error
	}

	task = nil

	return cpTask, nil
}

func (s *TaskSvr) CancelTask(taskId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.taskList[taskId]
	if !ok {
		return fmt.Errorf("task %s not exist", taskId)
	}

	switch task.Status {
	case entity.Queued, entity.Running:
		if task.Cancel != nil {
			task.Cancel()
		}
		task.Status = entity.Cancelled
		task.Progress.Phase = entity.Cancelled
		task.FinishedAt = new(time.Now())
	default:
	}

	return nil
}
