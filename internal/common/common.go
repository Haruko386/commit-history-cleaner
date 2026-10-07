package common

import "github.com/google/uuid"

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
