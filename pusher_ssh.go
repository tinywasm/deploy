package deploy

import (
	"fmt"

	"github.com/tinywasm/context"
	"github.com/tinywasm/wizard"
)

func init() {
	RegisterPusher(&SSHPusher{})
}

type SSHPusher struct{}

func (s *SSHPusher) Name() string { return "ssh" }

func (s *SSHPusher) Run(cfg *Config, p *Puller) error {
	pat, err := p.Store.Get("DEPLOY_GITHUB_PAT")
	if err != nil || pat == "" {
		return fmt.Errorf("deploy: GitHub PAT not configured")
	}
	for _, app := range cfg.Apps {
		script := SSHScript(app, "", pat)
		p.logger("# SSH script for", app.Name+":\n"+script)
	}
	return nil
}

func (s *SSHPusher) WizardSteps(store Store, log func(...any)) []*wizard.Step {
	return []*wizard.Step{
		{
			LabelText: "Server host (e.g. myserver.com)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("host cannot be empty")
				}
				ctx.Set(ctxServerHost, input)
				return true, store.Set("DEPLOY_SERVER_HOST", input)
			},
		},
		{
			LabelText: "SSH username (e.g. deploy)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("SSH user cannot be empty")
				}
				ctx.Set(ctxSSHUser, input)
				return true, store.Set("DEPLOY_SSH_USER", input)
			},
		},
		{
			LabelText: "SSH private key path (e.g. ~/.ssh/id_ed25519)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("SSH key path cannot be empty")
				}
				ctx.Set(ctxSSHKey, input)
				return true, store.Set("DEPLOY_SSH_KEY", input)
			},
		},
		{
			LabelText: "GitHub PAT (ghp_... or github_pat_... — needs repo read access)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("PAT cannot be empty")
				}
				ctx.Set(ctxPAT, input)
				if err := store.Set("DEPLOY_GITHUB_PAT", input); err != nil {
					return false, err
				}

				host := ctx.Value(ctxServerHost)
				user := ctx.Value(ctxSSHUser)
				key := ctx.Value(ctxSSHKey)
				return true, writeGHAWorkflow("ssh", host, fmt.Sprintf("user=%s key=%s", user, key))
			},
		},
	}
}
