package deploy

import (
	"fmt"

	"github.com/tinywasm/context"
	"github.com/tinywasm/goflare"
	"github.com/tinywasm/wizard"
)

func init() {
	RegisterPusher(&CloudflareWorkerPusher{})
}

// CloudflareWorkerPusher implements deployment specifically to Cloudflare Workers.
type CloudflareWorkerPusher struct {
	// Goflare instance used for deployment.
	Goflare *goflare.Goflare
}

func (s *CloudflareWorkerPusher) Name() string { return "cloudflareWorker" }

func (s *CloudflareWorkerPusher) Run(cfg *Config, p *Puller) error {
	if s.Goflare == nil {
		return fmt.Errorf("cloudflareWorker: goflare instance not configured in pusher")
	}
	return s.Goflare.DeployWorker(p.Store)
}

func (s *CloudflareWorkerPusher) WizardSteps(store Store, log func(...any)) []*wizard.Step {
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
			LabelText: "Worker API Token (Cloudflare dashboard -> My Profile -> API Tokens -> Custom Token with Workers:Edit)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if len(input) < 20 {
					return false, fmt.Errorf("token looks too short")
				}
				ctx.Set(ctxCFToken, input)
				// For Workers, we store it directly as CF_WORKER_TOKEN
				return true, store.Set("CF_WORKER_TOKEN", input)
			},
		},
		{
			LabelText: "Cloudflare Worker name",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("worker name cannot be empty")
				}
				accountID := ctx.Value(ctxCFAccount)
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
