# PLAN: goflare v0.1.0 Migration + Provider-Agnostic Architecture

> Date: 2026-04-09
> Status: Ready to execute
> Scope: `tinywasm/deploy`, `tinywasm/app`

---

## Goals

1. Fix broken API calls against goflare v0.1.0 (`SetupPages` removed, `Config` fields renamed)
2. Make `tinywasm/deploy` provider-agnostic via a `Provider` interface
3. `tinywasm/app` must never import `goflare` directly — only `deploy`

---

## Breaking Changes in goflare v0.1.0

| Old | New |
|-----|-----|
| `Config.AppRootDir` | removed |
| `Config.RelativeInputDirectory func() string` | `Config.Entry string` |
| `Config.RelativeOutputDirectory func() string` | `Config.OutputDir string` |
| `Config.MainInputFile` | removed (auto-detects `edge/main.go`) |
| `Config.OutputWasmFileName` | removed |
| `(*Goflare).SetupPages(...)` | removed — `DeployPages()` auto-creates project |
| Token key `"CF_PAGES_TOKEN"` | `"goflare/<ProjectName>"` |
| Artifacts: `worker.mjs`, `runtime.mjs`, `wasm_exec.js`, `worker.wasm` | `edge.js`, `edge.wasm` |

---

## Target File Structure

```
deploy/
├── provider.go              # Store interface + Provider interface
├── providers/
│   └── cloudflare/
│       └── provider.go      # Cloudflare implementation (wraps goflare)
├── composer.go              # NewDaemon — provider-agnostic DaemonConfig
├── puller.go                # Provider field replaces Goflare field
├── pusher.go                # Pusher interface — unchanged
├── pusher_pages.go          # DELETE — absorbed into cloudflare/provider.go
├── pusher_worker.go         # DELETE — absorbed into cloudflare/provider.go
└── wizard.go                # unchanged
```

---

## Step 1 — `deploy/provider.go` (new file)

Defines `Store` re-export and the `Provider` interface. Using a local `Store` alias
avoids a circular import when `providers/cloudflare` needs to reference it.

```go
package deploy

// Provider is the interface for a deployment target backend.
// Implementations wrap provider-specific tools (goflare, SSH, etc).
// tinywasm/app depends only on this interface — never on goflare directly.
type Provider interface {
    // Build compiles the project artifacts.
    Build() error

    // Deploy uploads built artifacts to the provider.
    Deploy(store Store) error

    // SetLog injects the application logger.
    SetLog(f func(...any))

    // WizardSteps returns wizard steps to collect provider credentials.
    WizardSteps(store Store, log func(...any)) []*wizard.Step

    // devwatch integration
    MainInputFileRelativePath() string
    NewFileEvent(fileName, extension, filePath, event string) error
    SupportedExtensions() []string
    UnobservedFiles() []string
}
```

---

## Step 2 — `deploy/providers/cloudflare/provider.go` (new file)

Wraps `goflare.Goflare`. Absorbs all logic from `pusher_pages.go` and `pusher_worker.go`.
Does NOT import `"github.com/tinywasm/deploy"` — receives `Store` as the interface
defined locally in the `deploy` package (passed by value, no import needed).

```go
package cloudflare

import (
    "fmt"

    twctx "github.com/tinywasm/context"
    "github.com/tinywasm/goflare"
    "github.com/tinywasm/wizard"
)

type Store interface {
    Get(key string) (string, error)
    Set(key, value string) error
}

// Provider implements deploy.Provider for Cloudflare Workers + Pages.
type Provider struct {
    gf *goflare.Goflare
}

func New(edgeDir, outputDir string) *Provider {
    return &Provider{
        gf: goflare.New(&goflare.Config{
            Entry:     edgeDir,
            OutputDir: outputDir,
        }),
    }
}

func (p *Provider) Build() error            { return p.gf.Build() }
func (p *Provider) Deploy(store Store) error { return p.gf.Deploy(store) }
func (p *Provider) SetLog(f func(...any))   { p.gf.SetLog(f) }

func (p *Provider) MainInputFileRelativePath() string { return p.gf.MainInputFileRelativePath() }
func (p *Provider) NewFileEvent(n, ext, path, ev string) error {
    return p.gf.NewFileEvent(n, ext, path, ev)
}
func (p *Provider) SupportedExtensions() []string { return p.gf.SupportedExtensions() }
func (p *Provider) UnobservedFiles() []string     { return p.gf.UnobservedFiles() }

func (p *Provider) WizardSteps(store Store, log func(...any)) []*wizard.Step {
    p.gf.SetLog(log)
    const ctxAccount = "cf_account"
    const ctxToken   = "cf_token"

    return []*wizard.Step{
        {
            LabelText: "Cloudflare Account ID",
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
                token     := ctx.Value(ctxToken)

                p.gf.Config.ProjectName = input
                p.gf.Config.AccountID   = accountID

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
```

---

## Step 3 — `deploy/puller.go` — replace `Goflare` field with `Provider`

```go
type Puller struct {
    Store      Store
    Process    ProcessManager
    Downloader Downloader
    Checker    HealthChecker
    ConfigPath string
    Provider   Provider   // replaces: Goflare *goflare.Goflare
    log        func(...any)
}
```

In `Puller.Run()`, replace the goflare-specific switch with a direct provider call:

```go
// Before:
switch s := strat.(type) {
case *CloudflarePagesPusher:
    s.Goflare = p.Goflare
case *CloudflareWorkerPusher:
    s.Goflare = p.Goflare
}
return strat.Run(cfg, p)

// After — provider handles build+deploy directly:
if p.Provider != nil {
    // Restore config from store (set during wizard)
    if accountID, err := p.Store.Get("CF_ACCOUNT_ID"); err == nil && accountID != "" {
        // Provider.Deploy reads credentials from store internally via goflare.GetToken
        _ = accountID // goflare reads CF_ACCOUNT_ID from its Config set at wizard time
    }
    return p.Provider.Deploy(p.Store)
}
return strat.Run(cfg, p)
```

Also update `GetSteps()` to delegate to `Provider.WizardSteps`:

```go
func (p *Puller) GetSteps() []*wizard.Step {
    if p.Provider != nil {
        return p.Provider.WizardSteps(p.Store, p.log)
    }
    // fallback: existing wizard.go logic
    return getStepsFromStore(p.Store, p.log)
}
```

---

## Step 4 — `deploy/composer.go` — provider-agnostic `DaemonConfig`

```go
type DaemonConfig struct {
    EdgeDir          string // source dir: "edge" (auto-detected by goflare if empty)
    OutputDir        string // build output: ".build/"
    DeployConfigPath string
    Store            Store
}

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

// Daemon struct
type Daemon struct {
    provider Provider  // was: edgeWorker *goflare.Goflare
    puller   *Puller
}

// EdgeWorker returns the provider for TUI registration (AddHandler).
func (d *Daemon) EdgeWorker() any { return d.provider }

// SetLog injects logger into both components.
func (d *Daemon) SetLog(f func(...any)) {
    d.provider.SetLog(f)
    d.puller.SetLog(f)
}

// devwatch delegation
func (d *Daemon) MainInputFileRelativePath() string { return d.provider.MainInputFileRelativePath() }
func (d *Daemon) NewFileEvent(n, ext, path, ev string) error { return d.provider.NewFileEvent(n, ext, path, ev) }
func (d *Daemon) SupportedExtensions() []string { return d.provider.SupportedExtensions() }
func (d *Daemon) UnobservedFiles() []string     { return d.provider.UnobservedFiles() }
```

---

## Step 5 — `deploy/pusher_pages.go` + `deploy/pusher_worker.go` — DELETE

All Cloudflare-specific logic now lives in `deploy/providers/cloudflare/provider.go`.
Remove both files entirely. Update any `init()` calls that registered these pushers.

---

## Step 6 — `deploy/store.go` — update sensitive key list

Replace `"CF_PAGES_TOKEN"` with `"goflare/*"` pattern awareness, or add the new key format:

```go
// goflare stores tokens as "goflare/<ProjectName>" — mark prefix as sensitive
func isSensitive(key string) bool {
    return sensitiveKeys[key] || strings.HasPrefix(key, "goflare/")
}
```

Replace all `sensitiveKeys[key]` checks with `isSensitive(key)`.

---

## Step 7 — `app/section-deploy.go` — update call site

```go
d := deploy.NewDaemon(&deploy.DaemonConfig{
    EdgeDir:          h.Config.CmdEdgeWorkerDir(),
    OutputDir:        h.Config.DeployEdgeWorkerDir(),
    DeployConfigPath: filepath.Join(h.RootDir, "deploy.yaml"),
    Store:            h.DB,
})
```

`tinywasm/app` never imports `goflare` — only `deploy`. ✅

---

## Execution Order

```
1. deploy/provider.go                      — create Provider interface
2. deploy/providers/cloudflare/provider.go — create Cloudflare impl (absorbs pusher_pages + pusher_worker)
3. deploy/store.go                         — add isSensitive() for "goflare/*" prefix
4. deploy/puller.go                        — replace Goflare field with Provider; update Run() + GetSteps()
5. deploy/composer.go                      — update DaemonConfig + Daemon struct + NewDaemon
6. deploy/pusher_pages.go                  — DELETE
7. deploy/pusher_worker.go                 — DELETE
8. app/section-deploy.go                   — update DaemonConfig call site (EdgeDir, OutputDir)
9. deploy/docs/ARQUITECTURE_DESIGN.md      — update to reflect Provider interface + providers/ structure
10. deploy/docs/DUAL_PURPOSE_SUPPORT.md    — update Cloudflare section to use new provider pattern
11. deploy/docs/IMPLEMENTATION_GUIDE.md    — update DaemonConfig fields + provider setup instructions
```

---

## Adding a new provider in the future

Create `deploy/providers/aws/provider.go` implementing `deploy.Provider`.
In `composer.go`, read `DEPLOY_TARGET` from store to select the provider.
No changes to `puller.go`, `composer.go` core logic, or `tinywasm/app`.
