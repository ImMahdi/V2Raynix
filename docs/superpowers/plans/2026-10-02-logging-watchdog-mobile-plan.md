# V2Raynix v0.9.1 Implementation Plan: Logging, Health Watchdog & Mobile UI

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement standardized English journalctl logging, automated connection health watchdog with auto-recovery, and a mobile-responsive UI with a hamburger navigation drawer.

**Architecture:** A concurrent Go watchdog inside `core.Supervisor` probes an HTTP 204 endpoint on a configurable interval (default: 60m), initiating automated tunnel restart if verification fails twice. Standardized English logs are written simultaneously to stdout/stderr with level/component tags and to the web ring buffer. The React frontend introduces a mobile drawer in `Navbar.jsx` with mobile-adapted CSS grids and controls in `index.css`.

**Tech Stack:** Go 1.23, React (Vite), CSS3 Glassmorphism & Media Queries, lucide-react.

**Spec:** `docs/superpowers/specs/2026-10-02-logging-watchdog-mobile-design.md`

## Global Constraints
- Target version: bump from `v0.9.0-beta` to `v0.9.1-beta`.
- Default health check interval: `60` minutes (`0` disables).
- Default health check URL: `http://cp.cloudflare.com/generate_204`.
- Log line format: `[YYYY-MM-DD HH:MM:SS] [LEVEL] [COMPONENT] Message`.
- Mobile navigation breakpoint: `@media (max-width: 768px)`.
- Zero external runtime dependencies: all assets embedded in Go single binary (`go:embed`).

## Review Focus
- **Probe Timeout Handling:** Health probe must respect a strict 5-second context timeout and not hang the watchdog loop.
- **Race-Free Watchdog Lifecycle:** Watchdog goroutine must start only on `connected` state, cancel cleanly on `disconnected`, and restart gracefully on config changes without orphan goroutines.
- **Idempotent Recovery:** If auto-recovery triggers while a user disconnects or during a manual restart, it must check state safely and avoid double-teardown panics.
- **Mobile Viewport Overflow:** The mobile drawer must be contained within viewport height (`100vh` or `100dvh`) with clean backdrop blur and touch dismiss.
- **Nil Store Defaults:** If existing `v2raynix.json` has missing health check fields, Store must fall back to 60 minutes and default URL without nil pointer dereference.

---

### Task 1: Standard English Structured Logger

**Files:**
- Create: `internal/core/logger.go`
- Create: `internal/core/logger_test.go`
- Modify: `internal/core/supervisor.go:440-468`

**Interfaces:**
- Produces: `FormatLogLine(t time.Time, level, component, message string) string`
- Produces: `Supervisor.Log(level, component, message string)`

- [ ] **Step 1: Write failing unit test for log formatting**

```go
func TestFormatLogLine(t *testing.T) {
    fixedTime := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
    line := FormatLogLine(fixedTime, "INFO", "TUNNEL", "Interface tun0 established")
    expected := "[2026-10-02 12:00:00] [INFO] [TUNNEL] Interface tun0 established"
    if line != expected {
        t.Fatalf("expected %q, got %q", expected, line)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/core -run TestFormatLogLine`  
Expected: FAIL (undefined `FormatLogLine`)

- [ ] **Step 3: Implement `FormatLogLine` and stdout/stderr emission in `internal/core/logger.go`**

Implement `FormatLogLine` and update `addLog` in `supervisor.go` to route `INFO`/`DEBUG` to `os.Stdout` and `WARN`/`ERROR` to `os.Stderr`, while preserving the in-memory ring buffer for the web UI.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/core -run TestFormatLogLine`  
Expected: PASS

- [ ] **Step 5: Commit Task 1**

```bash
git add internal/core/logger.go internal/core/logger_test.go internal/core/supervisor.go
git commit -m "feat(core): add structured English journalctl logger to supervisor"
```

---

### Task 2: Health Check Store Schema & Settings API

**Files:**
- Modify: `internal/store/store.go:15-35, 120-145`
- Modify: `internal/store/store_test.go:40-70`
- Modify: `internal/api/handlers_settings.go:20-50`

**Interfaces:**
- Consumes: `store.Settings` struct
- Produces: `Settings.HealthCheckIntervalMinutes int` (default: 60)
- Produces: `Settings.HealthCheckURL string` (default: `http://cp.cloudflare.com/generate_204`)

- [ ] **Step 1: Write failing unit test for new Settings defaults and persistence**

```go
func TestHealthCheckSettingsDefaults(t *testing.T) {
    st, cleanup := createTestStore(t)
    defer cleanup()
    
    settings, err := st.GetSettings()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if settings.HealthCheckIntervalMinutes != 60 {
        t.Errorf("expected default interval 60, got %d", settings.HealthCheckIntervalMinutes)
    }
    if settings.HealthCheckURL != "http://cp.cloudflare.com/generate_204" {
        t.Errorf("expected default url, got %s", settings.HealthCheckURL)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/store -run TestHealthCheckSettingsDefaults`  
Expected: FAIL

- [ ] **Step 3: Add fields to `Settings` struct and implement fallback migration**

In `internal/store/store.go`, add `HealthCheckIntervalMinutes int json:"healthCheckIntervalMinutes"` and `HealthCheckURL string json:"healthCheckURL"` to `Settings`. In `GetSettings()`, populate defaults when values are 0 or empty. Update `internal/api/handlers_settings.go` to serialize/deserialize these fields.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/store -run TestHealthCheckSettingsDefaults`  
Expected: PASS

- [ ] **Step 5: Commit Task 2**

```bash
git add internal/store/store.go internal/store/store_test.go internal/api/handlers_settings.go
git commit -m "feat(store): add health check configuration fields to settings schema"
```

---

### Task 3: Health Watchdog & Auto-Recovery Engine

**Files:**
- Create: `internal/core/watchdog.go`
- Create: `internal/core/watchdog_test.go`
- Modify: `internal/core/supervisor.go:80-140, 200-240`
- Modify: `internal/api/router.go:50-80`
- Create: `internal/api/handlers_health.go`

**Interfaces:**
- Produces: `Supervisor.CheckHealth(ctx context.Context, targetURL string) (bool, time.Duration, error)`
- Produces: `Supervisor.StartHealthWatchdog(ctx context.Context)`
- Produces: `POST /api/health/check` endpoint returning `{ "healthy": bool, "latencyMs": int, "error": string }`

- [ ] **Step 1: Write failing unit tests for probe execution and recovery trigger**

```go
func TestWatchdogProbeSuccessAndFailure(t *testing.T) {
    // 1. Success case with mock HTTP server returning 204
    // 2. Failure case with HTTP 500 triggering recovery
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/core -run TestWatchdogProbe`  
Expected: FAIL

- [ ] **Step 3: Implement `internal/core/watchdog.go` and `handlers_health.go`**

Implement HTTP probe with 5-second timeout. Connect watchdog loop to `Supervisor.state == "connected"`. If verification fails twice in a row, trigger `s.RestartTunnel()`. Wire `POST /api/health/check` in router.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/core -run TestWatchdogProbe`  
Expected: PASS

- [ ] **Step 5: Commit Task 3**

```bash
git add internal/core/watchdog.go internal/core/watchdog_test.go internal/core/supervisor.go internal/api/router.go internal/api/handlers_health.go
git commit -m "feat(core): implement connection health watchdog with automated recovery"
```

---

### Task 4: Responsive Mobile Layout & Hamburger Drawer Navigation

**Files:**
- Modify: `web/src/components/Navbar.jsx`
- Modify: `web/src/index.css:280-350`

**Interfaces:**
- Produces: Mobile hamburger toggle button with `Menu` and `X` icons.
- Produces: Full-screen mobile navigation drawer with backdrop blur and smooth sliding animation.
- Produces: `@media (max-width: 768px)` layout rules in `index.css`.

- [ ] **Step 1: Add mobile responsive styles to `web/src/index.css`**

Add responsive media queries for:
- Navbar: hide desktop tab row on `<= 768px`, show mobile menu toggle button.
- Drawer: `.mobile-drawer` overlay and container with slide-in animation.
- Main layout: reduce padding from `2rem 1.5rem` to `1rem 0.75rem` on mobile.

- [ ] **Step 2: Update `Navbar.jsx` with mobile state and drawer JSX**

Implement `isMobileMenuOpen` state, toggle button, backdrop click handler, and touch-friendly navigation buttons that automatically close the drawer on selection.

- [ ] **Step 3: Verify frontend compiles cleanly**

Run: `cd web && npm run build`  
Expected: Build succeeds with 0 errors.

- [ ] **Step 4: Commit Task 4**

```bash
git add web/src/components/Navbar.jsx web/src/index.css
git commit -m "feat(ui): add responsive mobile navigation drawer and media queries"
```

---

### Task 5: Mobile Dashboard & Settings Health Check UI

**Files:**
- Modify: `web/src/views/DashboardView.jsx`
- Modify: `web/src/views/SettingsView.jsx`
- Modify: `web/src/api/client.js`

**Interfaces:**
- Produces: `apiClient.checkHealth()` -> calls `POST /api/health/check`
- Produces: "Check Health Now" button on Dashboard with live response badge.
- Produces: "Health Check Interval (Minutes)" and "Health Check Probe URL" settings fields.

- [ ] **Step 1: Add `checkHealth` to `web/src/api/client.js`**

Implement `checkHealth: () => request('/health/check', { method: 'POST' })`.

- [ ] **Step 2: Update `DashboardView.jsx` with responsive grid & manual check button**

Convert metrics grid to auto-fit (`minmax(140px, 1fr)`), add health check trigger card with status badge.

- [ ] **Step 3: Update `SettingsView.jsx` with health check configuration inputs**

Add input for `healthCheckIntervalMinutes` (number, default 60) and `healthCheckURL` (text).

- [ ] **Step 4: Verify frontend build**

Run: `cd web && npm run build`  
Expected: PASS

- [ ] **Step 5: Commit Task 5**

```bash
git add web/src/views/DashboardView.jsx web/src/views/SettingsView.jsx web/src/api/client.js
git commit -m "feat(ui): add health check controls to dashboard and settings with responsive grids"
```

---

### Task 6: Full Integration Build, Test Verification & Version Bump

**Files:**
- Modify: `cmd/v2raynix/main.go:25` (Version = "0.9.1-beta")
- Modify: `web/package.json:3` ("version": "0.9.1-beta")
- Modify: `README.md`, `README.fa.md`, `README.zh-CN.md`, `README.ru.md`

- [ ] **Step 1: Run complete backend unit test suite**

Run: `go test -v ./...`  
Expected: All tests PASS.

- [ ] **Step 2: Rebuild embedded frontend distribution**

Run: `cd web && npm run build`  
Expected: `web/dist` generated cleanly.

- [ ] **Step 3: Compile standalone binary and verify in simulation mode**

Run: `go build -ldflags="-s -w" -o bin/v2raynix ./cmd/v2raynix`  
Verify: `./bin/v2raynix -version` prints `V2Raynix v0.9.1-beta`.

- [ ] **Step 4: Update README version badges to v0.9.1-beta across all 4 languages**

- [ ] **Step 5: Commit and tag v0.9.1-beta**

```bash
git add .
git commit -m "chore(release): bump version to v0.9.1-beta with logging, watchdog and mobile UI"
git tag -a -f v0.9.1-beta -m "Release v0.9.1-beta: Health Watchdog, Journalctl Logging and Mobile UI"
```
