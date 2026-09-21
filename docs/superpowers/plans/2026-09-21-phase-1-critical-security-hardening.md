# Phase 1: Security & Anti-Lockout Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remediate all 6 critical and high-priority Tier 1 vulnerabilities identified in the master audit report (Shell Command Injection `NET-01`, Supervisor Rollback Self-Deadlock `SUP-04`, Direct Outbound Routing Loop `CFG-01`, Insecure Admin Auto-Reset `API-01`, Empty Secret Key Forgery `AUTH-01`, and Hardcoded Web Port Lockout `SUP-06`), using strict Test-Driven Development (TDD).

**Architecture:** 
1. **Network Subsystem:** Eliminate shell execution (`sh -c`) in `ExecuteCommands`; transition to structured command invocation with discrete argument slices (`exec.Command(binary, argv...)`) and strict input validation (`net.ParseIP`, interface regex).
2. **Configmgr Subsystem:** Inject `streamSettings: { sockopt: { mark: 81 } }` into the `freedom` (direct) outbound in `GenerateXrayConfig` to exempt direct traffic from Table 100 loops.
3. **Core Supervisor Subsystem:** Eliminate reentrant mutex acquisition in `RollbackSafeMode()` by directly calling an unlocked internal rollback helper or unlocking prior to callback execution; extract dynamic `webPort` from system settings.
4. **Auth Subsystem:** Require a minimum 32-byte secret key in `GenerateJWT` and `ValidateJWT`, rejecting empty or weak keys with `ErrWeakSecret`.
5. **API Subsystem:** Restrict default admin initialization in `handleLogin` to `errors.Is(err, store.ErrNotFound)`, returning HTTP 500 on all other storage errors.

**Tech Stack:** Go 1.22+, `testing`, Linux Netfilter/iproute2 abstractions, HMAC-SHA256 JWT, Xray-core config schema.

**Spec:** `docs/audit/FINAL_COMPREHENSIVE_AUDIT_REPORT.md` (Phase 1 Remediation Roadmap).

---

## Global Constraints

- **TDD MANDATE:** Every fix must be preceded by a failing unit test that reproduces the defect, followed by the minimal code required to pass, and verified by running the test suite.
- **ZERO REGRESSION:** All existing unit tests across `./...` must continue to pass cleanly.
- **NO BREAKING API CHANGES:** REST endpoints (`/api/...`) and JSON response schemas must remain backward compatible with the frontend.
- **NO ARBITRARY SHELL STRINGS:** All OS-level command executions must pass binary names and argument arrays explicitly.

---

## Review Focus

1. **Shell Injection Surface:** Verify that strings with metacharacters (`;`, `&`, `|`, `` ` ``, `$()`) cannot be executed under any circumstances.
2. **Deadlock Freedom:** Verify that `RollbackSafeMode()` and automated countdown expiration never block or dead-lock `supervisor.mu`.
3. **Routing Table Isolation:** Verify that generated Xray configurations stamp `mark: 81` on both `proxy` and `direct` outbounds.
4. **Credential Safety:** Verify that storage reading errors (mocked disk failures) never overwrite user credentials with `"admin:admin"`.
5. **Key Entropy:** Verify that empty or short keys (< 32 bytes) are rejected by JWT generation and validation routines.

---

## File Structure

- Modify: `internal/network/routing.go:L23-L115`, `L199-L209`
- Test: `internal/network/routing_test.go`
- Modify: `internal/configmgr/generator.go:L54-L66`
- Test: `internal/configmgr/generator_test.go`
- Modify: `internal/core/supervisor.go:L181-L194`, `L216-L228`, `L273-L281`
- Test: `internal/core/supervisor_test.go`
- Modify: `internal/auth/auth.go:L47-L115`
- Test: `internal/auth/auth_test.go`
- Modify: `internal/api/router.go:L127-L141`
- Test: `internal/api/router_test.go`

---

### Task 1: Remediate Shell Injection & Validate Inputs in Network Routing (`NET-01`)

**Files:**
- Modify: `internal/network/routing.go`
- Test: `internal/network/routing_test.go`

**Interfaces:**
- Consumes: `remoteProxyIP`, `defaultIface`, `defaultGw`, `sshPort`, `webPort`
- Produces: `StructuredCommand{ Binary string, Args []string }` or direct argument execution via `ExecuteStructuredCommands([]Command)`

- [ ] **Step 1: Write failing test in `routing_test.go` for shell injection and input validation**
  Assert that inputs containing command separators or invalid IP formats fail validation or are not parsed into raw shell commands.
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/network -run TestCommandInjectionDefense -v`
- [ ] **Step 3: Implement input validation and structured command execution in `routing.go`**
  - Add `ValidateRoutingParams(remoteProxyIP, defaultIface, defaultGw string, sshPort, webPort int) error`
  - Replace `sh -c` in `ExecuteCommands` with discrete argument execution using `exec.Command(binary, args...)`.
- [ ] **Step 4: Run network tests to verify pass**
  `go test ./internal/network -v`
- [ ] **Step 5: Commit changes**
  `git add internal/network/ && git commit -m "fix(network): eliminate sh -c and enforce strict parameter validation (NET-01)"`

---

### Task 2: Inject Anti-Loop `sockopt.mark = 81` on Direct Outbound (`CFG-01`)

**Files:**
- Modify: `internal/configmgr/generator.go:L54-L66`
- Test: `internal/configmgr/generator_test.go`

**Interfaces:**
- Consumes: `GenerateXrayConfig(*store.ConfigItem, []*store.RoutingRule, int, int)`
- Produces: JSON payload with `streamSettings.sockopt.mark: 81` on both `proxy` and `direct` outbounds.

- [ ] **Step 1: Write failing test in `generator_test.go`**
  Assert that the generated config's `direct` outbound contains `streamSettings` with `sockopt.mark == 81`.
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/configmgr -run TestDirectOutboundMark -v`
- [ ] **Step 3: Update `generator.go` to attach `sockopt.mark = 81` to direct outbound**
  Add `streamSettings` with `sockopt.mark = 81` to the `freedom` outbound map.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/configmgr -v`
- [ ] **Step 5: Commit changes**
  `git add internal/configmgr/ && git commit -m "fix(configmgr): add sockopt mark 81 to direct outbound to prevent routing loops (CFG-01)"`

---

### Task 3: Fix Supervisor Rollback Self-Deadlock & Dynamic Web Port (`SUP-04` & `SUP-06`)

**Files:**
- Modify: `internal/core/supervisor.go:L181-L194`, `L216-L228`, `L273-L281`
- Test: `internal/core/supervisor_test.go`

**Interfaces:**
- Consumes: `Supervisor.RollbackSafeMode()`, `Supervisor.startSafeModeTimerLocked()`, `Supervisor.webPort`
- Produces: Non-blocking rollback and dynamic Web UI port exclusion in policy routing.

- [ ] **Step 1: Write failing test in `supervisor_test.go` for `RollbackSafeMode`**
  Test calling `RollbackSafeMode()` while tunnel is in mock connected state; verify it completes without deadlocking.
- [ ] **Step 2: Run test to verify deadlock**
  `go test ./internal/core -run TestSupervisor_RollbackSafeMode_NoDeadlock -timeout 5s -v`
- [ ] **Step 3: Refactor `RollbackSafeMode` and rollback callback**
  - In `RollbackSafeMode()`, do not hold `s.mu` when invoking `s.safeMode.CancelAndRollback()`, or invoke internal `s.stopTunnelLocked()` directly.
  - Dynamically read `webPort` from store settings or supervisor field instead of hardcoded `2080`.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/core -v`
- [ ] **Step 5: Commit changes**
  `git add internal/core/ && git commit -m "fix(core): resolve rollback self-deadlock and support dynamic web port (SUP-04, SUP-06)"`

---

### Task 4: Enforce Minimum Secret Key Entropy for JWT (`AUTH-01`)

**Files:**
- Modify: `internal/auth/auth.go:L47-L115`
- Test: `internal/auth/auth_test.go`

**Interfaces:**
- Consumes: `GenerateJWT(username string, secret []byte, duration time.Duration)`
- Consumes: `ValidateJWT(tokenString string, secret []byte)`
- Produces: `ErrWeakSecret` when `len(secret) < 32`.

- [ ] **Step 1: Write failing test in `auth_test.go`**
  Assert that passing an empty or short secret (< 32 bytes) returns `ErrWeakSecret`.
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/auth -run TestWeakSecretValidation -v`
- [ ] **Step 3: Add `ErrWeakSecret` and length validation to `auth.go`**
  Check `if len(secret) < 32 { return "", ErrWeakSecret }` in `GenerateJWT` and `ValidateJWT`.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/auth -v`
- [ ] **Step 5: Commit changes**
  `git add internal/auth/ && git commit -m "fix(auth): enforce minimum 32-byte secret key length for JWTs (AUTH-01)"`

---

### Task 5: Prevent Insecure Default Admin Auto-Reset on Store Errors (`API-01`)

**Files:**
- Modify: `internal/api/router.go:L127-L141`
- Test: `internal/api/router_test.go`

**Interfaces:**
- Consumes: `r.deps.Store.GetAdminUser()`
- Produces: HTTP 500 when store fails; only creates default admin if `errors.Is(err, store.ErrNotFound)`.

- [ ] **Step 1: Write failing test in `router_test.go`**
  Create a mock store that returns an I/O error on `GetAdminUser()`. Verify `POST /api/auth/login` returns HTTP 500 and does NOT overwrite credentials with `"admin"`.
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/api -run TestLoginStoreErrorHandling -v`
- [ ] **Step 3: Update `handleLogin` in `router.go`**
  Check error type: if `errors.Is(err, store.ErrNotFound)` or `(err == nil && admin == nil)`, initialize default admin. If `err != nil`, return HTTP 500 immediately.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/api -v`
- [ ] **Step 5: Commit changes**
  `git add internal/api/ && git commit -m "fix(api): prevent admin credential overwrite on transient store errors (API-01)"`

---

### Task 6: Full-Suite Verification & Remote Regression Smoke Test

**Files:**
- All packages across `./...`

- [ ] **Step 1: Run comprehensive test suite with race detector**
  `go test -race ./... -v`
- [ ] **Step 2: Verify git working tree clean**
  `git status`
- [ ] **Step 3: Update walkthrough and documentation**
  Update `walkthrough.md` with verification evidence.
