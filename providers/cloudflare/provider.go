package cloudflare

import (
	"fmt"

	twctx "github.com/tinywasm/context"
	"github.com/tinywasm/goflare"
	"github.com/tinywasm/wizard"
)


// Provider implements deploy.Provider for Cloudflare Workers + Pages.
type Provider struct {
	gf *goflare.Goflare
}

// New creates a new Cloudflare provider.
func New(edgeDir, outputDir string) *Provider {
	return &Provider{
		gf: goflare.New(&goflare.Config{
			Entry:     edgeDir,
			OutputDir: outputDir,
		}),
	}
}

func (p *Provider) Build() error { return p.gf.Build() }

func (p *Provider) Deploy(store interface {
	Get(string) (string, error)
	Set(string, string) error
}) error {
	// Restore config from store (set during wizard)
	if accountID, err := store.Get("CF_ACCOUNT_ID"); err == nil && accountID != "" {
		p.gf.Config.AccountID = accountID
	}
	if project, err := store.Get("CF_PROJECT"); err == nil && project != "" {
		p.gf.Config.ProjectName = project
	}

	// Determine if we should use DeployWorker or DeployPages based on DEPLOY_METHOD.
	// Defaults to DeployPages (goflare v0.1.0's main entry point).
	method, _ := store.Get("DEPLOY_METHOD")
	if method == "cloudflareWorker" {
		return p.gf.DeployWorker(store)
	}
	return p.gf.Deploy(store)
}

func (p *Provider) SetLog(f func(...any)) { p.gf.SetLog(f) }

func (p *Provider) MainInputFileRelativePath() string { return p.gf.MainInputFileRelativePath() }

func (p *Provider) NewFileEvent(fileName, extension, filePath, event string) error {
	return p.gf.NewFileEvent(fileName, extension, filePath, event)
}

func (p *Provider) SupportedExtensions() []string { return p.gf.SupportedExtensions() }

func (p *Provider) UnobservedFiles() []string { return p.gf.UnobservedFiles() }

func (p *Provider) Supports(method string) bool {
	return method == "cloudflarePages" || method == "cloudflareWorker"
}

func (p *Provider) WizardSteps(store interface {
	Get(string) (string, error)
	Set(string, string) error
}, log func(...any)) []*wizard.Step {
	p.gf.SetLog(log)
	const ctxAccount = "cf_account"
	const ctxToken = "cf_token"

	return []*wizard.Step{
		{
			LabelText: "Cloudflare Account ID (dashboard.cloudflare.com -> right sidebar)",
			OnInputFn: func(input string, ctx *twctx.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("account ID cannot be empty")
				}
				ctx.Set(ctxAccount, input)
				return true, nil
			},
		},
		{
			LabelText: "Cloudflare API Token (Workers:Edit + Pages:Edit)",
			OnInputFn: func(input string, ctx *twctx.Context) (bool, error) {
				if len(input) < 20 {
					return false, fmt.Errorf("token looks too short")
				}
				ctx.Set(ctxToken, input)
				return true, nil
			},
		},
		{
			LabelText: "Project name (auto-created on first deploy)",
			OnInputFn: func(input string, ctx *twctx.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("project name cannot be empty")
				}
				accountID := ctx.Value(ctxAccount)
				token := ctx.Value(ctxToken)

				p.gf.Config.ProjectName = input
				p.gf.Config.AccountID = accountID

				// Store token in goflare format: "goflare/<ProjectName>"
				if err := store.Set("goflare/"+input, token); err != nil {
					return false, fmt.Errorf("failed to store token: %w", err)
				}
				if err := store.Set("CF_ACCOUNT_ID", accountID); err != nil {
					return false, err
				}
				if err := store.Set("CF_PROJECT", input); err != nil {
					return false, err
				}
				return true, nil
			},
		},
	}
}
