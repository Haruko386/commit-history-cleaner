package test

import (
	"context"
	"strings"
	"testing"

	"example.com/m/v2/internal/service"
)

func TestHealthServiceReturnsNormalizedGitVersion(t *testing.T) {
	version, err := service.NewHealthSvr().CheckHealth(context.Background())
	if err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
	if version == "" {
		t.Fatal("CheckHealth() returned an empty Git version")
	}
	if version != strings.TrimSpace(version) {
		t.Fatalf("Git version contains surrounding whitespace: %q", version)
	}
	if strings.HasPrefix(version, "git version ") {
		t.Fatalf("Git version contains the command prefix: %q", version)
	}
}
