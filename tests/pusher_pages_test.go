package deploy_test

import (
	"strings"
	"testing"

	twctx "github.com/tinywasm/context"
	"github.com/tinywasm/deploy"
	"github.com/zalando/go-keyring"
)

// TestStrategy_PagesSetup exercises the Cloudflare Pages wizard setup flow.
// It verifies that wizard steps collect necessary data and attempt the setup API call.
func TestStrategy_PagesSetup(t *testing.T) {
	keyring.MockInit()
	baseStore := NewMockStore()
	store := deploy.NewSecureStore(baseStore)

	p := &deploy.Puller{Store: store}
	steps := p.GetSteps()
	ctx := twctx.Background()

	// Step 0: method = cloudflarePages
	if _, err := steps[0].OnInput("cloudflarePages", ctx); err != nil {
		t.Fatalf("step0 failed: %v", err)
	}

	// Step 1: Account ID
	if _, err := steps[1].OnInput("acc123", ctx); err != nil {
		t.Fatalf("step1 (account) failed: %v", err)
	}

	// Step 2: Bootstrap Token
	if _, err := steps[2].OnInput("this_is_a_bootstrap_token_long_enough", ctx); err != nil {
		t.Fatalf("step2 (token) failed: %v", err)
	}

	// Step 3: Project
	// This step calls goflare.SetupPages() which will make an actual HTTP POST request to Cloudflare.
	// Since our token is a dummy token, it will fail. We assert that it fails gracefully.
	// If it had succeeded, CF_PAGES_TOKEN would be securely stored via the injected SecureStore.
	_, err := steps[3].OnInput("myproject", ctx)
	if err == nil {
		t.Error("expected error from Cloudflare API during SetupPages with dummy token")
	} else if !strings.Contains(err.Error(), "Cloudflare Pages setup failed") {
		t.Errorf("expected Cloudflare Pages setup failure, got: %v", err)
	}
}
