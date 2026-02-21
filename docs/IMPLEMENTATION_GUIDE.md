# IMPLEMENTATION GUIDE (Phase 1 - Windows)

## 0. Development Rules (Compliance)

> **Mandatory Rules derived from `DEFAULT_LLM_SKILL.md` for this Backend/CLI project.**

### 0.1 Strict File Structure
- **Flat Hierarchy**: ALL files must be in the root `deploy/` package (except `cmd/` and `docs/`). No subfolders for logic.
- **Max 500 Lines**: If a file exceeds 500 lines, split it by domain (e.g., `manager.go` -> `manager_windows.go`, `manager_linux.go`).
- **Test Organization**: Since we expect >5 test files, **ALL** tests must be moved to a `tests/` subdirectory (e.g., `deploy/tests/`).

### 0.2 Diagram-Driven Testing (DDT)
- **Flow Coverage**: Every logic flow in `docs/diagrams/*.md` (e.g. `PROCESS_FLOW.md`) **MUST** have a corresponding Integration Test.
- **Branch Coverage**: If a diamond node `{Decision}` exists, tests must cover both `Yes` and `No` paths.
- **Failure Modes**: Network errors, timeouts, and busy states depicted in diagrams must be simulated in tests.

### 0.3 Zero External Libraries (Testing)
- **Constraint**: **NEVER** use `testify` or `gomega`. Use only `testing`, `net/http/httptest`, `reflect`.

### 0.4 Mandatory Dependency Injection & Mocking
- **No Global State**: `deploy.go` and `handler.go` must NOT call `exec.Command` or `http.Get` directly.
- **Interfaces**: Use `ProcessManager`, `Downloader`, `KeyManager` interfaces.
- **Mocking**: Tests must use mocks for all external interactions.

---

> **IMPORTANT**: This is a temporary implementation guide designed to serve as a work prompt for Phase 1. For the complete system architecture, refer to [ARQUITECTURE_DESIGN.md](ARQUITECTURE_DESIGN.md).

## 1. Scope
- **OS Support**: Windows Server 2012+ only for Phase 1.
- **Distribution**: Standalone binary `deploy.exe`.
- **Trigger**: Webhook-based (POST `/update`) from GitHub Actions.
- **Process Lifecycle**: Managed via Windows Startup Folder.
- **Phase 2 Preview**: Linux support via `systemd` will be documented in a separate guide.

## 2. Implementation Plan (Structure & Order)

**External Dependencies**: `gopkg.in/yaml.v3` (for `config.yaml`).

Implement components in this strict order. All logic files reside in the root `deploy/` package (Flat Hierarchy), tests in `tests/`, and the entry point in `cmd/deploy/`.

| Order | File (Root `deploy/`) | Responsibility |
|---|---|---|
| 1. | `config.go` | Define `Config`, `AppConfig`, and `Load(path)` logic. |
| 2. | `hmac.go` | Implement `HMACValidator` (Architecture §2). |
| 3. | `checker.go` | Implement logic to parse `status` and `can_restart` (Architecture §5). |
| 4. | `downloader.go` | Implement `HTTPDownloader` with Bearer Auth (Architecture §3). |
| 5. | `manager.go` | Implement `WindowsManager` with `taskkill` and `HIDE_WINDOW` (Architecture §5). |
| 6. | `shortcut.go` | Implement Windows Shortcut creation (Architecture §4). |
| 7. | `handler.go` | Orchestrate Security -> Download -> Process Control. |
| 8. | `wizard.go` | Interactive TUI for setup (Architecture §4). |
| 9. | `deploy.go` | Main `Run()` loop: Check Config ? Wizard : Server. |
| 10. | `cmd/deploy/main.go` | **Entry Point**: Inject dependencies (`RealManager`, `RealKeyring`) and start. |
| - | `tests/*_test.go` | Unit & Integration tests (moved to `tests/` subdirectory). |

## 5. Key Contracts

### Configuration
See [config.go](../config.go)

### Dependency Injection (Testability)
See interfaces in [deploy.go](../deploy.go) and [downloader.go](../downloader.go) and [manager.go](../manager.go).

**Implementation Strategy:**
- **Production (`cmd/deploy/main.go`)**: 
    - Inject `keyring.New()` from `tinywasm/keyring`.
    - Inject real `NewProcessManager()` (Windows specific).
    - Inject real `NewDownloader()` (net/http).
- **Testing**: 
    - **MUST** use a **Mock** implementation for ALL interfaces.
    - `MockProcessManager`: Records "started" or "stopped" calls in a slice/map; does NOT execute OS commands.
    - `MockDownloader`: Simulates file creation or network errors; does NOT make HTTP requests.

### Execution Flow (`Deploy.Run`)
See [deploy.go](../deploy.go) logic.

### Update Handler (`POST /update`)
The full orchestration flow is detailed in [PROCESS_FLOW](diagrams/PROCESS_FLOW.md) and [WORKFLOW](diagrams/WORKFLOW.md). The handler must implement the **Polling Loop** for busy applications, retrying the health check until `can_restart: true` or the `BusyTimeout` is reached.

## 6. OS Abstraction (Future Proofing)
Define a `ProcessManager` interface in `manager.go`. The Windows implementation (within the same or a separate file like `manager_windows.go`) must use `syscall.SysProcAttr{HideWindow: true}`. Linux support will be added later using build tags (`//go:build windows` and `//go:build linux`) and signals (`SIGTERM`).

## 7. Testing Strategy

> **CRITICAL**: Following the project's CORE principles, **external libraries are NOT allowed** for testing. Use only the Go Standard Library.

To ensure reliability and correctness of the deployment flow, the following tests must be implemented.

### 7.1 Unit Tests

Focus on logic isolation without external dependencies:

| Component | Test Case | Expected Behavior |
|---|---|---|
| **KeyManager** | `TestMock_GetSet` | Verify in-memory mock works (Crucial: Do NOT use real keyring). |
| **ProcessManager** | `TestMock_StartStop` | Verify mock records calls without executing OS commands. |
| **Downloader** | `TestMock_Download` | Verify mock simulates success/failure without network. |
| **HMACValidator** | `TestValidate_ValidSignature` | Signature matches payload and secret. |
| | `TestValidate_InvalidSignature` | Returns `signature mismatch` error. |
| | `TestValidate_MalformedFormat` | Returns `invalid signature format` error. |
| **Config** | `TestLoad_Success` | Correctly parses `config.yaml` into structs. |
| | `TestLoad_Defaults` | Applies default port (8080) if missing. |
| **Checker** | `TestCheck_Healthy` | Parses `status: "ok"` and `can_restart: true`. |
| | `TestCheck_Busy` | Parses `can_restart: false` (triggers 503). |
| **Downloader** | `TestDownload_Auth` | Retries with Bearer token header. |

### 7.2 Integration Tests (Flow Verification)

Use `httptest` to mock GitHub and local app health endpoints to verify the flow in [PROCESS_FLOW.md](diagrams/PROCESS_FLOW.md) and [SETUP_FLOW.md](diagrams/SETUP_FLOW.md). **Crucially**, tests must cover every branch of these diagrams:

#### PROCESS_FLOW Coverage
1.  **Happy Path Update**:
    - Mock GitHub valid HMAC signature.
    - Mock local app responding `200 OK`.
    - `MockProcessManager` asserts: `Stop(old)` called, `Start(new)` called.
    - **Verify**: Previous app killed (virtual), new binary moved (virtual/temp), new process started (virtual).
2.  **Unauthorized Request**:
    - Send invalid `X-Signature`.
    - **Verify**: Response `401 Unauthorized`, no download triggered.
3.  **Deployment Blocked (Busy)**:
    - Mock local app responding with `can_restart: false`.
    - **Verify**: Response `503 Service Unavailable`, app is NOT killed.
4.  **Automatic Rollback**:
    - Mock new version starting but failing POST-deploy health check (timeout/error).
    - **Verify**: `app-older.exe` restored to `app.exe`, previous process restarted, response `500 Internal Server Error`.

#### SETUP_FLOW Coverage (Interactive Wizard)
1.  **Missing Config**:
    - Run `deploy.exe` without `config.yaml` or keys.
    - **Verify**: Wizard launches (`IsConfigured() == false`).
2.  **Secret Provisioning**:
    - Complete wizard steps (HMAC secret, GitHub PAT).
    - **Verify**: Secrets saved to MockKeyring, `config.yaml` created.

### 7.3 Manual / OS Specific Verification

Since some behaviors depend on Windows internals:

- **Shortcut Verification**: Verify `.lnk` file is created correctly in the Startup folder with correct working directory.
- **Process Visibility**: Ensure `tasklist` shows the processes but no GUI/Console windows appear.
- **File Access**: Verify the rename strategy works correctly while the process is being terminated (no "Access Denied").

## 8. Reference Implementation

### 8.1 HMAC Validation
See [hmac.go](../hmac.go)

### 8.2 Downloader
See [downloader.go](../downloader.go)

### 8.3 Windows Management
See [manager_windows.go](../manager_windows.go) and [shortcut_windows.go](../shortcut_windows.go)

## 9. RFC: Dual Purpose Support (Client vs Server)

This section analyzes the proposal to split the application into two distinct binaries: a developer-side CLI and a server-side Agent.

### 9.1 Questions & Considerations

1.  **Authentication**: How will the CLI authenticate with the Server?
    *   *Current*: Webhook uses HMAC.
    *   *Proposal*: CLI generates the HMAC secret during the Wizard setup and stores it in the repository secrets (via GH Actions) and `config.yaml` on the server. The CLI itself doesn't talk directly to the server; it talks to GitHub (via `git push` or Action trigger), which then talks to the server.
2.  **State Management**:
    *   Server needs to know it's "configured".
    *   CLI needs to know if the project is "configured".
    *   *Solution*: CLI checks for `.github/workflows/deploy.yml` existence as a proxy for configuration.
3.  **Code Sharing**:
    *   Both binaries share `hmac.go`, `config.go`, etc.
    *   *Constraint*: Must maintain flat structure in `deploy/`.
    *   *Solution*: `cmd/deploy/main.go` and `cmd/updater/main.go` will both import from `github.com/tinywasm/deploy`.

### 9.2 Suggestions

*   **CLI Library**: Consider using `cobra` or `urfave/cli` for the `cmd/deploy` tool if flags become complex. For now, standard `flag` package is sufficient as per "Zero External Libraries" preference, though not strictly forbidden for the CLI part if needed.
*   **Version Control**: The CLI should verify it's on the `main` branch (or configured production branch) before allowing a deploy trigger to prevent accidental deployments of feature branches.

### 9.3 Pros & Cons

| Strategy | Pros | Cons |
|---|---|---|
| **Single Binary (Current)** | Easier distribution (one file). | Confusing usage (flag switches). Bloated binary (server code on dev machine). |
| **Dual Binary (Proposed)** | Clear separation of concerns. Smaller attack surface on server. specialized dependencies. | Two build artifacts. Slightly more complex build pipeline. |

### 9.4 Best Option / Recommendation

**Adopt the Dual Binary Strategy.**

The clear separation between "Developer Tools" and "Runtime Agent" aligns best with security best practices and usability.
*   `cmd/deploy`: Focuses on UX, wizard, and git integration.
*   `cmd/updater`: Focuses on stability, uptime, and process management.

See [DUAL_PURPOSE_SUPPORT.md](DUAL_PURPOSE_SUPPORT.md) for the detailed implementation plan.
