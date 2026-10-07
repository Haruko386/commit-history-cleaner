package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"example.com/m/v2/internal/common"
)

var (
	ErrGitNotAvailable    = errors.New("git is not available")
	ErrGithubUnauthorized = errors.New("github token is invalid")
	ErrGithubForbidden    = errors.New("github access is forbidden")
)

type HealthSvr struct {
	httpClient *http.Client
}

func NewHealthSvr() *HealthSvr {
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	return &HealthSvr{httpClient}
}

func (s *HealthSvr) CheckHealth(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "--version").Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", fmt.Errorf("%w: %v", ErrGitNotAvailable, err)
	}

	version := normalizeGitVersion(string(out))
	if version == "" {
		return "", fmt.Errorf("%w: git returned an empty version", ErrGitNotAvailable)
	}

	return version, nil
}

func normalizeGitVersion(output string) string {
	version := strings.TrimSpace(output)
	version = strings.TrimPrefix(version, "git version ")
	return strings.TrimSpace(version)
}

func (s *HealthSvr) CheckGithubConnection(ctx context.Context, token string) (bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return false, errors.New("github token is required")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/%s", common.GithubBaseUrl, "octocat"), nil)
	if err != nil {
		return false, fmt.Errorf("create http request error: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", common.GithubAPIVersion)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("http request error: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusUnauthorized:
		return false, ErrGithubUnauthorized
	case http.StatusForbidden:
		return false, ErrGithubForbidden
	default:
		return false, fmt.Errorf("github returned non-200 status code: %d", resp.StatusCode)
	}
}
