# V2Raynix v0.9.2 Implementation Plan: Full Terminal User Interface (TUI)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a comprehensive Terminal User Interface (TUI) that mirrors 100% of the Web Dashboard capabilities (Tunnel controls, Configs management, Smart Routing rules, Live Logs, Settings, Health Watchdog, and Core Engine Updater) inside headless SSH sessions.

**Architecture:** A modular ANSI console architecture centered around a dual-mode `TUIBridge` interface. When the background daemon is active, operations are executed via local REST API calls (`http://127.0.0.1:<port>/api`) to prevent state desynchronization. When the daemon is stopped, operations execute directly against `v2raynix.json`. Standard box-drawing characters and color badges render responsive tables, forms, and live status headers.

**Tech Stack:** Go 1.23, `golang.org/x/term`, `golang.org/x/crypto/bcrypt`, Standard HTTP & JSON libraries. Zero external CGO dependencies.

**Spec:** `docs/superpowers/specs/2026-10-02-full-tui-design.md`

## Global Constraints
- Target version: bump from `v0.9.1-beta` to `v0.9.2-beta`.
- Invocation: `v2raynix` (interactive TTY), `v2raynix menu`, `v2raynix setup`.
- Non-interactive scripts (`-port`, `-mock`, `setup --user ...`) must remain 100% backward-compatible.
- Navigation conventions: `0` or `b`/`B` to return to parent menu; `q`/`Q` to exit.
- Silent password masking: input must never echo characters or asterisks to terminal.
- 100% compatible with standard SSH terminals (PuTTY, Termius, xterm, Windows Terminal).

## Review Focus
- **Daemon Inactivity Fallback:** If the daemon is stopped, selecting live actions (e.g. stream logs or connect tunnel) must display a clear guidance banner ("Service is inactive. Start service in menu [6] first") without panic.
- **Table Auto-Formatting & Truncation:** Long config names or server addresses must truncate cleanly with ellipsis to fit within standard 80-column terminal displays without breaking border alignment.
- **Live Log Stream Exit:** The live log streaming loop must break cleanly on any keypress (Enter / 'q') without blocking stdin indefinitely.
- **Single-Binary Embedding:** CLI embedding must not conflict with embedded React web assets.
- **Input Validation:** Numeric menu options, port inputs, and share link formats must handle invalid characters gracefully with clear inline retry prompts.

---

### Task 1: TUI Foundation, Layout Engine & Live Status Header

**Files:**
- Create: `internal/cli/tui_layout.go`
- Create: `internal/cli/tui_layout_test.go`
- Modify: `internal/cli/setup.go:180-220`

**Interfaces:**
- Produces: `RenderBox(title string, lines []string, width int) string`
- Produces: `RenderTable(headers []string, rows [][]string, widths []int) string`
- Produces: `RenderStatusHeader(info LiveStatusInfo) string`
- Produces: `ReadMenuChoice(prompt string, validChoices []string, reader *bufio.Reader) (string, error)`

- [ ] **Step 1: Write failing unit test for table and box formatting**

```go
func TestRenderTable(t *testing.T) {
    headers := []string{"#", "Name", "Protocol"}
    rows := [][]string{{"1", "Server A", "vless"}}
    widths := []int{4, 15, 10}
    out := RenderTable(headers, rows, widths)
    if !strings.Contains(out, "Server A") || !strings.Contains(out, "┌") {
        t.Fatalf("unexpected table output:\n%s", out)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/cli -run TestRenderTable`  
Expected: FAIL

- [ ] **Step 3: Implement `internal/cli/tui_layout.go`**

Implement ANSI color constants, box borders, table formatter with auto-padding and column clipping, live status header renderer, and robust choice parser (`0`/`b` for back, `q` for quit).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run TestRenderTable`  
Expected: PASS

- [ ] **Step 5: Commit Task 1**

```bash
git add internal/cli/tui_layout.go internal/cli/tui_layout_test.go internal/cli/setup.go
git commit -m "feat(cli): implement TUI layout engine, table renderer and status header"
```

---

### Task 2: Dual-Mode Bridge (Daemon API Client & Direct Store)

**Files:**
- Create: `internal/cli/bridge.go`
- Create: `internal/cli/bridge_test.go`

**Interfaces:**
- Produces: `type TUIBridge interface`
  - `GetStatus() (LiveStatusInfo, error)`
  - `ConnectTunnel(configID string) error`
  - `DisconnectTunnel() error`
  - `ConfirmSafeMode() error`
  - `RollbackSafeMode() error`
  - `CheckHealth() (bool, int64, error)`
  - `ListConfigs() ([]store.ConfigItem, string, error)`
  - `AddConfigFromLink(link string) (*store.ConfigItem, error)`
  - `DeleteConfig(id string) error`
  - `ListRules() ([]store.RoutingRule, error)`
  - `AddRule(rule store.RoutingRule) error`
  - `DeleteRule(id string) error`
  - `ApplyPresets() error`
  - `GetLogs(limit int) ([]core.LogEntry, error)`
  - `CheckCoreUpdates() (*updater.EngineUpdateStatus, error)`
  - `UpdateCore(engine string) error`
- Produces: `NewTUIBridge(dataDir string, webPort int) TUIBridge`

- [ ] **Step 1: Write failing unit tests for bridge standalone and api modes**

```go
func TestTUIBridge_OfflineStoreFallback(t *testing.T) {
    // Verify bridge reads configs and rules directly from store when API is unavailable
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/cli -run TestTUIBridge`  
Expected: FAIL

- [ ] **Step 3: Implement `internal/cli/bridge.go`**

Implement automatic daemon detection on `127.0.0.1:<webPort>`. If active, make authenticated HTTP REST calls using admin credentials from store. If inactive, fall back to direct `store.Store` methods.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run TestTUIBridge`  
Expected: PASS

- [ ] **Step 5: Commit Task 2**

```bash
git add internal/cli/bridge.go internal/cli/bridge_test.go
git commit -m "feat(cli): implement dual-mode daemon and store bridge for TUI"
```

---

### Task 3: Tunnel Operations & Real-Time Logs Menus (Menu 1 & 4)

**Files:**
- Create: `internal/cli/menu_tunnel.go`
- Create: `internal/cli/menu_logs.go`
- Create: `internal/cli/menu_tunnel_test.go`

**Interfaces:**
- Produces: `HandleTunnelMenu(bridge TUIBridge, reader *bufio.Reader) error`
- Produces: `HandleLogsMenu(bridge TUIBridge, reader *bufio.Reader) error`

- [ ] **Step 1: Write failing unit test for tunnel menu navigation**

```go
func TestHandleTunnelMenu_BackOption(t *testing.T) {
    // Input "0" or "b" -> returns nil without error
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/cli -run TestHandleTunnelMenu`  
Expected: FAIL

- [ ] **Step 3: Implement `menu_tunnel.go` and `menu_logs.go`**

Implement:
- Tunnel menu: Connect config selection, Disconnect active tunnel, Safe Mode confirm/rollback, Immediate health check.
- Logs menu: Formatted recent 50 logs table with colored timestamps and levels, Live log tailing loop with non-blocking exit.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run TestHandleTunnelMenu`  
Expected: PASS

- [ ] **Step 5: Commit Task 3**

```bash
git add internal/cli/menu_tunnel.go internal/cli/menu_logs.go internal/cli/menu_tunnel_test.go
git commit -m "feat(cli): implement tunnel operations and real-time logs TUI menus"
```

---

### Task 4: Configurations & Smart Routing Menus (Menu 2 & 3)

**Files:**
- Create: `internal/cli/menu_configs.go`
- Create: `internal/cli/menu_routing.go`
- Create: `internal/cli/menu_configs_test.go`

**Interfaces:**
- Produces: `HandleConfigsMenu(bridge TUIBridge, reader *bufio.Reader) error`
- Produces: `HandleRoutingMenu(bridge TUIBridge, reader *bufio.Reader) error`

- [ ] **Step 1: Write failing unit test for configs menu operations**

```go
func TestHandleConfigsMenu_ListAndImport(t *testing.T) {
    // Verify importing vless link and listing table
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/cli -run TestHandleConfigsMenu`  
Expected: FAIL

- [ ] **Step 3: Implement `menu_configs.go` and `menu_routing.go`**

Implement:
- Configs menu: Table listing with Ping/HTTP delay columns, Import from share link, Import from JSON file, Batch latency test across all nodes, Single node test, Delete node.
- Routing menu: Active rules table sorted by priority, Interactive rule creation wizard, One-click presets (Bypass Domestic, Block Ads), Delete rule.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run TestHandleConfigsMenu`  
Expected: PASS

- [ ] **Step 5: Commit Task 4**

```bash
git add internal/cli/menu_configs.go internal/cli/menu_routing.go internal/cli/menu_configs_test.go
git commit -m "feat(cli): implement configs management and smart routing TUI menus"
```

---

### Task 5: Settings, Core Engine Updater & Service Supervisor (Menu 5 & 6)

**Files:**
- Create: `internal/cli/menu_settings.go`
- Create: `internal/cli/menu_service.go`
- Create: `internal/cli/menu_settings_test.go`

**Interfaces:**
- Produces: `HandleSettingsMenu(bridge TUIBridge, reader *bufio.Reader) error`
- Produces: `HandleServiceMenu(bridge TUIBridge, reader *bufio.Reader) error`

- [ ] **Step 1: Write failing unit test for settings menu**

```go
func TestHandleSettingsMenu_BackOption(t *testing.T) {
    // Input "0" -> returns nil
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/cli -run TestHandleSettingsMenu`  
Expected: FAIL

- [ ] **Step 3: Implement `menu_settings.go` and `menu_service.go`**

Implement:
- Settings menu: Silent admin password change, Web port update, Safe Mode timeout update, Health check watchdog interval & probe URL, Core engine update inspector & trigger.
- Service menu: Systemd start, stop, restart, status (`systemctl status v2raynix`), enable on boot.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run TestHandleSettingsMenu`  
Expected: PASS

- [ ] **Step 5: Commit Task 5**

```bash
git add internal/cli/menu_settings.go internal/cli/menu_service.go internal/cli/menu_settings_test.go
git commit -m "feat(cli): implement settings, core updater, and service management TUI menus"
```

---

### Task 6: Interactive Root Integration, Version Bump & Full Verification

**Files:**
- Modify: `cmd/v2raynix/main.go:25, 30-65`
- Modify: `internal/cli/setup.go:180-320`
- Modify: `web/package.json:3`
- Modify: `README.md`, `README.fa.md`, `README.zh-CN.md`, `README.ru.md`

**Interfaces:**
- Running `v2raynix` without flags inside interactive terminal launches Full TUI.
- Running `v2raynix menu` or `v2raynix setup` launches Full TUI.
- Running `v2raynix -port 2080` or `v2raynix setup --user ...` maintains daemon/script behavior.
- Version string bumped to `v0.9.2-beta`.

- [ ] **Step 1: Wire root command and subcommands in `cmd/v2raynix/main.go` and `setup.go`**

Check `term.IsTerminal` on stdin when no flags are supplied. If interactive, invoke Full TUI main loop.

- [ ] **Step 2: Run complete backend unit test suite**

Run: `go test -v ./...`  
Expected: 100% PASS.

- [ ] **Step 3: Rebuild frontend bundle & compile standalone binary**

Run: `cd web && npm run build && cd .. && go build -ldflags="-s -w" -o bin/v2raynix ./cmd/v2raynix`  
Verify: `./bin/v2raynix -version` prints `V2Raynix v0.9.2-beta`.

- [ ] **Step 4: Update version badge across all 4 README files**

- [ ] **Step 5: Commit and tag v0.9.2-beta**

```bash
git add .
git commit -m "chore(release): bump version to v0.9.2-beta with comprehensive Full TUI"
git tag -a -f v0.9.2-beta -m "Release v0.9.2-beta: Full Terminal User Interface (TUI)"
```
