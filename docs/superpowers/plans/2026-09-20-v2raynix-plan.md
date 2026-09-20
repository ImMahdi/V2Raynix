# V2Raynix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build V2Raynix, a self-contained Linux daemon and modern Web UI in Go and React for managing V2Ray/Xray configs, full-system tun2socks tunneling with anti-lockout protection, and a 2-minute Safe Mode auto-rollback timer.

**Architecture:** A single compiled Go binary orchestrating Xray and tun2socks child processes, providing policy-based routing with SSH bypass, serving a modern embedded React (Vite) Web UI, and persisting state in an embedded database.

**Tech Stack:** Go 1.22+, React 18, Vite, Lucide Icons, SQLite / JSON Store, tun2socks, Xray-core, Linux IP policy routing (`ip route` / `ip rule`).

**Spec:** `docs/superpowers/specs/2026-09-20-v2raynix-design.md`

## Global Constraints
- Target platform: Linux (x86_64 and arm64), compatible with Debian/Ubuntu, CentOS/RHEL, and Arch.
- Single binary delivery: All frontend production assets must be embedded via `//go:embed web/dist/*`.
- Default listening port for Web UI: `2080`.
- Safe Mode auto-rollback timeout: 120 seconds default.
- Zero external database services: All configuration and routing state stored in a local encrypted or `0600`-protected file store.
- Strict anti-lockout: SSH port 22 (or custom detected SSH port), incoming SSH client IP, and Web UI port must always bypass `tun0`.

## Review Focus
1. `Anti-Lockout Routing Exception`: When `tun0` activates, the default route change must NOT break established or new SSH connections on the server's primary interface.
2. `Safe Mode Timeout Trigger`: If the user disconnects or fails to click "Confirm & Keep" within 120 seconds, `tun0` and routing tables must cleanly revert without leaving the server unreachable.
3. `Config Parser Edge Cases`: Malformed share links (invalid base64, missing UUID, incomplete query parameters) must return structured validation errors instead of crashing the daemon.
4. `Process Cleanup on Termination`: If the daemon receives `SIGTERM`/`SIGINT` or encounters a panic, all `ip rule` entries and the `tun0` device must be completely purged from the Linux kernel.
5. `Xray Outbound Loop Avoidance`: The destination IP of the active remote V2Ray server must route through the physical gateway, never through `tun0`.

---

### Task 1: Go Workspace Initialization & Data Store Layer

**Files:**
- Create: `go.mod`
- Create: `internal/store/models.go`
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Produces:
  - `type ConfigItem struct { ID, Name, Protocol, RawURL, Server, Port, Latency, IsActive, CreatedAt }`
  - `type RoutingRule struct { ID, Target, TargetType, Action, IsEnabled }`
  - `type Store interface { GetConfigs(), SaveConfig(), DeleteConfig(), SetActiveConfig(id), GetRoutingRules(), SaveRoutingRule(), GetAdminUser(), SetAdminUser() }`

- [ ] **Step 1: Write the failing test**
Create `internal/store/store_test.go` testing database creation, adding a config, retrieving configs, and setting an active config.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test ./internal/store/...`
Expected: FAIL (package or files do not exist).

- [ ] **Step 3: Write minimal implementation**
Initialize `go.mod` (`module github.com/v2raynix/v2raynix`), create `internal/store/models.go` and `internal/store/store.go` with JSON/bbolt or SQLite storage.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test ./internal/store/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**
Run: `git add go.mod internal/store; git commit -m "feat(store): implement data models and persistence store"`

---

### Task 2: Authentication & Session Security Subsystem

**Files:**
- Create: `internal/auth/auth.go`
- Test: `internal/auth/auth_test.go`

**Interfaces:**
- Consumes: `store.Store`
- Produces:
  - `HashPassword(password string) (string, error)`
  - `CheckPassword(hashedPassword, password string) bool`
  - `GenerateJWT(username string, secret []byte, duration time.Duration) (string, error)`
  - `ValidateJWT(tokenString string, secret []byte) (*Claims, error)`

- [ ] **Step 1: Write the failing test**
Create `internal/auth/auth_test.go` testing password hashing, password verification with bcrypt, JWT token creation, token validation, and token expiration.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test ./internal/auth/...`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**
Implement `internal/auth/auth.go` using `golang.org/x/crypto/bcrypt` and `golang.org/x/crypto` / JWT handling.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test ./internal/auth/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**
Run: `git add internal/auth; git commit -m "feat(auth): implement bcrypt password hashing and JWT token generator"`

---

### Task 3: V2Ray / Xray Link Parsers & Outbound Generator

**Files:**
- Create: `internal/configmgr/parser.go`
- Create: `internal/configmgr/generator.go`
- Test: `internal/configmgr/parser_test.go`

**Interfaces:**
- Produces:
  - `ParseShareLink(link string) (*ConfigItem, error)` supporting `vless://`, `vmess://`, `trojan://`, `ss://`.
  - `GenerateXrayConfig(activeConfig *ConfigItem, rules []RoutingRule, localSocksPort, localHttpPort int) ([]byte, error)`

- [ ] **Step 1: Write the failing test**
Create `internal/configmgr/parser_test.go` with table-driven tests for sample valid and invalid links for VLESS (reality/ws/grpc), VMess, Trojan, and SS.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test ./internal/configmgr/...`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**
Implement URI parsing, base64 decoding, query string extraction, and JSON outbound generation for Xray core.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test ./internal/configmgr/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**
Run: `git add internal/configmgr; git commit -m "feat(configmgr): implement proxy link parsers and xray config generator"`

---

### Task 4: Ping & Latency Diagnostics Engine

**Files:**
- Create: `internal/pinger/pinger.go`
- Test: `internal/pinger/pinger_test.go`

**Interfaces:**
- Consumes: `store.ConfigItem`
- Produces:
  - `TCPPing(host string, port int, timeout time.Duration) (time.Duration, error)`
  - `RealHTTPDelay(socksProxyAddr, targetURL string, timeout time.Duration) (time.Duration, error)`
  - `BatchPing(configs []*ConfigItem, concurrency int) map[string]time.Duration`

- [ ] **Step 1: Write the failing test**
Create `internal/pinger/pinger_test.go` with mock TCP listener and test ping calculation and timeout handling.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test ./internal/pinger/...`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**
Implement TCP dial latency measurement and SOCKS5 proxy HTTP roundtrip latency measurement.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test ./internal/pinger/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**
Run: `git add internal/pinger; git commit -m "feat(pinger): implement TCP ping and real HTTP proxy latency diagnostics"`

---

### Task 5: Network Engine, Policy Routing & Safe Mode Controller

**Files:**
- Create: `internal/network/routing.go`
- Create: `internal/network/safemode.go`
- Test: `internal/network/safemode_test.go`

**Interfaces:**
- Produces:
  - `type SafeModeController struct { ... }`
  - `StartSafeMode(duration time.Duration, onRollback func())`
  - `ConfirmSafeMode()`
  - `CancelAndRollback()`
  - `GetDefaultGateway() (iface, gw string, err error)`
  - `BuildRoutingCommands(remoteProxyIP, defaultIface, defaultGw string, sshPort int) []string`

- [ ] **Step 1: Write the failing test**
Create `internal/network/safemode_test.go` testing timer triggering rollback when not confirmed, timer cancellation on confirm, and routing command construction with anti-lockout exclusions.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test ./internal/network/...`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**
Implement the countdown state machine in `safemode.go` and Linux routing table logic in `routing.go`.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test ./internal/network/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**
Run: `git add internal/network; git commit -m "feat(network): implement policy routing rules and safe mode auto-rollback controller"`

---

### Task 6: Process Supervisor (Xray & tun2socks)

**Files:**
- Create: `internal/core/supervisor.go`
- Test: `internal/core/supervisor_test.go`

**Interfaces:**
- Consumes: `internal/configmgr`, `internal/network`
- Produces:
  - `type Supervisor struct { ... }`
  - `StartTunnel(cfg *ConfigItem) error`
  - `StopTunnel() error`
  - `GetStatus() TunnelStatus`

- [ ] **Step 1: Write the failing test**
Create `internal/core/supervisor_test.go` with mock process runners verifying start, stop, state tracking, and cleanup execution on unexpected exit.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test ./internal/core/...`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**
Implement process lifecycle management, signal propagation, stdout/stderr log capture, and status telemetry.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test ./internal/core/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**
Run: `git add internal/core; git commit -m "feat(core): implement xray and tun2socks child process supervisor"`

---

### Task 7: REST API Handlers & HTTP Router

**Files:**
- Create: `internal/api/auth_handler.go`
- Create: `internal/api/config_handler.go`
- Create: `internal/api/routing_handler.go`
- Create: `internal/api/system_handler.go`
- Create: `internal/api/router.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- Consumes: `internal/store`, `internal/auth`, `internal/core`, `internal/pinger`, `internal/network`
- Produces:
  - `NewRouter(deps *Dependencies) http.Handler`
  - Endpoints:
    - `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me`, `POST /api/auth/password`
    - `GET /api/configs`, `POST /api/configs`, `DELETE /api/configs/{id}`, `POST /api/configs/{id}/activate`, `POST /api/configs/ping-all`
    - `POST /api/tunnel/connect`, `POST /api/tunnel/disconnect`, `GET /api/tunnel/status`, `POST /api/tunnel/safe-mode/confirm`
    - `GET /api/routing/rules`, `POST /api/routing/rules`, `DELETE /api/routing/rules/{id}`
    - `GET /api/system/logs`

- [ ] **Step 1: Write the failing test**
Create `internal/api/api_test.go` testing API responses, JWT authentication middleware, and validation checks.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test ./internal/api/...`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**
Implement handlers using Go's standard library `net/http` (or lightweight router) and register JSON endpoints.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test ./internal/api/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**
Run: `git add internal/api; git commit -m "feat(api): implement REST API endpoints and authentication middleware"`

---

### Task 8: React + Vite Frontend Setup & Design System

**Files:**
- Create: `web/package.json`
- Create: `web/vite.config.js`
- Create: `web/index.html`
- Create: `web/src/index.css`
- Create: `web/src/App.jsx`
- Create: `web/src/services/api.js`

**Interfaces:**
- Produces: Built frontend in `web/dist/` with dark mode, CSS variables, glassmorphism cards, responsive layout, and API client.

- [ ] **Step 1: Scaffold React + Vite structure**
Setup modern React web app with clean CSS token system, dark palette, Lucide icons, and API service layer.

- [ ] **Step 2: Verify build output**
Run: `cd web && npm install && npm run build`
Expected: Generates `web/dist/index.html` and assets.

- [ ] **Step 3: Commit**
Run: `git add web; git commit -m "feat(web): scaffold React frontend with dark theme design system"`

---

### Task 9: Web UI Pages, Components & Safe Mode Countdown Modal

**Files:**
- Create: `web/src/components/Navbar.jsx`
- Create: `web/src/components/SafeModeModal.jsx`
- Create: `web/src/components/ConfigCard.jsx`
- Create: `web/src/pages/DashboardPage.jsx`
- Create: `web/src/pages/ConfigsPage.jsx`
- Create: `web/src/pages/RoutingPage.jsx`
- Create: `web/src/pages/LogsPage.jsx`
- Create: `web/src/pages/SettingsPage.jsx`
- Create: `web/src/pages/LoginPage.jsx`

**Interfaces:**
- Produces: Complete interactive Web UI with:
  - Master Connect/Disconnect switch
  - 120s Safe Mode countdown dialog with "Confirm & Keep" / "Revert" actions
  - Config import modal & batch ping
  - Preset and custom routing rule editor
  - Streaming system log reader

- [ ] **Step 1: Implement UI components and views**
Build all pages and interactive components with smooth status transitions.

- [ ] **Step 2: Build and verify**
Run: `cd web && npm run build`
Expected: Clean build without errors in `web/dist/`.

- [ ] **Step 3: Commit**
Run: `git add web/src; git commit -m "feat(web): implement dashboard, config manager, routing editor, and safe mode modal"`

---

### Task 10: Single Binary Embedding, CLI Entrypoint & Linux Installer

**Files:**
- Create: `web/embed.go`
- Create: `cmd/v2raynix/main.go`
- Create: `scripts/v2raynix.service`
- Create: `scripts/install.sh`
- Modify: `README.md`

**Interfaces:**
- Produces:
  - `go:embed` binding `web/dist` directly into Go HTTP handler.
  - CLI flags: `-port`, `-data-dir`, `-init-admin`.
  - Systemd service template with `AmbientCapabilities=CAP_NET_ADMIN`.
  - Automated `install.sh` downloading/building binary and installing service.

- [ ] **Step 1: Write embed and CLI main.go**
Hook `web/embed.go` to serve the SPA on any unknown path, bind API router, handle graceful shutdown signals.

- [ ] **Step 2: Compile single binary test**
Run: `go build -o bin/v2raynix ./cmd/v2raynix`
Expected: Produces single standalone `bin/v2raynix` executable.

- [ ] **Step 3: Verify binary startup**
Run: `./bin/v2raynix -help` (or test run)
Expected: Prints flags and exits cleanly.

- [ ] **Step 4: Commit**
Run: `git add cmd web/embed.go scripts README.md; git commit -m "feat(distribution): add single binary embedding, CLI entrypoint, and systemd service"`
