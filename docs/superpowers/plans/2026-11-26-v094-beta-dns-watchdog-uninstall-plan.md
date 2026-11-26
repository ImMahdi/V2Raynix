# V2Raynix v0.9.4-beta Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver V2Raynix release `v0.9.4-beta` featuring DNS-over-Tunnel anti-censorship resolution, Watchdog decoupling from Safe Mode, in-flight Web connection conntrack protection, installer pipe hardening with universal `/usr/bin/` symlinks, and a fail-safe uninstaller across CLI, TUI, and shell installer.

**Architecture:** 
1. New `internal/network/dns.go` manages system-wide DNS routing via `resolvectl` (and `/etc/resolv.conf` fallback) when tunnel connects, steering DNS queries into `tun0` to bypass ISP DNS poisoning.
2. `internal/core/supervisor.go` decouples internal automated restarts from Safe Mode (internal restarts never re-arm the 120s timer), expands Watchdog probe sensitivity, and caches resolved proxy server IPs.
3. `internal/network/routing.go` updates iptables mangle PREROUTING to protect `ESTABLISHED,RELATED` in-flight connections alongside reverse proxy ports, preventing connect timeouts.
4. `internal/cli/uninstall.go` and `internal/network/cleanup.go` execute zero-lockout system uninstallation (dropping priority 2000 first) across CLI (`v2raynix uninstall`), TUI (Menu 6 option 4), and `install.sh --uninstall`.
5. `scripts/install.sh` and `cmd/v2raynix/main.go` resolve absolute executable paths (`os.Executable()`), eliminate `/dev/fd/63` failures via `curl ... | sudo bash`, and link binaries to `/usr/bin/`.

**Tech Stack:** Go 1.22+, Linux Kernel Policy Routing (`ip rule`, `ip route` table 100), Netfilter/iptables, `systemd-resolved` / `resolvectl`, Bash, React / Vite.

**Spec:** `docs/superpowers/specs/2026-11-26-v094-beta-dns-watchdog-uninstall-design.md`

## Global Constraints
- Target version: `0.9.4-beta`
- All Go code must compile with `-trimpath -ldflags="-s -w"` and maintain 100% pass on `go test ./...`.
- Zero SSH lockouts during tunnel connect, failover, or uninstall. Priority 2000 rule must always be deleted first during teardown.
- Default setting for `DNSOverTunnel`: `true`; default `RemoteDNS`: `"1.1.1.1,8.8.8.8"`.

## Review Focus
1. **DNS Revert on Unexpected Crash:** If daemon dies or crashes, DNS state must revert to prevent host internet from stalling.
2. **Safe Mode Decoupling:** Auto-restart from Watchdog must NOT start `SafeModeController` countdown.
3. **In-Flight Connect Response:** `POST /api/tunnel/connect` HTTP response must return immediately without browser client timeout.
4. **Symlink Coexistence:** Universal `/usr/bin/` symlinks must not overwrite unrelated third-party tools if user opted out.
5. **Zero-Lockout Uninstall:** Executing `v2raynix uninstall` over an active SSH session must maintain SSH connectivity throughout.

---

### Task 1: Store Model & Settings for DNS-over-Tunnel

**Files:**
- Modify: `internal/store/models.go`
- Modify: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `store.SystemSettings`
- Produces: `DNSOverTunnel bool` and `RemoteDNS string` in `store.SystemSettings`

- [ ] **Step 1: Write failing test in `internal/store/store_test.go`**
  Add `TestStore_DNSSettingsDefaultsAndPersistence`: verify new store initializes with `DNSOverTunnel: true` and `RemoteDNS: "1.1.1.1,8.8.8.8"`, and saving modified values persists across store reloads.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/store -run TestStore_DNSSettingsDefaultsAndPersistence`
  Expected: FAIL (fields undefined)
- [ ] **Step 3: Update `SystemSettings` in `internal/store/models.go` and `store.go`**
  Add `DNSOverTunnel bool` and `RemoteDNS string` to struct. In `store.go:New`, set defaults if empty.
- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./internal/store -run TestStore_DNSSettingsDefaultsAndPersistence`
  Expected: PASS
- [ ] **Step 5: Commit**
  `git add internal/store/models.go internal/store/store.go internal/store/store_test.go`
  `git commit -m "feat(store): add DNSOverTunnel and RemoteDNS settings"`

---

### Task 2: System DNS Manager (`internal/network/dns.go`)

**Files:**
- Create: `internal/network/dns.go`
- Create: `internal/network/dns_test.go`

**Interfaces:**
- Consumes: `remoteDNS string` (comma-delimited IP list)
- Produces: `ApplyDNSOverTunnel(remoteDNS string) error` and `RevertDNSOverTunnel() error`

- [ ] **Step 1: Write failing test in `internal/network/dns_test.go`**
  Add unit tests for `ParseDNSServers` (validates IPs, strips spaces, defaults to 1.1.1.1/8.8.8.8), and mock runner testing `resolvectl` command construction (`resolvectl dns tun0 ...`, `domain tun0 "~."`, `default-route tun0 yes`, `flush-caches`) and fallback `/etc/resolv.conf` backup and write.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/network -run TestDNS`
  Expected: FAIL (package/functions undefined)
- [ ] **Step 3: Implement `internal/network/dns.go`**
  Implement `ApplyDNSOverTunnel(remoteDNS string) error` and `RevertDNSOverTunnel() error`. Check for `resolvectl` existence; if present, configure `tun0` interface DNS; if not, manage `/etc/resolv.conf` with `/etc/v2raynix/resolv.conf.bak`.
- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./internal/network -run TestDNS`
  Expected: PASS
- [ ] **Step 5: Commit**
  `git add internal/network/dns.go internal/network/dns_test.go`
  `git commit -m "feat(network): implement DNS-over-Tunnel manager with resolvectl and resolv.conf fallback"`

---

### Task 3: Core Supervisor DNS Hook & Tunnel Lifecycle

**Files:**
- Modify: `internal/core/supervisor.go`
- Modify: `internal/core/supervisor_test.go`

**Interfaces:**
- Consumes: `network.ApplyDNSOverTunnel`, `network.RevertDNSOverTunnel`, `store.SystemSettings`
- Produces: Integrated DNS lifecycle in `StartTunnel` and `StopTunnel`

- [ ] **Step 1: Write failing test in `internal/core/supervisor_test.go`**
  Add test verifying that when tunnel starts with `DNSOverTunnel: true`, DNS configuration helper is executed; and on `StopTunnel`, DNS is reverted.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/core -run TestSupervisor_DNSLifecycle`
  Expected: FAIL
- [ ] **Step 3: Update `internal/core/supervisor.go`**
  In `StartTunnel`: after routing rules are applied and `tun2socks` started, query settings; if `DNSOverTunnel`, call `network.ApplyDNSOverTunnel(settings.RemoteDNS)`.
  In `stopTunnelLocked`: call `network.RevertDNSOverTunnel()`.
- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./internal/core -run TestSupervisor_DNSLifecycle`
  Expected: PASS
- [ ] **Step 5: Commit**
  `git add internal/core/supervisor.go internal/core/supervisor_test.go`
  `git commit -m "feat(core): integrate DNS-over-Tunnel lifecycle into tunnel startup and teardown"`

---

### Task 4: Web UI & TUI DNS Settings Controls

**Files:**
- Modify: `web/src/views/SettingsView.jsx`
- Modify: `internal/cli/menu_settings.go`
- Modify: `internal/cli/menu_settings_test.go`

**Interfaces:**
- Consumes: `/api/settings` GET and POST with `dnsOverTunnel` and `remoteDNS`
- Produces: Interactive toggles and inputs in Web UI and TUI Menu 5

- [ ] **Step 1: Write failing test in `internal/cli/menu_settings_test.go`**
  Add `TestHandleSettingsMenu_ConfigureDNSOverTunnel`: tests viewing and toggling DNS over tunnel and setting custom DNS servers in TUI.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/cli -run TestHandleSettingsMenu_ConfigureDNSOverTunnel`
  Expected: FAIL
- [ ] **Step 3: Implement Web UI and TUI controls**
  In `web/src/views/SettingsView.jsx`: add "DNS & Censorship Protection" card with toggle and upstream DNS input.
  In `internal/cli/menu_settings.go`: add option `[4] Configure DNS Over Tunnel (Anti-Poisoning)`.
- [ ] **Step 4: Run tests and frontend build**
  Run: `go test -v ./internal/cli -run TestHandleSettingsMenu_ConfigureDNSOverTunnel`
  Run: `npm --prefix web run build`
  Expected: PASS
- [ ] **Step 5: Commit**
  `git add web/src/views/SettingsView.jsx internal/cli/menu_settings.go internal/cli/menu_settings_test.go`
  `git commit -m "feat(ui): add DNS-over-Tunnel controls in Web Dashboard and TUI"`

---

### Task 5: Watchdog Safe Mode Decoupling & Sensitivity Hardening

**Files:**
- Modify: `internal/core/supervisor.go`
- Modify: `internal/core/watchdog.go`
- Modify: `internal/core/supervisor_watchdog_test.go`
- Modify: `internal/cli/commands.go`

**Interfaces:**
- Consumes: `Supervisor.RestartTunnel()`
- Produces: Internal auto-restarts that bypass Safe Mode; hardened probe retries (10s timeout, 3 retries, exponential backoff)

- [ ] **Step 1: Write failing test in `internal/core/supervisor_watchdog_test.go`**
  Add `TestWatchdog_AutoRestartDoesNotArmSafeMode`: verify that when `s.RestartTunnel()` executes, `s.safeMode` is NOT armed and does not auto-disconnect after safe mode timeout.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/core -run TestWatchdog_AutoRestartDoesNotArmSafeMode`
  Expected: FAIL
- [ ] **Step 3: Implement decoupling in `supervisor.go` and `watchdog.go`**
  Refactor `startTunnelInternal(cfg *store.ConfigItem, armSafeMode bool) error`.
  In `StartTunnel(cfg)`: pass `armSafeMode: true`.
  In `RestartTunnel()`: pass `armSafeMode: false`.
  In `watchdog.go`: increase probe timeout to 10s, 3 retries with backoff (5s, 10s).
  In `internal/cli/commands.go`: `v2raynix connect` automatically confirms safe mode unless `--safe-mode` flag is explicitly passed.
- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./internal/core -run TestWatchdog_AutoRestartDoesNotArmSafeMode`
  Run: `go test -v ./internal/cli -run TestHandleCLICommand_Connect`
  Expected: PASS
- [ ] **Step 5: Commit**
  `git add internal/core/supervisor.go internal/core/watchdog.go internal/core/supervisor_watchdog_test.go internal/cli/commands.go`
  `git commit -m "fix(core): decouple watchdog auto-restart from safe mode and harden probe tolerance"`

---

### Task 6: In-Flight Connection & Conntrack Protection

**Files:**
- Modify: `internal/network/routing.go`
- Modify: `internal/network/routing_test.go`
- Modify: `web/src/services/api.js`

**Interfaces:**
- Consumes: Netfilter mangle PREROUTING table, Web API client
- Produces: `NEW,ESTABLISHED,RELATED` connection mark preservation and 30s client connect timeout

- [ ] **Step 1: Write failing test in `internal/network/routing_test.go`**
  Add test checking `BuildRoutingCommands` includes `-m conntrack --ctstate NEW,ESTABLISHED,RELATED` for `V2RAYNIX_INBOUND` jump and protects reverse proxy ports (80/443).
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/network -run TestBuildRoutingCommands_ConntrackEstablished`
  Expected: FAIL
- [ ] **Step 3: Update `internal/network/routing.go` and `web/src/services/api.js`**
  In `routing.go`: update PREROUTING conntrack rule to match `NEW,ESTABLISHED,RELATED`. Add bypass rules for ports 80/443 if webPort != 80/443.
  In `web/src/services/api.js`: set `timeout: 30000` in `connectTunnel` and `disconnectTunnel`.
- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./internal/network -run TestBuildRoutingCommands_ConntrackEstablished`
  Expected: PASS
- [ ] **Step 5: Commit**
  `git add internal/network/routing.go internal/network/routing_test.go web/src/services/api.js`
  `git commit -m "fix(network): protect in-flight established connections and expand connect timeout to 30s"`

---

### Task 7: Unconditional Kernel Route & Mangle Teardown Helper

**Files:**
- Create: `internal/network/cleanup.go`
- Create: `internal/network/cleanup_test.go`

**Interfaces:**
- Consumes: Kernel routing tables and iptables
- Produces: `ForceTeardownNetwork(sshPort, webPort int) error`

- [ ] **Step 1: Write failing test in `internal/network/cleanup_test.go`**
  Add unit test verifying `BuildForceTeardownCommands` generates the exact fail-safe command sequence: priority 2000 dropped first, then table 100 flushed, tun0 deleted, priority rules 999-1013 removed, and iptables `V2RAYNIX_INBOUND` flushed and unlinked.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/network -run TestForceTeardown`
  Expected: FAIL
- [ ] **Step 3: Implement `internal/network/cleanup.go`**
  Implement `BuildForceTeardownCommands` and `ForceTeardownNetwork(sshPort, webPort int) error` with execution resiliency (ignoring non-zero returns if rule does not exist).
- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./internal/network -run TestForceTeardown`
  Expected: PASS
- [ ] **Step 5: Commit**
  `git add internal/network/cleanup.go internal/network/cleanup_test.go`
  `git commit -m "feat(network): implement zero-lockout unconditional network teardown engine"`

---

### Task 8: Native CLI & TUI Uninstall Command

**Files:**
- Create: `internal/cli/uninstall.go`
- Create: `internal/cli/uninstall_test.go`
- Modify: `internal/cli/commands.go`
- Modify: `internal/cli/commands_test.go`
- Modify: `internal/cli/menu_service.go`
- Modify: `cmd/v2raynix/main.go`

**Interfaces:**
- Consumes: `network.ForceTeardownNetwork`, `store.Store`
- Produces: `v2raynix uninstall [--purge] [--keep-data] [--remove-cores] [--yes]` and TUI Menu 6 Option 4

- [ ] **Step 1: Write failing test in `internal/cli/uninstall_test.go` and `commands_test.go`**
  Add unit tests for `RunUninstall` testing flag parsing (`--purge`, `--keep-data`, `--yes`), confirmation prompts, service stopping, data removal vs retention, and binary deletion.
- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v ./internal/cli -run TestUninstall`
  Expected: FAIL
- [ ] **Step 3: Implement `internal/cli/uninstall.go` and wire into `commands.go`, `menu_service.go`, `main.go`**
  Implement uninstallation flow with clear step-by-step progress logging (`[1/7] ... [OK]`). Wire into CLI command dispatcher and TUI Menu 6.
- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./internal/cli -run TestUninstall`
  Run: `go test -v ./internal/cli -run TestHandleCLICommand_Uninstall`
  Expected: PASS
- [ ] **Step 5: Commit**
  `git add internal/cli/uninstall.go internal/cli/uninstall_test.go internal/cli/commands.go internal/cli/commands_test.go internal/cli/menu_service.go cmd/v2raynix/main.go`
  `git commit -m "feat(cli): implement native fail-safe uninstall command and TUI integration"`

---

### Task 9: Installer Modernization & PATH Parity

**Files:**
- Modify: `scripts/install.sh`
- Modify: `cmd/v2raynix/main.go`
- Modify: `README.md`, `README.fa.md`, `README.zh-CN.md`, `README.ru.md`

**Interfaces:**
- Consumes: `scripts/install.sh` arguments (`--uninstall`, `--purge`), system package managers
- Produces: Piped installer execution without `/dev/fd/63`, `/usr/bin/` symlinks, self-elevation with `os.Executable()`, and `--uninstall` switch

- [ ] **Step 1: Update `scripts/install.sh` with auto-elevation, `/usr/bin/` symlinks, and `--uninstall` fallback**
  - Fix non-root execution check: auto-elevate if run from file; if piped, display clear `curl ... | sudo bash` instructions.
  - Install dependencies non-interactively: `export DEBIAN_FRONTEND=noninteractive`.
  - Add universal symlinks in `/usr/bin/` for `v2raynix`, `xray`, `tun2socks`.
  - Implement `--uninstall` switch: if `/usr/local/bin/v2raynix` exists, delegate to `v2raynix uninstall "$@"`; else run bash fallback teardown.
- [ ] **Step 2: Update `cmd/v2raynix/main.go` self-elevation**
  Use `os.Executable()` to pass absolute path to `sudo` when re-executing unprivileged interactive sessions.
- [ ] **Step 3: Update all 4 README files**
  Update installation one-liner to `curl -fsSL https://raw.githubusercontent.com/ImMahdi/V2Raynix/main/scripts/install.sh | sudo bash`.
  Add uninstallation section documenting `v2raynix uninstall` and `install.sh --uninstall`.
- [ ] **Step 4: Verify script syntax and tests**
  Run: `bash -n scripts/install.sh`
  Run: `go test -v ./...`
  Expected: PASS
- [ ] **Step 5: Commit**
  `git add scripts/install.sh cmd/v2raynix/main.go README.md README.fa.md README.zh-CN.md README.ru.md`
  `git commit -m "fix(installer): eliminate /dev/fd/63 error, link to /usr/bin, and add --uninstall switch"`

---

### Task 10: Version Bump (`v0.9.4-beta`), Compilation & Live Server Deployment

**Files:**
- Modify: `cmd/v2raynix/main.go`
- Modify: `web/package.json`
- Modify: `README*.md` (version badges)

**Interfaces:**
- Consumes: All completed subsystem tasks
- Produces: Compiled Linux & Windows binaries, git release tag `v0.9.4-beta`, and live server verification

- [ ] **Step 1: Bump version constants**
  In `cmd/v2raynix/main.go`: `Version = "0.9.4-beta"`, `BuildTime = "2026-11-26"`.
  In `web/package.json`: `"version": "0.9.4-beta"`.
  Update version badge to `v0.9.4--beta` across all 4 READMEs.
- [ ] **Step 2: Build frontend and run full test suite**
  Run: `npm --prefix web run build`
  Run: `go test -count=1 -v ./...`
  Expected: 100% PASS
- [ ] **Step 3: Compile release binaries with `-trimpath` and `-ldflags="-s -w"`**
  Linux: `bin/v2raynix`
  Windows: `bin/v2raynix.exe`
  Verify version output: `.\bin\v2raynix.exe -version`.
- [ ] **Step 4: Commit, tag, and push**
  `git add .`
  `git commit -m "chore(release): bump version to v0.9.4-beta with DNS-over-tunnel, watchdog resilience, and uninstaller"`
  `git tag -a -f v0.9.4-beta -m "Release v0.9.4-beta: DNS-over-Tunnel, Watchdog Decoupling, Zero-Timeout Connect, and Universal Uninstaller"`
  `git push origin main && git push origin --tags --force`
- [ ] **Step 5: Deploy to live server (`192.168.254.80:23313`) & verify**
  Upload binary, restart service, verify `v2raynix -version`, test `v2raynix health`, verify Telegram / DNS resolution over tunnel.
- [ ] **Step 6: Update GitHub release notes (concise English, Latest tag)**
  Update release notes via GitHub API with concise bullet points and set `prerelease: false`, `make_latest: "true"`.
