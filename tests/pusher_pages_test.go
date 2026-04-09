package deploy_test

import (
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

	// Step 3: Project name — now just stores credentials, no API call
	if _, err := steps[3].OnInput("myproject", ctx); err != nil {
		t.Fatalf("step3 (project) failed: %v", err)
	}

	// Verify token stored in goflare format
	token, err := store.Get("goflare/myproject")
	if err != nil || token == "" {
		t.Errorf("expected token stored as goflare/myproject, got err=%v token=%q", err, token)
	}
	// Verify AccountID and ProjectName stored
	if v, _ := store.Get("CF_ACCOUNT_ID"); v != "acc123" {
		t.Errorf("expected CF_ACCOUNT_ID=acc123, got %q", v)
	}
	if v, _ := store.Get("CF_PROJECT"); v != "myproject" {
		t.Errorf("expected CF_PROJECT=myproject, got %q", v)
	}
}
