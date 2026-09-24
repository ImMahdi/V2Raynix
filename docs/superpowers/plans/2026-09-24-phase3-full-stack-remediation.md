# Phase 3 Full-Stack Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the critical architectural vulnerabilities, concurrency race conditions, and cross-platform defects identified in the September 24 full-spectrum audit across V2Raynix.

**Architecture:** TDD-driven remediation executing across 8 granular tasks. Each task defines an isolated failing test proving the defect, implements the minimal fix, verifies passing tests without regressions, and commits cleanly. The tasks remediate: Core Supervisor watchdog races, Linux kernel FIB rules, VMess fragment parsing, WebPort resolution sync, Store latency persistence freezing, Updater EXDEV cross-device atomic copy, Pinger IPv6 formatting, and Web Frontend fetch timeouts.

**Tech Stack:** Go 1.22+, Linux Netfilter / `iproute2`, React 18, Vite.

**Spec:** [`docs/audit/FINAL_COMPREHENSIVE_AUDIT_REPORT_2026-09-24.md`](file:///c:/MaadZone/Github%20Projects/V2Raynix/docs/audit/FINAL_COMPREHENSIVE_AUDIT_REPORT_2026-09-24.md)

## Global Constraints
- Preserve 100% backward compatibility with existing `v2raynix.json` store schema.
- Preserve zero-downtime administrative SSH access on `192.168.254.80:23313`.
- Strict TDD cycle: write failing test (RED), write minimal fix (GREEN), verify regression suite (`go test ./...`).
- Strictly adhere to `subagent-debugging.md` rules and stop before any out-of-scope code refactoring.

## Review Focus
1. **Watchdog Reconnect Race:** Switching configs while tunnel is active must not trigger false crash teardown.
2. **Kernel FIB Port Rules:** `ip rule add` for port bypass must include `ipproto tcp` or fail with kernel `EINVAL`.
3. **WebPort Resolution:** Modifying web port via `v2raynix setup` must take effect on daemon restart without requiring manual command-line flag override.
4. **Store Mutex Freezing:** Batch ping operations must not lock `FileStore.mu` for multiple seconds over sequential `f.Sync()` disk writes.
5. **Cross-Device Updater Rename:** Core updater must succeed even when `/tmp` is mounted on an isolated `tmpfs` partition.

---

### Task 1: Core Supervisor Watchdog Reconnection Race (`SUP-01`, `SUP-02`)

**Files:**
- Modify: `internal/core/supervisor.go:L248-L270`
- Test: `internal/core/supervisor_watchdog_test.go`

**Interfaces:**
- Consumes: `s.xrayCmd`, `s.tun2socksCmd`, `s.state`, `s.mu`
- Produces: Generational or identity-aware `watchProcess(cmd *exec.Cmd, name string)` that checks whether `cmd` is still the active command before triggering emergency teardown.

- [ ] **Step 1: Write the failing test**
Create a test in `internal/core/supervisor_watchdog_test.go` that simulates rapid reconnection: starts a tunnel, immediately invokes `StartTunnel` with a second config, kills the first process, and verifies that the second tunnel remains in `"connected"` state rather than being torn down to `"disconnected"` by the stale watchdog.

```go
func TestSupervisor_ReconnectWatchdogRace(t *testing.T) {
    // Setup supervisor with mock mode
    // Start initial tunnel
    // Call StartTunnel again
    // Verify state stays "connected"
}
```

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v -run TestSupervisor_ReconnectWatchdogRace ./internal/core`
Expected: FAIL (or watchdog triggers teardown)

- [ ] **Step 3: Write minimal implementation in `supervisor.go`**
In `internal/core/supervisor.go`, modify `watchProcess` to verify that `cmd` is still active:
```go
func (s *Supervisor) watchProcess(cmd *exec.Cmd, name string) {
    err := cmd.Wait()

    s.mu.Lock()
    defer s.mu.Unlock()

    // If this command is no longer the active command for this role, ignore exit
    if name == "xray" && s.xrayCmd != cmd {
        return
    }
    if name == "tun2socks" && s.tun2socksCmd != cmd {
        return
    }

    if s.state == "connected" {
        s.addLog("error", fmt.Sprintf("Child process %s exited unexpectedly (%v). Initiating emergency teardown...", name, err))
        _ = s.stopTunnelLocked()
    }
}
```

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v -run TestSupervisor_ReconnectWatchdogRace ./internal/core`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/core/supervisor.go internal/core/supervisor_watchdog_test.go
git commit -m "fix(core): eliminate stale watchdog race on tunnel reconnect (SUP-01)"
```

---

### Task 2: Linux Kernel Routing Anti-Lockout FIB Rules (`NET-01`)

**Files:**
- Modify: `internal/network/routing.go:L44-L49`, `L86-L100`
- Test: `internal/network/routing_test.go`

**Interfaces:**
- Consumes: `sshPort int`, `webPort int`
- Produces: `BuildRoutingCommands()` and `BuildCleanupCommands()` with `ipproto tcp` and `ipproto udp` parameter constraints.

- [ ] **Step 1: Write the failing test**
In `internal/network/routing_test.go`, assert that the generated `ip rule` strings for SSH and Web management ports contain `ipproto tcp`:
```go
func TestBuildRoutingCommands_IncludesIPProto(t *testing.T) {
    cmds := BuildRoutingCommands("1.2.3.4", "eth0", "192.168.1.1", 22, 2080)
    foundSSH := false
    for _, cmd := range cmds {
        if strings.Contains(cmd, "sport 22") || strings.Contains(cmd, "dport 22") {
            if !strings.Contains(cmd, "ipproto tcp") {
                t.Fatalf("expected 'ipproto tcp' in rule: %s", cmd)
            }
            foundSSH = true
        }
    }
    if !foundSSH {
        t.Fatal("SSH rules not generated")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v -run TestBuildRoutingCommands_IncludesIPProto ./internal/network`
Expected: FAIL with "expected 'ipproto tcp' in rule"

- [ ] **Step 3: Write minimal implementation in `routing.go`**
Update `BuildRoutingCommands` and `BuildCleanupCommands`:
```go
// 3. Anti-lockout policy rules for SSH
fmt.Sprintf("ip rule add ipproto tcp sport %d table main priority 1000", sshPort),
fmt.Sprintf("ip rule add ipproto tcp dport %d table main priority 1001", sshPort),

// 4. Anti-lockout policy rules for Web UI
fmt.Sprintf("ip rule add ipproto tcp sport %d table main priority 1002", webPort),
fmt.Sprintf("ip rule add ipproto tcp dport %d table main priority 1003", webPort),
```
And cleanup:
```go
fmt.Sprintf("ip rule del ipproto tcp sport %d table main priority 1000", sshPort),
fmt.Sprintf("ip rule del ipproto tcp dport %d table main priority 1001", sshPort),
fmt.Sprintf("ip rule del ipproto tcp sport %d table main priority 1002", webPort),
fmt.Sprintf("ip rule del ipproto tcp dport %d table main priority 1003", webPort),
```

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/network`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/network/routing.go internal/network/routing_test.go
git commit -m "fix(network): specify ipproto tcp on port policy rules to prevent EINVAL (NET-01)"
```

---

### Task 3: Config Management VMess Fragment Stripping (`CFG-D3-01`)

**Files:**
- Modify: `internal/configmgr/generator.go:L248-L255`
- Test: `internal/configmgr/generator_test.go`

**Interfaces:**
- Consumes: `cfg.RawURL string`
- Produces: Sanitized Base64 decoding in `buildProxyOutbound` for VMess share links.

- [ ] **Step 1: Write the failing test**
In `internal/configmgr/generator_test.go`, write a test that calls `GenerateXrayConfig` with a VMess configuration containing `#RemarkFragment`:
```go
func TestGenerateXrayConfig_VMessWithRemarkFragment(t *testing.T) {
    link := "vmess://eyJhZGQiOiIxLjIuMy40IiwicG9ydCI6NDQzLCJpZCI6IjAwMDAwMDAwLTAwMDAtMDAwMC0wMDAwLTAwMDAwMDAwMDAwMCIsImFpZCI6MCwibmV0Ijoid3MiLCJ0eXBlIjoibm9uZSIsInBzIjoidGVzdCJ9#MyCustomServer"
    item, err := ParseShareLink(link)
    if err != nil {
        t.Fatalf("failed to parse: %v", err)
    }
    _, err = GenerateXrayConfig(item, nil, 10808, 10809)
    if err != nil {
        t.Fatalf("GenerateXrayConfig failed with fragment: %v", err)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v -run TestGenerateXrayConfig_VMessWithRemarkFragment ./internal/configmgr`
Expected: FAIL with base64 decoding error.

- [ ] **Step 3: Write minimal implementation in `generator.go`**
In `internal/configmgr/generator.go:case "vmess":`:
```go
b64 := strings.TrimPrefix(cfg.RawURL, "vmess://")
if idx := strings.IndexAny(b64, "#?"); idx != -1 {
    b64 = b64[:idx]
}
decoded, err := decodeBase64(b64)
```

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/configmgr`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/configmgr/generator.go internal/configmgr/generator_test.go
git commit -m "fix(configmgr): strip remark fragment in vmess outbound generation (CFG-D3-01)"
```

---

### Task 4: Unified WebPort Resolution & Service Synchronization (`DEFECT-06-01`)

**Files:**
- Modify: `cmd/v2raynix/main.go:L44, L126`, `scripts/v2raynix.service`
- Test: `internal/cli/setup_test.go`

**Interfaces:**
- Consumes: `flag.CommandLine`, `st.GetSettings().WebPort`
- Produces: Dynamic web port resolution respecting store settings when `-port` flag is unassigned.

- [ ] **Step 1: Write the failing test**
In `internal/cli/setup_test.go`, test that when store has custom WebPort (e.g. 3000), `ApplyWebPort` persists port 3000, and helper returns 3000.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v -run TestApplyWebPort ./internal/cli`

- [ ] **Step 3: Write minimal implementation in `cmd/v2raynix/main.go` and `scripts/v2raynix.service`**
In `cmd/v2raynix/main.go`:
```go
// Check if port flag was explicitly passed by the user
portExplicit := false
flag.Visit(func(f *flag.Flag) {
    if f.Name == "port" {
        portExplicit = true
    }
})

listenPort := *port
if !portExplicit && settings != nil && settings.WebPort > 0 {
    listenPort = settings.WebPort
}
addr := fmt.Sprintf("0.0.0.0:%d", listenPort)
```
In `scripts/v2raynix.service`:
Change `ExecStart=/usr/local/bin/v2raynix -port 2080 -data-dir /etc/v2raynix` to:
`ExecStart=/usr/local/bin/v2raynix -data-dir /etc/v2raynix` so that `main.go` resolves `settings.WebPort` dynamically from the store!

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/cli`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add cmd/v2raynix/main.go scripts/v2raynix.service internal/cli/setup_test.go
git commit -m "fix(cli): unify web port resolution between store settings and systemd service (DEFECT-06-01)"
```

---

### Task 5: Store Latency Persistence Batching (`STO-01`)

**Files:**
- Modify: `internal/store/store.go`, `internal/api/router.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `map[string]int` (node ID -> latency)
- Produces: `UpdateLatenciesBatch(results map[string]int) error` executing a single `persist()` flush.

- [ ] **Step 1: Write the failing test**
In `internal/store/store_test.go`, write `TestFileStore_UpdateLatenciesBatch` asserting that updating 20 nodes modifies latencies in memory and persists cleanly.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v -run TestFileStore_UpdateLatenciesBatch ./internal/store`
Expected: FAIL (method undefined)

- [ ] **Step 3: Write minimal implementation in `store.go` and `router.go`**
Add `UpdateLatenciesBatch` to `Store` interface and `FileStore`:
```go
func (fs *FileStore) UpdateLatenciesBatch(latencies map[string]int) error {
    fs.mu.Lock()
    defer fs.mu.Unlock()

    for id, lat := range latencies {
        if item, ok := fs.data.Configs[id]; ok {
            item.Latency = lat
        }
    }
    return fs.persist()
}
```
Update `handlePingAll` and `handleTestAll` in `internal/api/router.go` to use `UpdateLatenciesBatch(results)` instead of looping `UpdateLatency`.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/store ./internal/api`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/store/store.go internal/api/router.go internal/store/store_test.go
git commit -m "perf(store): add UpdateLatenciesBatch to eliminate disk freeze on batch ping (STO-01)"
```

---

### Task 6: Core Updater Cross-Device Link Fallback (`DEFECT-06-04`)

**Files:**
- Modify: `internal/updater/updater.go:L301-L328`
- Test: `internal/updater/updater_test.go`

**Interfaces:**
- Consumes: `srcPath string`, `dstPath string`
- Produces: `atomicMove(src, dst string) error` that falls back to copying if `os.Rename` returns an `EXDEV` error.

- [ ] **Step 1: Write the failing test**
In `internal/updater/updater_test.go`, test `atomicMove` helper function across simulated devices or verify fallback logic.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v -run TestAtomicMove ./internal/updater`

- [ ] **Step 3: Write minimal implementation in `updater.go`**
In `internal/updater/updater.go`:
```go
func atomicMove(src, dst string) error {
    err := os.Rename(src, dst)
    if err == nil {
        return nil
    }

    // Check for cross-device link error (EXDEV)
    var linkErr *os.LinkError
    if errors.As(err, &linkErr) {
        // Fallback: copy to temp file on destination directory then rename
        tmpDst := dst + ".tmp." + strconv.FormatInt(time.Now().UnixNano(), 10)
        if copyErr := copyFile(src, tmpDst); copyErr != nil {
            return fmt.Errorf("cross-device copy failed: %w", copyErr)
        }
        _ = os.Chmod(tmpDst, 0755)
        if renErr := os.Rename(tmpDst, dst); renErr != nil {
            _ = os.Remove(tmpDst)
            return fmt.Errorf("cross-device final rename failed: %w", renErr)
        }
        _ = os.Remove(src)
        return nil
    }
    return err
}
```

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/updater`
Expected: PASS

- [ ] **Step 5: Commit**
```bash
git add internal/updater/updater.go internal/updater/updater_test.go
git commit -m "fix(updater): add cross-device atomic move fallback for tmpfs (DEFECT-06-04)"
```

---

### Task 7: Pinger IPv6 Formatting & Context Timeout (`PING-02`, `PING-04`)

**Files:**
- Modify: `internal/pinger/pinger.go:L170-L178`
- Test: `internal/pinger/pinger_test.go`

**Interfaces:**
- Consumes: `cfg.Server string`, `cfg.Port int`
- Produces: Properly bracketed IPv6 address strings (`[::1]:1080`) using `net.JoinHostPort`.

- [x] **Step 1: Write the failing test**
In `internal/pinger/pinger_test.go`, test formatting of an IPv6 node address.

- [x] **Step 2: Run test to verify it fails**
Run: `go test -v -run TestIPv6Formatting ./internal/pinger`

- [x] **Step 3: Write minimal implementation in `pinger.go`**
In `internal/pinger/pinger.go`:
Use `net.JoinHostPort(cfg.Server, strconv.Itoa(cfg.Port))` instead of `fmt.Sprintf("%s:%d", ...)`.

- [x] **Step 4: Run test to verify it passes**
Run: `go test -v ./internal/pinger`
Expected: PASS

- [x] **Step 5: Commit**
```bash
git add internal/pinger/pinger.go internal/pinger/pinger_test.go
git commit -m "fix(pinger): use net.JoinHostPort for bracketed IPv6 formatting (PING-04)"
```

---

### Task 8: Web Frontend Fetch Timeouts & Full Build Verification (`WEB-01`)

**Files:**
- Modify: `web/src/services/api.js`
- Test: `web/` build pipeline

**Interfaces:**
- Consumes: `fetch(url, options)`
- Produces: Standard 10-second `AbortController` timeout on all frontend API requests to prevent permanent UI lockup.

- [ ] **Step 1: Add AbortController timeout to `apiFetch` in `web/src/services/api.js`**
```javascript
const controller = new AbortController();
const timeoutId = setTimeout(() => controller.abort(), 10000);

try {
    const response = await fetch(url, {
        ...options,
        signal: controller.signal,
    });
    clearTimeout(timeoutId);
    return response;
} catch (err) {
    clearTimeout(timeoutId);
    if (err.name === 'AbortError') {
        throw new Error('Request timed out');
    }
    throw err;
}
```

- [ ] **Step 2: Run frontend build to verify syntax and bundle**
Run: `cd web; npm run build`
Expected: `dist/` built successfully with 0 errors.

- [ ] **Step 3: Run full backend regression test suite**
Run: `go test ./...`
Expected: 100% PASS across all packages.

- [ ] **Step 4: Commit**
```bash
git add web/src/services/api.js
git commit -m "fix(web): add standard 10-second request timeout to prevent hanging promises (WEB-01)"
```
