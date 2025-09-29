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

// Config captures the paths and commands required to run the workflow.
type Config struct {
	TailwindInput   string
	TailwindOutput  string
	TailwindContent []string
	Binary          string
	ServerCommand   []string
	BaseURL         string
}

// DefaultConfig returns the conventions used by the DatastarUI fork.
func DefaultConfig() Config {
	return Config{
		TailwindInput:   "static/css/index.css",
		TailwindOutput:  "static/css/out.css",
		TailwindContent: []string{"./components/**/*", "./pages/**/*", "./layouts/**/*"},
		Binary:          "datastarui",
		ServerCommand:   nil,
		BaseURL:         "http://localhost:4242",
	}
}

// Run executes the full Bun-based Playwright workflow against the provided repo directory.
func Run(ctx context.Context, repoDir string, cfg Config) error {
	if err := Prepare(ctx, repoDir, cfg); err != nil {
		return err
	}

	srvCmd, err := startServer(ctx, repoDir, cfg)
	if err != nil {
		return fmt.Errorf("start server: %w", err)
	}
	defer stopServer(srvCmd)

	if err := waitForHTTP(cfg.BaseURL, 30*time.Second); err != nil {
		return fmt.Errorf("wait for server: %w", err)
	}

	if err := runCmd(ctx, repoDir, os.Environ(), "bun", "x", "playwright", "install"); err != nil {
		return fmt.Errorf("playwright install failed: %w", err)
	}

	env := append(os.Environ(), fmt.Sprintf("PLAYWRIGHT_BASE_URL=%s", cfg.BaseURL))
	if err := runCmd(ctx, repoDir, env, "bun", "x", "playwright", "test"); err != nil {
		return fmt.Errorf("playwright suite failed: %w", err)
	}

	return nil
}

// Prepare installs dependencies, regenerates templates, rebuilds Tailwind output, and ensures the Go binary is up to date.
func Prepare(ctx context.Context, repoDir string, cfg Config) error {
	if err := runBunInstall(ctx, repoDir); err != nil {
		return fmt.Errorf("bun install failed: %w", err)
	}

	if err := runTemplGenerate(ctx, repoDir); err != nil {
		return fmt.Errorf("templ generate failed: %w", err)
	}

	if err := rebuildTailwind(ctx, repoDir, cfg); err != nil {
		return fmt.Errorf("tailwind rebuild failed: %w", err)
	}

	if err := runGoBuild(ctx, repoDir, cfg); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	return nil
}

func runBunInstall(ctx context.Context, repoDir string) error {
	return runCmd(ctx, repoDir, os.Environ(), "bun", "install")
}

func runTemplGenerate(ctx context.Context, repoDir string) error {
	return runCmd(ctx, repoDir, os.Environ(), "templ", "generate")
}

func rebuildTailwind(ctx context.Context, repoDir string, cfg Config) error {
	args := []string{
		"x", "tailwindcss",
		"-i", cfg.TailwindInput,
		"-o", cfg.TailwindOutput,
	}
	for _, content := range cfg.TailwindContent {
		args = append(args, "--content", content)
	}

	if err := runCmd(ctx, repoDir, os.Environ(), "bun", args...); err != nil {
		return err
	}

	outPath := filepath.Join(repoDir, cfg.TailwindOutput)
	data, err := os.ReadFile(outPath)
	if err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])[:8]

	dir := filepath.Dir(outPath)
	base := filepath.Base(outPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ""
	}

	pattern := filepath.Join(dir, fmt.Sprintf("%s.*%s", name, ext))
	matches, _ := filepath.Glob(pattern)
	for _, match := range matches {
		if match == outPath {
			continue
		}
		_ = os.Remove(match)
	}

	hashedPath := filepath.Join(dir, fmt.Sprintf("%s.%s%s", name, hash, ext))
	if err := os.WriteFile(hashedPath, data, 0o644); err != nil {
		return err
	}

	return nil
}

func runGoBuild(ctx context.Context, repoDir string, cfg Config) error {
	env := append(os.Environ(), "GOWORK=off")
	args := []string{"build"}
	if cfg.Binary != "" {
		args = append(args, "-o", cfg.Binary)
	}
	args = append(args, "main.go")
	return runCmd(ctx, repoDir, env, "go", args...)
}

func startServer(ctx context.Context, repoDir string, cfg Config) (*exec.Cmd, error) {
	env := os.Environ()
	var cmd *exec.Cmd

	if len(cfg.ServerCommand) > 0 {
		cmd = exec.CommandContext(ctx, cfg.ServerCommand[0], cfg.ServerCommand[1:]...)
	} else {
		binaryPath := cfg.Binary
		if binaryPath != "" {
			binaryPath = filepath.Join(repoDir, binaryPath)
			if _, err := os.Stat(binaryPath); err == nil {
				cmd = exec.CommandContext(ctx, binaryPath)
			}
		}
		if cmd == nil {
			cmd = exec.CommandContext(ctx, "go", "run", ".")
			env = append(env, "GOWORK=off")
		}
	}

	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

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
	if len(env) > 0 {
		cmd.Env = env
	} else {
		cmd.Env = os.Environ()
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
