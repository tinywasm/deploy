package deploy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type UpdateRequest struct {
	Repo        string `json:"repo"`
	Tag         string `json:"tag"`
	Executable  string `json:"executable"`
	DownloadURL string `json:"download_url"`
}

type Handler struct {
	Config     *Config
	ConfigPath string
	Validator  *HMACValidator
	Downloader Downloader
	Process    ProcessManager
	Checker    HealthChecker // Use interface
	Keys       KeyManager
}

func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Validate Signature
	signature := r.Header.Get("X-Signature")
	if signature == "" {
		http.Error(w, "Missing signature", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if err := h.Validator.ValidateRequest(body, signature); err != nil {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// 2. Parse Payload
	var req UpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// 3. Find App Config
	var app *AppConfig
	for i := range h.Config.Apps {
		if h.Config.Apps[i].Executable == req.Executable {
			app = &h.Config.Apps[i]
			break
		}
	}
	if app == nil {
		http.Error(w, "App not configured", http.StatusNotFound)
		return
	}

	// 4. Check Health (Busy Loop)
	timeout := time.After(app.BusyTimeout)
	ticker := time.NewTicker(app.BusyRetryInterval)
	defer ticker.Stop()

WaitLoop:
	for {
		status, err := h.Checker.Check(app.HealthEndpoint)
		if err != nil {
			// If check fails (e.g. network error), assume not busy?
			break WaitLoop
		}
		if status.CanRestart {
			break WaitLoop
		}

		select {
		case <-timeout:
			http.Error(w, "Service busy", http.StatusServiceUnavailable)
			return
		case <-ticker.C:
			continue
		}
	}

	// 5. Download New Version
	token, err := h.Keys.Get("github", "pat")
	if err != nil {
		http.Error(w, "Missing GitHub token", http.StatusInternalServerError)
		return
	}

	tempFile := filepath.Join(h.Config.Updater.TempDir, req.Executable+".new")
	if err := h.Downloader.Download(req.DownloadURL, tempFile, token); err != nil {
		http.Error(w, fmt.Sprintf("Download failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 6. Stop Existing Process
	_ = h.Process.Stop(app.Executable)

	// 7. Backup Existing Binary
	appPath := filepath.Join(app.Path, app.Executable)
	backupPath := filepath.Join(app.Path, app.Executable+".old")

	if _, err := os.Stat(appPath); err == nil {
		if err := os.Rename(appPath, backupPath); err != nil {
			http.Error(w, fmt.Sprintf("Failed to backup: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// 8. Move New Binary
	if err := os.Rename(tempFile, appPath); err != nil {
		// Try to restore backup
		_ = os.Rename(backupPath, appPath)
		// Restart old process if move failed
		_ = h.Process.Start(appPath)
		http.Error(w, fmt.Sprintf("Failed to install: %v", err), http.StatusInternalServerError)
		return
	}

	// 9. Start New Process
	if err := h.Process.Start(appPath); err != nil {
		// Rollback
		// Rename failed binary to app-failed.exe
		failedPath := filepath.Join(app.Path, "app-failed.exe")
		_ = os.Remove(failedPath) // Ensure target doesn't exist (Windows)
		_ = os.Rename(appPath, failedPath)

		_ = os.Rename(backupPath, appPath)
		_ = h.Process.Start(appPath) // Try to restart old version
		http.Error(w, fmt.Sprintf("Failed to start: %v", err), http.StatusInternalServerError)
		return
	}

	// 10. Health Check New Process
	if app.StartupDelay > 0 {
		time.Sleep(app.StartupDelay)
	}

	newStatus, err := h.Checker.Check(app.HealthEndpoint)
	if err != nil || newStatus.Status != "ok" { // Assuming "ok" is success criteria
		// Rollback
		_ = h.Process.Stop(app.Executable)

		// Rename failed binary to app-failed.exe
		failedPath := filepath.Join(app.Path, "app-failed.exe")
		_ = os.Remove(failedPath) // Ensure target doesn't exist
		_ = os.Rename(appPath, failedPath)

		_ = os.Rename(backupPath, appPath)
		_ = h.Process.Start(appPath)
		http.Error(w, "New version failed health check", http.StatusInternalServerError)
		return
	}

	// 11. Update Config (Version)
	if req.Tag != "" {
		app.Version = req.Tag
		if h.ConfigPath != "" {
			if data, err := yaml.Marshal(h.Config); err == nil {
				_ = os.WriteFile(h.ConfigPath, data, 0644)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Update successful"))
}
