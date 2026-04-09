package deploy

import (
	"fmt"

	"github.com/tinywasm/context"
	"github.com/tinywasm/goflare"
	"github.com/tinywasm/wizard"
)

func init() {
	RegisterPusher(&CloudflarePagesPusher{})
}

// CloudflarePagesPusher implements deployment specifically to Cloudflare Pages.
type CloudflarePagesPusher struct {
	// Goflare instance used for deployment.
	Goflare *goflare.Goflare
}

func (s *CloudflarePagesPusher) Name() string { return "cloudflarePages" }

func (s *CloudflarePagesPusher) Run(cfg *Config, p *Puller) error {
	if s.Goflare == nil {
		return fmt.Errorf("cloudflarePages: goflare instance not configured in pusher")
	}
	return s.Goflare.DeployPages(p.Store)
}

func (s *CloudflarePagesPusher) WizardSteps(store Store, log func(...any)) []*wizard.Step {
	cf := s.Goflare
	if cf == nil {
		cf = goflare.New(nil)
	}
	cf.SetLog(log)

	return []*wizard.Step{
		{
			LabelText: "Cloudflare Account ID (dashboard.cloudflare.com -> right sidebar)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("account ID cannot be empty")
				}
				ctx.Set(ctxCFAccount, input)
				return true, nil
			},
		},
		{
			LabelText: "Bootstrap API Token (Cloudflare dashboard -> My Profile -> API Tokens -> Edit user API tokens)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if len(input) < 20 {
					return false, fmt.Errorf("token looks too short")
				}
				ctx.Set(ctxCFToken, input)
				return true, nil
			},
		},
		{
			LabelText: "Cloudflare Pages project name (will be auto-created on first deploy)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("project name cannot be empty")
				}
				accountID := ctx.Value(ctxCFAccount)
				token := ctx.Value(ctxCFToken)

				// Configure goflare instance for future DeployPages calls
				cf.Config.ProjectName = input
				cf.Config.AccountID = accountID

				// Store token in goflare's expected format: "goflare/<ProjectName>"
				if err := store.Set("goflare/"+input, token); err != nil {
					return false, fmt.Errorf("failed to store token: %w", err)
				}
				// Store AccountID and ProjectName for Puller.Run to reconstruct goflare.Config
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
