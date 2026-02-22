package deploy_test

import (
	"os"
	"path/filepath"
	"testing"

	twctx "github.com/tinywasm/context"
	"github.com/tinywasm/deploy"
)

// TestWizard_WebhookFlow exercises the full webhook wizard step sequence.
func TestWizard_WebhookFlow(t *testing.T) {
	store := NewMockStore()
	p := &deploy.Puller{
		Store: store,
	}

	steps := p.GetSteps()
	if len(steps) == 0 {
		t.Fatal("GetSteps() returned no steps")
	}

	ctx := twctx.Background()

	// Step 0: choose method = webhook
	ok, err := steps[0].OnInput("webhook", ctx)
	if err != nil || !ok {
		t.Fatalf("step0 (method) failed: %v", err)
	}
	if got, _ := store.Get("DEPLOY_METHOD"); got != "webhook" {
		t.Errorf("expected DEPLOY_METHOD=webhook, got %q", got)
	}

	// Step 1 (proxy[0]): server host
	ok, err = steps[1].OnInput("myserver.com:9000", ctx)
	if err != nil || !ok {
		t.Fatalf("step1 (host) failed: %v", err)
	}
	if got, _ := store.Get("DEPLOY_SERVER_HOST"); got != "myserver.com:9000" {
		t.Errorf("expected DEPLOY_SERVER_HOST=myserver.com:9000, got %q", got)
	}

	// Step 2 (proxy[1]): HMAC secret (min 32 chars)
	ok, err = steps[2].OnInput("this_secret_is_at_least_32_chars_long!", ctx)
	if err != nil || !ok {
		t.Fatalf("step2 (hmac) failed: %v", err)
	}
	if got, _ := store.Get("DEPLOY_HMAC_SECRET"); got == "" {
		t.Errorf("expected DEPLOY_HMAC_SECRET to be set")
	}

	// Step 3 (proxy[2]): GitHub PAT — triggers file generation
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	ok, err = steps[3].OnInput("ghp_testtoken123", ctx)
	if err != nil || !ok {
		t.Fatalf("step3 (pat) failed: %v", err)
	}
	if got, _ := store.Get("DEPLOY_GITHUB_PAT"); got != "ghp_testtoken123" {
		t.Errorf("expected DEPLOY_GITHUB_PAT=ghp_testtoken123, got %q", got)
	}
}

// TestWizard_SSHFlow exercises the SSH wizard step sequence.
func TestWizard_SSHFlow(t *testing.T) {
	store := NewMockStore()
	p := &deploy.Puller{Store: store}

	steps := p.GetSteps()
	ctx := twctx.Background()

	// Step 0: method = ssh
	if _, err := steps[0].OnInput("ssh", ctx); err != nil {
		t.Fatalf("step0 failed: %v", err)
	}

	// Step 1: host
	if _, err := steps[1].OnInput("myserver.com", ctx); err != nil {
		t.Fatalf("step1 (host) failed: %v", err)
	}

	// Step 2: ssh user
	if _, err := steps[2].OnInput("deploy", ctx); err != nil {
		t.Fatalf("step2 (user) failed: %v", err)
	}

	// Step 3: ssh key path
	if _, err := steps[3].OnInput("~/.ssh/id_ed25519", ctx); err != nil {
		t.Fatalf("step3 (key) failed: %v", err)
	}

	// Step 4: PAT — triggers file generation
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	if _, err := steps[4].OnInput("ghp_sshpat456", ctx); err != nil {
		t.Fatalf("step4 (pat) failed: %v", err)
	}

	if got, _ := store.Get("DEPLOY_SSH_USER"); got != "deploy" {
		t.Errorf("expected DEPLOY_SSH_USER=deploy, got %q", got)
	}
	if got, _ := store.Get("DEPLOY_SSH_KEY"); got != "~/.ssh/id_ed25519" {
		t.Errorf("expected DEPLOY_SSH_KEY=~/.ssh/id_ed25519, got %q", got)
	}
}

// TestWizard_InvalidMethod checks that an unknown method is rejected.
func TestWizard_InvalidMethod(t *testing.T) {
	store := NewMockStore()
	p := &deploy.Puller{Store: store}
	steps := p.GetSteps()
	ctx := twctx.Background()

	ok, err := steps[0].OnInput("ftp", ctx)
	if err == nil {
		t.Error("expected error for invalid method, got nil")
	}
	if ok {
		t.Error("expected ok=false for invalid method")
	}
}

// TestCreateDefaultConfig verifies a default config file is created.
func TestCreateDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := deploy.CreateDefaultConfig(configPath); err != nil {
		t.Fatalf("CreateDefaultConfig() error = %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if len(content) == 0 {
		t.Errorf("config file is empty")
	}

	// Calling again must be idempotent (no error, no overwrite)
	if err := deploy.CreateDefaultConfig(configPath); err != nil {
		t.Errorf("second CreateDefaultConfig() error = %v", err)
	}
}
