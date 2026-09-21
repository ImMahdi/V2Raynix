# Phase 3: Protocol Ingestion & Config Synthesis Robustness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remediate all protocol ingestion, share link parsing, and Xray configuration generation defects across `internal/configmgr`: VMess camouflage settings (`CFG-02`), dual-format Shadowsocks credential extraction (`CFG-03`), base64 decoding resilience & fragment/query stripping (`CFG-04`), VMess type assertion safety and alterId preservation (`CFG-05`), and VLESS Reality/Vision compatibility validation (`CFG-06`).

**Tech Stack:** Go 1.22+, `encoding/json`, `encoding/base64`, `net/url`, Xray-core configuration v1.8.x+.

**Spec:** `docs/audit/reports/03-configmgr-report.md` & `docs/audit/FINAL_COMPREHENSIVE_AUDIT_REPORT.md`.

---

## Global Constraints

- **TDD MANDATE:** Every fix must be preceded by a failing unit test reproducing the defect, followed by minimal code to pass, verified by test output.
- **ZERO REGRESSION:** All existing unit tests across `./...` must continue to pass cleanly.
- **BACKWARD COMPATIBILITY:** Existing public signatures (`ParseShareLink`, `GenerateXrayConfig`) must be preserved.

---

## Review Focus

1. **VMess Camouflage:** Does a VMess link with `ws` or `grpc` generate proper `wsSettings` (path, Host) and `grpcSettings` in Xray JSON?
2. **Legacy Shadowsocks:** Does `ss://base64(method:password@host:port)` generate valid non-empty `method` and `password`?
3. **MIME / Whitespace Sanitization:** Does `decodeBase64` handle whitespace and line breaks without errors?
4. **Link Fragments:** Does `parseVMess` cleanly strip `#Tag` before decoding?
5. **Reality Validation:** Does VLESS with `security=reality` reject empty `pbk` or `sni` with a clean error?

---

## File Structure

- Modify: `internal/configmgr/parser.go`
- Test: `internal/configmgr/parser_test.go`
- Modify: `internal/configmgr/generator.go`
- Test: `internal/configmgr/generator_test.go`

---

### Task 1: Base64 Sanitation, Fragment Stripping & Query Sanitization (`CFG-04`)

**Files:**
- Modify: `internal/configmgr/parser.go`
- Test: `internal/configmgr/parser_test.go`

- [ ] **Step 1: Write failing tests in `parser_test.go`**
  - Test `parseVMess` with `#RemarkFragment`.
  - Test `decodeBase64` with embedded whitespace and newlines (`\r\n`).
  - Test `parseShadowsocks` with SIP003 query parameters (`:8388?plugin=obfs-local#Tag`).
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/configmgr -run "TestParser_Base64Whitespace|TestParser_VMessFragment|TestParser_ShadowsocksQuery" -v`
- [ ] **Step 3: Implement fixes in `parser.go`**
  - Sanitize whitespace in `decodeBase64`.
  - Strip `#` and `?` in `parseVMess`.
  - Strip `?` from port string in `parseShadowsocks`.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/configmgr -v`
- [ ] **Step 5: Commit changes**
  `git add internal/configmgr/ && git commit -m "fix(configmgr): sanitize base64 whitespace, strip fragments and query parameters (CFG-04)"`

---

### Task 2: VMessJSON Type Safety, Boundary Validation & AlterId Support (`CFG-05`)

**Files:**
- Modify: `internal/configmgr/parser.go`
- Modify: `internal/configmgr/generator.go`
- Test: `internal/configmgr/parser_test.go`
- Test: `internal/configmgr/generator_test.go`

- [ ] **Step 1: Write failing tests in `parser_test.go` & `generator_test.go`**
  - Test `Port` delivered as `int`, `int64`, and invalid port `> 65535`.
  - Test preserving `aid` / `alterId: 64` in `GenerateXrayConfig`.
- [ ] **Step 2: Run tests to verify failure**
  `go test ./internal/configmgr -v`
- [ ] **Step 3: Implement type switch and alterId preservation**
  - Update `parseVMess` in `parser.go` for full port typing and boundary checks (`1..65535`).
  - Extract and honor `alterId` in `generator.go`.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/configmgr -v`
- [ ] **Step 5: Commit changes**
  `git add internal/configmgr/ && git commit -m "fix(configmgr): enforce port boundaries and preserve alterId in VMess (CFG-05)"`

---

### Task 3: Dual-Format Shadowsocks Outbound Credential Extraction (`CFG-03`)

**Files:**
- Modify: `internal/configmgr/generator.go`
- Test: `internal/configmgr/generator_test.go`

- [ ] **Step 1: Write failing test in `generator_test.go` for legacy Shadowsocks format**
  - Generate config from `ss://YmYtY2ZiOnRlc3RAMTkyLjE2OC4xMDAuMTo4ODg4#LegacyNode`.
  - Assert `method` and `password` are correctly populated.
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/configmgr -run TestGenerator_ShadowsocksLegacy -v`
- [ ] **Step 3: Implement dual-format decoding in `generator.go`**
  - Support both SIP002 (`base64(method:password)@host:port`) and Legacy (`base64(method:password@host:port)`).
  - Return error if credentials cannot be extracted.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/configmgr -run TestGenerator_ShadowsocksLegacy -v`
- [ ] **Step 5: Commit changes**
  `git add internal/configmgr/ && git commit -m "fix(configmgr): extract credentials from both SIP002 and legacy Shadowsocks formats (CFG-03)"`

---

### Task 4: Complete VMess Camouflage Synthesis (WebSocket, gRPC, HTTP/2, TLS SNI) (`CFG-02`)

**Files:**
- Modify: `internal/configmgr/generator.go`
- Test: `internal/configmgr/generator_test.go`

- [ ] **Step 1: Write failing test in `generator_test.go` for VMess transports**
  - Test VMess WebSocket: check `wsSettings.path` and `wsSettings.headers.Host`.
  - Test VMess gRPC: check `grpcSettings.serviceName`.
  - Test VMess TLS: check `tlsSettings.serverName`.
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/configmgr -run TestGenerator_VMessCamouflage -v`
- [ ] **Step 3: Implement VMess streamSettings synthesis in `generator.go`**
  - Map `wsSettings`, `grpcSettings`, `httpSettings`, and `tlsSettings`.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/configmgr -run TestGenerator_VMessCamouflage -v`
- [ ] **Step 5: Commit changes**
  `git add internal/configmgr/ && git commit -m "feat(configmgr): synthesize complete streamSettings for VMess transports (CFG-02)"`

---

### Task 5: VLESS Reality & XTLS Vision Flow Compatibility Checks (`CFG-06`)

**Files:**
- Modify: `internal/configmgr/generator.go`
- Test: `internal/configmgr/generator_test.go`

- [ ] **Step 1: Write failing tests in `generator_test.go`**
  - Test VLESS Reality without `pbk` returns descriptive error.
  - Test VLESS Reality without `sni` returns descriptive error.
  - Test VLESS Vision flow over WebSocket returns descriptive error.
- [ ] **Step 2: Run test to verify failure**
  `go test ./internal/configmgr -run TestGenerator_RealityValidation -v`
- [ ] **Step 3: Implement validation in `generator.go`**
  - Check non-empty `pbk` and `sni` when `security == "reality"`.
  - Validate transport compatibility when `flow == "xtls-rprx-vision"`.
- [ ] **Step 4: Run tests to verify pass**
  `go test ./internal/configmgr -v`
- [ ] **Step 5: Commit changes**
  `git add internal/configmgr/ && git commit -m "fix(configmgr): add strict Reality public key and Vision flow validation (CFG-06)"`

---

### Task 6: Full Regression Testing & Knowledge Graph Update

- [ ] **Step 1: Run complete test suite**
  `go test -count=1 ./...`
- [ ] **Step 2: Update graphify knowledge graph**
  `graphify update .`
- [ ] **Step 3: Update walkthrough artifact**
