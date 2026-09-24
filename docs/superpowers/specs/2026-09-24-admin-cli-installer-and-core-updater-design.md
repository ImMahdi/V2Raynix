# Design Specification: Admin CLI, Installer Engine, and Core Updater

- **Author:** Antigravity / Gemini & Project Architect
- **Date:** 2026-09-24
- **Status:** Approved by User / Ready for Implementation Planning
- **Scope:** Phase 1 of Terminal & System Lifecycle Management (Phase 2 TUI scheduled next)

---

## 1. Overview & Objectives

V2Raynix is designed as a standalone, zero-dependency Linux daemon with policy routing and web management. To make it completely portable and self-healing across any clean Linux server (Debian, Ubuntu, AlmaLinux, Rocky, Arch, Alpine, etc.), this specification defines the architecture for:
1. **Interactive & Scriptable CLI Administration (`v2raynix setup`):** Reset/change credentials, modify web listening port, control systemd service, and inspect network health directly from the terminal without web access.
2. **Password Visibility Toggle:** Show/hide password feature on the web login screen.
3. **Automated Dependency Engine in Installer (`scripts/install.sh`):** Auto-detect and download latest `xray-core` and `tun2socks` matching CPU architecture (`amd64`/`arm64`) with anti-censorship mirror fallbacks.
4. **Core Updater Subsystem (`internal/updater`):** Automated backend release detection for `xray` and `tun2socks`, non-intrusive UI badge indicator in the web header, and safe atomic one-click upgrades with automated rollback on failure.

---

## 2. Architecture & Component Decomposition

```
┌────────────────────────────────────────────────────────────────────────┐
│                          USER TERMINAL                                 │
│                                                                        │
│   $ v2raynix setup [flags]                                             │
│       ├── [1] Change/Reset Admin Username & Password                   │
│       ├── [2] Change Web Panel Port                                    │
│       ├── [3] Service Management (start, stop, restart, status)        │
│       ├── [4] Check & Update Xray / tun2socks Cores                    │
│       └── [5] View System Status & Access URLs                         │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ Direct Store / Systemctl
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                       V2RAYNIX CORE DAEMON                             │
│                                                                        │
│   ┌─────────────────────┐             ┌────────────────────────────┐   │
│   │ Store (Atomic JSON) │             │  internal/updater Subsystem│   │
│   │  /etc/v2raynix/     │             │  - Version extraction      │   │
│   │  v2raynix.json      │             │  - GitHub release polling  │   │
│   └──────────▲──────────┘             │  - Atomic binary swapper   │   │
│              │                        │  - Rollback on corruption  │   │
│              │                        └──────────────▲─────────────┘   │
│              │                                       │                 │
│   ┌──────────┴──────────┐                            │                 │
│   │ HTTP API Router     ├────────────────────────────┘                 │
│   │  /api/system/updates│                                              │
│   │  /api/system/update │                                              │
│   └──────────▲──────────┘                                              │
└──────────────┼─────────────────────────────────────────────────────────┘
               │ JSON / REST
┌──────────────┴─────────────────────────────────────────────────────────┐
│                        WEB MANAGEMENT PANEL                            │
│                                                                        │
│   - LoginPage: Show/Hide Password Eye Toggle                           │
│   - Header: Non-intrusive "⚡ Core Update Available" Badge             │
│   - UpdateCoreModal: Interactive version diff & Live upgrade progress  │
│   - SettingsPage: Core versions card with manual check button          │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Subsystem 1: Terminal Admin Setup (`internal/cli`)

### 3.1 Invocation & Modes
- In [`cmd/v2raynix/main.go`](file:///c:/MaadZone/Github%20Projects/V2Raynix/cmd/v2raynix/main.go), if `os.Args[1] == "setup"`, execution branches directly into `cli.RunSetup(dataDir, os.Args[2:])` and terminates cleanly.
- **Root Privilege Gate:** Checks if running as root (`os.Geteuid() == 0`). If non-root, displays:
  ```text
  [ERROR] v2raynix setup requires root privileges. Please run with sudo.
  ```

### 3.2 Interactive Menu Flow
When run without sub-arguments (`v2raynix setup`), presents an ANSI-colored console interface:
```text
======================================================
             🛡️  V2Raynix Server Setup
======================================================
[1] Change / Reset Admin Username & Password
[2] Change Web Panel Port (Current: <port>)
[3] Service Management (Status / Restart / Stop / Start)
[4] Check & Update Xray / tun2socks Cores
[5] View Current Status & Panel URL
[0] Exit
======================================================
Please select an option [0-5]: 
```

#### Detailed Operations:
1. **Option 1 (User / Password):**
   - Prompts for username (defaults to current).
   - Prompts for password with masked terminal input (using `golang.org/x/term` or standard character hiding).
   - Hashes password with `bcrypt.DefaultCost`.
   - Atomically updates `/etc/v2raynix/v2raynix.json`.
   - Prompts if the user wants to restart `v2raynix.service` now to invalidate existing sessions.
2. **Option 2 (Web Port):**
   - Prompts for new TCP port (1-65535).
   - Checks if port is already occupied via standard local socket binding test.
   - Updates `Settings.WebPort` in store.
   - Restarts `v2raynix.service` so the panel immediately starts listening on the new port.
3. **Option 3 (Service Management):**
   - Sub-menu allowing: Status (`systemctl status v2raynix`), Restart (`systemctl restart v2raynix`), Stop, or Start.
4. **Option 4 (Core Updates):**
   - Triggers the updater logic from terminal, printing progress to stdout.
5. **Option 5 (Status & Access):**
   - Displays primary public/private IP, active web port, service status, and direct login URL (`http://<ip>:<port>`).

### 3.3 Non-Interactive Flags
Supports automation via flags:
- `v2raynix setup --user <username> --pass <password>`
- `v2raynix setup --port <port>`
- `v2raynix setup --service restart|stop|start|status`

---

## 4. Subsystem 2: Installer Dependency Engine (`scripts/install.sh`)

### 4.1 Dependency Detection & Mirrors
In [`scripts/install.sh`](file:///c:/MaadZone/Github%20Projects/V2Raynix/scripts/install.sh):
- **CPU Architecture Detection:**
  - `x86_64` ➔ `amd64` (Xray: `Xray-linux-64.zip`, tun2socks: `tun2socks-linux-amd64.zip`)
  - `aarch64` / `arm64` ➔ `arm64` (Xray: `Xray-linux-arm64-v8a.zip`, tun2socks: `tun2socks-linux-arm64.zip`)
- **Mirror Fallback Strategy:**
  1. Primary: Direct GitHub Releases (`https://github.com/.../releases/download/...`)
  2. Fallback Mirror (for restricted networks / Iran VPS): `https://ghproxy.net/https://github.com/...` or fast reverse proxies.

### 4.2 Helper Functions & Firewall Integration
- `ensure_xray()`: Checks `command -v xray`. If absent:
  - Fetches latest release tag or official asset from GitHub Releases (fallback to anti-censorship mirror `ghproxy.net` if unreachable).
  - Unzips binary and geo data (`geoip.dat`, `geosite.dat`) into `/usr/local/share/xray` and `/usr/local/bin/xray`. (Critical: without geo data, rules like `geosite:ir` fail).
  - Verifies `xray version` succeeds.
- `ensure_tun2socks()`: Checks `command -v tun2socks`. If absent:
  - Downloads release zip from GitHub or mirror.
  - Extracts binary into `/usr/local/bin/tun2socks`.
  - Grants executable permissions (`chmod +x`).
  - Verifies `tun2socks -v` or `tun2socks -version` succeeds.
- **Firewall Integration:**
  - Detects if `ufw` is active: runs `ufw allow 2080/tcp` (or chosen port).
  - Detects if `firewalld` is active: runs `firewall-cmd --add-port=2080/tcp --permanent && firewall-cmd --reload`.

---

## 5. Subsystem 3: Core Updater (`internal/updater`) & API

### 5.1 Structure & Package Interfaces
Located in [`internal/updater/updater.go`](file:///c:/MaadZone/Github%20Projects/V2Raynix/internal/updater/updater.go):

```go
type CoreInfo struct {
    Name            string `json:"name"`             // "xray" or "tun2socks"
    CurrentVersion  string `json:"current_version"`  // e.g. "1.8.24"
    LatestVersion   string `json:"latest_version"`   // e.g. "25.1.30"
    UpdateAvailable bool   `json:"update_available"` // true if latest > current
    BinaryPath      string `json:"binary_path"`      // "/usr/local/bin/xray"
    ReleaseURL      string `json:"release_url"`
}

type UpdateStatus struct {
    Cores          map[string]CoreInfo `json:"cores"`
    LastCheckedAt  time.Time           `json:"last_checked_at"`
    IsChecking     bool                `json:"is_checking"`
    IsUpdating     bool                `json:"is_updating"`
    LastUpdateLog  string              `json:"last_update_log"`
}
```

### 5.2 Version Extraction
- `GetInstalledVersion(coreName string) (string, error)`:
  - For `xray`: Executes `xray -version` and extracts semantic version using regex `Xray (\d+\.\d+\.\d+)`.
  - For `tun2socks`: Executes `tun2socks -v` or `tun2socks -version` and extracts `v?(\d+\.\d+\.\d+)`.

### 5.3 GitHub Release Checking & Caching
- Queries GitHub Releases API:
  - `https://api.github.com/repos/XTLS/Xray-core/releases/latest`
  - `https://api.github.com/repos/xjasonlyu/tun2socks/releases/latest`
- Parses `tag_name` (e.g. `v25.1.30` or `v2.5.2`).
- Compares versions via standard SemVer logic.
- **Rate-Limit & Cache Strategy:**
  - GitHub unauthenticated rate limit is 60 req/hour.
  - Caches status in memory for 6 hours. Manual refresh ("Check for Updates") bypasses cache.
  - If GitHub responds with HTTP 403 (Rate Limited), gracefully logs warning and serves cached data or fallback without breaking the UI.

### 5.4 Safe Atomic Update Routine
When `UpdateCore(coreName string)` is invoked:
1. **State Lock:** Sets `IsUpdating = true` to prevent concurrent updates.
2. **Download Attempt (No Forced Tunnel):**
   - Attempts direct download (with mirror fallback).
   - The user is **NOT** mandated to connect the tunnel beforehand.
   - If download fails (e.g., connection timed out or blocked), aborts cleanly with a descriptive error message: `"Download failed: unable to reach release server. Please verify server internet connectivity or activate the proxy tunnel and try again."`
3. **Backup:** Copies `/usr/local/bin/<core>` to `/usr/local/bin/<core>.bak`.
4. **Binary & GeoData Unpack:**
   - Extracts binary and geo assets (`geosite.dat`, `geoip.dat`) to `/tmp/<core>.new`.
   - Tests standalone execution in `/tmp/` with `--version`.
5. **Atomic Swap (`os.Rename`):**
   - Pauses tunnel briefly if active.
   - Uses `os.Rename` (`mv`) to atomically swap `/tmp/<core>.new` into `/usr/local/bin/<core>` to prevent Linux `ETXTBSY (Text file busy)` errors.
   - Sets executable permissions (`chmod 0755`).
6. **Health Verification:**
   - Runs `/usr/local/bin/<core> -version`.
   - If execution fails or crashes: immediately restores `.bak` file (Automated Rollback).
7. **Post-Update:**
   - Cleans up temporary artifacts.
   - If tunnel was active before update, signals supervisor to reconnect with the new core.

### 5.5 API Endpoints
- `GET /api/system/updates`: Returns current `UpdateStatus`.
- `POST /api/system/check-updates`: Forces an immediate check against GitHub.
- `POST /api/system/update-core`: Body `{"core": "xray" | "tun2socks" | "all"}` triggers the safe update routine.

---

## 6. Subsystem 4: Web UI Enhancements

### 6.1 Password Visibility Toggle
In [`web/src/pages/LoginPage.jsx`](file:///c:/MaadZone/Github%20Projects/V2Raynix/web/src/pages/LoginPage.jsx):
- Adds a toggle button (`type="button"`) inside the password input using `Eye` and `EyeOff` icons from `lucide-react`.
- Toggles state `showPassword` (switching `input type="password"` ↔ `type="text"`).

### 6.2 Update Notification Badge (Header)
In [`web/src/components/Header.jsx`](file:///c:/MaadZone/Github%20Projects/V2Raynix/web/src/components/Header.jsx):
- Checks `/api/system/updates` periodically (every 5 minutes or on mount).
- If any core has `update_available: true`, renders a subtle neon pill badge:
  ```text
  [ ⚡ Core Update Available ]
  ```
- Clicking the badge opens the `UpdateCoreModal`.

### 6.3 Update Confirmation Modal (`UpdateCoreModal.jsx`)
- Displays side-by-side version comparison:
  | Core | Installed | Latest | Status | Action |
  |---|---|---|---|---|
  | Xray-core | `v1.8.24` | `v25.1.30` | ⚠️ Update Available | [Update] |
  | tun2socks | `v2.5.2` | `v2.5.2` | ✅ Up to date | - |
- Action buttons: "Update Selected" or "Update All".
- Displays live status spinner and log message during download and binary verification.

### 6.4 Settings Page Integration
In [`web/src/pages/SettingsPage.jsx`](file:///c:/MaadZone/Github%20Projects/V2Raynix/web/src/pages/SettingsPage.jsx):
- Adds a **Core Engine Versions** card showing installed versions and a **"Check for Updates"** button.

---

## 7. Operational Hardening Rulings

1. **Daemon Cache Synchronization:** Whenever credentials or web port are changed via `v2raynix setup`, the CLI executes `systemctl restart v2raynix` to prevent stale in-memory store states from overwriting disk changes.
2. **Download Independence:** Updates never force tunnel activation. If direct/mirror download fails, a clear actionable error is shown to the user.
3. **Zero-Downtime Atomic Swapping:** Uses Linux `rename()` semantics instead of in-place stream writing to prevent `ETXTBSY`.
4. **GeoData Completeness:** Xray installations always bundle `geosite.dat` and `geoip.dat` in `/usr/local/share/xray` to ensure rules (e.g. `geosite:ir`) resolve without crashes.
5. **Firewall Auto-Permit:** `install.sh` automatically checks and allows the web port in `ufw` and `firewalld`.

---

## 8. Testing & Verification Plan

1. **CLI Tests:**
   - Unit tests for password hashing and credential updates in `internal/cli/setup_test.go`.
   - Unit tests for port validation and store modification.
2. **Updater Tests:**
   - SemVer comparison unit tests in `internal/updater/updater_test.go`.
   - Mock HTTP server tests for GitHub Release parsing and error handling.
   - Atomic swap and rollback simulation tests.
3. **API Integration Tests:**
   - Test endpoints `GET /api/system/updates` and `POST /api/system/update-core` in `internal/api/api_test.go`.
4. **Web Frontend Tests:**
   - Vite build validation (`npm run build`).
5. **Live Server Verification:**
   - Deploy binary to `192.168.254.80`.
   - Run `v2raynix setup` over SSH terminal to test interactive menu.
   - Verify eye toggle on login page.
   - Trigger core version check and inspect UI badge.

