# Two-Tier Hybrid Routing (tun2socks + Xray) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a production-grade Two-Tier Hybrid Routing architecture in V2Raynix: Kernel-level policy routing via `tun2socks` (anti-lockout, private subnet bypass, process isolation) combined with Application-level smart routing via Xray (domain sniffing, GeoSite/GeoIP direct bypass, sockopt fwmark anti-loop).

**Architecture:** 
- **Tier 1 (Kernel/OS):** Creates `tun0` via `ip tuntap`, assigns IP `198.18.0.1/15`, creates policy routing rules (`table 100`) for non-fwmarked traffic (`not fwmark 0x51`), while pinning SSH (22, 23313), Web UI (2080), private RFC1918 networks, and upstream proxy server IP to `table main`.
- **Bridge:** `tun2socks` translates L3 IP packets from `tun0` to L4/L5 SOCKS5 on `127.0.0.1:10808`.
- **Tier 2 (Xray Engine):** Xray receives SOCKS5 traffic, sniffs HTTP/TLS/QUIC domains, applies Iran/direct bypass rules, and marks all outbound packets to the remote server with `sockopt.mark = 81` (0x51) so they bypass Tier 1 routing rules without infinite loops.

**Tech Stack:** Go 1.23+, Linux Netlink/iproute2, Xray-core v26+, tun2socks v2.7+, React 18 frontend.

**Spec:** `docs/superpowers/specs/2026-09-20-v2raynix-design.md`

## Global Constraints
- Target platform: Ubuntu 20.04/22.04/24.04 & Debian 11/12 (amd64 / arm64).
- Anti-lockout guarantee: Remote SSH connections (port 22 and custom port 23313) and Web UI (port 2080) must NEVER be dropped or routed through the proxy.
- Single binary deployment: Embedded web assets via `go:embed`.
- Atomic thread-safe storage at `/etc/v2raynix/v2raynix.json`.
- Strict TDD: Write failing tests before modifying implementation code.

## Review Focus
1. `tun0` device creation failure on modern kernels (`ip link add type tun` vs `ip tuntap add mode tun`).
2. Routing loop caused by Xray outbound packets re-entering `tun0` (prevented by `sockopt.mark = 81`).
3. Disconnected SSH session when default route changes (prevented by policy rules for sport/dport 23313 at priority 1000/1001).
4. DNS leak and DNS loop resolution (prevented by routing UDP 53 through tunnel).
5. Safe Mode auto-rollback timer if user cannot confirm network changes within 120s.

---

### Task 1: Xray Generator Outbound Sockopt (Fwmark 81) & Inbound Sniffing

**Files:**
- Modify: `internal/configmgr/generator.go`
- Test: `internal/configmgr/generator_test.go`

**Interfaces:**
- Consumes: `store.ConfigItem`, `store.RoutingRule`
- Produces: `GenerateXrayConfig(cfg *store.ConfigItem, rules []*store.RoutingRule, socksPort, httpPort int) (string, error)` where outbound streamSettings includes `sockopt: {"mark": 81}` and inbound SOCKS includes sniffing.

- [ ] **Step 1: Write the failing test**
  Add `TestGenerateXrayConfig_SockoptMarkAndSniffing` to `internal/configmgr/generator_test.go` asserting that generated JSON contains `streamSettings.sockopt.mark == 81` on proxy outbound and `sniffing.enabled == true` on SOCKS inbound.

- [ ] **Step 2: Run test to verify it fails**
  Run: `$env:PATH="C:\Users\Maad\go_sdk\go\bin;" + $env:PATH; go test -v -run TestGenerateXrayConfig_SockoptMarkAndSniffing ./internal/configmgr`
  Expected: FAIL (sockopt mark missing or sniffing missing).

- [ ] **Step 3: Write minimal implementation**
  In `internal/configmgr/generator.go`, add `SockoptConfig{Mark: 81}` to proxy outbound `StreamSettings` and enable sniffing on inbound.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v -run TestGenerateXrayConfig_SockoptMarkAndSniffing ./internal/configmgr`
  Expected: PASS.

- [ ] **Step 5: Commit**
  `git commit -am "feat(configmgr): add sockopt fwmark 81 and sniffing to generated Xray configs"`

---

### Task 2: Linux Routing Commands (`ip tuntap`) & Explicit Proxy Bypass

**Files:**
- Modify: `internal/network/routing.go`
- Test: `internal/network/routing_test.go`

**Interfaces:**
- Consumes: `BuildRoutingCommands(remoteProxyIP, defaultIface, defaultGw string, sshPort int, webPort int) []string`
- Produces: Ordered shell commands using `ip tuntap add dev tun0 mode tun` (with fallback/idempotent check), `ip addr add 198.18.0.1/15 dev tun0`, `ip link set dev tun0 up`, bypass rule for `remoteProxyIP` at priority 999, and anti-lockout rules.

- [ ] **Step 1: Write the failing test**
  Update `internal/network/routing_test.go` to assert that:
  - First command creates `tun0` using `ip tuntap add mode tun` or `ip tuntap add dev tun0 mode tun`.
  - A high-priority rule `ip rule add to <remoteProxyIP> table main priority 999` exists.
  - Teardown commands clean up both the rules and the tuntap device.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test -v -run TestBuildRoutingCommands ./internal/network`
  Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**
  In `internal/network/routing.go`:
  - Replace `ip link add dev %s type tun` with `ip tuntap add dev %s mode tun`.
  - Add `fmt.Sprintf("ip rule add to %s table main priority 999", remoteProxyIP)` to `BuildRoutingCommands`.
  - Add cleanup for rule 999 and `ip tuntap del dev %s mode tun` to `BuildCleanupCommands`.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./internal/network`
  Expected: PASS.

- [ ] **Step 5: Commit**
  `git commit -am "feat(network): use ip tuntap and add explicit remote proxy ip bypass rule"`

---

### Task 3: Supervisor Process Coordination & Sequential Start

**Files:**
- Modify: `internal/core/supervisor.go`
- Test: `internal/core/supervisor_test.go`

**Interfaces:**
- Consumes: `StartTunnel(cfg *store.ConfigItem) error`, `StopTunnel() error`
- Produces: Strict sequence: 1) Write config, 2) Start Xray, 3) Run routing commands (creating `tun0` and rules), 4) Start `tun2socks` bound to `tun0`.

- [ ] **Step 1: Write the test**
  Verify `TestSupervisor_Lifecycle` in `internal/core/supervisor_test.go` passes and ensure mock mode and real mode paths are cleanly separated.

- [ ] **Step 2: Run tests**
  Run: `go test -v ./internal/core`
  Expected: PASS.

- [ ] **Step 3: Commit**
  `git commit -am "refactor(core): ensure robust startup order and teardown in supervisor"`

---

### Task 4: Compilation, Deployment & Live Verification on Ubuntu Server

**Files:**
- Compile: `bin/v2raynix-linux-amd64`
- Deploy: Remote server `192.168.254.80:23313` (`/usr/local/bin/v2raynix`)

- [ ] **Step 1: Recompile Linux binary natively**
  Command: `$env:PATH="C:\Users\Maad\go_sdk\go\bin;" + $env:PATH; $env:GOOS="linux"; $env:GOARCH="amd64"; go build -v -o bin/v2raynix-linux-amd64 ./cmd/v2raynix`

- [ ] **Step 2: Upload binary via SFTP and restart systemd service**
  Use `scratch/sftp_upload.py` to upload to `/home/gemini/v2raynix-bin` and replace `/usr/local/bin/v2raynix`, then `systemctl restart v2raynix`.

- [ ] **Step 3: Trigger Live Tunnel Connect via API**
  Send `POST /api/tunnel/connect` on `http://192.168.254.80:2080`.

- [ ] **Step 4: Live Verification on Server (Evidence Collection)**
  Execute remote commands:
  - `ip a show dev tun0` -> Must show `UP` with `198.18.0.1/15`.
  - `ip route show table 100` -> Must show `default dev tun0`.
  - `curl -4 https://icanhazip.com` -> Must return the foreign proxy IP (NOT Iran `151.239.25.137`)!
  - SSH connectivity check -> Connection to `192.168.254.80:23313` must remain completely uninterrupted.
