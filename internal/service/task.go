package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/Haruko386/commit-history-cleaner/internal/entity"
)

type TaskSvr struct {
	mu           sync.Mutex
	taskList     map[string]*entity.Task
	repoTaskList map[string]string

	worker entity.Worker
}

func NewTaskSvr() *TaskSvr {
	return &TaskSvr{
		taskList:     make(map[string]*entity.Task),
		repoTaskList: make(map[string]string),
	}
}

func (s *TaskSvr) GetTask(taskId string) (*entity.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.taskList[taskId]
	if !ok {
		return nil, fmt.Errorf("task %s not exist", taskId)
	}

	return task, nil
}

func (s *TaskSvr) CancelTask(taskId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.taskList[taskId]
	if !ok {
		return fmt.Errorf("task %s not exist", taskId)
	}

	switch task.Status {
	case "queued", "running":
		task.Status = "cancelled"
		task.Progress.Phase = entity.Cancelled
		task.FinishedAt = new(time.Now())
	default:
	}

	return nil
}
