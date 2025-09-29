package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
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

	timeout := flag.Duration("timeout", 2*time.Minute, "overall timeout for code generation steps")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err := pw.Prepare(ctx, repoDir); err != nil {
		log.Fatalf("code generation failed: %v", err)
	}

	log.Println("Code generation completed successfully")
}
