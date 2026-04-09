package deploy

import "github.com/tinywasm/wizard"

func init() {
	RegisterPusher(&CloudflarePagesPusher{})
	RegisterPusher(&CloudflareWorkerPusher{})
}

type CloudflarePagesPusher struct{}

func (s *CloudflarePagesPusher) Name() string { return "cloudflarePages" }
func (s *CloudflarePagesPusher) Run(cfg *Config, p *Puller) error {
	if p.Provider != nil {
		return p.Provider.Deploy(p.Store)
	}
	return nil
}
func (s *CloudflarePagesPusher) WizardSteps(store Store, log func(...any)) []*wizard.Step { return nil }

type CloudflareWorkerPusher struct{}

func (s *CloudflareWorkerPusher) Name() string { return "cloudflareWorker" }
func (s *CloudflareWorkerPusher) Run(cfg *Config, p *Puller) error {
	if p.Provider != nil {
		return p.Provider.Deploy(p.Store)
	}
	return nil
}
func (s *CloudflareWorkerPusher) WizardSteps(store Store, log func(...any)) []*wizard.Step { return nil }
