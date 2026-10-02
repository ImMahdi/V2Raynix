# Specification: V2Raynix v0.9.1 Feature Suite

**Topic:** Standard English Journalctl Logging, Automated Health Watchdog with Auto-Recovery, and Mobile-Responsive UI Overhaul  
**Date:** 2026-10-02  
**Status:** Approved by User  
**Target Version:** v0.9.1-beta  

---

## 1. Executive Summary

This specification outlines the technical design for three major enhancements in V2Raynix:
1. **Standard English Logging for `journalctl`:** Structured, level-tagged logging written simultaneously to stdout/stderr and the in-memory web ring buffer for frictionless headless server troubleshooting.
2. **Periodic Health Watchdog & Auto-Recovery:** Configurable periodic health testing through the active proxy core to detect stalled tunnels and automatically restore service without risking SSH connectivity.
3. **Mobile-Responsive UI Overhaul:** Responsive layout refactoring featuring an animated drawer hamburger menu for navigation and adaptable CSS grids/tables for mobile viewports.

---

## 2. Technical Architecture & Component Design

### 2.1 Standard English Logging (`journalctl` & Stdout)

#### Current State:
`Supervisor.addLog(level, message)` only appends entries to `s.logs` (in-memory ring buffer for the React Web UI). Nothing is streamed to standard output during runtime, resulting in an empty or near-empty `journalctl -u v2raynix` output during active tunnel sessions.

#### Target Architecture:
- Format specification:
  ```text
  [YYYY-MM-DD HH:MM:SS] [LEVEL] [COMPONENT] Message...
  ```
- Levels: `INFO`, `WARN`, `ERROR`, `DEBUG`.
- Components: `CORE`, `TUNNEL`, `ROUTING`, `WATCHDOG`, `API`, `UPDATER`.
- Routing rule:
  - `INFO`, `DEBUG` written to `os.Stdout`.
  - `WARN`, `ERROR` written to `os.Stderr`.
  - Both stream simultaneously into `systemd-journald` when running under `v2raynix.service`, allowing administrators to run:
    ```bash
    journalctl -u v2raynix -f
    journalctl -u v2raynix -p err -n 50
    ```
  - Standardized English messages across all backend events (e.g. interface creation, routing table modification, core watchdog status, and API requests).

---

### 2.2 Periodic Health Watchdog & Auto-Recovery

#### Store & Settings Schema:
Two new fields added to `store.Settings`:
- `HealthCheckIntervalMinutes` (`int`, default: `60`, `0` = disabled).
- `HealthCheckURL` (`string`, default: `http://cp.cloudflare.com/generate_204`).

#### Watchdog Daemon Behavior (`internal/core/watchdog.go`):
- Runs as a background goroutine managed by `Supervisor`.
- Controlled via `context.Context` with clean cancellation upon tunnel stop or daemon shutdown.
- When `Supervisor.state == "connected"`:
  1. A ticker fires every `HealthCheckIntervalMinutes` (or immediately when triggered via API).
  2. Executes an HTTP probe with a 5-second timeout:
     - Uses an `http.Client` routed through local SOCKS5 inbound (`127.0.0.1:10808` or via `tun0`).
     - Verifies HTTP status code `204` or `200`.
  3. If probe succeeds:
     - Records log entry: `[INFO] [WATCHDOG] Connection health probe succeeded (target: <url>, latency: <duration>)`.
  4. If probe fails (timeout or error):
     - Logs warning: `[WARN] [WATCHDOG] Connection health probe failed (<err>). Retrying in 3 seconds...`
     - Retries probe once.
     - If retry also fails:
       - Logs error: `[ERROR] [WATCHDOG] Connection health verification failed twice. Triggering auto-recovery restart...`
       - Calls `Supervisor.RestartTunnel()` to cleanly teardown and re-orchestrate `tun0` and Xray core.
       - Confirms recovery outcome in logs.

#### API Endpoints:
- `POST /api/health/check`: Triggers an immediate manual health check probe and returns `{ "healthy": true/false, "latencyMs": 120, "error": "" }`.
- `GET /api/settings` and `POST /api/settings`: Read and update watchdog interval and probe URL.

---

### 2.3 Mobile-Responsive UI & Hamburger Navigation

#### Breakpoint Definition:
- Standard mobile breakpoint: `@media (max-width: 768px)`
- Small mobile breakpoint: `@media (max-width: 480px)`

#### Navigation (`web/src/components/Navbar.jsx`):
- Desktop (`> 768px`): Retains the sleek horizontal tab navigation bar.
- Mobile (`≤ 768px`):
  - Brand and beta badge on left.
  - Hamburger menu button (`Menu` / `X` icon from `lucide-react`) on right.
  - Toggling hamburger button opens a mobile drawer modal overlay (`backdrop-filter: blur(16px)`).
  - Drawer presents vertically-stacked navigation buttons with icons, active tab highlights, pending core update badge, and logout action.
  - Clicking any tab automatically navigates and closes the drawer.

#### Layouts & Views (`web/src/index.css` & views):
- **Dashboard (`DashboardView.jsx`):**
  - Metrics cards convert from 4-column grid (`repeat(4, 1fr)`) to 2-column or 1-column on mobile.
  - System speed graph and tunnel control card adapt to 100% width.
  - Add "Check Health Now" button with live status indicator.
- **Configs (`ConfigsView.jsx`):**
  - Table wrapper with `overflow-x: auto` and `-webkit-overflow-scrolling: touch` ensuring no horizontal layout break.
  - Action buttons wrapped with appropriate touch targets.
- **Settings (`SettingsView.jsx`):**
  - Add inputs for "Health Check Interval (Minutes)" and "Health Check Probe URL".
  - Grid adapts to single column on mobile.

---

## 3. Testing & Verification Strategy

### 3.1 Unit Testing (Go TDD)
- **`internal/store/store_test.go`:**
  - Verify default values for `HealthCheckIntervalMinutes` (60) and `HealthCheckURL`.
  - Verify saving and loading custom interval and URL values.
- **`internal/core/watchdog_test.go`:**
  - Mock HTTP server testing:
    1. Probe returns HTTP 204 -> Watchdog logs success, no restart triggered.
    2. Probe times out / returns HTTP 500 twice -> Watchdog triggers `RestartTunnel()`.
    3. Watchdog stops cleanly on context cancellation.
- **`internal/core/logging_test.go`:**
  - Verify `addLog` emits structured English log format to stdout/stderr.

### 3.2 Frontend Build & Lint Verification
- Run `npm run build` inside `web/` to guarantee clean zero-warning compilation of JSX, CSS, and assets.

### 3.3 End-to-End Simulation Testing
- Compile binary `bin/v2raynix`.
- Launch with `-port 2080 -mock`.
- Test `POST /api/health/check` and verify structured log lines emitted to terminal.
- Test mobile drawer toggle in responsive view.

---

## 4. Work Breakdown & Execution Plan

1. **Phase 1: Backend Logging & Store Updates**
   - Implement structured English logger in `internal/core`.
   - Update `store.Settings` with health check schema & tests.
2. **Phase 2: Health Check Watchdog & Auto-Recovery Engine**
   - Implement watchdog loop, HTTP probe, retry mechanism, and auto-restart policy.
   - Implement `/api/health/check` handler.
   - Comprehensive unit tests in `watchdog_test.go`.
3. **Phase 3: Frontend Responsive Design & Mobile Drawer**
   - Implement hamburger button & slide-down drawer in `Navbar.jsx`.
   - Add responsive CSS in `index.css` (`@media (max-width: 768px)`).
   - Update `SettingsView.jsx` with health check configuration inputs.
   - Update `DashboardView.jsx` with manual health check button and status badge.
4. **Phase 4: Full-Stack Verification & Release Preparation**
   - Full TDD test suite execution.
   - Compile single binary with embedded frontend.
   - Update documentation and prepare v0.9.1 release.
