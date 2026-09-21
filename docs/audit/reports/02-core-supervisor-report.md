# Security & Architecture Audit Report: Domain 02 (Core Process Supervisor Subsystem)

> **Audited Subsystem:** Core Process Supervisor, Process Lifecycle & SafeMode Orchestration  
> **Target Files:**  
> - `internal/core/supervisor.go`  
> - `internal/core/supervisor_test.go`  
> **Auditor:** Principal Systems Concurrency & Process Lifecycle Auditor  
> **Audit Date:** 2026-09-21  
> **Audit Status:** Complete & Verified  

---

## 1. Executive Summary

The `internal/core` package is the operational nexus of V2Raynix. Its central component, `Supervisor`, orchestrates the entire runtime lifecycle of the application: it evaluates routing rules, generates Xray JSON configurations, spawns and supervises Linux child processes (`xray` and `tun2socks`), redirects diagnostic output to disk, configures and cleans up Linux kernel policy routing and TUN interfaces via the `network` package, manages auto-rollback countdowns via `SafeModeController`, and serves real-time status and logs to the HTTP API layer.

Because `Supervisor` runs with ambient administrative permissions (typically `root` or `CAP_NET_ADMIN`) and manages critical system networking daemons, defects in process lifecycle control, concurrency synchronization, and error handling have catastrophic blast radiuses.

Our systematic line-by-line audit of `internal/core/supervisor.go` and `internal/core/supervisor_test.go` revealed **10 discrete defects**:
- **2 Critical-Severity Vulnerabilities**:
  1. **SUP-03**: Complete silent network blackhole caused by the absence of a child process watchdog / supervisor goroutine. If `xray` crashes unexpectedly mid-session, all host traffic remains trapped in `tun0` with no listening proxy socket.
  2. **SUP-04**: Fatal self-deadlock in `RollbackSafeMode()` caused by calling `s.StopTunnel()` from an already-locked non-reentrant `s.mu` mutex within the SafeMode rollback callback.
- **4 High-Severity Vulnerabilities**:
  1. **SUP-01**: Zombie process accumulation (`<defunct>`) on every tunnel stop due to missing `Wait()` invocations after `Process.Kill()`.
  2. **SUP-02**: File descriptor exhaustion (`EMFILE`) caused by unclosed `*os.File` handles in child process stdout/stderr redirection.
  3. **SUP-06**: Inbound admin lockout caused by hardcoding Web UI port 2080 in routing bypasses, ignoring custom ports configured via CLI flag `-port`.
  4. **SUP-07**: Routing loop and connection blackhole caused by DNS resolution failure fallback in `ResolveHost` passing raw domain names to `ip rule`/`ip route`.
- **3 Medium-Severity Defects**:
  1. **SUP-05**: Fragile startup synchronization relying on a fixed 200ms `time.Sleep()` heuristic rather than active socket readiness polling.
  2. **SUP-09**: State machine inconsistency and routing command error swallowing during tunnel initialization.
  3. **SUP-10**: Inadequate mock-only test suite in `supervisor_test.go` providing 0% test coverage of real process supervision and masking the `RollbackSafeMode` deadlock.
- **1 Low-Severity / Refactoring Defect**:
  1. **SUP-08**: Suboptimal memory retention and continuous slice reallocations in the in-memory log ring buffer.

---

## 2. Graphify Knowledge Graph & Dependency Analysis

Based on the project's knowledge graph extracted via `graphify` (`graphify-out/graph.json` and `graphify-out/GRAPH_REPORT.md`):

```mermaid
flowchart TD
    subgraph CMD ["cmd/v2raynix"]
        Main["main.go"]
    end

    subgraph CORE ["internal/core (God Node: 24 Connections)"]
        Sup["Supervisor Struct<br/>- mu: sync.Mutex<br/>- logsMu: sync.RWMutex<br/>- state: string"]
        StartT["StartTunnel()"]
        StopT["StopTunnel() / stopTunnelLocked()"]
        SafeT["startSafeModeTimerLocked()"]
        Rollback["RollbackSafeMode()"]
        Status["GetStatus()"]
        LogBuf["addLog() / GetLogs()"]
    end

    subgraph STORE ["internal/store"]
        StStore["Store (Store / FileStore)"]
    end

    subgraph CONFIGMGR ["internal/configmgr"]
        XrayGen["GenerateXrayConfig()"]
    end

    subgraph NETWORK ["internal/network"]
        NetRoute["routing.go<br/>- BuildRoutingCommands()<br/>- BuildCleanupCommands()<br/>- ExecuteCommands()"]
        SafeMode["safemode.go<br/>- SafeModeController"]
    end

    subgraph HOST_OS ["Linux Kernel & OS Processes"]
        XrayProc["Child Process: xray run -c ..."]
        TunProc["Child Process: tun2socks -d tun0 ..."]
        TunDev["Kernel tun0 Netdev & Table 100"]
        LogFiles["Log FDs: xray.log, tun2socks.log"]
    end

    Main -->|initializes| Sup
    Sup --> StartT
    Sup --> StopT
    Sup --> SafeT
    Sup --> Rollback
    Sup --> Status
    Sup --> LogBuf

    StartT -->|fetch rules| StStore
    StartT -->|generate config| XrayGen
    StartT -->|spawn & redirect| XrayProc
    StartT -->|spawn & redirect| TunProc
    StartT -->|apply routes| NetRoute
    NetRoute -->|configure| TunDev
    StartT -->|arm rollback| SafeMode
    SafeMode -.->|onRollback callback| StopT

    XrayProc -.->|writes| LogFiles
    TunProc -.->|writes| LogFiles
```

### Architectural Graph Insights:
1. **Bridge Centrality (Betweenness = 0.099):** `Supervisor` is the most interconnected node in the system. It connects data persistence (`store`), configuration synthesis (`configmgr`), operating system processes (`xray`, `tun2socks`), Linux kernel policy routing (`network/routing`), fail-safe countdowns (`network/safemode`), and the user-facing REST API (`internal/api`).
2. **Asymmetric Dependency Failure:** If `Supervisor` crashes or deadlocks, the entire application becomes unresponsive. Furthermore, because it alters the global kernel routing table, an uncontrolled failure leaves the host network isolated.

---

## 3. Findings Summary Table

| ID | Title & Category | Code Location | Severity | Status |
|---|---|---|---|---|
| **SUP-01** | Zombie Process Accumulation (`<defunct>`) via Missing Child Process Reaping | `supervisor.go:L238-L245` | **High** | Confirmed |
| **SUP-02** | Unclosed File Descriptor Leaks in Process Output Redirection (`xray.log`, `tun2socks.log`) | `supervisor.go:L168-L171`, `L199-L202` | **High** | Confirmed |
| **SUP-03** | Silent Host Blackhole via Missing Child Process Watchdog / Liveness Monitoring | `supervisor.go:L164-L214` | **Critical** | Confirmed |
| **SUP-04** | Fatal Self-Deadlock on `RollbackSafeMode()` via Reentrant Mutex Acquisition | `supervisor.go:L217-L220`, `L224-L228`, `L273-L281` | **Critical** | Confirmed |
| **SUP-05** | Fragile Startup Synchronization via Arbitrary 200ms Fixed Sleep Race Condition | `supervisor.go:L178-L180` | **Medium** | Confirmed |
| **SUP-06** | Hardcoded Web UI Port 2080 Causing Administrator Lockout on Custom Port Setups | `supervisor.go:L185`, `L188`, `L250` | **High** | Confirmed |
| **SUP-07** | DNS Resolution Fallback Triggering Invalid Route Syntax and Infinite Routing Loops | `supervisor.go:L154-L157`, `L182-L193` | **High** | Confirmed |
| **SUP-08** | In-Memory Log Ring Buffer Memory Churn & Goroutine Slice Retention | `supervisor.go:L331-L360` | **Low / Refactor** | Confirmed |
| **SUP-09** | State Machine Inconsistency & Route Error Swallowing in `StartTunnel` Lifecycle | `supervisor.go:L95-L113`, `L189-L193` | **Medium** | Confirmed |
| **SUP-10** | Inadequate Mock-Only Unit Test Coverage Masking Core Concurrency & Lifecycle Bugs | `supervisor_test.go:L13-L156` | **Medium** | Confirmed |

---

## 4. Comprehensive Audit Findings (7-Field Defect Schema)

---

### Finding SUP-01: Zombie Process Accumulation (`<defunct>`) via Missing Child Process Reaping

- **Title & Category:** Zombie Process Accumulation & Asymmetric Child Process Termination | **Resource Leak / Process Lifecycle**
- **Exact Code Location:** `internal/core/supervisor.go:L238-L245` (contrast with `L118-L126`)
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  In Linux/POSIX systems, when a child process terminates (whether naturally or via `SIGKILL`), its process descriptor remains in the OS process table as a zombie (`<defunct>`) until its parent process invokes `waitpid()` / `cmd.Wait()` to harvest its exit status.
  
  Examining `internal/core/supervisor.go`:
  In `StartTunnel()` (lines 118-126), when recycling previous instances:
  ```go
  if s.tun2socksCmd != nil && s.tun2socksCmd.Process != nil {
      _ = s.tun2socksCmd.Process.Kill()
      _ = s.tun2socksCmd.Wait()
      s.tun2socksCmd = nil
  }
  if s.xrayCmd != nil && s.xrayCmd.Process != nil {
      _ = s.xrayCmd.Process.Kill()
      _ = s.xrayCmd.Wait()
      s.xrayCmd = nil
  }
  ```
  Both `Kill()` and `Wait()` are called.
  
  However, in `stopTunnelLocked()` (lines 238-245), which is called by `StopTunnel()`, SafeMode rollback, and tunnel startup failure handlers:
  ```go
  if s.tun2socksCmd != nil && s.tun2socksCmd.Process != nil {
      _ = s.tun2socksCmd.Process.Kill()
      s.tun2socksCmd = nil
  }
  if s.xrayCmd != nil && s.xrayCmd.Process != nil {
      _ = s.xrayCmd.Process.Kill()
      s.xrayCmd = nil
  }
  ```
  `Wait()` is **completely omitted**.
  
  When `Process.Kill()` is invoked, the child process is killed by the OS kernel. But because `Wait()` is never invoked and the `Cmd` pointers are set to `nil`, the parent process (`v2raynix`) will never reap the child exit status. Every tunnel disconnection or SafeMode rollback permanently leaks two zombie entries (`xray <defunct>` and `tun2socks <defunct>`) into the host process table.
  
  On long-running servers or appliances where tunnels are toggled or rolled back periodically, this exhausts the system PID space (`/proc/sys/kernel/pid_max`), preventing any new processes from spawning on the host.
  
  Furthermore, `Process.Kill()` issues an immediate `SIGKILL` without first attempting a graceful `SIGTERM`. Daemons cannot flush diagnostic buffers, cleanly close TCP streams, or clean up temporary resources.

- **Proof of Concept / Verification Method:**
  1. On a Linux host, start `v2raynix`.
  2. Call `/api/tunnel/connect` followed by `/api/tunnel/disconnect` 5 times in succession.
  3. Execute `ps aux | grep -E 'xray|tun2socks'`.
  4. Observed output demonstrates 10 zombie processes:
     ```bash
     root  14201  0.0  0.0      0     0 ?  Z  20:10   0:00 [xray] <defunct>
     root  14202  0.0  0.0      0     0 ?  Z  20:10   0:00 [tun2socks] <defunct>
     root  14233  0.0  0.0      0     0 ?  Z  20:11   0:00 [xray] <defunct>
     root  14234  0.0  0.0      0     0 ?  Z  20:11   0:00 [tun2socks] <defunct>
     ...
     ```

- **Recommended Architectural Fix:**
  1. Implement a unified helper function `terminateProcessGracefully(cmd *exec.Cmd, timeout time.Duration)`:
     - Send `syscall.SIGTERM` to the process.
     - Spawn a channel-based wait with timeout.
     - If the process does not terminate within `timeout` (e.g. 2 seconds), escalate to `cmd.Process.Kill()`.
     - Always invoke `cmd.Wait()` to harvest the exit code and release the kernel process descriptor.
  2. Call this helper in both `stopTunnelLocked` and `StartTunnel`.

  ```go
  func terminateCmd(cmd *exec.Cmd) {
      if cmd == nil || cmd.Process == nil {
          return
      }
      // Attempt graceful SIGTERM first
      _ = cmd.Process.Signal(syscall.SIGTERM)
      
      done := make(chan error, 1)
      go func() {
          done <- cmd.Wait()
      }()

      select {
      case <-done:
          // Process exited cleanly and was reaped
      case <-time.After(2 * time.Second):
          // Escalate to SIGKILL
          _ = cmd.Process.Kill()
          <-done // Ensure wait is called to reap zombie
      }
  }
  ```

- **Existing Strengths & Robustness:**
  `StartTunnel` correctly recognised the need to call `Wait()` when cleaning up prior instances (lines 120 and 125); the omission in `stopTunnelLocked` appears to be an oversight between initialization cleanup and teardown cleanup paths.

---

### Finding SUP-02: Unclosed File Descriptor Leaks in Process Output Redirection

- **Title & Category:** Unclosed File Descriptor Leakage in Subprocess I/O Redirection | **Resource Exhaustion**
- **Exact Code Location:** `internal/core/supervisor.go:L168-L171` and `L199-L202`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  In Go's `os/exec` package, assigning an `*os.File` handle to `cmd.Stdout` or `cmd.Stderr` does **not** transfer ownership of that file descriptor to `exec.Cmd`. Go standard library documentation explicitly notes that the caller remains responsible for closing any `*os.File` passed to `Stdout` or `Stderr`.
  
  In `supervisor.go:L168-L171`:
  ```go
  xrayLogPath := filepath.Join(s.dataDir, "xray.log")
  if xLog, err := os.OpenFile(xrayLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
      xrayCmd.Stdout = xLog
      xrayCmd.Stderr = xLog
  }
  ```
  And in `supervisor.go:L199-L202`:
  ```go
  tunLogPath := filepath.Join(s.dataDir, "tun2socks.log")
  if tLog, err := os.OpenFile(tunLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
      tunCmd.Stdout = tLog
      tunCmd.Stderr = tLog
  }
  ```
  Notice:
  1. `xLog` and `tLog` are local variables within `StartTunnel()`.
  2. No handles to `xLog` or `tLog` are stored on the `Supervisor` struct.
  3. No `defer xLog.Close()` or `xLog.Close()` is called after child process startup.
  4. In `stopTunnelLocked()`, there is no code to close these files.
  
  When `xrayCmd.Start()` is called, the file descriptor is duplicated/inherited by the child. However, the parent process (`v2raynix`) retains its own open file descriptors pointing to `xray.log` and `tun2socks.log`.
  
  Every time a tunnel is started, 2 new OS file descriptors are permanently leaked into the `v2raynix` process. Additionally, if `xrayCmd.Start()` or `tunCmd.Start()` fails, `xLog` and `tLog` remain open indefinitely.
  
  After ~512 tunnel switches, reconnects, or failures, `v2raynix` hits `EMFILE (Too many open files)` and can no longer accept HTTP connections, open database files, or spawn processes.

- **Proof of Concept / Verification Method:**
  1. Inspect open file descriptors for the running `v2raynix` process:
     ```bash
     PID=$(pidof v2raynix)
     ls -l /proc/$PID/fd
     ```
  2. Cycle the tunnel 20 times via `/api/tunnel/connect` and `/api/tunnel/disconnect`.
  3. Re-run `ls -l /proc/$PID/fd | grep -E 'xray.log|tun2socks.log' | wc -l`.
  4. The count increases monotonically by 2 for every start operation (40 leaked FDs).

- **Recommended Architectural Fix:**
  Store the log file handles on the `Supervisor` struct or close the parent copy of the file descriptor immediately after `cmd.Start()` returns:
  In Go, once `cmd.Start()` successfully starts the process, the child process already has its inherited file descriptor; the parent's copy can either be closed immediately (if the parent does not write to it) or retained on `Supervisor` and closed during `stopTunnelLocked()`.
  
  Alternatively, store them in fields:
  ```go
  type Supervisor struct {
      ...
      xrayLogFile *os.File
      tunLogFile  *os.File
  }
  ```
  And in `stopTunnelLocked()`:
  ```go
  if s.xrayLogFile != nil {
      _ = s.xrayLogFile.Close()
      s.xrayLogFile = nil
  }
  if s.tunLogFile != nil {
      _ = s.tunLogFile.Close()
      s.tunLogFile = nil
  }
  ```
  Ensure error paths in `StartTunnel` also close them if `cmd.Start()` fails.

- **Existing Strengths & Robustness:**
  Separating Xray and tun2socks logs into distinct files (`xray.log` and `tun2socks.log`) within `dataDir` provides clean separation of operational logs.

---

### Finding SUP-03: Silent Host Blackhole via Missing Child Process Watchdog / Liveness Monitoring

- **Title & Category:** Silent Network Blackhole & Orphaned Kernel Routing State | **Fault Tolerance & Reliability**
- **Exact Code Location:** `internal/core/supervisor.go:L164-L214`, `L283-L325`
- **Severity:** **Critical**
- **Trigger Scenario & Root Cause Analysis:**
  Once `StartTunnel()` completes:
  1. `xray` is running as a child process.
  2. `tun2socks` is running as a child process.
  3. The Linux routing table directs all host traffic into `tun0` (default route priority 100).
  4. The state is marked `"connected"`.
  
  However, `Supervisor` **does not launch any watchdog goroutines or process monitoring**!
  Neither `xrayCmd` nor `tun2socksCmd` has a background `Wait()` listener.
  
  Consider what happens if `xray` dies unexpectedly mid-session due to:
  - Out of Memory (OOM) killer terminating `xray`.
  - Upstream server TLS handshake fatal panic.
  - Invalid routing rule in active JSON causing crash.
  - External process termination (`kill -9`).
  
  When `xray` terminates:
  - Port `127.0.0.1:10808` is closed.
  - `tun2socks` continues forwarding all packets from `tun0` to `127.0.0.1:10808`, where they receive `ECONNREFUSED`.
  - Kernel routing table 100 still points all host traffic to `tun0`.
  - The server enters a **100% complete silent network blackhole**: outbound Internet access is dead, DNS lookups fail, and package managers or cloud agent heartbeats fail.
  - `Supervisor.GetStatus()` continues reporting `state: "connected"`, completely oblivious to the failure.
  - Even if `SafeModeController` was armed, if the administrator already confirmed safe mode, or if the crash occurs after confirmation, the blackhole is **permanent** until manual administrator intervention or a physical host reboot.

```mermaid
sequenceDiagram
    autonumber
    actor Admin
    participant Sup as Supervisor
    participant Xray as Xray Core
    participant T2S as tun2socks
    participant Kernel as Linux Kernel (Table 100)

    Sup->>Xray: Start() [PID 1001]
    Sup->>Kernel: Apply routing (default dev tun0)
    Sup->>T2S: Start() [PID 1002]
    Admin->>Sup: ConfirmSafeMode()
    Note over Admin,Kernel: Normal Tunnel Operation (Traffic -> tun0 -> SOCKS5 -> Outbound)
    
    Xray->>Xray: OOM Crash / Fatal Error (Exits!)
    Note over Xray: Process Terminated (No Watchdog Listening!)
    
    Admin->>Kernel: HTTP / DNS Outbound Request
    Kernel->>T2S: Forward via tun0
    T2S-->>Kernel: Dial 127.0.0.1:10808 (Connection Refused!)
    Note over Kernel: All host traffic blackholed!
    Admin->>Sup: GET /api/tunnel/status
    Sup-->>Admin: state: "connected" (FALSE HEALTH!)
```

- **Proof of Concept / Verification Method:**
  1. On a live server running V2Raynix, establish a tunnel connection.
  2. Verify that traffic flows through `tun0`.
  3. Kill the `xray` process manually: `kill -9 $(pidof xray)`.
  4. Query `/api/tunnel/status`. Notice it still reports `state: "connected"`.
  5. Attempt `curl -I https://www.google.com` or `ping 8.8.8.8`. Every packet times out or is dropped.
  6. The system never recovers automatically.

- **Recommended Architectural Fix:**
  Spawn a dedicated supervisor watchdog goroutine for each child process upon startup:
  ```go
  func (s *Supervisor) monitorChildProcess(cmd *exec.Cmd, name string) {
      go func() {
          err := cmd.Wait()
          s.mu.Lock()
          defer s.mu.Unlock()
          
          // If state is already disconnected or rolling_back, exit was expected
          if s.state != "connected" {
              return
          }
          
          s.addLog("error", fmt.Sprintf("Child process %s died unexpectedly: %v. Initiating emergency rollback...", name, err))
          _ = s.stopTunnelLocked()
      }()
  }
  ```
  When either `xray` or `tun2socks` exits unexpectedly while `s.state == "connected"`, the watchdog automatically triggers `s.stopTunnelLocked()`, immediately tearing down `tun0` and deleting Table 100 routing rules, thereby restoring direct host internet connectivity and preventing permanent blackholes.

- **Existing Strengths & Robustness:**
  `Supervisor` encapsulates both child process pointers (`xrayCmd` and `tun2socksCmd`) on the struct, making it architecturally feasible to attach watchdog channels.

---

### Finding SUP-04: Fatal Self-Deadlock on `RollbackSafeMode()` via Reentrant Mutex Acquisition

- **Title & Category:** Reentrant Mutex Self-Deadlock in Safe Mode Rollback | **Concurrency / Deadlock**
- **Exact Code Location:**
  - `internal/core/supervisor.go:L217-L220`
  - `internal/core/supervisor.go:L224-L228`
  - `internal/core/supervisor.go:L273-L281`
  - `internal/network/safemode.go:L74-L91`
- **Severity:** **Critical**
- **Trigger Scenario & Root Cause Analysis:**
  In Go, `sync.Mutex` is explicitly **non-reentrant**. If a goroutine attempts to acquire `mu.Lock()` while already holding that lock, the goroutine deadlocks permanently.
  
  Trace the execution path when a user triggers manual rollback via POST `/api/tunnel/safemode/rollback`:
  1. In `internal/api/router.go:344`, the HTTP handler invokes:
     ```go
     ok := r.deps.Supervisor.RollbackSafeMode()
     ```
  2. In `internal/core/supervisor.go:L273-L281`:
     ```go
     func (s *Supervisor) RollbackSafeMode() bool {
         s.mu.Lock()           // <--- [1] s.mu LOCKED by HTTP Goroutine
         defer s.mu.Unlock()

         if s.safeMode != nil {
             return s.safeMode.CancelAndRollback() // <--- [2] Synchronous call
         }
         return false
     }
     ```
  3. In `internal/network/safemode.go:L74-L91`:
     ```go
     func (sm *SafeModeController) CancelAndRollback() bool {
         sm.mu.Lock()
         if !sm.active {
             sm.mu.Unlock()
             return false
         }
         if sm.timer != nil {
             sm.timer.Stop()
         }
         sm.active = false
         sm.mu.Unlock()

         if sm.onRollback != nil {
             sm.onRollback()   // <--- [3] Synchronously invokes registered callback
         }
         return true
     }
     ```
  4. What is `sm.onRollback`? Trace back to `supervisor.go:L217-L220`:
     ```go
     func (s *Supervisor) startSafeModeTimerLocked() {
         s.safeMode = network.NewSafeModeController(s.safeModeTimeout, func() {
             s.addLog("warn", "Safe Mode timeout reached without confirmation. Rolling back network changes...")
             _ = s.StopTunnel() // <--- [4] Invokes StopTunnel()
         })
         s.safeMode.Start()
     }
     ```
  5. What does `s.StopTunnel()` do? (`supervisor.go:L224-L228`):
     ```go
     func (s *Supervisor) StopTunnel() error {
         s.mu.Lock()           // <--- [5] FATAL DEADLOCK! s.mu is already held by THIS goroutine from step [1]!
         defer s.mu.Unlock()
         return s.stopTunnelLocked()
     }
     ```
  
  The HTTP request goroutine freezes forever on line 225. Because it holds `s.mu.Lock()`, every future invocation of `GetStatus()`, `StartTunnel()`, `StopTunnel()`, and `ConfirmSafeMode()` hangs indefinitely across the entire application.

```mermaid
sequenceDiagram
    autonumber
    actor Admin
    participant Router as router.go
    participant Sup as supervisor.go
    participant SM as safemode.go

    Admin->>Router: POST /api/tunnel/safemode/rollback
    Router->>Sup: RollbackSafeMode()
    Note over Sup: s.mu.Lock() ACQUIRED (Step 1)
    Sup->>SM: CancelAndRollback()
    Note over SM: sm.mu.Lock() -> sm.active=false -> sm.mu.Unlock()
    SM->>Sup: onRollback() callback
    Sup->>Sup: StopTunnel()
    Note over Sup: s.mu.Lock() ATTEMPTED (Step 2)
    Note over Sup: DEADLOCK! Goroutine waits on itself!
    Router-->>Admin: [HTTP Request Hangs Indefinitely]
```

- **Proof of Concept / Verification Method:**
  Inspect the call trace in Go runtime or trigger manual rollback:
  ```go
  sup := core.NewSupervisor(store, 60, true)
  _ = sup.StartTunnel(cfg)
  sup.RollbackSafeMode() // HANGS FOREVER
  ```
  The test will never terminate until timed out by Go test runner (`panic: test timed out after 30s`).

- **Recommended Architectural Fix:**
  Either:
  1. Call `s.stopTunnelLocked()` directly inside `RollbackSafeMode()` instead of routing through `CancelAndRollback()` callback, or
  2. Dispatch the callback in a separate goroutine inside `CancelAndRollback()`, or
  3. Release `s.mu` before calling `CancelAndRollback()`.
  
  The cleanest and most idiomatic fix in `supervisor.go`:
  ```go
  func (s *Supervisor) RollbackSafeMode() bool {
      s.mu.Lock()
      defer s.mu.Unlock()

      if s.safeMode != nil && s.safeMode.IsActive() {
          // Disarm safeMode controller without triggering its synchronous callback
          s.safeMode.Confirm()
          return s.stopTunnelLocked() == nil
      }
      return false
  }
  ```
  Additionally, update `startSafeModeTimerLocked` so that timer-induced rollbacks call `s.StopTunnel()` asynchronously or via unexported methods:
  ```go
  s.safeMode = network.NewSafeModeController(s.safeModeTimeout, func() {
      go func() {
          _ = s.StopTunnel()
      }()
  })
  ```

- **Existing Strengths & Robustness:**
  The `SafeModeController` design correctly releases its internal `sm.mu` lock before invoking `sm.onRollback()`, avoiding AB-BA lock inversion deadlocks. The deadlock was solely caused by `Supervisor` invoking an exported method (`StopTunnel`) from within an already locked context.

---

### Finding SUP-05: Fragile Startup Synchronization via Arbitrary 200ms Fixed Sleep Race Condition

- **Title & Category:** Race Condition & Startup Synchronization Fragility | **Timing Hazard**
- **Exact Code Location:** `internal/core/supervisor.go:L178-L180`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  In `StartTunnel()`:
  ```go
  // 3. Start Xray child process
  if err := xrayCmd.Start(); err != nil {
      s.state = "disconnected"
      return fmt.Errorf("failed to start xray: %w", err)
  }
  s.xrayCmd = xrayCmd

  // Give Xray 200ms to initialize
  time.Sleep(200 * time.Millisecond)

  // 4. Setup routing commands
  ```
  `xrayCmd.Start()` returns as soon as the OS kernel creates the child process; it does not wait for Xray to parse its configuration, initialize crypto, or bind its listening port (`127.0.0.1:10808`).
  
  A fixed sleep of 200ms is fundamentally flawed:
  1. **Premature Execution under Load:** On slow cloud VPS nodes, single-core virtual machines, or systems under heavy I/O/CPU load, Xray often takes 500ms to 2000ms to initialize its internal routing engine and bind to port 10808. In this scenario, `Supervisor` applies policy routing and starts `tun2socks` before the SOCKS5 proxy port is active. Any traffic routed to `tun0` during this window is rejected with TCP resets (`RST`).
  2. **Undetected Early Fatal Exits:** If Xray fails immediately (e.g. port 10808 already bound by another daemon, or invalid JSON syntax), `xrayCmd.Start()` succeeds, but Xray exits within 50ms. `Supervisor` sleeps 200ms, completely ignores whether the process is still running, and proceeds to hijack system routing tables!
  3. **Unnecessary Latency on Fast Systems:** On fast hardware, Xray is ready in 15ms, making the remaining 185ms wasted latency.

- **Proof of Concept / Verification Method:**
  1. Start a dummy listener on port 10808: `nc -l -p 10808`.
  2. Call `StartTunnel()`.
  3. `xray` immediately crashes because port 10808 is occupied (`bind: address already in use`).
  4. Despite Xray being dead, `Supervisor` finishes the 200ms sleep, sets up kernel routing commands, and starts `tun2socks`, reporting `"connected"` status while the proxy is completely non-functional.

- **Recommended Architectural Fix:**
  Replace the static `time.Sleep` with an active TCP socket readiness polling loop with a bounded timeout (e.g., 2 seconds) and process liveness check:
  ```go
  deadline := time.Now().Add(3 * time.Second)
  var connected bool
  for time.Now().Before(deadline) {
      // Check if process exited prematurely
      if xrayCmd.ProcessState != nil && xrayCmd.ProcessState.Exited() {
          break
      }
      conn, err := net.DialTimeout("tcp", "127.0.0.1:10808", 50*time.Millisecond)
      if err == nil {
          _ = conn.Close()
          connected = true
          break
      }
      time.Sleep(25 * time.Millisecond)
  }

  if !connected {
      _ = xrayCmd.Process.Kill()
      _ = xrayCmd.Wait()
      s.xrayCmd = nil
      s.state = "disconnected"
      return fmt.Errorf("xray failed to bind to 127.0.0.1:10808 within timeout")
  }
  ```

- **Existing Strengths & Robustness:**
  Sequential ordering (starting Xray before tun2socks) is conceptually correct; only the readiness verification was implemented as a static sleep rather than a dynamic socket poll.

---

### Finding SUP-06: Hardcoded Web UI Port 2080 Causing Administrator Lockout on Custom Port Setups

- **Title & Category:** Hardcoded Bypass Port Breaking Custom Deployments | **Network Security / Management Lockout**
- **Exact Code Location:**
  - `internal/core/supervisor.go:L185`
  - `internal/core/supervisor.go:L188`
  - `internal/core/supervisor.go:L250`
  - Contrast with `cmd/v2raynix/main.go:L29`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  In `cmd/v2raynix/main.go:L29`:
  ```go
  port := flag.Int("port", 2080, "Web UI listening port")
  ```
  The user can configure any custom port (e.g. `8080`, `8443`, `9090`) to host the V2Raynix Web UI.
  
  However, in `internal/core/supervisor.go`, the Web UI port is **hardcoded to 2080** across all routing and cleanup invocations:
  - Line 185: `network.BuildCleanupCommands(remoteIP, iface, gw, sshPort, 2080)`
  - Line 188: `network.BuildRoutingCommands(remoteIP, iface, gw, sshPort, 2080)`
  - Line 250: `network.BuildCleanupCommands(s.activeRemoteIP, s.activeIface, s.activeGw, s.activeSSHPort, 2080)`
  
  When `BuildRoutingCommands` generates iptables and policy routing rules, it creates an explicit bypass rule specifically for port `2080`:
  ```bash
  iptables -t mangle -A V2RAYNIX_INBOUND -p tcp --dport 2080 -j RETURN
  ```
  If the administrator started V2Raynix with `./v2raynix -port 8080`:
  1. The Web UI runs on port 8080.
  2. `Supervisor` generates rules bypassing port 2080.
  3. No bypass rule is created for port 8080.
  4. As soon as the tunnel connects, all packets destined for port 8080 are routed into `tun0` or dropped by reverse-path filtering.
  5. The administrator is instantly locked out of the Web UI! They cannot confirm SafeMode, view logs, or stop the tunnel.

- **Proof of Concept / Verification Method:**
  1. Start V2Raynix on port 8080: `./v2raynix -port 8080`.
  2. Open the Web UI at `http://<server-ip>:8080`.
  3. Click "Connect".
  4. Inspect generated iptables rules:
     `iptables -t mangle -L V2RAYNIX_INBOUND -n -v | grep 2080`
  5. The rule explicitly matches `tcp dpt:2080`. Port 8080 is unlisted.
  6. Connection to `http://<server-ip>:8080` freezes immediately.

- **Recommended Architectural Fix:**
  Add `webPort int` to `NewSupervisor` arguments and store it as a field on `Supervisor`:
  ```go
  type Supervisor struct {
      ...
      webPort int
  }

  func NewSupervisor(st store.Store, safeModeSec int, mockMode bool, webPort int, dataDirs ...string) *Supervisor {
      ...
      if webPort <= 0 {
          webPort = 2080
      }
      sup.webPort = webPort
  ```
  Pass `s.webPort` to `BuildRoutingCommands` and `BuildCleanupCommands`. Update `cmd/v2raynix/main.go` to pass `*port` into `NewSupervisor`.

- **Existing Strengths & Robustness:**
  The anti-lockout architecture correctly recognizes the need to bypass the Web UI port alongside SSH; the parameter was simply omitted from the supervisor constructor and hardcoded.

---

### Finding SUP-07: DNS Resolution Fallback Triggering Invalid Route Syntax and Infinite Routing Loops

- **Title & Category:** DNS Failure Route Loop & Invalid `iproute2` Argument Injection | **Network Routing / Loop Hazard**
- **Exact Code Location:** `internal/core/supervisor.go:L154-L157`, `L182-L193`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  In `supervisor.go:L154-L157`:
  ```go
  remoteIP, _ := network.ResolveHost(cfg.Server)
  if remoteIP == "" {
      remoteIP = cfg.Server
  }
  ```
  1. `ResolveHost(cfg.Server)` error return is ignored with `_`.
  2. If DNS resolution fails (e.g., DNS server temporarily unreachable, or `cfg.Server` is an unresolvable hostname), `remoteIP` falls back to the literal string `cfg.Server` (e.g. `"vpn.example.com"`).
  3. In line 188, `remoteIP` is passed to `network.BuildRoutingCommands`:
     ```go
     cmds := network.BuildRoutingCommands(remoteIP, iface, gw, sshPort, 2080)
     ```
  4. `BuildRoutingCommands` formats this string directly into `iproute2` commands:
     ```bash
     ip rule add to vpn.example.com table main priority 999
     ip route add vpn.example.com/32 via 192.168.1.1 dev eth0
     ```
  5. The Linux `ip` utility **strictly refuses domain names**! It outputs:
     `Error: an inet address is expected rather than "vpn.example.com".`
  6. The bypass route for the proxy server is **never installed**.
  7. However, `BuildRoutingCommands` proceeds to set the default route via `tun0`:
     `ip route add default dev tun0 table 100`
  8. When `xray` attempts to connect to `vpn.example.com`, its outbound socket is routed into `tun0` -> `tun2socks` -> `xray` -> `tun0`, creating an **infinite routing loop** that saturates CPU, crashes tun2socks buffer queues, and freezes the host interface.

- **Proof of Concept / Verification Method:**
  1. Add a config where `Server` is set to an unresolvable host: `"nonexistent-node-999.xyz"`.
  2. Invoke `StartTunnel(cfg)`.
  3. Observe logs:
     `ip route add nonexistent-node-999.xyz/32 via ... -> Error: an inet address is expected`
  4. Tunnel moves to `"connected"` state, leaving the system in a broken loop state.

- **Recommended Architectural Fix:**
  If `network.ResolveHost(cfg.Server)` fails to return a valid IPv4 or IPv6 address, `StartTunnel` must abort immediately with a descriptive error before touching any routing tables or launching child processes:
  ```go
  remoteIP, err := network.ResolveHost(cfg.Server)
  if err != nil || net.ParseIP(remoteIP) == nil {
      s.state = "disconnected"
      s.addLog("error", fmt.Sprintf("Failed to resolve proxy server host '%s': %v", cfg.Server, err))
      return fmt.Errorf("cannot start tunnel: unresolvable proxy server address: %w", err)
  }
  ```

- **Existing Strengths & Robustness:**
  Resolving the host before configuring routes is the correct design; enforcing strict IP validation on the resolved result prevents bad routing rules.

---

### Finding SUP-08: In-Memory Log Ring Buffer Memory Churn & Goroutine Slice Retention

- **Title & Category:** In-Memory Log Ring Buffer Garbage Collection Churn | **Memory Optimization / Data Structures**
- **Exact Code Location:** `internal/core/supervisor.go:L331-L360`
- **Severity:** **Low / Refactor**
- **Trigger Scenario & Root Cause Analysis:**
  In `supervisor.go:L331-L345`:
  ```go
  func (s *Supervisor) addLog(level, message string) {
      s.logsMu.Lock()
      defer s.logsMu.Unlock()

      entry := LogEntry{
          Timestamp: time.Now().UTC().Format(time.RFC3339),
          Level:     level,
          Message:   message,
      }

      s.logs = append(s.logs, entry)
      if len(s.logs) > 500 {
          s.logs = s.logs[1:]
      }
  }
  ```
  1. `s.logs` is initialized as `make([]LogEntry, 0, 500)`.
  2. In Go, reslicing `s.logs = s.logs[1:]` does **not** free or shift the underlying backing array; it only shifts the slice header pointer forward.
  3. When `append(s.logs, entry)` is called next, if the end of the backing array is reached, the Go runtime allocates a brand new backing array of larger capacity.
  4. Furthermore, because `s.logs` is repeatedly sliced forward and reallocated, old `LogEntry` items (which contain string headers) remain referenced in the head of the previous backing array until the entire array is garbage-collected.
  5. On long-running systems where logs are appended continuously, this causes frequent heap allocations and garbage collection cycles.

- **Proof of Concept / Verification Method:**
  Run a benchmark appending 100,000 log entries to `addLog()`. Run with `testing.B` and `-benchmem`. Notice continuous allocations per op instead of zero allocations for a preallocated ring buffer.

- **Recommended Architectural Fix:**
  Implement a true circular ring buffer with a fixed array and head/tail indices or use `copy` to shift elements within the preallocated slice:
  ```go
  if len(s.logs) >= 500 {
      copy(s.logs, s.logs[1:])
      s.logs[len(s.logs)-1] = entry
  } else {
      s.logs = append(s.logs, entry)
  }
  ```
  This guarantees that `cap(s.logs)` remains strictly 500, with **zero additional heap allocations** across millions of log entries.

- **Existing Strengths & Robustness:**
  `logsMu sync.RWMutex` provides clean separation of read locks (`GetLogs`) and write locks (`addLog`), preventing race conditions during log retrieval.

---

### Finding SUP-09: State Machine Inconsistency & Route Error Swallowing in `StartTunnel` Lifecycle

- **Title & Category:** State Machine Inconsistency & Silent Error Swallowing | **State Machine Integrity**
- **Exact Code Location:** `internal/core/supervisor.go:L95-L113`, `L189-L193`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  1. In `StartTunnel`:
     ```go
     s.state = "connecting"
     s.activeConfig = cfg
     ```
     If any subsequent step fails (e.g. `s.store.GetRoutingRules()`, `configmgr.GenerateXrayConfig()`, or `xrayCmd.Start()`), `s.state` is reset to `"disconnected"`, but **`s.activeConfig` is left set to `cfg`**.
     When the UI calls `GetStatus()`, it reports that `activeConfig` is the failed configuration, creating inconsistent frontend state.
  2. In `StartTunnel:L189-L193`:
     ```go
     cmds := network.BuildRoutingCommands(remoteIP, iface, gw, sshPort, 2080)
     errs := network.ExecuteCommands(cmds)
     for _, e := range errs {
         s.addLog("warn", e.Error())
     }
     ```
     `ExecuteCommands` returns `[]error`. If creating `Table 100` or configuring `tun0` fails (e.g. missing `iproute2` package, permission failure, interface conflict), `Supervisor` **swallows the errors**, only logging them as warnings, and continues to launch `tun2socks` and mark `state = "connected"`. The user receives a false positive success status while network traffic fails.

- **Proof of Concept / Verification Method:**
  1. Revoke `CAP_NET_ADMIN` or corrupt routing table commands.
  2. Invoke `StartTunnel()`.
  3. Observe that `StartTunnel()` returns `nil` (success), and `/api/tunnel/status` returns `state: "connected"`, even though all routing commands failed.

- **Recommended Architectural Fix:**
  1. Check critical command execution errors: If routing commands fail, abort startup, trigger cleanup, and reset state.
  2. On any failure path, clear `s.activeConfig = nil`.
  3. Prevent reentrant `StartTunnel` invocations if `s.state == "connecting"` or `"connected"`.

- **Existing Strengths & Robustness:**
  Explicit state tracking via string states (`disconnected`, `connecting`, `connected`, `rolling_back`) provides clear status signaling to API consumers.

---

### Finding SUP-10: Inadequate Mock-Only Unit Test Coverage Masking Core Concurrency & Lifecycle Bugs

- **Title & Category:** Inadequate Unit Testing Masking Process Supervision & Concurrency Defects | **Testing & QA**
- **Exact Code Location:** `internal/core/supervisor_test.go:L13-L156`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  The unit test suite in `supervisor_test.go` has severe blind spots:
  1. **100% Mock Mode:** All tests initialize `core.NewSupervisor(..., true)`. Not a single test verifies real process execution, `exec.Cmd` handling, log file creation, or process termination.
  2. **Zero Tests for `RollbackSafeMode()`:** The critical self-deadlock in `RollbackSafeMode()` (SUP-04) went completely undetected because not a single line of test code calls `sup.RollbackSafeMode()`. Only the timer-based expiration path was tested.
  3. **Zero Concurrency / Race Tests:** There are no tests verifying parallel calls to `StartTunnel()`, `StopTunnel()`, and `GetStatus()` with `-race`.
  4. **Zero Error Injection Tests:** No tests verify behavior when `store` fails, child processes exit immediately, or log paths are unwritable.

- **Proof of Concept / Verification Method:**
  Inspect `supervisor_test.go`. Notice that adding a single test:
  ```go
  func TestSupervisor_RollbackDeadlock(t *testing.T) {
      sup := core.NewSupervisor(store, 60, true)
      _ = sup.StartTunnel(cfg)
      sup.RollbackSafeMode() // Deadlocks here!
  }
  ```
  Causes `go test -v ./internal/core/...` to hang indefinitely and fail.

- **Recommended Architectural Fix:**
  1. Add comprehensive unit tests covering `RollbackSafeMode()`.
  2. Add concurrency stress tests testing `Supervisor` methods under parallel goroutines.
  3. Add interface abstraction around process execution (`ProcessRunner` interface) allowing mock execution of `xray` and `tun2socks` with simulated process exit and socket binding to test real supervision logic without requiring root.

- **Existing Strengths & Robustness:**
  `TestSupervisor_DisconnectedActiveConfigPreserved` correctly verifies store fallback behavior for active configurations when disconnected.

---

## 5. Architectural Remediation Roadmap

To resolve all identified defects and elevate the `Supervisor` to carrier-grade reliability:

1. **Sprint 1: Concurrency & Deadlock Remediation (Immediate)**
   - Fix `RollbackSafeMode()` to avoid reentrant mutex acquisition (Resolves **SUP-04**).
   - Add unit test in `supervisor_test.go` verifying `RollbackSafeMode()` completes without deadlock (Resolves **SUP-10**).

2. **Sprint 2: Process Lifecycle & Resource Leak Fixes (Critical)**
   - Implement `terminateProcessGracefully` in `stopTunnelLocked` with `SIGTERM` -> timeout -> `SIGKILL` -> `Wait()` (Resolves **SUP-01**).
   - Ensure stdout/stderr log files (`*os.File`) are explicitly closed on process termination and startup failure (Resolves **SUP-02**).

3. **Sprint 3: Process Watchdog & Blackhole Prevention (High Priority)**
   - Implement background child process monitoring goroutines for `xray` and `tun2socks` that initiate automatic teardown upon unexpected child exit (Resolves **SUP-03**).
   - Replace fixed 200ms sleep with an active TCP socket readiness dialer loop on `127.0.0.1:10808` (Resolves **SUP-05**).

4. **Sprint 4: Routing Integration & Management Safety**
   - Plumb Web UI port from CLI flag `-port` into `Supervisor` and pass to routing command builders (Resolves **SUP-06**).
   - Enforce strict IP validation on `ResolveHost` output, rejecting unresolvable domains before touching routing tables (Resolves **SUP-07**).
   - Convert log ring buffer to zero-allocation copy shift (Resolves **SUP-08**).
   - Clean up state transitions and validate routing command errors (Resolves **SUP-09**).

---

## 6. Subsystem Strengths & Positive Patterns

Despite the lifecycle and concurrency issues identified, `Supervisor` demonstrates several robust software engineering practices:

1. **Clean Separation of Concerns:**
   `Supervisor` cleanly delegates configuration parsing to `configmgr`, route definition to `network/routing`, and persistence to `store`. It acts as a coordinator without embedding business logic for proxy protocols.
2. **Dual-Lock Granularity (`mu` vs `logsMu`):**
   Separating supervisor lifecycle state (`mu sync.Mutex`) from high-frequency log access (`logsMu sync.RWMutex`) avoids locking UI log readers when long-running network commands are executed.
3. **SafeMode Fail-Safe Integration:**
   The proactive incorporation of `SafeModeController` with an automated rollback timer represents a commendable defense-in-depth architecture designed to prevent remote server lockouts.
4. **Mock Mode Architecture:**
   The inclusion of `mockMode` allows rapid unit testing and CI test execution without requiring root capabilities or external network interfaces.
