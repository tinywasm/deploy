package deploy_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tinywasm/deploy"
	"gopkg.in/yaml.v3"
)

func TestHandleUpdate_Rollback_CreatesFailedArtifact(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "app")
	os.MkdirAll(appDir, 0755)

	exePath := filepath.Join(appDir, "app.exe")
	os.WriteFile(exePath, []byte("old binary"), 0755)

	config := &deploy.Config{
		Updater: deploy.ConfigUpdater{
			TempDir: tmpDir,
		},
		Apps: []deploy.AppConfig{
			{
				Name:              "app",
				Executable:        "app.exe",
				Path:              appDir,
				HealthEndpoint:    "http://localhost/health",
				StartupDelay:      0,
				BusyRetryInterval: 10 * time.Millisecond,
				BusyTimeout:       100 * time.Millisecond,
			},
		},
	}

	secret := "secret"
	validator := deploy.NewHMACValidator(secret)
	downloader := NewMockDownloader() // Returns "mock downloaded content"
	procManager := NewMockProcessManager()
	checker := NewMockHealthChecker()
	// Simulate failure on second check (post-deploy)
	checker.QueueResponses = map[string][]*deploy.HealthStatus{
		"http://localhost/health": {
			{Status: "ok", CanRestart: true}, // First check: OK to update
			nil,                              // Second check: Simulate failure (nil triggers error)
		},
	}

	keys := NewMockKeyManager()
	keys.Set("github", "pat", "token")

	handler := &deploy.Handler{
		Config:     config,
		Validator:  validator,
		Downloader: downloader,
		Process:    procManager,
		Checker:    checker,
		Keys:       keys,
	}

	payload := []byte(`{"executable":"app.exe","download_url":"http://github.com/release"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/update", bytes.NewReader(payload))
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()

	handler.HandleUpdate(w, req)

	// Expect Rollback (500)
	if w.Result().StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error, got %d", w.Result().StatusCode)
	}

	// Verify app-failed.exe exists
	failedArtifactPath := filepath.Join(appDir, "app-failed.exe")
	if _, err := os.Stat(failedArtifactPath); os.IsNotExist(err) {
		t.Errorf("expected app-failed.exe to be created")
	} else {
		content, _ := os.ReadFile(failedArtifactPath)
		if string(content) != "mock downloaded content" {
			t.Errorf("expected app-failed.exe content 'mock downloaded content', got '%s'", string(content))
		}
	}

	// Verify app.exe is restored
	content, _ := os.ReadFile(exePath)
	if string(content) != "old binary" {
		t.Errorf("expected app.exe restored to 'old binary', got '%s'", string(content))
	}
}

func TestHandleUpdate_Success_UpdatesConfig(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "app")
	os.MkdirAll(appDir, 0755)

	configPath := filepath.Join(tmpDir, "config.yaml")
	initialConfig := `
updater:
  port: 8080
apps:
  - name: app
    version: "1.0.0"
    executable: app.exe
    path: ` + appDir + `
    health_endpoint: http://localhost/health
`
	os.WriteFile(configPath, []byte(initialConfig), 0644)

	// Load config properly to have pointers set up?
	// Handler uses *Config. If it modifies it in memory, we need to check if it persists to disk.
	// The implementation should write to disk.

	config, err := deploy.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	exePath := filepath.Join(appDir, "app.exe")
	os.WriteFile(exePath, []byte("old binary"), 0755)

	secret := "secret"
	validator := deploy.NewHMACValidator(secret)
	downloader := NewMockDownloader()
	procManager := NewMockProcessManager()
	checker := NewMockHealthChecker() // Default is healthy
	keys := NewMockKeyManager()
	keys.Set("github", "pat", "token")

	handler := &deploy.Handler{
		Config:     config,
		ConfigPath: configPath, // We need to inject ConfigPath to Handler to save it
		Validator:  validator,
		Downloader: downloader,
		Process:    procManager,
		Checker:    checker,
		Keys:       keys,
	}

	payload := []byte(`{"executable":"app.exe","tag":"1.1.0","download_url":"http://github.com/release"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/update", bytes.NewReader(payload))
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()

	handler.HandleUpdate(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d. Body: %s", w.Result().StatusCode, w.Body.String())
	}

	// Verify config.yaml updated
	updatedConfigBytes, _ := os.ReadFile(configPath)
	var updatedConfig deploy.Config
	if err := yaml.Unmarshal(updatedConfigBytes, &updatedConfig); err != nil {
		t.Fatalf("failed to parse updated config: %v", err)
	}

	if updatedConfig.Apps[0].Version != "1.1.0" {
		t.Errorf("expected version 1.1.0, got %s", updatedConfig.Apps[0].Version)
	}
}
