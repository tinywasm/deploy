// Package deploy implements a lightweight continuous deployment agent.
// It supports three modes:
//   - cloudflare: uploads build artifacts to Cloudflare Pages via API.
//   - webhook: an HTTP daemon on the server that GitHub Actions triggers via POST.
//   - ssh: generates a shell script that GitHub Actions runs via SSH on the server.
//
// Usage:
//
//	d := &deploy.Deploy{Store: db, Process: mgr, Downloader: dl, Checker: checker}
//	d.Run()
package deploy

import (
	"fmt"
	"net/http"
	"os"
)

// Deploy is the main orchestrator for all deployment modes.
// Store must be injected — kvdb.KVStore satisfies the Store interface directly.
type Deploy struct {
	Store      Store
	Process    ProcessManager
	Downloader Downloader
	Checker    HealthChecker
	ConfigPath string
	log        func(...any)
}

// SetLog injects a logger (called by tinywasm/app after registration with TUI).
func (d *Deploy) SetLog(f func(...any)) { d.log = f }

func (d *Deploy) logger(msgs ...any) {
	if d.log != nil {
		d.log(msgs...)
	}
}

// IsConfigured returns true if a deploy method has been stored.
func (d *Deploy) IsConfigured() bool {
	method, err := d.Store.Get("DEPLOY_METHOD")
	return err == nil && method != ""
}

// Run executes the deployment based on the stored DEPLOY_METHOD.
// Called from cmd/deploy/main.go for standalone daemon mode.
func (d *Deploy) Run() error {
	method, err := d.Store.Get("DEPLOY_METHOD")
	if err != nil || method == "" {
		return fmt.Errorf("deploy: not configured — run wizard first (DEPLOY_METHOD not set)")
	}

	cfg, err := Load(d.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deploy: load config: %w", err)
	}
	if cfg == nil {
		if err := CreateDefaultConfig(d.ConfigPath); err != nil {
			return fmt.Errorf("deploy: create default config: %w", err)
		}
		cfg, err = Load(d.ConfigPath)
		if err != nil {
			return fmt.Errorf("deploy: reload config: %w", err)
		}
	}

	switch method {
	case "cloudflare":
		return d.runCloudflare()
	case "webhook":
		return d.runWebhook(cfg)
	case "ssh":
		return d.runSSH(cfg)
	default:
		return fmt.Errorf("deploy: unknown method %q", method)
	}
}

func (d *Deploy) runCloudflare() error {
	cf := NewCFClient(d.Store)
	cf.SetLog(d.logger)
	// Output dir from store or default
	return cf.Deploy("deploy/cloudflare", "_worker.js", "worker.wasm")
}

func (d *Deploy) runWebhook(cfg *Config) error {
	hmacSecret, err := d.Store.Get("DEPLOY_HMAC_SECRET")
	if err != nil || hmacSecret == "" {
		return fmt.Errorf("deploy: HMAC secret not configured")
	}

	handler := &Handler{
		Config:     cfg,
		ConfigPath: d.ConfigPath,
		Validator:  NewHMACValidator(hmacSecret),
		Downloader: d.Downloader,
		Process:    d.Process,
		Checker:    d.Checker,
		Keys:       d.Store,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/update", handler.HandleUpdate)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", cfg.Updater.Port)
	d.logger("Starting deploy agent on", addr)
	return http.ListenAndServe(addr, mux)
}

func (d *Deploy) runSSH(cfg *Config) error {
	pat, err := d.Store.Get("DEPLOY_GITHUB_PAT")
	if err != nil || pat == "" {
		return fmt.Errorf("deploy: GitHub PAT not configured")
	}
	for _, app := range cfg.Apps {
		script := SSHScript(app, "", pat)
		d.logger("# SSH script for", app.Name+":\n"+script)
	}
	return nil
}
