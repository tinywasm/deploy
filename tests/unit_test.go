package tests

import (
	"os"
	"testing"
	"time"

	"github.com/tinywasm/deploy"
)

// ─── HMACValidator ────────────────────────────────────────────────────────────

func TestHMAC_ValidSignature(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"app":"myapp","version":"v1.2.3"}`)
	v := deploy.NewHMACValidator(secret)

	sig := deploy.SignPayload(secret, payload)
	if err := v.Validate(payload, sig); err != nil {
		t.Fatalf("expected valid signature, got: %v", err)
	}
}

func TestHMAC_InvalidSignature(t *testing.T) {
	v := deploy.NewHMACValidator("secret")
	err := v.Validate([]byte("payload"), "sha256=deadbeef00")
	if err == nil {
		t.Fatal("expected error for wrong signature")
	}
}

func TestHMAC_MalformedSignature(t *testing.T) {
	v := deploy.NewHMACValidator("secret")
	err := v.Validate([]byte("payload"), "nosuffix")
	if err == nil || err.Error() != "invalid signature format" {
		t.Fatalf("expected 'invalid signature format', got: %v", err)
	}
}

// ─── Config ───────────────────────────────────────────────────────────────────

func TestConfig_Load_Success(t *testing.T) {
	yaml := `
mode: webhook
port: 9001
apps:
  - name: myapp
    service: myapp.service
    executable: myapp
    path: /srv/myapp
    health_url: http://localhost:8080/health
`
	f, _ := os.CreateTemp("", "deploy-*.yaml")
	f.WriteString(yaml)
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := deploy.Load(f.Name())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9001 {
		t.Errorf("expected port 9001, got %d", cfg.Port)
	}
	if len(cfg.Apps) != 1 || cfg.Apps[0].Name != "myapp" {
		t.Errorf("unexpected apps: %+v", cfg.Apps)
	}
}

func TestConfig_Load_Defaults(t *testing.T) {
	f, _ := os.CreateTemp("", "deploy-*.yaml")
	f.WriteString("apps: []\n")
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := deploy.Load(f.Name())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9000 {
		t.Errorf("expected default port 9000, got %d", cfg.Port)
	}
	if cfg.Mode != "webhook" {
		t.Errorf("expected default mode webhook, got %s", cfg.Mode)
	}
}

// ─── HealthChecker ────────────────────────────────────────────────────────────

func TestMockChecker_Healthy(t *testing.T) {
	ch := &mockChecker{responses: []error{nil}}
	if err := ch.Check("http://x/health", 3, time.Millisecond); err != nil {
		t.Fatalf("expected healthy, got: %v", err)
	}
}

func TestMockChecker_Busy(t *testing.T) {
	busyErr := errBusy("app busy: can_restart=false")
	ch := &mockChecker{responses: []error{busyErr, busyErr, busyErr}}
	err := ch.Check("http://x/health", 3, time.Millisecond)
	if err == nil {
		t.Fatal("expected busy error")
	}
}

// ─── ProcessManager (mock) ────────────────────────────────────────────────────

func TestMockProcessManager_StartStop(t *testing.T) {
	mgr := &mockProcessManager{}
	if err := mgr.Stop("myapp.service"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := mgr.Start("/srv/myapp/myapp"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(mgr.stopped) != 1 || mgr.stopped[0] != "myapp.service" {
		t.Errorf("unexpected stopped: %v", mgr.stopped)
	}
	if len(mgr.started) != 1 || mgr.started[0] != "/srv/myapp/myapp" {
		t.Errorf("unexpected started: %v", mgr.started)
	}
}

// ─── Downloader (mock) ────────────────────────────────────────────────────────

func TestMockDownloader_Success(t *testing.T) {
	dl := &mockDownloader{}
	if err := dl.Download("http://example.com/bin", "/tmp/bin", "tok"); err != nil {
		t.Fatalf("Download: %v", err)
	}
}

func TestMockDownloader_Error(t *testing.T) {
	dl := &mockDownloader{err: errBusy("network error")}
	if err := dl.Download("http://example.com/bin", "/tmp/bin", "tok"); err == nil {
		t.Fatal("expected download error")
	}
}

// ─── HotSwap & Rollback ───────────────────────────────────────────────────────

func TestHotSwap_Success(t *testing.T) {
	files := &mockFileOps{}
	backup, err := deploy.HotSwap(files, "/srv/app/myapp", "/tmp/myapp-new")
	if err != nil {
		t.Fatalf("HotSwap: %v", err)
	}
	if backup != "/srv/app/myapp-older" {
		t.Errorf("unexpected backup path: %s", backup)
	}
	if len(files.renames) != 2 {
		t.Errorf("expected 2 renames, got %d", len(files.renames))
	}
}

func TestHotSwap_BackupFails(t *testing.T) {
	files := &mockFileOps{renameErr: errBusy("access denied")}
	_, err := deploy.HotSwap(files, "/srv/app/myapp", "/tmp/myapp-new")
	if err == nil {
		t.Fatal("expected error when backup rename fails")
	}
}

func TestRollback_Success(t *testing.T) {
	files := &mockFileOps{}
	mgr := &mockProcessManager{}
	err := deploy.Rollback(files, "/srv/app/myapp", "/srv/app/myapp-older", mgr, "myapp.service")
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if len(mgr.stopped) == 0 {
		t.Error("expected Stop to be called")
	}
	if len(mgr.started) == 0 {
		t.Error("expected Start to be called")
	}
}

// ─── SSH Script ───────────────────────────────────────────────────────────────

func TestSSHScript_ContainsKeyCommands(t *testing.T) {
	app := deploy.AppConfig{
		Name:       "myapp",
		Service:    "myapp.service",
		Executable: "myapp",
		Path:       "/srv/myapp",
		HealthURL:  "http://localhost:8080/health",
		Rollback:   true,
		HealthRetry: 3,
	}
	script := deploy.SSHScript(app, "http://example.com/myapp", "mytoken")

	checks := []string{"curl", "systemctl stop", "systemctl start", "health"}
	for _, want := range checks {
		if !contains(script, want) {
			t.Errorf("SSH script missing %q", want)
		}
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

type busyError string

func (b busyError) Error() string { return string(b) }

func errBusy(msg string) error { return busyError(msg) }

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
