package playwright

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Run executes the full Bun-based Playwright workflow against the provided repo directory.
func Run(ctx context.Context, repoDir string) error {
	if err := Prepare(ctx, repoDir); err != nil {
		return err
	}

	srvCmd, err := startServer(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("start server: %w", err)
	}
	defer stopServer(srvCmd)

	if err := waitForHTTP("http://localhost:4242", 30*time.Second); err != nil {
		return fmt.Errorf("wait for server: %w", err)
	}

	if err := runCmd(ctx, repoDir, nil, "bun", "x", "playwright", "install"); err != nil {
		return fmt.Errorf("playwright install failed: %w", err)
	}

	if err := runCmd(ctx, repoDir, nil, "bun", "x", "playwright", "test"); err != nil {
		return fmt.Errorf("playwright suite failed: %w", err)
	}

	return nil
}

// Prepare installs dependencies, regenerates templates, rebuilds Tailwind output, and ensures the Go binary is up to date.
func Prepare(ctx context.Context, repoDir string) error {
	steps := []struct {
		name string
		run  func(context.Context, string) error
	}{
		{"bun install", runBunInstall},
		{"templ generate", runTemplGenerate},
		{"tailwind rebuild", rebuildTailwind},
		{"go build", runGoBuild},
	}

	for _, step := range steps {
		if err := step.run(ctx, repoDir); err != nil {
			return fmt.Errorf("%s failed: %w", step.name, err)
		}
	}

	return nil
}

func runBunInstall(ctx context.Context, repoDir string) error {
	return runCmd(ctx, repoDir, nil, "bun", "install")
}

func runTemplGenerate(ctx context.Context, repoDir string) error {
	return runCmd(ctx, repoDir, nil, "templ", "generate")
}

func rebuildTailwind(ctx context.Context, repoDir string) error {
	args := []string{
		"x", "tailwindcss",
		"-i", "static/css/index.css",
		"-o", "static/css/out.css",
		"--content", "./components/**/*",
		"--content", "./pages/**/*",
		"--content", "./layouts/**/*",
	}
	if err := runCmd(ctx, repoDir, nil, "bun", args...); err != nil {
		return err
	}

	outPath := filepath.Join(repoDir, "static/css/out.css")
	data, err := os.ReadFile(outPath)
	if err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])[:8]

	pattern := filepath.Join(repoDir, "static/css", "out.*.css")
	matches, _ := filepath.Glob(pattern)
	for _, match := range matches {
		if strings.HasSuffix(match, "out.css") {
			continue
		}
		_ = os.Remove(match)
	}

	hashedPath := filepath.Join(repoDir, "static/css", fmt.Sprintf("out.%s.css", hash))
	if err := os.WriteFile(hashedPath, data, 0o644); err != nil {
		return err
	}

	return nil
}

func runGoBuild(ctx context.Context, repoDir string) error {
	env := append(os.Environ(), "GOWORK=off")
	return runCmd(ctx, repoDir, env, "go", "build", "-o", "datastarui", "main.go")
}

func startServer(ctx context.Context, repoDir string) (*exec.Cmd, error) {
	binaryPath := filepath.Join(repoDir, "datastarui")
	var cmd *exec.Cmd
	if _, err := os.Stat(binaryPath); err == nil {
		cmd = exec.CommandContext(ctx, binaryPath)
	} else {
		cmd = exec.CommandContext(ctx, "go", "run", ".")
		cmd.Env = append(os.Environ(), "GOWORK=off")
	}
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func stopServer(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
	}
}

func runCmd(ctx context.Context, repoDir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = repoDir
	if env != nil {
		cmd.Env = env
	}
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
