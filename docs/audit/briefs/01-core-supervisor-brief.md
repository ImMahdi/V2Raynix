# Domain Briefing 01: Core Supervisor & Process Lifecycle Subsystem

> **Subagent Role:** Principal Systems Concurrency & Process Lifecycle Auditor  
> **Audited Files:** `internal/core/supervisor.go`, `internal/core/supervisor_test.go`  
> **Output Report File:** `docs/audit/reports/01-core-supervisor-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Subsystem Architecture & Responsibilities
The `internal/core` package houses the `Supervisor`, which orchestrates the entire operational lifecycle of V2Raynix:
- Launching the Xray-core proxy binary (`xray run -c xray-active.json`).
- Launching the `tun2socks` TUN interface process (`tun2socks -d tun0 -p socks5://...`).
- Invoking network routing setup and teardown (`network.ExecuteCommands`).
- Managing the thread-safe in-memory ring-buffer for system and process logs.
- Supervising the `SafeModeController` timer and handling emergency automatic rollback.

### State Transitions:
`disconnected` -> `connecting` -> `connected` -> `rolling_back` -> `disconnected`
Thread safety is guarded by `s.mu sync.Mutex` for supervisor state and `s.logsMu sync.RWMutex` for logs.

---

## 2. Critical Audit Directives & Failure Modes to Investigate

Auditor must systematically examine:
1. **Child Process Watchdog & Silent Blackhole:**
   - If Xray or tun2socks crashes unexpectedly (SIGSEGV, OOM Killer, invalid server config), does the Supervisor detect process death?
   - If not, does the system enter a silent blackhole state where all machine traffic is routed into a dead TUN interface?
2. **Zombie Process Accumulation (`<defunct>`):**
   - In `stopTunnelLocked()`: are processes killed via `cmd.Process.Kill()` properly reaped using `cmd.Wait()`?
3. **File Descriptor Leaks in Process Output Redirection:**
   - Does `os.OpenFile` for `xray.log` and `tun2socks.log` leak open descriptors across tunnel reconnect cycles?
4. **Startup Race Conditions:**
   - Does Xray startup rely on fixed `time.Sleep` instead of deterministic TCP socket probing?
5. **Deadlock in Safe Mode Rollback:**
   - In `startSafeModeTimerLocked()`, does the timer callback calling `StopTunnel()` risk deadlock with `s.mu`?
6. **Log Ring Buffer Memory Growth & Concurrency:**
   - Verify ring buffer capacity bounds under high log output rates.

---

## 3. Subagent Execution Instructions
1. Run `python -m graphify.cli explain supervisor` or query graph to understand dependencies.
2. Progressively write findings to `docs/audit/reports/01-core-supervisor-report.md`.
3. Every finding MUST follow the 7-field defect schema:
   - **ID & Title:** (e.g. `SUP-01: ...`)
   - **Code Location:** (File and lines `Lxx-Lyy`)
   - **Severity:** (Critical / High / Medium / Low)
   - **Trigger & Root Cause:** Detailed mechanism.
   - **PoC / Verification:** How to reproduce deterministically.
   - **Recommended Fix:** Exact minimal fix pattern.
   - **Strengths:** Good practices preserved.
4. STRICTLY READ-ONLY: Do not edit any code files.
