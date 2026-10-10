package entity

import "time"

type CommitInfo struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	// TODO(refactor): merge `AuthorName` and `AuthorEmail` to Author
	AuthorName  string       `json:"authorName"`
	AuthorEmail string       `json:"authorEmail"`
	AuthoredAt  time.Time    `json:"authoredAt"`
	Commiter    Commiter     `json:"commiter"`
	CommittedAt time.Time    `json:"committedAt"`
	ParentSHAs  []string     `json:"parentShas"`
	Refs        Refs         `json:"refs"`
	Stats       Stats        `json:"stats,omitempty"`
	Files       []CommitFile `json:"files,omitempty"`
}

type CommitsQuery struct {
	Cursor             *string    `form:"cursor"`
	Limit              int        `form:"limit"`
	Query              *string    `form:"query"`
	Author             *string    `form:"author"`
	Since              *time.Time `form:"since"`
	Until              *time.Time `form:"until"`
	Ref                string     `form:"ref"`
	MinIntroducedBytes int64      `form:"minIntroducedBytes"`
}

type CommitFileQuery struct {
	Cursor *string `form:"cursor"`
	Limit  int     `form:"limit"`
	Sort   *string `form:"sort"`
	Order  *string `form:"order"`
}

type CommitFile struct {
	Path            string     `json:"path"`
	PreviousPath    *string    `json:"previousPath"`
	Status          FileStatus `json:"status"`
	OldBlob         *string    `json:"oldBlob"`
	NewBlob         *string    `json:"newBlob"`
	OldBytes        *int64     `json:"oldBytes"`
	NewBytes        *int64     `json:"newBytes"`
	IntroducedBytes int64      `json:"introducedBytes"`
	Additions       *int       `json:"additions"`
	Deletions       *int       `json:"deletions"`
	Binary          bool       `json:"binary"`
}

type FileStatus string

const (
	FileStatusAdded       FileStatus = "added"
	FileStatusModified    FileStatus = "modified"
	FileStatusDeleted     FileStatus = "deleted"
	FileStatusRenamed     FileStatus = "renamed"
	FileStatusCopied      FileStatus = "copied"
	FileStatusTypeChanged FileStatus = "type_changed"
)

type CommitSummary struct {
	SHA            string    `json:"sha"`
	ShortSHA       string    `json:"shortSha"`
	Subject        string    `json:"subject"`
	Author         Author    `json:"author"`
	AuthoredAt     time.Time `json:"authoredAt"`
	CommittedAt    time.Time `json:"committedAt"`
	Parents        []string  `json:"parents"`
	Refs           Refs      `json:"refs"`
	Stats          Stats     `json:"stats,omitempty"`
	AnalysisStatus string    `json:"analysisStatus"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Commiter struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Refs struct {
	Branches []string `json:"branches"`
	Tags     []string `json:"tags"`
}

type Stats struct {
	IntroducedBytes int64 `json:"introducedBytes"`
	SnapshotBytes   int64 `json:"snapshotBytes"`
	AddedFiles      int   `json:"addedFiles"`
	ModifiedFiles   int   `json:"modifiedFiles"`
	DeletedFiles    int   `json:"deletedFiles"`
}
