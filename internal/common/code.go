package common

const EditorScript = `#!/bin/sh
todo_file="$1"
tmp_file="${todo_file}.tmp"
while IFS= read -r line; do
    sha=$(echo "$line" | awk '{print $2}')
    if echo "$TARGET_SHAS" | grep -qw "$sha"; then
        echo "$line" | sed 's/^pick /drop /'
    else
        echo "$line"
    fi
done < "$todo_file" > "$tmp_file"
mv "$tmp_file" "$todo_file"
`

const (
	InvalidRequest         = "INVALID_REQUEST"
	PathRequired           = "PATH_REQUIRED"
	RepositoryNotFound     = "REPOSITORY_NOT_FOUND"
	RepositoryNotScanned   = "REPOSITORY_NOT_SCANNED"
	NoRepositoryOpen       = "NO_REPOSITORY_OPEN"
	CommitNotFound         = "COMMIT_NOT_FOUND"
	ObjectNotFound         = "OBJECT_NOT_FOUND"
	TaskNotFound           = "TASK_NOT_FOUND"
	PathNotReadable        = "PATH_NOT_READABLE"
	NotAGitRepository      = "NOT_A_GIT_REPOSITORY"
	ScanAlreadyRunning     = "SCAN_ALREADY_RUNNING"
	ScanAlreadyCompleted   = "SCAN_ALREADY_COMPLETED"
	ScanAlreadyCancelled   = "SCAN_ALREADY_CANCELLED"
	RepositoryChanged      = "REPOSITORY_CHANGED"
	WorkingTreeDirty       = "WORKING_TREE_DIRTY"
	CleanupUnsupported     = "CLEANUP_UNSUPPORTED"
	GitNotAvailable        = "GIT_NOT_AVAILABLE"
	HealthCheckTimeout     = "HEALTH_CHECK_TIMEOUT"
	HealthCheckCancelled   = "HEALTH_CHECK_CANCELLED"
	HealthCheckFailed      = "HEALTH_CHECK_FAILED"
	GithubUnauthorized     = "GITHUB_UNAUTHORIZED"
	GithubForbidden        = "GITHUB_FORBIDDEN"
	GithubConnectionFailed = "GITHUB_CONNECTION_FAILED"
	GitCommandFailed       = "GIT_COMMAND_FAILED"
	InternalError          = "INTERNAL_ERROR"
)
