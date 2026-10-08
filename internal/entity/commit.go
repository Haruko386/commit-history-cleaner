package entity

import "time"

type CommitInfo struct {
	SHA          string    `json:"sha"`
	Message      string    `json:"message"`
	AuthorName   string    `json:"authorName"`
	AuthorEmail  string    `json:"authorEmail"`
	AuthoredTime time.Time `json:"authoredAt"`
	ParentSHAs   []string  `json:"parentShas"`
}
