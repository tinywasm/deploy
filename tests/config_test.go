package deploy_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tinywasm/deploy"
)

func TestLoad_Success(t *testing.T) {
	configContent := `
updater:
  port: 9090
  temp_dir: /tmp/custom
apps:
  - name: app1
    busy_retry_interval: 5s
    busy_timeout: 1m
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	cfg, err := deploy.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Updater.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Updater.Port)
	}
	if cfg.Updater.TempDir != "/tmp/custom" {
		t.Errorf("expected temp_dir /tmp/custom, got %s", cfg.Updater.TempDir)
	}
	if len(cfg.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(cfg.Apps))
	}
	if cfg.Apps[0].BusyRetryInterval != 5*time.Second {
		t.Errorf("expected busy_retry_interval 5s, got %v", cfg.Apps[0].BusyRetryInterval)
	}
	if cfg.Apps[0].BusyTimeout != time.Minute {
		t.Errorf("expected busy_timeout 1m, got %v", cfg.Apps[0].BusyTimeout)
	}
}

func TestLoad_Defaults(t *testing.T) {
	configContent := `
updater:
  log_level: debug
apps:
  - name: app1
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	cfg, err := deploy.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Updater.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Updater.Port)
	}
	if cfg.Updater.TempDir == "" {
		t.Error("expected default temp_dir, got empty string")
	}
	if len(cfg.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(cfg.Apps))
	}
	if cfg.Apps[0].BusyRetryInterval != 10*time.Second {
		t.Errorf("expected default busy_retry_interval 10s, got %v", cfg.Apps[0].BusyRetryInterval)
	}
	if cfg.Apps[0].BusyTimeout != 5*time.Minute {
		t.Errorf("expected default busy_timeout 5m, got %v", cfg.Apps[0].BusyTimeout)
	}
}

func TestLoad_Fail(t *testing.T) {
	configContent := `invalid: yaml: content`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	_, err := deploy.Load(configPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoad_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yaml")

	_, err := deploy.Load(configPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
