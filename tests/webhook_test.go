package deploy_test

import (
	"os"
	"testing"

	twctx "github.com/tinywasm/context"
	"github.com/tinywasm/deploy"
	"github.com/zalando/go-keyring"
)

// TestStrategy_WebhookSetup exercises the webhook setup flow
// ensuring sensitive keys end up in the keyring and NOT the base store.
func TestStrategy_WebhookSetup(t *testing.T) {
	keyring.MockInit()
	baseStore := NewMockStore()
	store := deploy.NewSecureStore(baseStore)

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

	// Step 1: server host
	ok, err = steps[1].OnInput("myserver.com:9000", ctx)
	if err != nil || !ok {
		t.Fatalf("step1 (host) failed: %v", err)
	}

	// Step 2: HMAC secret (min 32 chars)
	hmacInput := "this_secret_is_at_least_32_chars_long!"
	ok, err = steps[2].OnInput(hmacInput, ctx)
	if err != nil || !ok {
		t.Fatalf("step2 (hmac) failed: %v", err)
	}

	// Step 3: GitHub PAT
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	patInput := "ghp_testtoken123"
	ok, err = steps[3].OnInput(patInput, ctx)
	if err != nil || !ok {
		t.Fatalf("step3 (pat) failed: %v", err)
	}

	// --- Verification of Secure Storage ---

	// 1. Should be in Keyring
	pat, err := keyring.Get(deploy.KeyringServiceName, "DEPLOY_GITHUB_PAT")
	if err != nil || pat != patInput {
		t.Errorf("expected DEPLOY_GITHUB_PAT in keyring: %q, got error: %v", patInput, err)
	}

	hmac, err := keyring.Get(deploy.KeyringServiceName, "DEPLOY_HMAC_SECRET")
	if err != nil || hmac != hmacInput {
		t.Errorf("expected DEPLOY_HMAC_SECRET in keyring: %q, got error: %v", hmacInput, err)
	}

	// 2. Should NOT be in base Store
	if val, _ := baseStore.Get("DEPLOY_GITHUB_PAT"); val != "" {
		t.Errorf("expected DEPLOY_GITHUB_PAT to be empty in base store, got %q", val)
	}

	if val, _ := baseStore.Get("DEPLOY_HMAC_SECRET"); val != "" {
		t.Errorf("expected DEPLOY_HMAC_SECRET to be empty in base store, got %q", val)
	}

	// 3. Non-sensitive config should be in base Store
	if host, err := baseStore.Get("DEPLOY_SERVER_HOST"); err != nil || host != "myserver.com:9000" {
		t.Errorf("expected DEPLOY_SERVER_HOST in base store, got err: %v", err)
	}
}
