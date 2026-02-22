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
)

// GetSteps implements the interface expected by tinywasm/wizard.New().
// Returns the initial steps; method-specific steps are injected dynamically
// via the Step 1 OnInputFn after the user chooses a deploy method.
func (p *Puller) GetSteps() []*wizard.Step {
	// Step 1: choose method — injects remaining steps into ctx via side-effect.
	dynamic := &dynamicSteps{}

	pushers := AvailablePushers()

	step1 := &wizard.Step{
		LabelText: "Choose option number (1-4):",
		OnShowFn: func(log func(...any)) {
			// Print menu only when step 1 becomes active
			pushers := AvailablePushers()
			menu := "Available methods:\n"
			for i, s := range pushers {
				menu += fmt.Sprintf("  %d) %s\n", i+1, s)
			}
			log(menu)
		},
		OnInputFn: func(input string, ctx *context.Context) (bool, error) {
			method := strings.TrimSpace(input)
			// DEPRECATED: Handle legacy inputs by mapping them to "cloudflarePages".
			// Kept for backward compatibility with old configuration prompts.
			if strings.EqualFold(method, "cloudflare") || strings.EqualFold(method, "edgeworker") {
				method = "cloudflarePages"
			}

			strat, err := GetPusher(method)
			if err != nil {
				return false, fmt.Errorf("invalid method — choose: %s", strings.Join(pushers, ", "))
			}

			method = strat.Name() // Ensure correct casing matches pusher definition

			ctx.Set(ctxMethod, method)
			if err := p.Store.Set("DEPLOY_METHOD", method); err != nil {
				return false, fmt.Errorf("store method: %w", err)
			}
			dynamic.steps = strat.WizardSteps(p.Store, p.log) // pass the file logger for actual execution logs
			return true, nil
		},
	}

	// Proxy steps delegate to dynamic.steps[idx] once populated
	proxy := make([]*wizard.Step, 5)
	for i := range proxy {
		idx := i // capture
		proxy[idx] = &wizard.Step{
			LabelText: "",
			DefaultFn: func(ctx *context.Context) string {
				if idx >= len(dynamic.steps) || dynamic.steps[idx] == nil {
					return ""
				}
				if dynamic.steps[idx].DefaultFn != nil {
					return dynamic.steps[idx].DefaultFn(ctx)
				}
				return ""
			},
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if idx >= len(dynamic.steps) || dynamic.steps[idx] == nil {
					// No more steps for this method — skip gracefully
					return true, nil
				}
				return dynamic.steps[idx].OnInputFn(input, ctx)
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

// ── file generation ───────────────────────────────────────────────────────────

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
