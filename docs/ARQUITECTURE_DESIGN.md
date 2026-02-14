# deploy - Architecture & Design

> **Status**: APPROVED
> **Version**: 1.0.0
> **Last Updated**: 2026-02-14

> ⚠️ **Implementation Status**: DESIGN PHASE — The code described below is not yet implemented.
> The current codebase only contains the `Deploy` struct and `keyring.KeyManager` integration.

## Executive Summary

**deploy** is a lightweight, dependency-free Continuous Deployment (CD) agent designed specifically for **Windows Server 2012**. It enables secure, automated updates for applications directly from GitHub Actions, eliminating the need for complex infrastructure like Docker or Kubernetes on legacy Windows environments.

**Key Features:**
*   **Zero Dependencies**: Distributable as a single Go binary; no runtime required.
*   **Push-Based Architecture**: Triggered by GitHub Actions via a publicly-reachable Endpoint (configurable).
*   **Security-First**: Uses HMAC for request validation and Windows Credential Manager (DPAPI) for secure secret storage.
*   **Resilient Deployment**: Automatic rollback functionality if the new version fails health checks.

---

## 1. System Components

The architecture leverages existing Windows capabilities (Startup Folder, Task Scheduler) and secure tunneling to expose a local updater service to GitHub Actions.

[System Components](./diagrams/COMPONENTS.md)

### High Level Flow

[Process Flow](./diagrams/PROCESS_FLOW.md)

---

## 2. Security Architecture

### Authentication & Secrets
The system enforces a strict "Zero Trust" model for incoming requests and secret management.

#### HMAC Signature Validation
*   **Mechanism**: All requests from GitHub Actions must include an `X-Signature` header containing an HMAC-SHA256 hash of the payload.
*   **Secret**: A shared secret key is stored securely on both ends (GitHub Secrets and Windows Credential Manager).
*   **Protection**: Prevents replay attacks and unauthorized deployment attempts.

**Generating the Shared Secret:**
```bash
# On local machine, generate strong secret
openssl rand -base64 64 | tr -d '\n' > hmac-secret.txt
```

**Mechanism**: All requests from GitHub Actions must include an `X-Signature` header containing an HMAC-SHA256 hash of the payload. Validated against a shared secret using constant-time comparison.

Refer to [IMPLEMENTATION_GUIDE.md](IMPLEMENTATION_GUIDE.md#81-hmac-validation-hmacgo) for the Go implementation.

#### Windows Keyring (DPAPI)
*   **Implementation**: `tinywasm/keyring` library.
*   **Storage**: Secrets are stored in the Windows Credential Manager, encrypted with the user's login credentials.
*   **Secrets Managed**:
    *   `hmac-secret`: Shared secret for validating GitHub webhooks.
    *   `github-pat`: Personal Access Token for downloading assets from private repositories.

> **Note**: `tinywasm/keyring` uses `go-keyring` internally, which is cross-platform:
> Windows (Credential Manager/DPAPI), macOS (Keychain), Linux (Secret Service/D-Bus).

---

## 3. Deployment Workflow

The deployment process is atomic and supports automatic rollback.

[Deployment Workflow](./diagrams/WORKFLOW.md)

### GitHub Actions Configuration

```yaml
# .github/workflows/deploy.yml
name: Deploy to Production

on:
  push:
    branches: [main]
    tags:
      - 'v*'

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Build binary
        run: |
          GOOS=windows GOARCH=amd64 go build -o myapp-service.exe ./cmd/server
      
      - name: Create Release
        id: create_release
        uses: actions/create-release@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          tag_name: ${{ github.ref_name }}
          release_name: Release ${{ github.ref_name }}
          draft: false
          prerelease: false
      
      - name: Upload Release Asset
        id: upload_asset
        uses: actions/upload-release-asset@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          upload_url: ${{ steps.create_release.outputs.upload_url }}
          asset_path: ./myapp-service.exe
          asset_name: myapp-service.exe
          asset_content_type: application/octet-stream
      
      - name: Generate HMAC Signature
        id: hmac
        run: |
          PAYLOAD=$(jq -n \
            --arg repo "${{ github.repository }}" \
            --arg tag "${{ github.ref_name }}" \
            --arg exe "myapp-service.exe" \
            --arg url "${{ steps.upload_asset.outputs.browser_download_url }}" \
            '{repo: $repo, tag: $tag, executable: $exe, download_url: $url}')
          
          SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "${{ secrets.DEPLOY_HMAC_SECRET }}" | sed 's/^.*= //')
          
          echo "payload=$PAYLOAD" >> $GITHUB_OUTPUT
                    echo "signature=sha256=$SIGNATURE" >> $GITHUB_OUTPUT

            - name: Trigger Deployment
                run: |
                    curl -X POST "${{ secrets.DEPLOY_ENDPOINT }}/update" \
                        -H "Content-Type: application/json" \
                        -H "X-Signature: ${{ steps.hmac.outputs.signature }}" \
                        -d '${{ steps.hmac.outputs.payload }}'
```

#### Asset Downloader
Files are downloaded via authenticated HTTP requests using the GitHub Personal Access Token (PAT).

Refer to [IMPLEMENTATION_GUIDE.md](IMPLEMENTATION_GUIDE.md#82-downloader-downloadergo) for the implementation details.

---

## 4. Setup & Configuration

### Interactive Setup Wizard
The first time `deploy.exe` is run, it detects missing secrets and launches an interactive setup wizard to securely provision the Keyring.

[Setup Wizard](./diagrams/SETUP_FLOW.md)

### Configuration File (`config.yaml`)
Located at `d:\apps\deploy\config.yaml`.

```yaml
# d:\apps\deploy\config.yaml
updater:
  port: 8080
  log_level: info
  log_file: d:\apps\deploy\logs\deploy.log
  temp_dir: d:\apps\temp
  
  # Retries on failure
  retry:
    max_attempts: 3
    delay: 5s

apps:
  - name: myapp-service
    version: "1.2.3"
    executable: myapp-service.exe
    path: d:\apps\myapp-service
    port: 1200
    health_endpoint: /health
    health_timeout: 5s
    startup_delay: 3s
    
    # Rollback configuration
    rollback:
      enabled: true
      keep_versions: 1  # Only -older
      auto_rollback_on_failure: true
    
  - name: otra-app
    version: "2.0.0"
    executable: otra-app.exe
    path: d:\apps\otra-app
    port: 3000
    health_endpoint: /health
    health_timeout: 5s
    startup_delay: 2s
    
    rollback:
      enabled: true
      keep_versions: 1
      auto_rollback_on_failure: true
```

#### Automation
The system can automate the creation of Windows Startup shortcuts via PowerShell COM objects.

Refer to [IMPLEMENTATION_GUIDE.md](IMPLEMENTATION_GUIDE.md#83-windows-management-managergo--shortcutgo) for the automation code and scripts.

---

## 5. Process Management Strategy

### "Startup Folder" Consistency
Since the applications are started via the Windows Startup Folder, consistent filenames are critical. 

*   **Constraint**: The executable pointed to by the shortcut (e.g., `myapp-service.exe`) must **always** exist.
*   **Solution**: The updater performs a rename-and-replace strategy:
    1.  Rename current `app.exe` to `app-older.exe`.
    2.  Move new binary to `app.exe`.
This ensures that if the server reboots, the Startup Folder shortcut will always launch the latest deployed version.

### Process Management Implementation
The updater uses `taskkill` for termination and `CREATE_NEW_PROCESS_GROUP` for decoupled execution. 

Refer to [IMPLEMENTATION_GUIDE.md](IMPLEMENTATION_GUIDE.md#83-windows-management-managergo--shortcutgo) for details.

---

## 6. Technical Reference

### Exposing `deploy` to the Internet

`deploy` binds to a local port (default: 8080). GitHub Actions needs a publicly accessible URL.
Configure the `DEPLOY_ENDPOINT` GitHub Secret with the base URL (no trailing slash).

**Required GitHub Secrets:**
| Secret | Description |
|---|---|
| `DEPLOY_HMAC_SECRET` | Shared HMAC-SHA256 secret (min 32 chars) |
| `DEPLOY_ENDPOINT` | Full URL where deploy is reachable (e.g. `https://deploy.example.com`) |

**Options:**

#### Option A: Cloudflare Tunnel (Optional, recommended for Windows Server 2012)
No public IP needed. Cloudflare creates a secure outbound tunnel from the server.
Set `DEPLOY_ENDPOINT` = `https://deploy.yourdomain.com`

Follow the Cloudflare steps below to create `deploy-tunnel` and map `deploy.yourdomain.com` to the tunnel.

#### Option B: Public IP + Port Forwarding
If the server has a static public IP and firewall access:
`DEPLOY_ENDPOINT` = `http://your.server.ip:8080`

#### Option C: Private Network / VPN
If GitHub Actions has network access (self-hosted runner or VPN):
`DEPLOY_ENDPOINT` = `http://10.0.0.5:8080`

#### Cloudflare Tunnel
Recommended for exposing the local endpoint without public IPs.

Refer to [IMPLEMENTATION_GUIDE.md](IMPLEMENTATION_GUIDE.md#84-server-operations) for setup steps.

### Critical Constraints
- **Naming Consistency**: Executable names must match the startup shortcuts.
- **TLS 1.2**: Must be enabled manually on Windows Server 2012 for GitHub API access.
    ```powershell
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    ```
- **Permissions**: User must have Write access to `d:\apps` and Process control rights.
