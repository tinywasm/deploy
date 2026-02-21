# DUAL PURPOSE SUPPORT: CLI & AGENT SPLIT

## 1. Overview

This document outlines the architectural shift to support two distinct operational modes:
1.  **Developer Tool (`cmd/deploy`)**: Runs on the developer's machine to initiate deployments or configure the project.
2.  **Server Agent (`cmd/updater`)**: Runs on the target server to listen for update requests and manage the application process.

This proposal adheres to the strict coding rules defined in [IMPLEMENTATION_GUIDE.md](IMPLEMENTATION_GUIDE.md).

## 2. Architecture

The current single-binary approach will be split into two specialized binaries to separate concerns (Client vs Server).

### 2.1 Component Responsibilities

| Binary | Location | Environment | Responsibility |
|---|---|---|---|
| **Deploy CLI** | `cmd/deploy/main.go` | Developer Machine | Validates project context, configures GitHub Actions, triggers deployment. |
| **Updater Agent** | `cmd/updater/main.go` | Target Server (Windows) | Listens for webhooks, validates HMAC, performs atomic updates and rollbacks. |

### 2.2 Directory Structure Impact

```
cmd/
├── deploy/      # Developer CLI
│   └── main.go
└── updater/     # Server Agent (New)
    └── main.go
```

Shared logic (Config, HMAC, Downloader) remains in the root `deploy/` package but will be imported by both binaries as needed.

## 3. Detailed Workflows

### 3.1 Developer CLI (`cmd/deploy`)

The CLI acts as the entry point for developers. It intelligently detects the project state.

**Logic Flow:**
1.  **Check Context**: Verify execution is within a valid Go project root.
    -   Must contain `go.mod`.
    -   Must be a Git repository (`.git/` exists).
2.  **Check Configuration**:
    -   Verify existence of `.github/workflows/deploy.yml`.
3.  **Action**:
    -   **If Configured**: Initiate deployment process (e.g., trigger git push or manual workflow dispatch).
    -   **If Missing**: Launch the **Setup Wizard**.
        -   Prompt for Server IP/Port.
        -   Prompt for Secrets (HMAC, GitHub PAT).
        -   Generate `.github/workflows/deploy.yml`.
        -   Generate `config.yaml` for the server.

**Reference Diagram:**
[CLI Workflow Diagram](diagrams/CLI_WORKFLOW.md)

### 3.2 Server Agent (`cmd/updater`)

The Agent is a long-running process on the Windows Server.

**Logic Flow:**
1.  **Startup**: Load `config.yaml`, initialize `KeyManager` (Windows Credential Manager).
2.  **Listen**: Start HTTP server on configured port (default 8080).
3.  **Handle Update**:
    -   Validate `X-Signature` using HMAC.
    -   Download new binary to temporary location.
    -   **Stop** current application process.
    -   **Backup** current binary to `app-failed.exe`.
    -   **Swap** new binary to `app.exe`.
    -   **Start** new application process.
    -   **Health Check**: Query `/health`.
        -   If OK: Respond 200 to GitHub.
        -   If Fail: **Rollback** (Restore backup, restart old process), Respond 500.

**Reference Diagram:**
[Server Agent Workflow Diagram](diagrams/UPDATER_WORKFLOW.md)

## 4. Implementation Plan

### Phase 1: Refactoring

1.  **Create `cmd/updater/`**:
    -   Move the server listening logic (`http.ListenAndServe`, `Handler`) from `deploy.go` to `cmd/updater/main.go`.
    -   Ensure `updater` imports the necessary shared components (`hmac.go`, `downloader.go`, `manager.go`).

2.  **Refactor `cmd/deploy/`**:
    -   Remove server listening logic.
    -   Implement `checkContext()` function (Check `go.mod`, `.git`).
    -   Implement `checkWorkflow()` function.
    -   Update `Wizard` to generate `.github/workflows/deploy.yml`.

3.  **Shared Library Adjustments**:
    -   Ensure `deploy` package exposes reusable functions for both CLI and Agent without tightly coupling them to a specific `main` execution.

## 5. Coding Standards Compliance

-   **File Structure**: All shared logic remains in root `deploy/`. New specific logic for CLI or Agent stays in `cmd/` or designated files (e.g., `cli_check.go` in root if shared, or kept in `cmd` if exclusive).
-   **Testing**: New integration tests must be added to `tests/` covering the CLI detection logic (mocking file system) and the Updater's listening loop.
-   **Dependencies**: No external libraries for testing. Use `os` and `testing` packages.
