package common

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"
)

const (
	GithubAPIVersion = "2026-03-10"
	GithubBaseUrl    = "https://api.github.com"
)

func GenerateUUID() string {
	id := uuid.New()
	return id.String()
}

func GenerateRequestID() string {
	return "req" + "_" + GenerateUUID()
}

func GenerateRepoID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return "repo" + "_" + hex.EncodeToString(sum[:])
}

func GenerateTaskID() string {
	return "task" + "_" + GenerateUUID()
}
