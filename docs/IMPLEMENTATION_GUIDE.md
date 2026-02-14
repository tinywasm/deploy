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
```go
type Config struct {
    Port     int        // default: 8080
    LogLevel string
    LogFile  string
    TempDir  string
    Apps     []AppConfig
}

type AppConfig struct {
    Name           string
    Version        string
    Executable     string
    Path           string
    Port           int
    HealthEndpoint     string
    HealthTimeout      time.Duration
    StartupDelay       time.Duration
    BusyRetryInterval  time.Duration // default: 10s
    BusyTimeout        time.Duration // default: 5m
    Rollback           RollbackConfig
}
```

### Dependency Injection (Testability)

To ensure robust testing without side effects (executing processes, network calls, system keyring), all external interactions must be decoupled via interfaces.

```go
// defined in deploy.go or specific files
type KeyManager interface {
    Get(service, user string) (string, error)
    Set(service, user, password string) error
}

type ProcessManager interface {
    Start(exePath string) error
    Stop(exeName string) error
    // potentially CheckHealth, etc.
}

type Downloader interface {
    Download(url, dest string, token string) error
}

// Deploy struct must accept these interfaces
type Deploy struct {
    Keyring    KeyManager
    ProcMan    ProcessManager
    Downloader Downloader
    // ... other dependencies
}
```

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
1. Attempt to load `config.yaml` (create default if missing).
2. If `Keys.IsConfigured()` is false, execute `setup.NewWizard().Run()`.
3. Check for `--admin` flag to run the admin menu (Phase 2 feature).
4. Launch the HTTP server on configured port.

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

Below are the base implementations for key components. These follow the interfaces defined in §5.

### 8.1 HMAC Validation (`hmac.go`)
```go
package deploy

import (
    "crypto/hmac"
    "crypto/sha256"
    "crypto/subtle"
    "encoding/hex"
    "fmt"
    "strings"
)

type HMACValidator struct {
    secret []byte
}

func NewHMACValidator(secret string) *HMACValidator {
    return &HMACValidator{secret: []byte(secret)}
}

func (v *HMACValidator) ValidateRequest(payload []byte, signature string) error {
    if !strings.HasPrefix(signature, "sha256=") {
        return fmt.Errorf("invalid signature format")
    }
    providedSig := strings.TrimPrefix(signature, "sha256=")
    providedBytes, err := hex.DecodeString(providedSig)
    if err != nil {
        return fmt.Errorf("invalid hex signature: %w", err)
    }
    
    mac := hmac.New(sha256.New, v.secret)
    mac.Write(payload)
    if subtle.ConstantTimeCompare(providedBytes, mac.Sum(nil)) != 1 {
        return fmt.Errorf("signature mismatch")
    }
    return nil
}
```

### 8.2 Downloader (`downloader.go`)
```go
package deploy

import (
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "time"
)

type HTTPDownloader struct {
    client *http.Client
}

func NewDownloader() *HTTPDownloader {
    return &HTTPDownloader{client: &http.Client{Timeout: 10 * time.Minute}}
}

func (d *HTTPDownloader) Download(url, dest, token string) error {
    if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
        return err
    }
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Accept", "application/octet-stream")
    
    resp, err := d.client.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("download failed: %d", resp.StatusCode)
    }
    
    out, err := os.Create(dest)
    if err != nil { return err }
    defer out.Close()
    _, err = io.Copy(out, resp.Body)
    return err
}
```

### 8.3 Windows Management (`manager.go` / `shortcut.go`)

#### Process Manager
```go
package deploy

import (
    "os/exec"
    "syscall"
    "time"
)

type WindowsManager struct{}

func (m *WindowsManager) Stop(exeName string) error {
    cmd := exec.Command("taskkill", "/F", "/IM", exeName)
    cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
    return cmd.Run() // Ignore "not found" error in actual implementation logic
}

func (m *WindowsManager) Start(exePath string) error {
    cmd := exec.Command(exePath)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        HideWindow:    true,
        CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
    }
    return cmd.Start()
}
```

