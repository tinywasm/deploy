# PLAN: Migrate to goflare v0.1.0 API

> Date: 2026-04-09
> Status: Ready to execute
> Scope: `deploy/composer.go`, `deploy/pusher_pages.go`, `deploy/tests/pusher_pages_test.go`, `app/section-deploy.go`

---

## Root Cause

`goflare` v0.1.0 replaced the old `client`-based `Config` (with function fields like
`RelativeInputDirectory`, `RelativeOutputDirectory`, etc.) with a plain struct using
`Entry`, `OutputDir`, `ProjectName`, `AccountID`. It also removed `SetupPages()` — the
Pages project is now auto-created inside `DeployPages()` on first upload.

---

## Breaking Changes Summary

| Old API | New API |
|---------|---------|
| `Config.AppRootDir` | removed |
| `Config.RelativeInputDirectory func() string` | `Config.Entry string` |
| `Config.RelativeOutputDirectory func() string` | `Config.OutputDir string` |
| `Config.MainInputFile` | removed (auto `worker/main.go`) |
| `Config.OutputWasmFileName` | removed |
| `(*Goflare).SetupPages(store, accountID, token, project)` | removed — `DeployPages()` auto-creates project |
| Token stored as `"CF_PAGES_TOKEN"` | stored as `"goflare/<ProjectName>"` |

---

## File 1 — `deploy/composer.go`

### Problem
`NewDaemon` passes old Config fields to `goflare.New`:
```go
gw := goflare.New(&goflare.Config{
    AppRootDir:              cfg.AppRootDir,              // ❌ removed
    RelativeInputDirectory:  func() string { return inputDir },  // ❌ removed
    RelativeOutputDirectory: func() string { return outputDir }, // ❌ removed
    MainInputFile:           "main.go",                   // ❌ removed
    OutputWasmFileName:      cfg.OutputWasmFileName,      // ❌ removed
})
```

### Fix — update `goflare.New` call and clean `DaemonConfig`

```go
// DaemonConfig — remove AppRootDir and OutputWasmFileName (no longer forwarded to goflare)
type DaemonConfig struct {
    CmdEdgeWorkerDir    string // → goflare Config.Entry
    DeployEdgeWorkerDir string // → goflare Config.OutputDir
    DeployConfigPath    string
    Store               Store
}

// NewDaemon
func NewDaemon(cfg *DaemonConfig) *Daemon {
    gw := goflare.New(&goflare.Config{
        Entry:     cfg.CmdEdgeWorkerDir,
        OutputDir: cfg.DeployEdgeWorkerDir,
    })
    // ... rest unchanged
}
```

> Note: `app/section-deploy.go` passes `AppRootDir` and `OutputWasmFileName` — these
> fields must be removed from `DaemonConfig` and from the call site in `section-deploy.go`.

---

## File 2 — `app/section-deploy.go`

### Problem
Passes fields that will be removed from `DaemonConfig`:
```go
d := deploy.NewDaemon(&deploy.DaemonConfig{
    AppRootDir:          h.Config.RootDir,           // ❌ remove
    CmdEdgeWorkerDir:    h.Config.CmdEdgeWorkerDir(),
    DeployEdgeWorkerDir: h.Config.DeployEdgeWorkerDir(),
    OutputWasmFileName:  "app.wasm",                 // ❌ remove
    DeployConfigPath:    filepath.Join(h.RootDir, "deploy.yaml"),
    Store:               h.DB,
})
```

### Fix
```go
d := deploy.NewDaemon(&deploy.DaemonConfig{
    CmdEdgeWorkerDir:    h.Config.CmdEdgeWorkerDir(),
    DeployEdgeWorkerDir: h.Config.DeployEdgeWorkerDir(),
    DeployConfigPath:    filepath.Join(h.RootDir, "deploy.yaml"),
    Store:               h.DB,
})
```

---

## File 3 — `deploy/pusher_pages.go`

### Problem
Wizard step 3 calls `cf.SetupPages(store, accountID, bootstrapToken, input)` which
no longer exists. In v0.1.0 `DeployPages` auto-creates the project on first deploy.

The token must be stored with key `"goflare/<ProjectName>"` (not `"CF_PAGES_TOKEN"`).
`AccountID` and `ProjectName` must be set on `cf.Config` before `DeployPages` is called.

### Fix — replace `SetupPages` call with config + token storage

```go
// Step 3: Project name
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
```

> Note: `Puller.Run` injects the goflare instance into `CloudflarePagesPusher`. The goflare
> instance is created in `NewDaemon` with `Entry`/`OutputDir` only. `AccountID` and
> `ProjectName` must be read back from the store when `DeployPages` is called, OR stored
> on `cf.Config` at wizard completion. The simplest approach: after the wizard completes,
> have `Puller.Run` read `CF_ACCOUNT_ID` and `CF_PROJECT` from the store and set them on
> the injected `Goflare.Config` before calling `s.Goflare.DeployPages(p.Store)`.

### Additional fix — `Puller.Run` must populate goflare Config before deploy

In `puller.go`, before `strat.Run(cfg, p)`:
```go
case *CloudflarePagesPusher:
    s.Goflare = p.Goflare
    // Restore Config values from store (set during wizard)
    if accountID, err := p.Store.Get("CF_ACCOUNT_ID"); err == nil {
        s.Goflare.Config.AccountID = accountID
    }
    if project, err := p.Store.Get("CF_PROJECT"); err == nil {
        s.Goflare.Config.ProjectName = project
    }
```

---

## File 4 — `deploy/tests/pusher_pages_test.go`

### Problem
Test expects `SetupPages` to make a live CF API call and fail with
`"Cloudflare Pages setup failed"`. With the new approach, the wizard only stores
credentials — no API call, no error at wizard time.

### Fix — update test to verify credential storage, not API validation

```go
func TestStrategy_PagesSetup(t *testing.T) {
    keyring.MockInit()
    baseStore := NewMockStore()
    store := deploy.NewSecureStore(baseStore)

    p := &deploy.Puller{Store: store}
    steps := p.GetSteps()
    ctx := twctx.Background()

    // Step 0: method = cloudflarePages
    if _, err := steps[0].OnInput("cloudflarePages", ctx); err != nil {
        t.Fatalf("step0 failed: %v", err)
    }
    // Step 1: Account ID
    if _, err := steps[1].OnInput("acc123", ctx); err != nil {
        t.Fatalf("step1 (account) failed: %v", err)
    }
    // Step 2: Token
    if _, err := steps[2].OnInput("this_is_a_bootstrap_token_long_enough", ctx); err != nil {
        t.Fatalf("step2 (token) failed: %v", err)
    }
    // Step 3: Project name — now just stores credentials, no API call
    if _, err := steps[3].OnInput("myproject", ctx); err != nil {
        t.Fatalf("step3 (project) failed: %v", err)
    }

    // Verify token stored in goflare format
    token, err := store.Get("goflare/myproject")
    if err != nil || token == "" {
        t.Errorf("expected token stored as goflare/myproject, got err=%v token=%q", err, token)
    }
    // Verify AccountID and ProjectName stored
    if v, _ := store.Get("CF_ACCOUNT_ID"); v != "acc123" {
        t.Errorf("expected CF_ACCOUNT_ID=acc123, got %q", v)
    }
    if v, _ := store.Get("CF_PROJECT"); v != "myproject" {
        t.Errorf("expected CF_PROJECT=myproject, got %q", v)
    }
}
```

---

## Execution Order

```
1. deploy/composer.go        — update DaemonConfig struct + goflare.New call
2. app/section-deploy.go     — remove deleted DaemonConfig fields
3. deploy/puller.go          — populate goflare.Config from store before CloudflarePagesPusher.Run
4. deploy/pusher_pages.go    — replace SetupPages wizard step with store-based approach
5. deploy/tests/pusher_pages_test.go — update test expectations
```

Steps 1 and 2 are coupled (struct change + call site). Steps 3–5 are independent of 1–2.
All changes are in `tinywasm/deploy` and `tinywasm/app`. No changes needed in `tinywasm/goflare`.
