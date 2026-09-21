# Domain Briefing 01: Network & Policy Routing Subsystem

> **Subagent Role:** Principal Linux Network & Kernel Systems Auditor  
> **Audited Files:** `internal/network/routing.go`, `internal/network/safemode.go`, `internal/network/routing_test.go`  
> **Output Report File:** `docs/audit/reports/01-network-routing-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Deep Knowledge Transfer & Architecture

The `network` package is the critical kernel-interfacing layer of V2Raynix. It configures Linux policy routing, TUN virtual network devices, netfilter connection tracking, and anti-lockout safeguards to redirect system outbound traffic through `tun2socks` and `xray-core` without disconnecting active SSH or Web UI management sessions.

### Key Architectural Concepts:
1. **Virtual TUN Device (`tun0`):**
   - Device IP: `198.18.0.1/15` (RFC 2544 benchmark testing range).
   - Created with `ip tuntap add dev tun0 mode tun` and activated via `ip link set dev tun0 up`.
   - Tun2socks binds to `tun0` and proxies layer-3 packets to a local SOCKS5 inbound (`127.0.0.1:10808`).
2. **Linux Policy Routing Table (`TableID = 100`):**
   - Default route in Table 100 points directly to `tun0`: `ip route add default dev tun0 table 100`.
   - Catch-all policy rule routes unmarked outbound traffic: `ip rule add not fwmark 0x51 table 100 priority 2000`.
3. **Inbound Connection Marking & Asymmetric Routing Prevention:**
   - Linux servers with single physical NICs suffer asymmetric routing drops if reply packets for inbound connections (SSH, Web UI, API) get routed into `tun0`.
   - Solution: Netfilter connection tracking (`conntrack`) and mark `0x52`:
     - Inbound traffic on default interface receives `CONNMARK --set-mark 0x52` on `NEW` connections in custom mangle chain `V2RAYNIX_INBOUND`.
     - In `OUTPUT`, `CONNMARK --restore-mark` restores mark `0x52` onto reply packets.
     - Policy routing rule `ip rule add fwmark 0x52 table main priority 1008` directs reply packets back through the physical gateway.
4. **Anti-Lockout Safeguards:**
   - Remote proxy server IP bypass: `ip rule add to <remoteProxyIP> table main priority 999` and explicit static route `ip route add <remoteProxyIP>/32 via <gw> dev <iface>`.
   - SSH and Web management port bypass rules (priority 1000-1003).
   - RFC1918 private subnets bypass (10/8, 172.16/12, 192.168/16, 127/8) priority 1010-1013.
5. **SafeMode Controller (`safemode.go`):**
   - Timer-based deadlock breaker: starts countdown (default 120s). If administrator does not confirm via Web UI or API, auto-executes rollback teardown commands.

---

## 2. Concerns, Pitfalls & Critical Warnings

When auditing this domain, pay hyper-vigilant attention to these failure modes:
1. **Command Injection & Shell Quoting:**
   `ExecuteCommands` passes raw string slices to `sh -c cmd`. If `remoteProxyIP`, `iface`, `gw`, or port variables contain unsanitized input or metacharacters, arbitrary shell command execution occurs as root.
2. **Missing Prerequisite Tooling Checks:**
   Commands rely on `ip`, `iptables`, `sh`. Are there pre-flight checks verifying kernel modules (`tun`, `xt_connmark`, `iptable_mangle`) or binaries exist?
3. **IPv6 Traffic Leaks & Broken Routing:**
   `BuildRoutingCommands` only configures IPv4 `ip rule` and `iptables`. What happens if the host has IPv6 enabled? Does IPv6 traffic bypass the proxy, leak real DNS/IP, or blackhole?
4. **MTU & MSS Clamping Issues:**
   `tun0` interface is created without specifying MTU. What is the default MTU? Without MTU adjustment (e.g. 1500 vs 1420) or TCPMSS clamping, large TCP packets or fragmentation can cause connection stalls.
5. **Teardown Idempotency & Error Accumulation:**
   If `BuildCleanupCommands` fails halfway through or runs when rules do not exist, does `ExecuteCommands` halt or accumulate errors? Can leftover rules cause cascading routing corruption?
6. **Concurrency & Race Conditions in `safemode.go`:**
   In `safemode.go`, `onRollback` is executed outside mutex lock after timer fires. What if `Confirm()` or `CancelAndRollback()` is called concurrently?

---

## 3. Lead Architect Directives & Step-by-Step Instructions

Follow these rules with absolute precision:

### Step 1: Mandatory Architectural Graph Exploration (`graphify`)
Before reading line-by-line, run:
```powershell
python -m graphify.cli explain network
# or
python -m graphify.cli query "how does network package interact with supervisor and core"
```
Also inspect `graphify-out/wiki/` to map out caller-callee relationships.

### Step 2: Initialize Report Scaffold
Immediately create `docs/audit/reports/01-network-routing-report.md` with:
- Executive Summary
- Graphify Dependency Analysis
- Audit Findings Ledger (adhering to the 7-field schema)
- Robustness & Positive Patterns

### Step 3: Step-by-Step Systematic Code Inspection & Incremental Updates
Audit each area sequentially. **Update your report immediately upon identifying an issue:**
1. **Shell Execution & Command Construction (`routing.go:L23-L115`, `L199-L209`):** Check command formatting, escaping, string concatenation, and injection risks.
2. **Interface & IP Detection (`routing.go:L117-L197`):** Audit `GetDefaultRoute`, `DetectSSHPort`, `ResolveHost`. Look for IPv6 edge cases, multiple SSH ports, and file reading errors.
3. **Anti-Lockout & Kernel Table Management (`routing.go:L32-L69`):** Audit priority order, rule conflicts, routing loop potential, and netfilter chain lifecycle.
4. **SafeMode Timer Lifecycle (`safemode.go:L1-L117`):** Audit mutex locking, timer cancellation, race conditions, and goroutine leaks.

### Step 4: Strict Adherence to 7-Field Defect Schema
Every finding in the report must include:
1. `Title & Category` (Security / Concurrency & Race / Resource Leaks / Network & Kernel / Logic & Edge-Cases)
2. `Code Location` (Exact file and line range `Lxx-Lyy`)
3. `Severity` (`Critical`, `High`, `Medium`, `Low / Refactor`)
4. `Trigger Scenario & Root Cause Analysis`
5. `Proof of Concept / Verification Method` (reproduction test or command)
6. `Recommended Architectural Fix`
7. `Existing Strengths & Robustness`

**REMINDER:** Strictly read-only. Do not edit source code. Stay 100% focused on your assigned domain.
