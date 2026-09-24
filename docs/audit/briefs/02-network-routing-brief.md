# Domain Briefing 02: Network Routing & Kernel SafeMode Subsystem

> **Subagent Role:** Principal Linux Kernel Networking & Netfilter Routing Auditor  
> **Audited Files:** `internal/network/routing.go`, `internal/network/safemode.go`, `internal/network/*_test.go`  
> **Output Report File:** `docs/audit/reports/02-network-routing-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Subsystem Architecture & Responsibilities
The `internal/network` package implements the Linux policy-based routing engine:
- Directing outbound traffic into the `tun0` interface created by `tun2socks`.
- Protecting the remote proxy server IP and local SSH/Web administration ports from routing loops and lockout using Netfilter marks (`fwmark 81`, `connmark 0x52`) and multiple routing tables (`table 100`).
- Managing the emergency rollback controller (`SafeModeController`) with confirmation pinging.

---

## 2. Critical Audit Directives & Failure Modes to Investigate

Auditor must systematically examine:
1. **Command Execution Safety & Shell Injection Risks:**
   - In `ExecuteCommands()`, how are system commands constructed? Are user-controlled inputs (remote IP, interface names, ports) sanitized against shell metacharacters?
2. **SSH / Web Management Port Exclusion Hazards:**
   - Does `BuildRoutingCommands()` properly exclude the current SSH port and Web UI port under all iptables/nftables configurations?
   - What happens if the Web port is not default `2080`? Is it passed dynamically or hardcoded?
3. **Routing Table Cleanup Completeness:**
   - In `BuildCleanupCommands()`, are all policy rules (`ip rule del table 100`, `ip rule del fwmark 81`), routes, and iptables mangle/nat rules fully purged?
   - Can repetitive connect/disconnect cycles leave duplicate or dangling iptables rules?
4. **Default Gateway & Interface Resolution:**
   - How does `GetDefaultRoute()` determine the primary network interface? Does it handle multi-homed servers, policy routes, or missing default routes safely?
5. **SafeMode State Machine Integrity:**
   - In `safemode.go`, verify timer cancellation, atomic state transitions, and callback safety when network connectivity fails.
6. **DNS Leak & MTU Clamping:**
   - Are DNS queries directed through the tunnel, or do they leak through physical interfaces?
   - Is TCP MSS clamping implemented to prevent packet fragmentation drops over TUN?

---

## 3. Subagent Execution Instructions
1. Run `python -m graphify.cli explain network` or query graph to understand dependencies.
2. Progressively write findings to `docs/audit/reports/02-network-routing-report.md`.
3. Every finding MUST follow the 7-field defect schema (ID, Code Location, Severity, Trigger/Root Cause, PoC, Recommended Fix, Strengths).
4. STRICTLY READ-ONLY: Do not edit any code files.
