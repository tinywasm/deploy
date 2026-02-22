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
	"os"

	"github.com/tinywasm/goflare"
)

// Puller is the main orchestrator for all deployment modes.
// Store must be injected — kvdb.KVStore satisfies the Store interface directly.
type Puller struct {
	Store      Store
	Process    ProcessManager
	Downloader Downloader
	Checker    HealthChecker
	ConfigPath string
	Goflare    *goflare.Goflare // Injected for edgeWorker strategy
	log        func(...any)
}

// SetLog injects a logger (called by tinywasm/app after registration with TUI).
func (p *Puller) SetLog(f func(...any)) { p.log = f }

func (p *Puller) logger(msgs ...any) {
	if p.log != nil {
		p.log(msgs...)
	}
}

// Name returns the TUI tab label for the orchestrator.
func (p *Puller) Name() string { return "DEPLOY/DAEMON" }

// IsConfigured returns true if a deploy method has been stored.
func (p *Puller) IsConfigured() bool {
	method, err := p.Store.Get("DEPLOY_METHOD")
	return err == nil && method != ""
}

// Run executes the deployment based on the stored DEPLOY_METHOD.
// Called from cmd/deploy/main.go for standalone daemon mode.
func (p *Puller) Run() error {
	method, err := p.Store.Get("DEPLOY_METHOD")
	if err != nil || method == "" {
		return fmt.Errorf("deploy: not configured — run wizard first (DEPLOY_METHOD not set)")
	}

	// DEPRECATED: Internal naming consistency. Strategy name is now "cloudflarePages" or "cloudflareWorker".
	// The store might still hold "cloudflare" or "edgeworker" from older app configs.
	// Kept for seamless backward compatibility; do not remove unless breaking changes are allowed.
	if method == "cloudflare" || method == "edgeworker" {
		method = "cloudflarePages"
	}

	strat, err := GetPusher(method)
	if err != nil {
		return err
	}

	cfg, err := Load(p.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deploy: load config: %w", err)
	}
	if cfg == nil {
		if err := CreateDefaultConfig(p.ConfigPath); err != nil {
			return fmt.Errorf("deploy: create default config: %w", err)
		}
		cfg, err = Load(p.ConfigPath)
		if err != nil {
			return fmt.Errorf("deploy: reload config: %w", err)
		}
	}

	// Inject Goflare if the strategy needs it.
	switch s := strat.(type) {
	case *CloudflarePagesPusher:
		s.Goflare = p.Goflare
	case *CloudflareWorkerPusher:
		s.Goflare = p.Goflare
	}

	return strat.Run(cfg, p)
}
