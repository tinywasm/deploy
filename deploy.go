// Package deploy implements a lightweight continuous deployment agent.
// It supports two modes:
//   - webhook: an HTTP daemon on the server that GitHub Actions triggers via POST.
//   - ssh: generates a shell script that GitHub Actions runs via SSH on the server.
//
// Usage:
//
//	d := deploy.New(keys, mgr, downloader, checker, files)
//	d.Run("deploy.yaml")
package deploy

import (
	"fmt"
	"log"
	"net/http"
)

// KeyManager abstracts secret retrieval (injected from tinywasm/keyring or mocks).
type KeyManager interface {
	GetHMACSecret() (string, error)
	GetGitHubPAT() (string, error)
	IsConfigured() bool
}

// Deploy is the entry point for the deployment agent.
// All dependencies are injected for testability.
type Deploy struct {
	Keys    KeyManager
	Mgr     ProcessManager
	DL      Downloader
	Checker HealthChecker
	Files   FileOps
}

// New creates a Deploy with all required dependencies.
func New(keys KeyManager, mgr ProcessManager, dl Downloader, checker HealthChecker, files FileOps) *Deploy {
	return &Deploy{
		Keys:    keys,
		Mgr:     mgr,
		DL:      dl,
		Checker: checker,
		Files:   files,
	}
}

// Run loads config from configPath and starts the appropriate deployment mode.
// In webhook mode it blocks serving HTTP.
// In ssh mode it prints the generated script to stdout and exits.
func (d *Deploy) Run(configPath string) error {
	cfg, err := Load(configPath)
	if err != nil {
		return fmt.Errorf("deploy: load config: %w", err)
	}

	if !d.Keys.IsConfigured() {
		return fmt.Errorf("deploy: secrets not configured — run setup first")
	}

	switch cfg.Mode {
	case "ssh":
		return d.runSSHMode(cfg)
	default:
		return d.runWebhookMode(cfg)
	}
}

func (d *Deploy) runWebhookMode(cfg *Config) error {
	secret, err := d.Keys.GetHMACSecret()
	if err != nil {
		return fmt.Errorf("deploy: get HMAC secret: %w", err)
	}

	validator := NewHMACValidator(secret)
	wh := NewWebhookHandler(cfg, validator, d.Keys, d.DL, d.Checker, d.Mgr, d.Files)

	mux := http.NewServeMux()
	mux.Handle("/update", wh)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("deploy: webhook listening on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (d *Deploy) runSSHMode(cfg *Config) error {
	pat, err := d.Keys.GetGitHubPAT()
	if err != nil {
		return fmt.Errorf("deploy: get PAT: %w", err)
	}

	for _, app := range cfg.Apps {
		// In SSH mode, download URL must be set externally (e.g. via env or config extension)
		// Print the script for each app — the caller/CI uses it
		script := SSHScript(app, "", pat)
		log.Printf("# SSH script for %s:\n%s\n", app.Name, script)
	}
	return nil
}
