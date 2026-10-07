package service

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var ErrGitNotAvailable = errors.New("git is not available")

type HealthSvr struct {
}

func NewHealthSvr() *HealthSvr {
	return &HealthSvr{}
}

func (h *HealthSvr) CheckHealth(ctx context.Context) (string, error) {
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
