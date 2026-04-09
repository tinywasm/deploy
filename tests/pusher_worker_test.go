package deploy_test

import (
	"testing"

	twctx "github.com/tinywasm/context"
	"github.com/tinywasm/deploy"
	"github.com/zalando/go-keyring"
)

// TestStrategy_WorkerSetup exercises the Cloudflare Worker wizard setup flow
// ensuring the API token ends up in the keyring and NOT the base store.
func TestStrategy_WorkerSetup(t *testing.T) {
	keyring.MockInit()
	baseStore := NewMockStore()

	d := deploy.NewDaemon(&deploy.DaemonConfig{
		EdgeDir:   "edge",
		OutputDir: ".build",
		Store:     baseStore,
	})
	p := d.Puller()
	puller := p.(*deploy.Puller)

	steps := puller.GetSteps()
	ctx := twctx.Background()

	// Step 0: method = cloudflareWorker
	if _, err := steps[0].OnInput("cloudflareWorker", ctx); err != nil {
		t.Fatalf("step0 failed: %v", err)
	}

	// Step 1: Account ID
	if _, err := steps[1].OnInput("acc123", ctx); err != nil {
		t.Fatalf("step1 (account) failed: %v", err)
	}

	// Step 2: Token
	tokenInput := "this_is_a_token_long_enough"
	if _, err := steps[2].OnInput(tokenInput, ctx); err != nil {
		t.Fatalf("step2 (token) failed: %v", err)
	}

	// Step 3: Project
	if _, err := steps[3].OnInput("myproject", ctx); err != nil {
		t.Fatalf("step3 (project) failed: %v", err)
	}

	// --- Verification of Secure Storage ---

	// 1. Should be in Keyring
	val, err := keyring.Get(deploy.KeyringServiceName, "goflare/myproject")
	if err != nil || val != tokenInput {
		t.Errorf("expected goflare/myproject in keyring, got err=%v", err)
	}

	// 2. Should NOT be in base Store
	if val, _ := baseStore.Get("goflare/myproject"); val != "" {
		t.Errorf("expected goflare/myproject to be empty in base store, got %q", val)
	}

	// 3. Non-sensitive config should be in base Store
	if acc, err := baseStore.Get("CF_ACCOUNT_ID"); err != nil || acc != "acc123" {
		t.Errorf("expected CF_ACCOUNT_ID in base store")
	}
	if proj, err := baseStore.Get("CF_PROJECT"); err != nil || proj != "myproject" {
		t.Errorf("expected CF_PROJECT in base store")
	}
}
