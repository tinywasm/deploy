package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tinywasm/context"
	"github.com/tinywasm/wizard"
)

const (
	ctxMethod     = "DEPLOY_METHOD"
	ctxServerHost = "DEPLOY_SERVER_HOST"
	ctxHMAC       = "DEPLOY_HMAC_SECRET"
	ctxPAT        = "DEPLOY_GITHUB_PAT"
	ctxSSHUser    = "DEPLOY_SSH_USER"
	ctxSSHKey     = "DEPLOY_SSH_KEY"
	ctxCFAccount  = "CF_ACCOUNT_ID"
	ctxCFToken    = "CF_BOOTSTRAP_TOKEN"
	ctxCFProject  = "CF_PROJECT"
)

// deployWizard holds a Store reference so wizard steps can persist answers.
type deployWizard struct {
	store   Store
	cfSteps []*wizard.Step // dynamic steps injected based on chosen method
	log     func(...any)
}

// GetSteps implements the interface expected by tinywasm/wizard.New().
// Returns the initial steps; method-specific steps are injected dynamically
// via the Step 1 OnInputFn after the user chooses a deploy method.
func (d *Deploy) GetSteps() []*wizard.Step {
	dw := &deployWizard{
		store: d.Store,
		log:   d.log,
	}
	return dw.buildSteps()
}

func (dw *deployWizard) buildSteps() []*wizard.Step {
	// Step 1: choose method — injects remaining steps into ctx via side-effect.
	// tinywasm/wizard reads steps once at New() time, so we use a shared slice
	// that Step 1 populates and subsequent steps read from.
	dynamic := &dynamicSteps{}

	step1 := &wizard.Step{
		LabelText: "Deploy method: cloudflare | webhook | ssh",
		OnInputFn: func(input string, ctx *context.Context) (bool, error) {
			method := strings.ToLower(strings.TrimSpace(input))
			switch method {
			case "cloudflare", "webhook", "ssh":
			default:
				return false, fmt.Errorf("invalid method — choose: cloudflare, webhook, or ssh")
			}
			ctx.Set(ctxMethod, method)
			if err := dw.store.Set("DEPLOY_METHOD", method); err != nil {
				return false, fmt.Errorf("store method: %w", err)
			}
			dynamic.steps = dw.stepsForMethod(method, ctx)
			return true, nil
		},
	}

	// Proxy steps delegate to dynamic.steps[i] once populated
	proxy := make([]*wizard.Step, 5)
	for i := range proxy {
		idx := i // capture
		proxy[idx] = &wizard.Step{
			LabelText: "",
			DefaultFn: func(ctx *context.Context) string {
				if idx >= len(dynamic.steps) {
					return ""
				}
				return dynamic.steps[idx].DefaultValue(ctx)
			},
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if idx >= len(dynamic.steps) {
					// No more steps for this method — skip gracefully
					return true, nil
				}
				return dynamic.steps[idx].OnInput(input, ctx)
			},
		}
	}

	// Combine: step1 + up to 5 proxy steps
	all := make([]*wizard.Step, 0, 6)
	all = append(all, step1)
	all = append(all, proxy...)
	return all
}

// dynamicSteps holds method-specific steps, populated after Step 1 completes.
type dynamicSteps struct {
	steps []*wizard.Step
}

func (dw *deployWizard) stepsForMethod(method string, ctx *context.Context) []*wizard.Step {
	switch method {
	case "cloudflare":
		return dw.cloudfareSteps()
	case "webhook":
		return dw.webhookSteps()
	case "ssh":
		return dw.sshSteps()
	}
	return nil
}

func (dw *deployWizard) cloudfareSteps() []*wizard.Step {
	cf := NewCFClient(dw.store)
	cf.SetLog(dw.log)

	return []*wizard.Step{
		{
			LabelText: "Cloudflare Account ID (dashboard.cloudflare.com → right sidebar)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("account ID cannot be empty")
				}
				ctx.Set(ctxCFAccount, input)
				return true, nil
			},
		},
		{
			LabelText: "Bootstrap API Token (Cloudflare dashboard → My Profile → API Tokens → Edit user API tokens)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if len(input) < 20 {
					return false, fmt.Errorf("token looks too short")
				}
				ctx.Set(ctxCFToken, input)
				return true, nil
			},
		},
		{
			LabelText: "Cloudflare Pages project name (create it first at pages.cloudflare.com)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("project name cannot be empty")
				}
				accountID := ctx.Value(ctxCFAccount)
				bootstrapToken := ctx.Value(ctxCFToken)
				if err := cf.Setup(accountID, bootstrapToken, input); err != nil {
					return false, fmt.Errorf("Cloudflare setup failed: %w", err)
				}
				return true, nil
			},
		},
	}
}

func (dw *deployWizard) webhookSteps() []*wizard.Step {
	return []*wizard.Step{
		{
			LabelText: "Server host:port for webhook daemon (e.g. myserver.com:9000)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("host cannot be empty")
				}
				ctx.Set(ctxServerHost, input)
				return true, dw.store.Set("DEPLOY_SERVER_HOST", input)
			},
		},
		{
			LabelText: "HMAC Secret (min 32 chars, used to validate webhook from GitHub)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if len(input) < 32 {
					return false, fmt.Errorf("secret must be at least 32 characters")
				}
				ctx.Set(ctxHMAC, input)
				return true, dw.store.Set("DEPLOY_HMAC_SECRET", input)
			},
		},
		{
			LabelText: "GitHub PAT (ghp_... or github_pat_... — needs repo read access)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("PAT cannot be empty")
				}
				ctx.Set(ctxPAT, input)
				if err := dw.store.Set("DEPLOY_GITHUB_PAT", input); err != nil {
					return false, err
				}
				return true, dw.generateWebhookFiles(ctx)
			},
		},
	}
}

func (dw *deployWizard) sshSteps() []*wizard.Step {
	return []*wizard.Step{
		{
			LabelText: "Server host (e.g. myserver.com)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("host cannot be empty")
				}
				ctx.Set(ctxServerHost, input)
				return true, dw.store.Set("DEPLOY_SERVER_HOST", input)
			},
		},
		{
			LabelText: "SSH username (e.g. deploy)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("SSH user cannot be empty")
				}
				ctx.Set(ctxSSHUser, input)
				return true, dw.store.Set("DEPLOY_SSH_USER", input)
			},
		},
		{
			LabelText: "SSH private key path (e.g. ~/.ssh/id_ed25519)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("SSH key path cannot be empty")
				}
				ctx.Set(ctxSSHKey, input)
				return true, dw.store.Set("DEPLOY_SSH_KEY", input)
			},
		},
		{
			LabelText: "GitHub PAT (ghp_... or github_pat_... — needs repo read access)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("PAT cannot be empty")
				}
				ctx.Set(ctxPAT, input)
				if err := dw.store.Set("DEPLOY_GITHUB_PAT", input); err != nil {
					return false, err
				}
				return true, dw.generateSSHFiles(ctx)
			},
		},
	}
}

// ── file generation ───────────────────────────────────────────────────────────

func (dw *deployWizard) generateWebhookFiles(ctx *context.Context) error {
	host := ctx.Value(ctxServerHost)
	if err := writeGHAWorkflow("webhook", host, ""); err != nil {
		return err
	}
	return CreateDefaultConfig("deploy.yaml")
}

func (dw *deployWizard) generateSSHFiles(ctx *context.Context) error {
	host := ctx.Value(ctxServerHost)
	user := ctx.Value(ctxSSHUser)
	key := ctx.Value(ctxSSHKey)
	return writeGHAWorkflow("ssh", host, fmt.Sprintf("user=%s key=%s", user, key))
}

func writeGHAWorkflow(method, host, extra string) error {
	dir := ".github/workflows"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create workflows dir: %w", err)
	}
	path := filepath.Join(dir, "deploy.yml")
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}

	var content string
	switch method {
	case "webhook":
		content = fmt.Sprintf(`name: Deploy
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Trigger deploy webhook
        run: |
          PAYLOAD=$(cat release.json)
          SIG=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "${{ secrets.DEPLOY_HMAC_SECRET }}" | awk '{print "sha256="$2}')
          curl -X POST http://%s/update \
            -H "X-Signature: $SIG" \
            -H "Content-Type: application/json" \
            -d "$PAYLOAD"
`, host)
	case "ssh":
		content = fmt.Sprintf(`name: Deploy
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Deploy via SSH
        run: |
          # SSH deploy to %s
          # Configure SSH key in GitHub secrets as DEPLOY_SSH_KEY
          echo "Deploy via SSH — configure your SSH steps here"
# extra: %s
`, host, extra)
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// CreateDefaultConfig creates a default deploy.yaml if it does not exist.
func CreateDefaultConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(`updater:
  port: 8080
  log_level: info
  temp_dir: ./temp

apps: []
`), 0644)
}
