# Phase 2: Process Supervision & Concurrency Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement robust child process lifecycle management and high-concurrency diagnostic safeguards across V2Raynix: child process crash watchdog (`SUP-03`), zombie process reaping (`SUP-01`), log file descriptor cleanup (`SUP-02`), active socket readiness polling (`SUP-05`), bounded worker pool with context cancellation in Pinger (`PING-01`, `PING-02`), and IPv6 literal dialing support (`PING-04`).

**Architecture:**
1. **Core Supervisor Watchdog:** Attach an asynchronous monitor to `xrayCmd` and `tun2socksCmd`. If either daemon crashes or exits while the tunnel is marked `connected`, immediately initiate an emergency teardown (`StopTunnel`) to restore host networking and prevent a total network blackhole.
2. **Process Reaping & FD Management:** Maintain explicit `*os.File` handles for process log redirection (`xray.log`, `tun2socks.log`) on the `Supervisor` struct and close them upon teardown. Ensure `Process.Kill()` is strictly paired with process status reaping (`cmd.Wait()`).
3. **Socket Readiness Synchronization:** Replace the arbitrary `time.Sleep(200ms)` race condition with active TCP socket polling on `127.0.0.1:10808` with a 2-second deadline, verifying Xray is actually accepting connections before applying kernel routes.
4. **Bounded Pinger Worker Pool:** Refactor `BatchPing` into a true worker pool consuming from a jobs channel, bounding concurrency strictly to `concurrency` goroutines regardless of input slice size. Add `context.Context` support (`TCPPingContext`, `BatchPingContext`) and use `net.JoinHostPort` for RFC-compliant IPv6 literal dialing.

**Tech Stack:** Go 1.22+, `os/exec`, `sync`, `net`, `context`, Linux process management.

**Spec:** `docs/audit/FINAL_COMPREHENSIVE_AUDIT_REPORT.md` (Phase 2 Remediation Roadmap).

---

## Global Constraints

- **TDD MANDATE:** Every fix must be preceded by a failing unit test reproducing the defect, followed by minimal code to pass, verified by test output.
- **ZERO REGRESSION:** All existing unit tests across `./...` must continue to pass cleanly.
- **BACKWARD COMPATIBILITY:** Existing public function signatures (`TCPPing`, `BatchPing`, `NewSupervisor`, `StartTunnel`, `StopTunnel`) must be preserved. New features are introduced via extensions or internal helpers.
- **ZERO RESOURCE LEAKS:** No orphaned goroutines, unclosed file descriptors, or zombie processes may remain after start/stop cycles.

---

## Review Focus

1. **Child Daemon Crash Recovery:** If the Xray child process exits unexpectedly, does the Supervisor cleanly tear down routing and return to `disconnected` state?
2. **FD Leakage:** When tunnels are started and stopped repeatedly, are log file descriptors closed without leaking OS file handles?
3. **Goroutine Bounding:** If 10,000 configs are passed to `BatchPing`, does the runtime spawn at most `concurrency` goroutines?
4. **Context Propagation:** If `req.Context()` is cancelled during a batch ping, do in-flight dials abort immediately?
5. **IPv6 Dialing:** Can `TCPPing` dial IPv6 literals (`::1`, `2001:db8::1`) without returning "too many colons in address"?

---

## File Structure

- Modify: `internal/core/supervisor.go`
- Test: `internal/core/supervisor_test.go`
- Modify: `internal/pinger/pinger.go`
- Test: `internal/pinger/pinger_test.go`
- Modify: `internal/api/router.go:L291-L305`

---

### Task 1: Implement Child Process Watchdog & Emergency Teardown (`SUP-03`)

**Files:**
- Modify: `internal/core/supervisor.go`
- Test: `internal/core/supervisor_test.go`

**Interfaces:**
- Consumes: `xrayCmd *exec.Cmd`, `tun2socksCmd *exec.Cmd`, `s.state`
- Produces: `s.watchProcess(cmd *exec.Cmd, name string)` goroutine triggering `s.StopTunnel()` on unexpected exit.

- [ ] **Step 1: Write failing test in `supervisor_test.go` for child process crash**
  Spawn a mock process that exits after 50ms; assert that the supervisor detects the exit, logs the error, and transitions state to `disconnected`.
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/core -run TestSupervisor_ChildCrashWatchdog -v`
- [ ] **Step 3: Implement process watchdog in `supervisor.go`**
  Add `watchProcess(cmd *exec.Cmd, name string)` to wait on `cmd.Wait()` in background; if `s.state == "connected"`, log error and execute `_ = s.StopTunnel()`.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/core -run TestSupervisor_ChildCrashWatchdog -v`
- [ ] **Step 5: Commit changes**
  `git add internal/core/ && git commit -m "feat(core): add child process watchdog to prevent network blackholes (SUP-03)"`

---

### Task 2: Fix Zombie Process Reaping & File Descriptor Leaks (`SUP-01` & `SUP-02`)

**Files:**
- Modify: `internal/core/supervisor.go`
- Test: `internal/core/supervisor_test.go`

**Interfaces:**
- Consumes: `xrayLogFile *os.File`, `tunLogFile *os.File`
- Produces: Clean process termination via `killAndReap` and explicit `Close()` on log handles.

- [ ] **Step 1: Write failing test in `supervisor_test.go` for file descriptor retention**
  Verify that after `StartTunnel` and `StopTunnel`, log file handles are closed and processes reaped.
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/core -run TestSupervisor_LogFDCloseAndReap -v`
- [ ] **Step 3: Implement `killAndReap` and file closer in `supervisor.go`**
  - Store `xrayLogFile` and `tunLogFile` on `Supervisor`.
  - In `stopTunnelLocked()`, close file handles and ensure `cmd.Wait()` reaps exit status.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/core -v`
- [ ] **Step 5: Commit changes**
  `git add internal/core/ && git commit -m "fix(core): reap child processes and close log file descriptors on teardown (SUP-01, SUP-02)"`

---

### Task 3: Replace Fixed Sleep with Active Socket Polling (`SUP-05`)

**Files:**
- Modify: `internal/core/supervisor.go`
- Test: `internal/core/supervisor_test.go`

**Interfaces:**
- Consumes: `waitForPortReady(addr string, timeout time.Duration) bool`
- Produces: Polling dial on `127.0.0.1:10808` before proceeding with network routing setup.

- [ ] **Step 1: Write failing test in `supervisor_test.go` for `waitForPortReady`**
  Test socket readiness helper against a deferred TCP listener.
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/core -run TestWaitForPortReady -v`
- [ ] **Step 3: Implement `waitForPortReady` in `supervisor.go`**
  Poll `net.DialTimeout` every 25ms up to deadline; replace `time.Sleep(200ms)`. Skip in `mockMode`.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/core -v`
- [ ] **Step 5: Commit changes**
  `git add internal/core/ && git commit -m "fix(core): replace 200ms sleep with active socket readiness polling (SUP-05)"`

---

### Task 4: Worker Pool, Context Support & IPv6 in Pinger (`PING-01`, `PING-02`, `PING-04`)

**Files:**
- Modify: `internal/pinger/pinger.go`
- Modify: `internal/api/router.go:L291-L305`
- Test: `internal/pinger/pinger_test.go`

**Interfaces:**
- Consumes: `TCPPingContext(ctx, host, port, timeout)`
- Consumes: `BatchPingContext(ctx, configs, concurrency, timeout)`
- Produces: Bounded worker pool (max `concurrency` goroutines), context cancellation, and `net.JoinHostPort` IPv6 support.

- [ ] **Step 1: Write failing tests in `pinger_test.go`**
  - `TestPinger_IPv6Support`: test `::1` formatting.
  - `TestPinger_ContextCancellation`: test immediate abort on cancelled context.
  - `TestPinger_WorkerPoolBound`: test that 1,000 configs only spawn `concurrency` workers.
- [ ] **Step 2: Run tests to verify failure**
  `go test ./internal/pinger -v`
- [ ] **Step 3: Implement worker pool, context methods, and IPv6 join in `pinger.go`**
  - Use `net.JoinHostPort` in `TCPPingContext`.
  - Refactor `BatchPingContext` to launch worker pool over `jobs := make(chan *store.ConfigItem)`.
  - Update `router.go:handlePingAll` to pass `req.Context()`.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/pinger -v`
  `go test ./internal/api -v`
- [ ] **Step 5: Commit changes**
  `git add internal/pinger/ internal/api/ && git commit -m "feat(pinger): add bounded worker pool, context cancellation, and IPv6 support (PING-01, PING-02, PING-04)"`

---

### Task 5: Full Regression Testing & Knowledge Graph Update

- [ ] **Step 1: Run complete test suite**
  `go test -count=1 ./...`
- [ ] **Step 2: Update graphify knowledge graph**
  `graphify update .`
- [ ] **Step 3: Update documentation & walkthrough**
  Commit all ledger and walkthrough artifacts.
