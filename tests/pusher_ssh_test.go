package deploy_test

import (
	"os"
	"testing"

	twctx "github.com/tinywasm/context"
	"github.com/tinywasm/deploy"
	"github.com/zalando/go-keyring"
)

// TestStrategy_SSHSetup exercises the SSH wizard setup flow
// ensuring sensitive keys end up in the keyring and NOT the base store.
func TestStrategy_SSHSetup(t *testing.T) {
	keyring.MockInit()
	baseStore := NewMockStore()
	store := deploy.NewSecureStore(baseStore)

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
	keyPath := "~/.ssh/id_ed25519"
	if _, err := steps[3].OnInput(keyPath, ctx); err != nil {
		t.Fatalf("step3 (key) failed: %v", err)
	}

	// Step 4: PAT
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	patInput := "ghp_sshpat456"
	if _, err := steps[4].OnInput(patInput, ctx); err != nil {
		t.Fatalf("step4 (pat) failed: %v", err)
	}

	// --- Verification of Secure Storage ---

	// 1. Should be in Keyring
	pat, err := keyring.Get(deploy.KeyringServiceName, "DEPLOY_GITHUB_PAT")
	if err != nil || pat != patInput {
		t.Errorf("expected DEPLOY_GITHUB_PAT in keyring: %q", patInput)
	}

	key, err := keyring.Get(deploy.KeyringServiceName, "DEPLOY_SSH_KEY")
	if err != nil || key != keyPath {
		t.Errorf("expected DEPLOY_SSH_KEY in keyring: %q", keyPath)
	}

	// 2. Should NOT be in base Store
	if val, _ := baseStore.Get("DEPLOY_GITHUB_PAT"); val != "" {
		t.Errorf("expected DEPLOY_GITHUB_PAT to be empty in base store, got %q", val)
	}

	if val, _ := baseStore.Get("DEPLOY_SSH_KEY"); val != "" {
		t.Errorf("expected DEPLOY_SSH_KEY to be empty in base store, got %q", val)
	}

	// 3. Non-sensitive config should be in base Store
	if user, err := baseStore.Get("DEPLOY_SSH_USER"); err != nil || user != "deploy" {
		t.Errorf("expected DEPLOY_SSH_USER in base store")
	}
}
