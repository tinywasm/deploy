package deploy

import (
	"fmt"
	"net/http"

	"github.com/tinywasm/context"
	"github.com/tinywasm/wizard"
)

func init() {
	RegisterPusher(&WebhookTrigger{})
}

type WebhookTrigger struct{}

func (s *WebhookTrigger) Name() string { return "webhook" }

func (s *WebhookTrigger) Run(cfg *Config, p *Puller) error {
	hmacSecret, err := p.Store.Get("DEPLOY_HMAC_SECRET")
	if err != nil || hmacSecret == "" {
		return fmt.Errorf("deploy: HMAC secret not configured")
	}

	handler := &Handler{
		Config:     cfg,
		ConfigPath: p.ConfigPath,
		Validator:  NewHMACValidator(hmacSecret),
		Downloader: p.Downloader,
		Process:    p.Process,
		Checker:    p.Checker,
		Keys:       p.Store,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/update", handler.HandleUpdate)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", cfg.Updater.Port)
	p.logger("Starting puller agent on", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *WebhookTrigger) WizardSteps(store Store, log func(...any)) []*wizard.Step {
	return []*wizard.Step{
		{
			LabelText: "Server host:port for webhook daemon (e.g. myserver.com:9000)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if input == "" {
					return false, fmt.Errorf("host cannot be empty")
				}
				ctx.Set(ctxServerHost, input)
				return true, store.Set("DEPLOY_SERVER_HOST", input)
			},
		},
		{
			LabelText: "HMAC Secret (min 32 chars, used to validate webhook from GitHub)",
			OnInputFn: func(input string, ctx *context.Context) (bool, error) {
				if len(input) < 32 {
					return false, fmt.Errorf("secret must be at least 32 characters")
				}
				ctx.Set(ctxHMAC, input)
				return true, store.Set("DEPLOY_HMAC_SECRET", input)
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
				// We need to generate the files.
				// Since generateWebhookFiles is currently in wizard.go,
				// and it's package-private, we can either call it or move it.
				// For now, let's call a shared helper or move the logic here.
				host := ctx.Value(ctxServerHost)
				if err := writeGHAWorkflow("webhook", host, ""); err != nil {
					return false, err
				}
				if err := CreateDefaultConfig("deploy.yaml"); err != nil {
					return false, err
				}
				return true, nil
			},
		},
	}
}
