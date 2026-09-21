# Inbound Connection Preservation via Netfilter Connmark Policy Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable automatic, transparent, and bidirectional connectivity for any inbound service (e.g. Nginx on ports 80/443, Docker container ports) without manual port whitelist maintenance or asymmetric routing drops, while preserving 100% full-tunneling for outbound requests and hard anti-lockout for SSH and Web UI.

**Architecture:** Utilize Netfilter Connection Tracking (`connmark`) in a dedicated `mangle` chain (`V2RAYNIX_INBOUND`) to mark connections arriving on the physical interface with `0x52`. Restore the mark on outgoing reply packets (`OUTPUT`), and route marked packets via `table main` (`priority 1008`). Outbound-initiated traffic has no mark and routes into `tun0` (`priority 2000`).

**Tech Stack:** Go (1.22+), Linux Netfilter (`iptables -t mangle`, `connmark`), `iproute2` (`ip rule`, `ip route`), `tun2socks`, `xray`.

**Spec:** Two-Tier Hybrid Routing Extension for Automatic Inbound Preservation.

---

## Global Constraints

- Never disrupt SSH access on port 22 or 23313; explicit `ip rule` entries at priorities 1000/1001 remain active as hard defense-in-depth.
- Web UI on port 2080 must remain explicitly bypassed via kernel rules (priorities 1002/1003).
- Dedicated iptables chain `V2RAYNIX_INBOUND` must be used to ensure 100% atomic creation and cleanup without disturbing existing firewall rules (UFW/Docker).
- Fwmark values: Xray outbound = `0x51` (81), Inbound conntrack = `0x52` (82).
- Zero residual rules after tunnel stop or Safe Mode rollback.

## Review Focus

1. **Idempotency on startup:** Starting the tunnel after a crash or ungraceful shutdown must flush any existing `V2RAYNIX_INBOUND` or `0x52` rules without failing.
2. **Asymmetric routing elimination:** Inbound HTTP requests on arbitrary ports (e.g., 80, 443, 8080) must receive replies via the physical interface, completing the TCP 3-way handshake.
3. **Outbound isolation:** Outbound requests originating from the server (e.g. `curl https://icanhazip.com`) must NOT be marked `0x52` and must exit through `tun0` -> proxy.
4. **Clean teardown:** Tunnel disconnect must remove `V2RAYNIX_INBOUND` chain and `fwmark 0x52` rule cleanly.
5. **Safe Mode integration:** If safe mode times out without confirmation, conntrack rules and chain must be fully rolled back.

---

### Task 1: Netfilter Connmark Policy Routing Commands & Unit Tests

**Files:**
- Modify: `internal/network/routing.go`
- Modify: `internal/network/routing_test.go`

**Interfaces:**
- Constants: `InboundFwmark = "0x52"`, `InboundPriority = 1008`, `InboundChain = "V2RAYNIX_INBOUND"`
- Functions: `BuildRoutingCommands(...) []string`, `BuildCleanupCommands(...) []string`

- [ ] **Step 1: Write the failing tests**
  In `internal/network/routing_test.go`:
  - Assert that `BuildRoutingCommands` includes:
    - `iptables -t mangle -N V2RAYNIX_INBOUND`
    - `iptables -t mangle -F V2RAYNIX_INBOUND`
    - `iptables -t mangle -A PREROUTING -i <iface> -m conntrack --ctstate NEW -j V2RAYNIX_INBOUND`
    - `iptables -t mangle -A V2RAYNIX_INBOUND -j CONNMARK --set-mark 0x52`
    - `iptables -t mangle -A OUTPUT -m connmark --mark 0x52 -j CONNMARK --restore-mark`
    - `ip rule add fwmark 0x52 table main priority 1008`
  - Assert that `BuildCleanupCommands` includes:
    - `iptables -t mangle -D PREROUTING -i <iface> -m conntrack --ctstate NEW -j V2RAYNIX_INBOUND`
    - `iptables -t mangle -D OUTPUT -m connmark --mark 0x52 -j CONNMARK --restore-mark`
    - `iptables -t mangle -F V2RAYNIX_INBOUND`
    - `iptables -t mangle -X V2RAYNIX_INBOUND`
    - `ip rule del fwmark 0x52 table main`

- [ ] **Step 2: Run test to verify it fails (Red)**
  Run: `go test -v ./internal/network`
  Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**
  In `internal/network/routing.go`:
  - Define `InboundFwmark = "0x52"`, `InboundPriority = 1008`, `InboundChain = "V2RAYNIX_INBOUND"`.
  - Inject iptables commands and `ip rule add fwmark 0x52 table main priority 1008` into `BuildRoutingCommands`.
  - Inject cleanup commands (delete rule, jump rules, flush and delete chain) into `BuildCleanupCommands`.

- [ ] **Step 4: Run test to verify it passes (Green)**
  Run: `go test -v ./internal/network`
  Expected: PASS.

- [ ] **Step 5: Commit**
  `git commit -am "feat(network): implement netfilter connmark policy routing for inbound traffic preservation"`

---

### Task 2: Supervisor Coordination & Verification

**Files:**
- Modify: `internal/core/supervisor.go`
- Test: `internal/core/supervisor_test.go`

- [ ] **Step 1: Verify startup and teardown sequence in supervisor**
  Ensure that when `StartTunnel` is called, `cleanupCmds` are executed first (cleaning any leftover iptables chains), then routing commands are applied.
  Ensure that `stopTunnelLocked` cleanly runs `BuildCleanupCommands`.

- [ ] **Step 2: Run core tests**
  Run: `go test -v ./internal/core`
  Expected: PASS.

- [ ] **Step 3: Commit**
  `git commit -am "refactor(core): verify connmark lifecycle in supervisor process management"`

---

### Task 3: Compilation, Deployment & Live Multi-Scenario Verification

**Files:**
- Compile: `bin/v2raynix-linux-amd64`
- Deploy: Remote server `192.168.254.80` (`/usr/local/bin/v2raynix`)

- [ ] **Step 1: Recompile Linux binary**
  `go build -v -o bin/v2raynix-linux-amd64 ./cmd/v2raynix`

- [ ] **Step 2: Deploy to test server and restart service**
  Upload via SFTP, replace `/usr/local/bin/v2raynix`, and restart `v2raynix` service.

- [ ] **Step 3: Connect tunnel via API**
  Call `/api/tunnel/connect` and confirm Safe Mode.

- [ ] **Step 4: Live Verification - Scenario A: Inbound Web Service Accessibility**
  Start a dummy HTTP server on port 8080:
  `python3 -m http.server 8080`
  From outside the tunnel (e.g. from the management machine or local LAN), run:
  `curl -I http://192.168.254.80:8080`
  Must return `HTTP/1.0 200 OK`!

- [ ] **Step 5: Live Verification - Scenario B: Outbound Tunneling Integrity**
  From the test server, run:
  `curl -4 https://icanhazip.com`
  Must return the foreign proxy IP (`63.176.97.83` or `2a05:...`), proving outbound traffic remains 100% tunneled.

- [ ] **Step 6: Live Verification - Scenario C: Teardown Cleanliness**
  Disconnect tunnel via API.
  Run `iptables -t mangle -L V2RAYNIX_INBOUND 2>&1` -> Must report `No chain/target/match by that name`.
  Run `ip rule show` -> Must show no residual `0x52` rules.
