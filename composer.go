// Daemon composes the goflare Edge Worker compiler with the Puller orchestrator
// into a single unit for tinywasm/app. app imports only tinywasm/deploy.
package deploy

import (
	"github.com/tinywasm/goflare"
	"github.com/tinywasm/wizard"
)

// DaemonConfig holds all configuration needed to initialise both goflare and Puller.
type DaemonConfig struct {
	AppRootDir          string
	CmdEdgeWorkerDir    string // relative input dir for edge worker source
	DeployEdgeWorkerDir string // relative output dir for compiled artifacts
	OutputWasmFileName  string // e.g. "app.wasm"
	DeployConfigPath    string // path to deploy.yaml
	Store               Store
}

// Daemon composes the goflare Edge Worker handler with the Puller orchestrator.
type Daemon struct {
	edgeWorker *goflare.Goflare
	puller     *Puller
}

// NewDaemon creates a Daemon with fully-initialised goflare and Puller internals.
func NewDaemon(cfg *DaemonConfig) *Daemon {
	inputDir := cfg.CmdEdgeWorkerDir
	outputDir := cfg.DeployEdgeWorkerDir

	gw := goflare.New(&goflare.Config{
		AppRootDir:              cfg.AppRootDir,
		RelativeInputDirectory:  func() string { return inputDir },
		RelativeOutputDirectory: func() string { return outputDir },
		MainInputFile:           "main.go",
		OutputWasmFileName:      cfg.OutputWasmFileName,
	})

	puller := &Puller{
		Store:      NewSecureStore(cfg.Store),
		Process:    NewProcessManager(),
		Downloader: NewDownloader(),
		Checker:    NewChecker(),
		ConfigPath: cfg.DeployConfigPath,
		Goflare:    gw,
	}

	return &Daemon{edgeWorker: gw, puller: puller}
}

// EdgeWorker returns the goflare handler for TUI registration (AddHandler).
func (d *Daemon) EdgeWorker() any { return d.edgeWorker }

// Puller returns the puller handler for TUI registration (AddHandler).
func (d *Daemon) Puller() any { return d.puller }

// SetLog injects a logger into both components.
func (d *Daemon) SetLog(f func(...any)) {
	d.edgeWorker.SetLog(f)
	d.puller.SetLog(f)
}

// IsConfigured reports whether a deploy method has been stored.
func (d *Daemon) IsConfigured() bool { return d.puller.IsConfigured() }

// GetSteps delegates to Puller — satisfies the wizard.New item interface.
func (d *Daemon) GetSteps() []*wizard.Step { return d.puller.GetSteps() }

// ── devwatch.FilesEventHandlers — all delegate to edgeWorker (goflare) ────────

func (d *Daemon) MainInputFileRelativePath() string {
	return d.edgeWorker.MainInputFileRelativePath()
}

func (d *Daemon) NewFileEvent(fileName, extension, filePath, event string) error {
	return d.edgeWorker.NewFileEvent(fileName, extension, filePath, event)
}

func (d *Daemon) SupportedExtensions() []string { return d.edgeWorker.SupportedExtensions() }

func (d *Daemon) UnobservedFiles() []string { return d.edgeWorker.UnobservedFiles() }
