package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
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

	timeout := flag.Duration("timeout", 5*time.Minute, "overall timeout for playwright run")
	flag.Parse()

	baseCtx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	go func() {
		select {
		case <-sigs:
			cancel()
		case <-baseCtx.Done():
		}
	}()

	if err := pw.Run(baseCtx, repoDir); err != nil {
		log.Fatalf("playwright run failed: %v", err)
	}

	log.Println("Playwright suite completed successfully")
}
