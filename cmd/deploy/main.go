package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/tinywasm/deploy"
)

func main() {
	// Initialize dependencies
	keys := deploy.NewSystemKeyManager()
	process := deploy.NewProcessManager()
	downloader := deploy.NewDownloader()
	checker := deploy.NewChecker()

	// Determine config path
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to determine executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	configPath := filepath.Join(exeDir, "config.yaml")

	d := &deploy.Deploy{
		Keys:       keys,
		Process:    process,
		Downloader: downloader,
		Checker:    checker,
		ConfigPath: configPath,
	}

	if err := d.Run(); err != nil {
		log.Fatalf("deploy agent failed: %v", err)
	}
}
