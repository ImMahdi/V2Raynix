# Domain Briefing 07: REST API & HTTP Router Subsystem

> **Subagent Role:** Principal API Security & Web Services Auditor  
> **Audited Files:** `internal/api/router.go`, `internal/api/router_test.go`  
> **Output Report File:** `docs/audit/reports/07-api-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Deep Knowledge Transfer & Architecture

The `api` package exposes the HTTP REST interface using Go 1.22+ enhanced standard library routing patterns (`"POST /api/..."`), connecting the React frontend to the Supervisor, Store, Auth, and Pinger subsystems.

### Key Architectural Concepts:
1. **Routing Architecture:**
   - Implemented using standard library `http.NewServeMux()` with Go 1.22 method + path pattern matching (e.g. `POST /api/configs/{id}/activate`).
   - `Dependencies` struct holds references to `store.Store`, `core.Supervisor`, `JWTSecret`, and optional embedded `StaticFS`.
2. **Middleware & Authentication:**
   - Global `ServeHTTP`: Adds CORS headers (`Access-Control-Allow-Origin: *`) and handles `OPTIONS` pre-flight requests.
   - `requireAuth`: Higher-order middleware extracting `Bearer <token>` from the `Authorization` header, validating JWT claims via `auth.ValidateJWT`.
3. **Core API Endpoints:**
   - Auth: `/api/auth/login`, `/api/auth/me`, `/api/auth/password`.
   - Configs: `/api/configs`, `/api/configs/{id}`, `/api/configs/{id}/activate`, `/api/configs/ping-all`.
   - Tunnel & SafeMode: `/api/tunnel/status`, `/api/tunnel/connect`, `/api/tunnel/disconnect`, `/api/tunnel/safe-mode/confirm`, `/api/tunnel/safe-mode/rollback`.
   - Routing Rules & Logs: `/api/routing/rules`, `/api/system/logs`.
   - Static SPA Serving: Fallback handler for frontend assets.

---

## 2. Concerns, Pitfalls & Critical Warnings

When auditing this domain, pay hyper-vigilant attention to these failure modes:
1. **Default Credential Auto-Reset Security Vulnerability:**
   In `handleLogin()`:
   ```go
   admin, err := r.deps.Store.GetAdminUser()
   if err != nil || admin == nil {
       hash, _ := auth.HashPassword("admin")
       admin = &store.UserAccount{ Username: "admin", PasswordHash: hash }
       _ = r.deps.Store.SetAdminUser(admin)
   }
   ```
   If the store is temporarily locked, slow, or encounters an ephemeral reading glitch (`err != nil`), the system automatically resets the administrator password to `"admin"`! An attacker triggering an error or store contention could force a password reset to default credentials.
2. **Denial of Service via Unbounded Request Bodies:**
   Every handler invokes `json.NewDecoder(req.Body).Decode(&body)` directly without wrapping `req.Body` in `http.MaxBytesReader(w, req.Body, limit)`. An unauthenticated attacker sending a continuous 500MB stream to `POST /api/auth/login` can cause OOM crashes.
3. **Missing Login Rate Limiting & Brute-Force Vulnerability:**
   `handleLogin` has zero rate limiting, CAPTCHA, or exponential lockout. An automated attacker can brute-force the administrator password at thousands of attempts per second over localhost or LAN.
4. **I/O Starvation in Batch Config Upload:**
   In `handleCreateConfig()`, `body.Content` is split into lines and each line is parsed and saved via `r.deps.Store.SaveConfig(item)`. Each `SaveConfig()` executes a synchronous disk write and atomic rename. Uploading a subscription file containing 1,000 configs triggers 1,000 separate disk writes, freezing the API thread.
5. **Permissive Wildcard CORS on Root Control Plane:**
   `Access-Control-Allow-Origin: *` is applied globally. Because this service runs as root and manages kernel routing, permissive CORS exposes the management server to malicious websites if the administrator browses the web while logged in.
6. **Internal Error Message Disclosure:**
   Handlers return raw `err.Error()` in HTTP 500 responses (`map[string]string{"error": err.Error()}`), exposing internal filesystem paths, OS error codes, and architectural details to clients.

---

## 3. Lead Architect Directives & Step-by-Step Instructions

### Step 1: Mandatory Architectural Graph Exploration (`graphify`)
Before reading line-by-line, run:
```powershell
python -m graphify.cli explain api
# or
python -m graphify.cli query "how does api package route requests to supervisor and store"
```
Check connections between `Router`, `Supervisor`, and `Store`.

### Step 2: Initialize Report Scaffold
Immediately create `docs/audit/reports/07-api-report.md` with standard sections.

### Step 3: Step-by-Step Systematic Code Inspection & Incremental Updates
Audit each area sequentially and update the report immediately:
1. **Authentication Flow & Credential Security (`router.go:L117-L204`):** Check admin auto-initialization, password changes, and rate limiting.
2. **Request Validation & DoS Protections (`router.go:L95-L113`, `L214-L256`):** Check body size limits, input sanitization, and batch handling.
3. **Tunnel Control Endpoints (`router.go:L306-L350`):** Check state synchronization, error responses, and race conditions.
4. **CORS & HTTP Security Headers (`router.go:L38-L51`):** Audit header configurations, Content-Security-Policy, and X-Content-Type-Options.

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
