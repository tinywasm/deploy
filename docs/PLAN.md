# Plan — `deploy` as the single deployment package, with injected executors

> **STATUS: NOT DISPATCHED — recorded, deliberately deferred (2026-07-12).**
> The owner's attention is on making `goflare` work end-to-end first (D1 + router +
> file uploads in `goflare-demo`). This plan exists so the design is not lost, not
> because it is scheduled. **Do not dispatch it without an explicit go-ahead.**

## Goal

One package owns *deployment* as a concept: `tinywasm/deploy`. It holds the **logic**
(what a deployment is, its lifecycle, its wizard, its health checks) and **not** the
knowledge of any particular target. The concrete **executor** is *injected at mount
time* by the host.

`tinywasm/app` mounts whichever executors are available, so a developer — or an LLM
through MCP — can list them and pick one:

```
tinywasm deploy --list
  localServer   deploy to a server you administer (puller agent, health check, rollback)
  cloudflare    deploy to Cloudflare (Pages Functions / Worker artifacts)
```

## Why the current shape does not deliver this

The registry is already there — `Pusher`, `RegisterPusher`, `GetPusher`,
`AvailablePushers` in [pusher.go](../pusher.go). Two things prevent it from being
dependency injection:

1. **Registration happens via `init()` *inside* `deploy`.** `pusher_cloudflare.go` and
   `pusher_ssh.go` call `RegisterPusher` from their own `init()`, which means `deploy`
   **imports its own executors**. That is the inverse of injection: `deploy` depends on
   the targets instead of the targets depending on `deploy`. It also means every
   consumer of `deploy` links every executor, whether or not it will ever use it.

2. **The interface leaks the local-server model.** `Run(cfg *Config, p *Puller) error`
   forces every executor to accept a `Puller` — an agent that pulls an artifact onto a
   machine you administer, health-checks it and rolls back. That concept is meaningless
   for an edge deploy, which is an *upload* to an API. This mismatch is why
   `CloudflarePagesPusher` and `CloudflareWorkerPusher` are **empty stubs** (`Run` does
   nothing, `WizardSteps` returns `nil`) while the working implementation lives, 487
   lines of it, in `goflare/cloudflare.go`. The shape did not fit, so nobody filled it.

## Changes

1. **Delete the Cloudflare stubs.** `pusher_cloudflare.go` goes away. Confirmed with the
   owner: they are leftovers of an idea, not a commitment. The real implementation is in
   `goflare` and will be adapted to the new contract (see change 4).

2. **Split the contract so the local-server model stops leaking.** `deploy` defines an
   `Executor` that any target can satisfy without knowing what a `Puller` is:

   ```go
   // package deploy
   type Executor interface {
       Name() string        // "localServer", "cloudflare" — the identifier a human/LLM picks
       Describe() string    // one line, shown by `--list` and by MCP
       Deploy(a Artifact) error
       WizardSteps(store Store, log func(...any)) []*wizard.Step
   }
   ```

   `Artifact` is what the build produced (paths + metadata) — deliberately *not* a
   `Puller`. The puller/health-check/rollback machinery stays inside the local-server
   executor, which is the only one that needs it.

3. **Invert the registration: the host injects, `deploy` does not import.** Remove the
   `init()`-based self-registration. The host wires the set explicitly:

   ```go
   // in tinywasm/app
   deploy.Register(localserver.New())  // deploy's own subpackage
   deploy.Register(goflare.Executor()) // injected from outside — deploy never imports goflare
   ```

   `deploy` keeps `AvailableExecutors() []Executor` — that is the list MCP exposes, and
   it now reflects **what the host mounted**, not what happened to be compiled in.

4. **Executors deploy owns live as subpackages; foreign ones come from their own repo.**
   `deploy/executors/localserver` (the puller model, SSH, webhook). The Cloudflare
   executor is supplied *by `goflare`* — it is goflare that knows the Cloudflare API,
   and it already has the code. `deploy` never grows a Cloudflare dependency.

## Acceptance criteria

- `deploy`'s `go.mod` requires **no** target-specific dependency: no `goflare`, no
  Cloudflare client. A dependency graph check proves it.
- No `init()` in `deploy` registers anything. Registration only happens when a host calls
  `deploy.Register(...)`.
- `AvailableExecutors()` on a host that mounted nothing returns empty — not a list of
  accidentally-linked executors.
- No `Puller` appears in the `Executor` interface or in any signature an edge executor
  must satisfy.
- `pusher_cloudflare.go` no longer exists, and no stub `Run` returns `nil` while
  pretending to deploy.

## Sequencing (why this is deferred)

This plan **depends on `goflare` being settled first**, not the other way round: the
Cloudflare executor is extracted *from* `goflare/cloudflare.go`, so it should be moved
once, after goflare's own restructuring, rather than moved twice. The immediate work is
`goflare` + `goflare-demo` (D1, router, file uploads):

https://github.com/tinywasm/goflare/blob/main/docs/PLAN.md
