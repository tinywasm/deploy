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
	"net/http"
	"os"
)

// KeyManager defines the interface for managing secrets.
type KeyManager interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
}

type Deploy struct {
	Keys       KeyManager
	Process    ProcessManager
	Downloader Downloader
	Checker    HealthChecker
	ConfigPath string
}

// Run executes the main deployment loop.
func (d *Deploy) Run() error {
	// 1. Attempt to load config
	cfg, err := Load(d.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		// Log warning but proceed if config file is corrupted? No, fail.
		// If file doesn't exist, we might create it later.
		// But if err is other than NotExist, return error.
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 2. Check if configured (Keys exist)
	// We check for GitHub PAT and HMAC Secret.
	_, patErr := d.Keys.Get("github", "pat")
	_, hmacErr := d.Keys.Get("deploy", "hmac_secret")

	if patErr != nil || hmacErr != nil {
		// Run Wizard
		wizard := NewWizard(d.Keys)
		if err := wizard.Run(); err != nil {
			return fmt.Errorf("wizard failed: %w", err)
		}

		// If config was missing, ensure default is created now
		if cfg == nil {
			if err := CreateDefaultConfig(d.ConfigPath); err != nil {
				return fmt.Errorf("failed to create default config: %w", err)
			}
			// Load again
			cfg, err = Load(d.ConfigPath)
			if err != nil {
				return fmt.Errorf("failed to reload config: %w", err)
			}
		}
	} else if cfg == nil {
		// Keys exist but config missing? Create default.
		if err := CreateDefaultConfig(d.ConfigPath); err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
		cfg, err = Load(d.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	// 3. Start Server
	hmacSecret, err := d.Keys.Get("deploy", "hmac_secret")
	if err != nil {
		return fmt.Errorf("failed to retrieve HMAC secret: %w", err)
	}

	handler := &Handler{
		Config:     cfg,
		ConfigPath: d.ConfigPath,
		Validator:  NewHMACValidator(hmacSecret),
		Downloader: d.Downloader,
		Process:    d.Process,
		Checker:    d.Checker,
		Keys:       d.Keys,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/update", handler.HandleUpdate)
	// Health check for deploy agent itself?
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", cfg.Updater.Port)
	fmt.Printf("Starting deploy agent on %s\n", addr)
	return http.ListenAndServe(addr, mux)
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
