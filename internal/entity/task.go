package entity

import "time"

// task type
const (
	RepositoryScan = "repository_scan"
)

// task status
const (
	Queued    = "queued"
	Running   = "running"
	Completed = "completed"
	Cancelled = "cancelled"
	Failed    = "failed"
)

type Task struct {
	TaskID     string     `json:"taskId"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	Progress   Progress   `json:"progress"`
	CreatedAt  *time.Time `json:"createdAt"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

type Progress struct {
	Phase   string   `json:"phase"`
	Current int      `json:"current"`
	Total   *int     `json:"total"`
	Percent *float64 `json:"percent"`
}
