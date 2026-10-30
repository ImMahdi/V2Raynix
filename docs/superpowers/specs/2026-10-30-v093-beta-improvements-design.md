# Specification: V2Raynix v0.9.3-beta Quality of Life & Architecture Improvements

**Author:** V2Raynix Core Team  
**Date:** 2026-10-30  
**Target Version:** `v0.9.3-beta`  
**Status:** Approved for Implementation Planning  

---

## 1. Overview & Motivation

Following the successful release and real-world deployment of `v0.9.2-beta` with Full Terminal User Interface (TUI), practical field testing on production Linux nodes and reverse-proxied domains highlighted several critical usability gaps, reverse-proxy timeout issues during engine upgrades, and missing CLI automation features:

1. **Unused Metrics Noise:** Traffic speed tracking is currently stubbed as `0 B/s`, creating unnecessary visual clutter in both TUI headers and web cards.
2. **Safe Mode Disconnect Surprises:** When connecting via TUI, users were dropped back to the main menu without prominent Safe Mode confirmation prompts, leading to unintentional rollbacks after 120 seconds.
3. **TUI Input Traps:** Several input prompts (e.g. adding configs, entering rules) lacked explicit cancel/back instructions, forcing users to use `Ctrl+C` to abort.
4. **Core Engine Upgrade Timeouts:** Upgrading Xray/Sing-box failed with `context deadline exceeded` due to hardcoded 5s and 10s HTTP client timeouts across `bridge.go` and `web/src/services/api.js`, which also breaks under standard reverse proxies (Nginx / Cloudflare).
5. **Redundant Dashboard Badges:** The "SSH Protection - Anti-Lockout Active" card in the web UI consumed valuable dashboard real estate without actionable utility.
6. **Mobile Navigation Usability:** The mobile dropdown menu slid down below the header, which felt awkward on vertical/mobile devices compared to a modern slide-in side drawer.
7. **Lack of Headless CLI Automation:** Administrators managing servers via Ansible, bash scripts, or quick SSH commands lacked direct CLI arguments (e.g., `v2raynix connect [node] [--force]`).
8. **Brand Identity Consistency:** The navbar header displayed a generic `ShieldCheck` icon instead of the official V2Raynix lightning bolt (`Zap`) icon featured in the project favicon.

---

## 2. Categorization & Scope

The improvements for `v0.9.3-beta` are organized into four distinct technical categories:

| Category | Component Scope | Key Objectives |
| :--- | :--- | :--- |
| **Category 1: Web UI Polish** | `web/src/views/DashboardView.jsx`<br/>`web/src/components/Navbar.jsx`<br/>`web/src/index.css` | - Remove Anti-Lockout card.<br/>- Convert top-sliding mobile menu to full-height side drawer.<br/>- Replace navbar icon with brand lightning logo (`Zap`). |
| **Category 2: TUI UX & Guard** | `internal/cli/tui_layout.go`<br/>`internal/cli/menu_tunnel.go`<br/>`internal/cli/menu_configs.go`<br/>`internal/cli/menu_routing.go`<br/>`internal/cli/menu_settings.go` | - Remove stubbed `Traffic: ▲ 0 B/s ▼ 0 B/s` from header.<br/>- Implement interactive **Safe Mode Guard Screen** immediately after connection.<br/>- Add clear cancel/back instructions to all input prompts. |
| **Category 3: Core & Updater Resilience** | `internal/cli/bridge.go`<br/>`internal/updater/updater.go`<br/>`internal/api/router.go`<br/>`web/src/services/api.js`<br/>`web/src/components/UpdateCoreModal.jsx` | - Increase client timeout for updater endpoints to 180s.<br/>- Support non-blocking background upgrade with state polling (`/api/system/updates`) to survive reverse proxy drops. |
| **Category 4: Headless CLI Automation** | `cmd/v2raynix/main.go`<br/>`internal/cli/commands.go`<br/>`internal/cli/bridge.go` | - Implement direct argument commands: `connect [--force]`, `disconnect`, `status`, `health`, `list`, `confirm`, `rollback`. |

---

## 3. Detailed Specifications

### 3.1 Category 1: Web UI Enhancements

#### 3.1.1 Anti-Lockout Card Removal
- In `web/src/views/DashboardView.jsx`, remove the `SSH Protection / Anti-Lockout Active` card block (lines 316–351).
- The remaining cards (`Health Status`, `Active Config`, `Tunnel Uptime`) will cleanly distribute across the responsive grid (`repeat(auto-fit, minmax(240px, 1fr))`).

#### 3.1.2 Side Drawer for Mobile Navigation
- Replace the top-dropdown drawer (`top: 64px`, `animation: slideDown`) with a sliding side drawer:
  - Width: `280px`, max-width: `85vw`, height: `100vh`, fixed position at `top: 0`, `right: 0` (or `left: 0` based on layout).
  - Background: `rgba(15, 23, 42, 0.98)` with `backdrop-filter: blur(16px)` and deep shadow.
  - Smooth animation: `transform: translateX(100%)` to `translateX(0)`.
  - Backdrop: `position: fixed; inset: 0; background: rgba(0, 0, 0, 0.7);` closing on click.
  - Header in drawer: close button (`X`), brand logo, and user profile badge.

#### 3.1.3 Brand Logo in Navbar
- In `web/src/components/Navbar.jsx`, replace the `<ShieldCheck size={22} color="#ffffff" />` with the official brand lightning icon (`Zap`) styled with a glowing gradient badge matching the SVG favicon.

---

### 3.2 Category 2: TUI UX & Interactive Guard

#### 3.2.1 Traffic Stats Removal
- In `internal/cli/tui_layout.go`:
  - In `RenderStatusHeader(info LiveStatusInfo)`, remove the line:
    `trafficStr := fmt.Sprintf("Traffic: ▲ %s  ▼ %s", FormatSpeed(info.UploadSpeed), FormatSpeed(info.DownloadSpeed))`
  - The status header box height reduces by 1 row, leaving clean, accurate operational data:
    ```text
    ┌─ V2Raynix System Status ───────────────────────────────────────────────┐
    │ ● Service: Active    ● Tunnel: Connected (US - HighSpeed)              │
    │ Uptime: 45m 12s      Safe Mode: 84s remaining                         │
    │ ● Health: Healthy (184ms)    Web UI: http://127.0.0.1:2080             │
    └────────────────────────────────────────────────────────────────────────┘
    ```

#### 3.2.2 Immediate Interactive Safe Mode Guard Screen
- In `internal/cli/menu_tunnel.go`, upon successful invocation of `bridge.ConnectTunnel(selected.ID)`:
  - Do NOT exit back to the menu immediately.
  - Automatically enter `handleSafeModeGuard(bridge, reader, selected)`:
    - Renders an interactive verification box with an active countdown timer.
    - Options:
      - `[1] Probe Health & Latency (Verify connection)`
      - `[2] Confirm Safe Mode (Make permanent & keep tunnel)`
      - `[3] Emergency Rollback (Abort & restore original routes)`
    - If user chooses `[1]`, run `bridge.CheckHealth()` and print latency without closing the guard.
    - If user chooses `[2]`, call `bridge.ConfirmSafeMode()`, print success message, and return to tunnel menu.
    - If user chooses `[3]`, call `bridge.RollbackSafeMode()`, print rollback message, and return.
    - If safe mode expires during the loop, report automatic rollback and exit loop.

#### 3.2.3 Clear Cancel/Back Hints in All Prompts
- Audit and update all input prompts across `internal/cli/menu_*.go`:
  - `menu_configs.go`: `Enter share link (vless://...) or raw JSON [or enter '0' / 'b' to cancel]:`
  - `menu_routing.go`: `Enter pattern/target [or enter '0' / 'b' to cancel]:`
  - `menu_settings.go`: `Enter new web port (1-65535) [current %d, or '0' to cancel]:`
  - Accept `0`, `b`, `B`, `cancel`, `exit`, or empty input (where appropriate) as an immediate safe return.

---

### 3.3 Category 3: Core & Updater Resilience

#### 3.3.1 Client Timeout Expansion
- In `internal/cli/bridge.go`:
  - When calling `/api/system/update-core`, use a dedicated long-lived HTTP client (`180 * time.Second`) instead of the default 5s client.
- In `web/src/services/api.js`:
  - Support custom request timeout options in `request(endpoint, options)` (defaulting to 15s for normal APIs, but allowing `options.timeout = 180000` for core updates).

#### 3.3.2 Non-Blocking Background Upgrade & Status Polling
- In `internal/updater/updater.go`:
  - Add thread-safe state tracking: `isUpdating bool`, `lastError string`, `currentEngine string`.
  - In `UpdateCore(engine string)`, if already updating, return `errUpdateInProgress`.
  - When triggered, perform the heavy `downloadFile` and `unzipExtractFile` asynchronously or protected by a cancellation-aware context.
- In `internal/api/router.go`:
  - Endpoint `POST /api/system/update-core`:
    - Launches or triggers the update, responding with `{"status": "updating", "engine": engine}` or completing within an extended context.
  - Endpoint `GET /api/system/updates`:
    - Returns current update state: `{"isUpdating": bool, "lastLog": string, "cores": {...}}`.
  - If a user accesses the dashboard through a reverse proxy (e.g., Nginx, Caddy, Cloudflare) that enforces a 30s or 60s proxy timeout, the web UI or TUI polls `/api/system/updates` every 2 seconds until `isUpdating == false`, eliminating `504 Gateway Timeout` errors.

---

### 3.4 Category 4: Headless CLI Scripting Interface

#### 3.4.1 Subcommands & Syntax
The `v2raynix` binary will support direct non-interactive CLI argument execution:

```bash
# 1. Connect to a configuration (by name, ID, or partial match)
v2raynix connect [config-id-or-name] [--force]
# --force skips safe-mode timer or immediately confirms it

# 2. Disconnect active tunnel
v2raynix disconnect

# 3. Print current service and tunnel status
v2raynix status [--json]

# 4. Run immediate connectivity and latency test
v2raynix health

# 5. List all saved configurations
v2raynix list

# 6. Safe mode controls
v2raynix confirm
v2raynix rollback
```

#### 3.4.2 Execution Mechanism
- Implemented in `internal/cli/commands.go` using `TUIBridge`.
- If the systemd daemon is active, commands route through the local REST API with the IPC token (`/etc/v2raynix/.ipc_token`).
- If the daemon is inactive, commands operate directly on `/etc/v2raynix/v2raynix.json` or inform the user to start the service (`systemctl start v2raynix`).
- Formats output with clean ANSI badges or `--json` for machine readability.

---

## 4. Test Strategy & Acceptance Criteria

1. **Unit Tests (Go):**
   - `internal/cli/tui_layout_test.go`: Verify `RenderStatusHeader` excludes traffic lines and preserves all other fields.
   - `internal/cli/menu_tunnel_test.go`: Verify `handleSafeModeGuard` launches after connection and responds to test, confirm, and rollback inputs.
   - `internal/cli/bridge_test.go`: Verify `UpdateCore` does not time out under simulated slow downloads.
   - `internal/cli/commands_test.go`: Verify all CLI subcommands (`connect`, `disconnect`, `status`, `health`, `list`) succeed and return correct exit codes (0 for success, 1 for error).
   - `internal/updater/updater_test.go`: Verify thread-safe `isUpdating` state tracking and error handling.
2. **Frontend Tests (React / Vite):**
   - Run `npm run build` cleanly without lint/syntax errors.
   - Verify sidebar drawer toggle and animation on mobile viewport widths (< 768px).
   - Verify removal of Anti-Lockout card and proper grid rendering.
3. **End-to-End Live Verification:**
   - Deploy build to live Linux server (`192.168.254.80`).
   - Run `v2raynix connect [node] --force` and confirm immediate permanent connection.
   - Run interactive TUI, connect to a node, and verify immediate Safe Mode prompt.
   - Trigger Xray core update from both TUI and web panel through reverse proxy port and verify success.
