package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tinywasm/deploy"
)

// envStore reads deploy config from environment variables.
// Used by the standalone daemon where kvdb is not available.
type envStore struct{}

func (e *envStore) Get(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("%s not set", key)
	}
	return val, nil
}

func (e *envStore) Set(key, value string) error {
	return os.Setenv(key, value)
}

func main() {
	process := deploy.NewProcessManager()
	downloader := deploy.NewDownloader()
	checker := deploy.NewChecker()

	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to determine executable path: %v", err)
	}
	configPath := filepath.Join(filepath.Dir(exePath), "config.yaml")

	d := &deploy.Deploy{
		Store:      &envStore{},
		Process:    process,
		Downloader: downloader,
		Checker:    checker,
		ConfigPath: configPath,
	}

	if err := d.Run(); err != nil {
		log.Fatalf("deploy agent failed: %v", err)
	}
}
