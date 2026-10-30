# V2Raynix v0.9.3-beta Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement v0.9.3-beta quality-of-life improvements: clean up unused traffic and anti-lockout UI elements, replace mobile navigation with a full-height sliding side drawer, unify brand logo, implement an interactive Safe Mode Guard in TUI, eliminate core updater timeouts under reverse proxies via async jobs & extended deadlines, and introduce direct headless CLI automation commands (`connect [--force]`, `disconnect`, `status`, `health`, `list`).

**Architecture:** 
- Frontend updates polish React views (`DashboardView.jsx`, `Navbar.jsx`, `index.css`).
- TUI updates eliminate dead metrics from `RenderStatusHeader`, insert an interactive verification loop post-connect in `menu_tunnel.go`, and add clear cancellation affordances in prompts.
- Core & Updater resilience expands timeouts across `bridge.go` and `api.js` while introducing asynchronous job execution and status polling in `updater.go` and `router.go`.
- CLI architecture provides a dedicated command parser in `internal/cli/commands.go` invoked from `cmd/v2raynix/main.go`, utilizing the dual-mode `TUIBridge`.

**Tech Stack:** Go 1.26, React 18, Vite, Lucide React, CSS3 animations, systemd, tun2socks, Xray-core.

**Spec:** `docs/superpowers/specs/2026-10-30-v093-beta-improvements-design.md`

## Global Constraints
- Go code must pass `go test ./...` with zero failures.
- Web code must build cleanly via `npm run build` in `web/` with zero errors.
- Version string must be bumped to `0.9.3-beta` in `cmd/v2raynix/main.go`, `web/package.json`, and all 4 READMEs.
- Keep strict backward compatibility with existing CLI flags (`-port`, `-mock`, `setup`, `menu`).
- All binaries must be built using `-trimpath` and `-ldflags="-s -w"`.

## Review Focus
1. TUI Safe Mode Guard loop must handle premature user disconnect or daemon failure gracefully without hang.
2. Direct CLI `v2raynix connect --force` must bypass or immediately confirm Safe Mode so remote SSH users are not locked out if they disconnect.
3. Core updater must not hang or throw context deadline exceeded when downloading 20MB archives over slow networks or reverse proxies.
4. Mobile side drawer must trap/close properly on backdrop click, route navigation, or escape key, preventing overlay scroll locking bugs.
5. All input prompts in TUI must handle empty string or cancellation keys (`0`/`b`) without crashing or infinite re-prompting.

---

## Phase 1: Web UI Modernization & Polish

### Task 1: Remove Anti-Lockout Card from Dashboard & Clean Up Grid Layout
**Files:**
- Modify: `web/src/views/DashboardView.jsx:315-352`
- Test: `web/src/views/DashboardView.jsx`

**Interfaces:**
- Consumes: `DashboardView` props (`status`, `configs`, `onToggleTunnel`, `onSelectTab`, `toggling`)
- Produces: Clean 3-card metric grid (`Health Status`, `Active Config`, `Tunnel Uptime`) without unused Anti-Lockout card.

- [ ] **Step 1: Inspect `DashboardView.jsx` and identify the card to remove**
- [ ] **Step 2: Remove lines 316-351 containing the `SSH Protection / Anti-Lockout Active` glass-card**
- [ ] **Step 3: Update `gridTemplateColumns` in `DashboardView.jsx` for optimal responsive spacing**
- [ ] **Step 4: Verify frontend builds cleanly**
  Run: `cd web && npm run build`
  Expected: Build succeeds with 0 errors.
- [ ] **Step 5: Commit**
  ```bash
  git add web/src/views/DashboardView.jsx
  git commit -m "fix(ui): remove redundant anti-lockout card from dashboard"
  ```

---

### Task 2: Replace Mobile Slide-Down Menu with Full-Height Sliding Side Drawer
**Files:**
- Modify: `web/src/components/Navbar.jsx:150-270`
- Modify: `web/src/index.css:370-420`

**Interfaces:**
- Consumes: `Navbar` props (`activeTab`, `onSelectTab`, `user`, `onLogout`, `hasUpdate`, `onOpenUpdateModal`)
- Produces: Mobile slide-in side drawer from right edge with smooth transform and backdrop dismissal.

- [ ] **Step 1: Update CSS in `web/src/index.css` for `.mobile-drawer-overlay` and `.mobile-drawer`**
  Set `.mobile-drawer-overlay` to `position: fixed; inset: 0; background: rgba(0,0,0,0.7); z-index: 1000;`.
  Set `.mobile-drawer` to `position: fixed; top: 0; right: 0; width: 280px; max-width: 85vw; height: 100vh; animation: slideDrawer 0.25s ease-out;`.
- [ ] **Step 2: Update `web/src/components/Navbar.jsx` drawer markup**
  Include drawer header with brand title, close button (`X`), navigation items, update button, and user logout footer.
- [ ] **Step 3: Verify frontend compilation**
  Run: `cd web && npm run build`
  Expected: PASS
- [ ] **Step 4: Commit**
  ```bash
  git add web/src/components/Navbar.jsx web/src/index.css
  git commit -m "feat(ui): convert mobile navigation to full-height sliding side drawer"
  ```

---

### Task 3: Replace Navbar Shield Icon with Official V2Raynix Lightning Logo
**Files:**
- Modify: `web/src/components/Navbar.jsx:1-65`

**Interfaces:**
- Consumes: Lucide `Zap` icon
- Produces: Official brand lightning badge in desktop navbar and mobile drawer header matching favicon.

- [ ] **Step 1: In `web/src/components/Navbar.jsx`, replace `<ShieldCheck size={22} color="#ffffff" />` with `<Zap size={20} fill="#ffffff" color="#ffffff" />`**
- [ ] **Step 2: Verify alignment and styling of brand badge**
- [ ] **Step 3: Run frontend build**
  Run: `cd web && npm run build`
  Expected: PASS
- [ ] **Step 4: Commit**
  ```bash
  git add web/src/components/Navbar.jsx
  git commit -m "style(ui): replace navbar shield with official lightning logo"
  ```

---

## Phase 2: TUI Refinement & Interactive Safe Mode Guard

### Task 4: Remove Unused Traffic Line from TUI Status Header
**Files:**
- Modify: `internal/cli/tui_layout.go:300-340`
- Modify: `internal/cli/tui_layout_test.go:120-160`

**Interfaces:**
- Consumes: `LiveStatusInfo`
- Produces: `RenderStatusHeader(info LiveStatusInfo) string` without stubbed `Traffic: ▲ 0 B/s ▼ 0 B/s` row.

- [ ] **Step 1: Write failing unit test in `internal/cli/tui_layout_test.go`**
  Verify that `RenderStatusHeader` output does not contain the word `Traffic:` while still containing `Service:`, `Tunnel:`, `Uptime:`, and `Health:`.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/cli -run TestRenderStatusHeader`
  Expected: FAIL
- [ ] **Step 3: In `internal/cli/tui_layout.go`, remove `trafficStr` line from `RenderStatusHeader`**
- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./internal/cli -run TestRenderStatusHeader`
  Expected: PASS
- [ ] **Step 5: Commit**
  ```bash
  git add internal/cli/tui_layout.go internal/cli/tui_layout_test.go
  git commit -m "fix(cli): remove unused traffic stats line from TUI status header"
  ```

---

### Task 5: Implement Interactive Safe Mode Guard Screen After Connection in TUI
**Files:**
- Modify: `internal/cli/menu_tunnel.go:105-135`
- Create/Modify: `internal/cli/menu_tunnel_test.go`

**Interfaces:**
- Consumes: `TUIBridge.ConnectTunnel(id string)`, `TUIBridge.CheckHealth()`, `TUIBridge.ConfirmSafeMode()`, `TUIBridge.RollbackSafeMode()`
- Produces: `handleSafeModeGuard(bridge TUIBridge, reader *bufio.Reader, selected store.ConfigItem) error`

- [ ] **Step 1: Write failing unit tests in `internal/cli/menu_tunnel_test.go`**
  - `TestHandleConnectConfig_SafeModeGuard_Confirm`: Connecting launches guard; selecting option `2` triggers `ConfirmSafeMode` and returns.
  - `TestHandleConnectConfig_SafeModeGuard_Rollback`: Connecting launches guard; selecting option `3` triggers `RollbackSafeMode` and returns.
  - `TestHandleConnectConfig_SafeModeGuard_ProbeHealth`: Selecting option `1` triggers `CheckHealth` and prints latency.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/cli -run TestHandleConnectConfig_SafeModeGuard`
  Expected: FAIL
- [ ] **Step 3: Implement `handleSafeModeGuard` in `internal/cli/menu_tunnel.go`**
  Immediately after successful connection, present interactive guard box displaying remaining safe-mode seconds, probe option, confirm option, and rollback option.
- [ ] **Step 4: Run tests to verify they pass**
  Run: `go test -v ./internal/cli -run TestHandleConnectConfig_SafeModeGuard`
  Expected: PASS
- [ ] **Step 5: Commit**
  ```bash
  git add internal/cli/menu_tunnel.go internal/cli/menu_tunnel_test.go
  git commit -m "feat(cli): prompt user with interactive safe mode guard immediately after connect"
  ```

---

### Task 6: Audit and Standardize Cancel/Back Prompts Across All TUI Menus
**Files:**
- Modify: `internal/cli/menu_configs.go:130-180`
- Modify: `internal/cli/menu_routing.go:100-160`
- Modify: `internal/cli/menu_settings.go:70-130`
- Modify: `internal/cli/menu_configs_test.go`

**Interfaces:**
- Consumes: `bufio.Reader`
- Produces: User-friendly prompts indicating `(or enter '0' / 'b' to cancel)` on all text input steps.

- [ ] **Step 1: Add unit tests verifying cancel/abort on empty string, '0', or 'b' across config, routing, and settings prompts**
- [ ] **Step 2: Run test to verify coverage**
  Run: `go test -v ./internal/cli -run TestHandleConfigsMenu`
- [ ] **Step 3: Update prompts in `menu_configs.go`, `menu_routing.go`, and `menu_settings.go` with clear cancel hints**
- [ ] **Step 4: Run tests to verify pass**
  Run: `go test -v ./internal/cli`
  Expected: PASS
- [ ] **Step 5: Commit**
  ```bash
  git add internal/cli/menu_configs.go internal/cli/menu_routing.go internal/cli/menu_settings.go
  git commit -m "ux(cli): add explicit cancel and back shortcuts to all TUI prompts"
  ```

---

## Phase 3: Core & Updater Resilience (Reverse-Proxy & Timeout Fix)

### Task 7: Fix Update Timeouts in TUI Bridge & Web API Client
**Files:**
- Modify: `internal/cli/bridge.go:620-640`
- Modify: `web/src/services/api.js:45-65`
- Modify: `internal/cli/bridge_test.go`

**Interfaces:**
- Consumes: `/api/system/update-core`
- Produces: Dedicated 180s client timeout for core update requests in both Go bridge and browser fetch.

- [ ] **Step 1: Write failing test in `internal/cli/bridge_test.go` simulating a slow core update endpoint (e.g. taking 8 seconds)**
  Verify default client would fail, while `UpdateCore` with extended timeout succeeds.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/cli -run TestBridge_UpdateCoreTimeout`
  Expected: FAIL
- [ ] **Step 3: In `internal/cli/bridge.go`, implement dedicated long-timeout HTTP client (180s) for `UpdateCore`**
- [ ] **Step 4: In `web/src/services/api.js`, allow `options.timeout` override, passing 180000ms for `updateCore`**
- [ ] **Step 5: Run tests to verify pass**
  Run: `go test -v ./internal/cli -run TestBridge_UpdateCoreTimeout`
  Expected: PASS
- [ ] **Step 6: Commit**
  ```bash
  git add internal/cli/bridge.go internal/cli/bridge_test.go web/src/services/api.js
  git commit -m "fix(updater): expand request timeouts to 180s to prevent context deadline exceeded"
  ```

---

### Task 8: Implement Background Async Update and Progress Tracking in Updater & API Router
**Files:**
- Modify: `internal/updater/updater.go:290-380`
- Modify: `internal/updater/updater_test.go`
- Modify: `internal/api/router.go:610-635`
- Modify: `internal/api/api_test.go`

**Interfaces:**
- Consumes: `Updater.UpdateCore(core string) error`, `Updater.GetStatus() UpdateStatus`
- Produces: Non-blocking core upgrade capability with thread-safe `IsUpdating`, `LastLog`, and `LastError` status polling.

- [ ] **Step 1: Write unit tests in `internal/updater/updater_test.go` for concurrent update rejection and status reporting**
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/updater -run TestUpdater_ConcurrentUpdate`
  Expected: FAIL
- [ ] **Step 3: Update `internal/updater/updater.go`**
  Add mutex-guarded `isUpdating` flag, updating `u.status.IsUpdating = true` and recording progress into `u.status.LastLog`.
- [ ] **Step 4: Update `internal/api/router.go`**
  Ensure `/api/system/updates` reports live progress, enabling reverse-proxy clients to poll status safely without connection dropouts.
- [ ] **Step 5: Run all backend tests**
  Run: `go test -v ./internal/updater ./internal/api`
  Expected: PASS
- [ ] **Step 6: Commit**
  ```bash
  git add internal/updater/updater.go internal/updater/updater_test.go internal/api/router.go internal/api/api_test.go
  git commit -m "feat(updater): support resilient background updates with thread-safe status polling"
  ```

---

## Phase 4: Headless CLI Scripting & Release Packaging

### Task 9: Implement Headless CLI Subcommands
**Files:**
- Create: `internal/cli/commands.go`
- Create: `internal/cli/commands_test.go`
- Modify: `cmd/v2raynix/main.go:30-100`

**Interfaces:**
- Consumes: `TUIBridge`
- Produces: `HandleCLICommand(args []string, bridge TUIBridge) error`
  Supported commands:
  - `v2raynix connect <node> [--force]`
  - `v2raynix disconnect`
  - `v2raynix status [--json]`
  - `v2raynix health`
  - `v2raynix list`
  - `v2raynix confirm`
  - `v2raynix rollback`

- [ ] **Step 1: Write failing unit tests in `internal/cli/commands_test.go`**
  Test execution of `connect`, `connect --force`, `disconnect`, `status`, `health`, and `list` against mock bridge.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/cli -run TestHandleCLICommand`
  Expected: FAIL
- [ ] **Step 3: Create `internal/cli/commands.go` implementing `HandleCLICommand`**
- [ ] **Step 4: Wire `HandleCLICommand` into `cmd/v2raynix/main.go`**
  Inspect `os.Args[1]`. If it matches any headless subcommand, execute it directly with proper exit codes (0 on success, 1 on error).
- [ ] **Step 5: Run tests to verify pass**
  Run: `go test -v ./internal/cli -run TestHandleCLICommand`
  Expected: PASS
- [ ] **Step 6: Commit**
  ```bash
  git add internal/cli/commands.go internal/cli/commands_test.go cmd/v2raynix/main.go
  git commit -m "feat(cli): implement direct headless automation subcommands"
  ```

---

### Task 10: Version Bump (`v0.9.3-beta`), Documentation Updates across all 4 READMEs, Compilation & Tag
**Files:**
- Modify: `cmd/v2raynix/main.go:27` (Version = "0.9.3-beta")
- Modify: `web/package.json:4` ("version": "0.9.3-beta")
- Modify: `README.md`, `README.fa.md`, `README.zh-CN.md`, `README.ru.md`

**Interfaces:**
- Version badges and strings updated to `v0.9.3-beta`.
- Rebuilt frontend embedded in binary.
- Standalone binaries compiled for Windows and Linux with `-trimpath`.

- [ ] **Step 1: Bump version strings in `main.go` and `web/package.json`**
- [ ] **Step 2: Update version badges and CLI command documentation in all 4 README files**
- [ ] **Step 3: Rebuild web frontend**
  Run: `cd web && npm run build`
- [ ] **Step 4: Run full repository test suite**
  Run: `go test -v ./...`
  Expected: 100% PASS across all packages.
- [ ] **Step 5: Compile release binaries with `-trimpath`**
  Run:
  ```powershell
  $env:GOOS="linux"; $env:GOARCH="amd64"; go build -trimpath -ldflags="-s -w" -o bin/v2raynix ./cmd/v2raynix; $env:GOOS="windows"
  go build -trimpath -ldflags="-s -w" -o bin/v2raynix.exe ./cmd/v2raynix
  ```
- [ ] **Step 6: Verify binary version**
  Run: `.\bin\v2raynix.exe -version`
  Expected: `V2Raynix v0.9.3-beta (built 2026-10-30)`
- [ ] **Step 7: Commit, tag, and push**
  ```bash
  git add .
  git commit -m "chore(release): bump version to v0.9.3-beta with UI polish, resilient updater, and CLI commands"
  git tag -a -f v0.9.3-beta -m "Release v0.9.3-beta: Polish, Safe Mode Guard, Resilient Updater, and Headless CLI"
  git push origin main
  git push origin --tags --force
  ```
