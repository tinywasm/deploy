package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/tinywasm/deploy"
)

func main() {
	admin := flag.Bool("admin", false, "Run admin menu")
	flag.Parse()

	// Initialize dependencies
	keys := deploy.NewSystemKeyManager()

	if *admin {
		wizard := deploy.NewWizard(keys)
		if err := wizard.RunAdmin(); err != nil {
			log.Fatalf("admin menu failed: %v", err)
		}
		return
	}

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
