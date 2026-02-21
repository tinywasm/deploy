package deploy

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

// updatePayload is the JSON body sent by the GitHub Action webhook.
type updatePayload struct {
	App        string `json:"app"`         // matches AppConfig.Name
	Version    string `json:"version"`     // new version tag
	DownloadURL string `json:"download_url"` // direct asset download URL
}

// WebhookHandler is the HTTP handler for POST /update.
type WebhookHandler struct {
	cfg       *Config
	hmac      *HMACValidator
	keys      KeyManager
	dl        Downloader
	checker   HealthChecker
	mgr       ProcessManager
	files     FileOps
}

// NewWebhookHandler creates a handler with all dependencies injected.
func NewWebhookHandler(
	cfg *Config,
	hmac *HMACValidator,
	keys KeyManager,
	dl Downloader,
	checker HealthChecker,
	mgr ProcessManager,
	files FileOps,
) *WebhookHandler {
	return &WebhookHandler{
		cfg:     cfg,
		hmac:    hmac,
		keys:    keys,
		dl:      dl,
		checker: checker,
		mgr:     mgr,
		files:   files,
	}
}

// ServeHTTP handles POST /update.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 1. Validate HMAC signature
	sig := r.Header.Get("X-Signature")
	if err := h.hmac.Validate(body, sig); err != nil {
		log.Println("deploy: HMAC validation failed:", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Parse payload
	var payload updatePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// 3. Find app config
	app := h.findApp(payload.App)
	if app == nil {
		http.Error(w, "app not registered", http.StatusNotFound)
		return
	}

	// 4. Skip if already at this version
	if app.Version == payload.Version {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("already up to date"))
		return
	}

	// 5. Get GitHub PAT
	pat, err := h.keys.GetGitHubPAT()
	if err != nil {
		log.Println("deploy: keyring error:", err)
		http.Error(w, "keyring error", http.StatusInternalServerError)
		return
	}

	// 6. Download new binary
	newBin := filepath.Join(h.cfg.TempDir, payload.App+"-new")
	if err := h.dl.Download(payload.DownloadURL, newBin, pat); err != nil {
		log.Println("deploy: download failed:", err)
		http.Error(w, "download failed", http.StatusInternalServerError)
		return
	}

	// 7. Pre-flight health check (warn-only if app is down)
	healthURL := app.HealthURL
	if healthURL != "" {
		interval := 2 * time.Second
		if err := h.checker.Check(healthURL, app.HealthRetry, interval); err != nil {
			log.Println("deploy: pre-flight health:", err)
			// If app is BUSY (can_restart=false), reject
			if err.Error() == "app busy: can_restart=false" {
				http.Error(w, "service unavailable: app busy", http.StatusServiceUnavailable)
				return
			}
			// App is DOWN — log and continue
			log.Println("deploy: app was down before deploy, continuing")
		}
	}

	// 8. Hot-swap
	currentBin := filepath.Join(app.Path, app.Executable)
	backupBin, err := HotSwap(h.files, currentBin, newBin)
	if err != nil {
		log.Println("deploy: hot-swap failed:", err)
		http.Error(w, "hot-swap failed", http.StatusInternalServerError)
		return
	}

	// 9. Stop old + start new
	_ = h.mgr.Stop(app.Service)
	time.Sleep(300 * time.Millisecond)
	if err := h.mgr.Start(currentBin); err != nil {
		log.Println("deploy: start failed:", err)
		http.Error(w, "start failed", http.StatusInternalServerError)
		return
	}

	// 10. Post-deploy health check with rollback on failure
	time.Sleep(app.StartupWait)
	if healthURL != "" {
		if err := h.checker.Check(healthURL, app.HealthRetry, 2*time.Second); err != nil {
			log.Println("deploy: post-deploy health check failed, rolling back:", err)
			if rbErr := Rollback(h.files, currentBin, backupBin, h.mgr, app.Service); rbErr != nil {
				log.Println("deploy: rollback also failed:", rbErr)
			}
			http.Error(w, "deploy failed: rollback executed", http.StatusInternalServerError)
			return
		}
	}

	// 11. Update version in config
	app.Version = payload.Version
	log.Printf("deploy: %s updated to %s\n", payload.App, payload.Version)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("deployed " + payload.Version))
}

func (h *WebhookHandler) findApp(name string) *AppConfig {
	for i := range h.cfg.Apps {
		if h.cfg.Apps[i].Name == name {
			return &h.cfg.Apps[i]
		}
	}
	return nil
}
