# Domain Briefing 06: REST API, Admin CLI, Installer & Core Updater Subsystem

> **Subagent Role:** Principal API Security, CLI Management & Systems Deployment Auditor  
> **Audited Files:** `internal/api/*`, `internal/cli/*`, `internal/updater/*`, `cmd/v2raynix/*`, `scripts/install.sh`  
> **Output Report File:** `docs/audit/reports/06-api-cli-updater-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Subsystem Architecture & Responsibilities
This domain encapsulates administrative interfaces and deployment workflows:
- `internal/api`: REST API endpoints for authentication, configs, tunnel controls, logs, ping, and core updates.
- `internal/cli`: Terminal setup interface (`v2raynix setup`) with interactive menu and scriptable flags (`--user`, `--pass`, `--port`, `--service`).
- `internal/updater`: Core version checking against GitHub releases, safe atomic downloading, executable replacement, and rollback.
- `cmd/v2raynix/main.go`: Main daemon entrypoint, argument parsing, store initialization, HTTP listener binding.
- `scripts/install.sh`: Production installer provisioning systemd services, Xray core, and tun2socks dependencies.

---

## 2. Critical Audit Directives & Failure Modes to Investigate

Auditor must systematically examine:
1. **Web Port Configuration Discrepancy (CRITICAL INVESTIGATION):**
   - In `internal/cli/setup.go`: `ApplyWebPort()` updates `settings.WebPort` in the store and calls `RestartService()`.
   - In `cmd/v2raynix/main.go`: How is the HTTP server address constructed (`addr := fmt.Sprintf("0.0.0.0:%d", *port)`)? Does `main.go` prioritize CLI `-port` flag over `settings.WebPort` in the store?
   - In `systemd/v2raynix.service`: Is `-port 2080` hardcoded in `ExecStart`? If a user changes port via `setup` or Web settings, does the daemon actually bind to the new port on restart, or does it stay pinned to 2080?
2. **API Denial of Service & Request Body Limits:**
   - In `router.go` and handlers: is `http.MaxBytesReader` enforced across endpoints accepting JSON bodies (configs, subscriptions) to prevent memory exhaustion DoS?
3. **Authentication Endpoint Rate Limiting & Brute-Force:**
   - Are login requests rate-limited? What prevents automated dictionary attacks against `/api/auth/login`?
4. **CORS & Error Information Disclosure:**
   - Is `Access-Control-Allow-Origin` overly permissive on administrative endpoints?
   - Do 500 error responses disclose internal file paths, stack traces, or OS details via `err.Error()`?
5. **Core Updater Atomicity & Symlink / Execution Safety:**
   - In `internal/updater`: is binary replacement atomic? What happens if target binary is currently executing (`text file busy` on Linux)? Does rollback work if a corrupted binary is downloaded?
6. **Installer Script (`scripts/install.sh`) Security:**
   - Are external downloads (Xray, tun2socks) verified against checksums?
   - Are mirror fallbacks secure against MITM tampering?
   - Does systemd unit configuration enforce least privilege where possible?

---

## 3. Subagent Execution Instructions
1. Run `python -m graphify.cli explain api` or query graph to understand dependencies.
2. Progressively write findings to `docs/audit/reports/06-api-cli-updater-report.md`.
3. Every finding MUST follow the 7-field defect schema (ID, Code Location, Severity, Trigger/Root Cause, PoC, Recommended Fix, Strengths).
4. STRICTLY READ-ONLY: Do not edit any code files.
