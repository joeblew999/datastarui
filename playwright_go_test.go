package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	pw "github.com/coreycole/datastarui/internal/playwright"
)

func TestPlaywrightSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("playwright suite skipped in short mode")
	}

	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun runtime not found; skipping playwright suite")
	}

	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("locate repo directory: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Setenv("PLAYWRIGHT_BASE_URL", "http://localhost:4242")

	if err := pw.Run(ctx, repoDir); err != nil {
		t.Fatalf("playwright suite failed: %v", err)
	}
}
