// Daemon composes the goflare Edge Worker compiler with the Puller orchestrator
// into a single unit for tinywasm/app. app imports only tinywasm/deploy.
package deploy

import (
	"github.com/tinywasm/deploy/providers/cloudflare"
	"github.com/tinywasm/wizard"
)

// DaemonConfig holds all configuration needed to initialise both the provider and Puller.
type DaemonConfig struct {
	EdgeDir          string // relative input dir for edge worker source
	OutputDir        string // relative output dir for compiled artifacts
	DeployConfigPath string // path to deploy.yaml
	Store            Store
}

// Daemon composes the provider handler with the Puller orchestrator.
type Daemon struct {
	provider Provider
	puller   *Puller
}

// NewDaemon creates a Daemon with fully-initialised provider and Puller internals.
func NewDaemon(cfg *DaemonConfig) *Daemon {
	provider := cloudflare.New(cfg.EdgeDir, cfg.OutputDir)

	puller := &Puller{
		Store:      NewSecureStore(cfg.Store),
		Process:    NewProcessManager(),
		Downloader: NewDownloader(),
		Checker:    NewChecker(),
		ConfigPath: cfg.DeployConfigPath,
		Provider:   provider,
	}

	return &Daemon{provider: provider, puller: puller}
}

// EdgeWorker returns the provider handler for TUI registration (AddHandler).
func (d *Daemon) EdgeWorker() any { return d.provider }

// Puller returns the puller handler for TUI registration (AddHandler).
func (d *Daemon) Puller() any { return d.puller }

// SetLog injects a logger into both components.
func (d *Daemon) SetLog(f func(...any)) {
	d.provider.SetLog(f)
	d.puller.SetLog(f)
}

// IsConfigured reports whether a deploy method has been stored.
func (d *Daemon) IsConfigured() bool { return d.puller.IsConfigured() }

// GetSteps delegates to Puller — satisfies the wizard.New item interface.
func (d *Daemon) GetSteps() []*wizard.Step { return d.puller.GetSteps() }

// ── devwatch.FilesEventHandlers — all delegate to provider ────────

func (d *Daemon) MainInputFileRelativePath() string {
	return d.provider.MainInputFileRelativePath()
}

func (d *Daemon) NewFileEvent(fileName, extension, filePath, event string) error {
	return d.provider.NewFileEvent(fileName, extension, filePath, event)
}

func (d *Daemon) SupportedExtensions() []string { return d.provider.SupportedExtensions() }

func (d *Daemon) UnobservedFiles() []string { return d.provider.UnobservedFiles() }
