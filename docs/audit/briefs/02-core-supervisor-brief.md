# Domain Briefing 02: Core Process Supervisor Subsystem

> **Subagent Role:** Principal Systems Concurrency & Process Lifecycle Auditor  
> **Audited Files:** `internal/core/supervisor.go`, `internal/core/supervisor_test.go`  
> **Output Report File:** `docs/audit/reports/02-core-supervisor-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Deep Knowledge Transfer & Architecture

The `core` package houses the `Supervisor`, which orchestrates the runtime lifecycle of V2Raynix: launching the Xray core proxy process, launching the `tun2socks` tunneling process, calling the `network` routing package, managing the in-memory ring-buffer for system logs, and triggering the safe-mode rollback mechanism.

### Key Architectural Concepts:
1. **Supervisor State Machine:**
   - Valid states: `disconnected`, `connecting`, `connected`, `rolling_back`.
   - Thread safety: Guarded by `s.mu sync.Mutex` for supervisor state and `s.logsMu sync.RWMutex` for logs.
2. **Process Orchestration Lifecycle (`StartTunnel`):**
   - Kills pre-existing `tun2socks` and `xray` processes if running.
   - Fetches routing rules from `store.Store`.
   - Generates Xray JSON via `configmgr.GenerateXrayConfig` and writes to `xray-active.json` in `dataDir`.
   - Discovers default route and SSH port via `network.GetDefaultRoute()` and `network.DetectSSHPort()`.
   - Starts `xray run -c xray-active.json`, redirecting output to `xray.log`.
   - Waits 200ms for Xray socket readiness.
   - Executes network cleanup + routing setup commands via `network.ExecuteCommands`.
   - Starts `tun2socks -d tun0 -p socks5://127.0.0.1:10808`, redirecting output to `tun2socks.log`.
   - Transitions state to `connected` and arms the `SafeModeController` countdown timer.
3. **Teardown & Rollback (`StopTunnel` & `stopTunnelLocked`):**
   - Transitions state to `rolling_back`.
   - Halts SafeMode controller.
   - Sends `Process.Kill()` to `tun2socksCmd` and `xrayCmd`.
   - Executes `network.BuildCleanupCommands` to tear down routes and delete `tun0`.
   - Transitions state to `disconnected`.
4. **Log Ring Buffer:**
   - Maintains fixed-capacity buffer (500 entries) of `LogEntry`. Drops oldest logs when capacity is exceeded.

---

## 2. Concerns, Pitfalls & Critical Warnings

When auditing this domain, pay hyper-vigilant attention to these failure modes:
1. **Zombie Process Accumulation (`<defunct>`):**
   In `stopTunnelLocked()`: `_ = s.tun2socksCmd.Process.Kill()` and `_ = s.xrayCmd.Process.Kill()` are invoked, but `Wait()` is NEVER called on teardown! On Linux/POSIX systems, a killed child process whose exit status is not reaped via `Wait()` remains a zombie in the process table.
2. **File Descriptor Leakage in Process Redirection:**
   In `StartTunnel()`, `os.OpenFile(xrayLogPath, ...)` and `os.OpenFile(tunLogPath, ...)` return open `*os.File` handles assigned to `cmd.Stdout` and `cmd.Stderr`. Go's `exec.Cmd` does NOT close user-supplied `*os.File` descriptors after execution or failure. Repeatedly starting and stopping tunnels will leak OS file descriptors.
3. **Blackhole Silent Failure (Missing Child Process Watchdog):**
   If `xray` crashes mid-session (e.g. invalid server certificate, OOM killer, network anomaly), does the Supervisor detect process termination? If not, `tun0` and policy routing remain active, sending all outbound machine traffic into a dead socket (complete network blackhole).
4. **Hardcoded Ports & Service Configuration:**
   `BuildRoutingCommands(remoteIP, iface, gw, sshPort, 2080)` hardcodes Web UI port `2080`. What if the web server runs on port `8080` or is configured via environment variables?
5. **Arbitrary Sleep Latency (`time.Sleep(200 * time.Millisecond)`):**
   Starting Xray and sleeping 200ms is a race hazard. On slow CPUs or high-load virtual machines, Xray may take 500ms+ to bind its SOCKS port, causing `tun2socks` or routing to fail. Conversely, on fast systems it wastes time. A proper socket dial polling check is required.
6. **Deadlock in Safe Mode Rollback Callback:**
   `startSafeModeTimerLocked()` registers a callback calling `s.StopTunnel()`. `StopTunnel()` acquires `s.mu.Lock()`. Is there any scenario where `s.mu` is already held when `onRollback` is dispatched? Verify lock reentrancy and deadlock safety.

---

## 3. Lead Architect Directives & Step-by-Step Instructions

### Step 1: Mandatory Architectural Graph Exploration (`graphify`)
Before reading line-by-line, run:
```powershell
python -m graphify.cli explain supervisor
# or
python -m graphify.cli query "how does core supervisor interact with configmgr and network"
```
Also inspect `graphify-out/wiki/` to understand how Supervisor is utilized by `internal/api/handlers.go`.

### Step 2: Initialize Report Scaffold
Immediately create `docs/audit/reports/02-core-supervisor-report.md` with standard sections.

### Step 3: Step-by-Step Systematic Code Inspection & Incremental Updates
Audit each area sequentially and update the report immediately upon finding defects:
1. **Process Lifecycle Management (`supervisor.go:L116-L127`, `L164-L177`, `L195-L209`, `L238-L245`):** Check process reaping, signal propagation, zombie leaks, and file descriptor management.
2. **Startup Synchronization & Race Conditions (`supervisor.go:L178-L180`):** Audit the 200ms fixed sleep vs active socket polling.
3. **Process Liveness Monitoring:** Check what happens when Xray or tun2socks crashes unexpectedly.
4. **SafeMode Integration & Mutex Locks (`supervisor.go:L216-L229`, `L259-L281`):** Audit mutex acquisition order, callback safety, and state machine transitions.
5. **Log Buffer Concurrency (`supervisor.go:L331-L360`):** Audit `logsMu` locking and potential slice memory leaks.

### Step 4: Strict Adherence to 7-Field Defect Schema
Every finding in the report must include:
1. `Title & Category`
2. `Code Location` (`Lxx-Lyy`)
3. `Severity` (`Critical`, `High`, `Medium`, `Low / Refactor`)
4. `Trigger Scenario & Root Cause Analysis`
5. `Proof of Concept / Verification Method`
6. `Recommended Architectural Fix`
7. `Existing Strengths & Robustness`

**REMINDER:** Strictly read-only. Do not edit source code. Stay 100% focused on your assigned domain.
