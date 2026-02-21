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
)

func TestHandleUpdate_Success(t *testing.T) {
	// Setup dependencies
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
				BusyTimeout:       10 * time.Millisecond,
				BusyRetryInterval: 1 * time.Millisecond,
				StartupDelay:      0,
			},
		},
	}

	secret := "secret"
	validator := deploy.NewHMACValidator(secret)
	downloader := NewMockDownloader()
	procManager := NewMockProcessManager()
	checker := NewMockHealthChecker()
	keys := NewMockStore()
	keys.Set("DEPLOY_GITHUB_PAT", "token")

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

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d. Body: %s", resp.StatusCode, w.Body.String())
	}

	// Verify Stop called
	if len(procManager.Stopped) == 0 {
		t.Errorf("expected Stop('app.exe'), got nothing")
	} else if procManager.Stopped[0] != "app.exe" {
		t.Errorf("expected Stop('app.exe'), got %s", procManager.Stopped[0])
	}

	// Verify Start called
	if len(procManager.Started) == 0 {
		t.Errorf("expected Start called")
	} else if procManager.Started[0] != exePath {
		t.Errorf("expected Start('%s'), got %s", exePath, procManager.Started[0])
	}

	// Verify file content is updated
	content, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("failed to read app file: %v", err)
	}
	if string(content) != "mock downloaded content" {
		t.Errorf("expected content 'mock downloaded content', got '%s'", string(content))
	}
}

func TestHandleUpdate_Busy(t *testing.T) {
	tmpDir := t.TempDir()
	config := &deploy.Config{
		Updater: deploy.ConfigUpdater{TempDir: tmpDir},
		Apps: []deploy.AppConfig{
			{
				Name:              "app",
				Executable:        "app.exe",
				Path:              tmpDir, // Just use tmpDir as app path
				HealthEndpoint:    "http://localhost/health",
				BusyTimeout:       10 * time.Millisecond,
				BusyRetryInterval: 1 * time.Millisecond,
			},
		},
	}

	secret := "secret"
	validator := deploy.NewHMACValidator(secret)
	downloader := NewMockDownloader()
	procManager := NewMockProcessManager()
	checker := NewMockHealthChecker()
	checker.Responses = map[string]*deploy.HealthStatus{
		"http://localhost/health": {Status: "busy", CanRestart: false},
	}
	keys := NewMockStore()
	keys.Set("DEPLOY_GITHUB_PAT", "token")

	handler := &deploy.Handler{
		Config:     config,
		Validator:  validator,
		Downloader: downloader,
		Process:    procManager,
		Checker:    checker,
		Keys:       keys,
	}

	payload := []byte(`{"executable":"app.exe"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/update", bytes.NewReader(payload))
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()

	handler.HandleUpdate(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d", resp.StatusCode)
	}
}

func TestHandleUpdate_InvalidSignature(t *testing.T) {
	handler := &deploy.Handler{
		Validator: deploy.NewHMACValidator("secret"),
	}
	req := httptest.NewRequest("POST", "/update", bytes.NewReader([]byte("{}")))
	req.Header.Set("X-Signature", "sha256=invalid")
	w := httptest.NewRecorder()

	handler.HandleUpdate(w, req)
	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Result().StatusCode)
	}
}
