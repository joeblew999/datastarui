package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
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

	baseCtx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	// Ensure dependencies are installed before launching the suite.
	if err := runCmd(baseCtx, repoDir, "bun", "install"); err != nil {
		t.Fatalf("bun install failed: %v", err)
	}
	if err := runCmd(baseCtx, repoDir, "bun", "x", "playwright", "install"); err != nil {
		t.Fatalf("playwright install failed: %v", err)
	}

	// Launch the dev server (prefer local binary, fall back to go run).
	serverCmd, err := serverCommand(baseCtx, repoDir)
	if err != nil {
		t.Fatalf("prepare server command: %v", err)
	}

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start datastarui server: %v", err)
	}

	t.Cleanup(func() {
		_ = serverCmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() {
			_ = serverCmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = serverCmd.Process.Kill()
		}
	})

	if err := waitForHTTP("http://localhost:4242", 30*time.Second); err != nil {
		t.Fatalf("server did not become ready: %v", err)
	}

	if err := runCmd(baseCtx, repoDir, "bun", "x", "playwright", "test"); err != nil {
		t.Fatalf("playwright suite failed: %v", err)
	}
}

func serverCommand(ctx context.Context, repoDir string) (*exec.Cmd, error) {
	binaryPath := filepath.Join(repoDir, "datastarui")
	if _, err := os.Stat(binaryPath); err == nil {
		cmd := exec.CommandContext(ctx, binaryPath)
		cmd.Dir = repoDir
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd, nil
	}

	cmd := exec.CommandContext(ctx, "go", "run", ".")
	cmd.Dir = repoDir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd, nil
}

func runCmd(ctx context.Context, repoDir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitForHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return errors.New("timeout waiting for http endpoint")
}
