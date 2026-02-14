# deploy

Automated Continuous Deployment (CD) agent for Windows/Linux Server. Receives webhooks from GitHub Actions via a configurable public Endpoint, downloads releases, performs health checks, and executes automatic rollbacks.


## Documentation

| Document | Description |
|---|---|
| [Architecture & Design](docs/ARQUITECTURE_DESIGN.md) | Executive summary, security model, workflow, and setup |
| [Implementation Guide](docs/IMPLEMENTATION_GUIDE.md) | **[TEMPORARY]** Phase 1 Windows work prompt |
| [System Components](docs/diagrams/COMPONENTS.md) | Directory layout, keyring, and network ports |
| [Process Flow](docs/diagrams/PROCESS_FLOW.md) | High-level deployment flow on Windows Server |
| [Deployment Workflow](docs/diagrams/WORKFLOW.md) | Detailed sequence diagram (GitHub Actions → Windows) |
| [Setup Flow](docs/diagrams/SETUP_FLOW.md) | Interactive first-run setup wizard flow |

## Quick Summary

- **Push-based**: GitHub Actions triggers deployment via POST to a configurable endpoint (set via the `DEPLOY_ENDPOINT` secret)
- **Security**: HMAC-SHA256 request validation + cross-platform keyring for secret storage
- **Resilient**: Automatic rollback if health check fails after deploy
- **Zero-dependency binary**: Single `.exe`, no runtime required

## Dependencies

- [`tinywasm/keyring`](../keyring) — Cross-platform secret storage (DPAPI on Windows, Keychain on macOS, Secret Service on Linux)
