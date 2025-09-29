package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	pw "github.com/coreycole/datastarui/internal/playwright"
)

func main() {
	repoDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("locate repo directory: %v", err)
	}

	if _, err := exec.LookPath("bun"); err != nil {
		log.Fatalf("bun runtime not found: %v", err)
	}

	cfg := pw.DefaultConfig()

	tailwindInput := flag.String("tailwind-input", cfg.TailwindInput, "path to Tailwind input file")
	tailwindOutput := flag.String("tailwind-output", cfg.TailwindOutput, "path to Tailwind output file")
	tailwindContent := flag.String("tailwind-content", strings.Join(cfg.TailwindContent, ","), "comma-separated Tailwind content globs")
	binary := flag.String("binary", cfg.Binary, "compiled server binary name (relative to repo root)")
	serverCmd := flag.String("server-command", "", "custom server command (quoted string, e.g. './bin/app --flag')")
	baseURL := flag.String("base-url", cfg.BaseURL, "base URL used by Playwright and readiness checks")
	timeout := flag.Duration("timeout", 2*time.Minute, "overall timeout for code generation steps")
	flag.Parse()

	cfg.TailwindInput = *tailwindInput
	cfg.TailwindOutput = *tailwindOutput
	cfg.Binary = *binary
	cfg.BaseURL = *baseURL
	if strings.TrimSpace(*tailwindContent) != "" {
		cfg.TailwindContent = splitCSV(*tailwindContent)
	}
	if strings.TrimSpace(*serverCmd) != "" {
		cfg.ServerCommand = strings.Fields(*serverCmd)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err := pw.Prepare(ctx, repoDir, cfg); err != nil {
		log.Fatalf("code generation failed: %v", err)
	}

	log.Println("Code generation completed successfully")
}

func splitCSV(input string) []string {
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
