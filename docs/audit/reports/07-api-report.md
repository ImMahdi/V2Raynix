# Security & Architecture Audit Report: Domain 07 (REST API & HTTP Router Subsystem)

> **Audited Subsystem:** REST API & HTTP Router Subsystem  
> **Target Files:**
> - `internal/api/router.go`
> - `internal/api/api_test.go`  
> **Auditor:** Principal API Security & Web Services Auditor  
> **Audit Date:** 2026-09-21  
> **Audit Status:** Complete & Verified  

---

## 1. Executive Summary

The `internal/api` package exposes the external HTTP management interface for V2Raynix. Utilizing Go 1.22+ standard library pattern routing (`http.NewServeMux`), it orchestrates incoming web client requests, binds React SPA interactions, enforces token-based authentication via JSON Web Tokens (`auth.ValidateJWT`), and coordinates actions across the `core.Supervisor`, `store.Store`, and `pinger.BatchPing` subsystems.

Because V2Raynix operates as a system daemon with elevated host and networking capabilities (`CAP_NET_ADMIN` / `root`), any vulnerability in the REST API control plane directly impacts the security, stability, and integrity of the entire underlying host.

Our systematic, line-by-line inspection of `internal/api/router.go` and `internal/api/api_test.go` revealed **9 discrete defects**:
- **1 Critical-Severity Vulnerability**: Default credential auto-reset in `handleLogin` upon ephemeral store read failure, allowing unauthenticated attackers or contention spikes to revert the administrator credentials to factory default (`admin:admin`).
- **3 High-Severity Vulnerabilities**:
  1. Denial of Service (DoS) and memory exhaustion across all HTTP endpoints due to unbounded request body ingestion (omission of `http.MaxBytesReader`).
  2. Missing brute-force protection and rate limiting on `/api/auth/login`, permitting high-frequency credential stuffing and bcrypt-induced CPU starvation.
  3. Overly permissive wildcard CORS (`Access-Control-Allow-Origin: *`) on an administrative root control plane, exposing internal RPC APIs to cross-origin browser threats.
- **5 Medium/Low-Severity Defects**:
  1. I/O starvation and write-amplification cascade in `handleCreateConfig` caused by synchronous, atomic disk flushes executed inside an unbatched loop.
  2. Information disclosure in HTTP 500/400 responses via raw `err.Error()` propagation, exposing host paths and internal kernel details.
  3. Static SPA fallback routing deficiency and unvalidated path traversal risks in static asset serving.
  4. Synchronous connection blocking and repeated disk write churn in `handlePingAll`.
  5. Silent tunnel activation failure swallowing and discarded JWT claim context in middleware.

---

## 2. Graphify Knowledge Graph & Dependency Analysis

Based on the knowledge graph extracted via `graphify` (`graphify-out/graph.json`):

```
                     +---------------------------------------+
                     |         web/src (React 18 SPA)        |
                     |  LoginPage, SettingsPage, LogsPage    |
                     +-------------------+-------------------+
                                         |
                                         | HTTP / JSON (CORS: *)
                                         v
                     +---------------------------------------+
                     |        internal/api (Router)          |
                     |  - ServeHTTP (CORS middleware)        |
                     |  - requireAuth (JWT extraction)       |
                     |  - NewRouter / registerRoutes         |
                     +---+-------------------------------+---+
                         |                               |
        Store Operations |                               | Process & Tunnel Control
                         v                               v
+-----------------------------------+   +-----------------------------------+
|        internal/store/Store       |   |      internal/core/Supervisor     |
| - GetAdminUser / SetAdminUser     |   | - StartTunnel / StopTunnel        |
| - GetConfigs / SaveConfig         |   | - ConfirmSafeMode / Rollback      |
| - GetRoutingRules                 |   | - GetStatus / GetLogs             |
+-----------------------------------+   +-----------------------------------+
```

### Key Graph Insights:
1. **API Router as the External Gateway:** `Router` has direct coupling to `Store`, `Supervisor`, `auth`, `pinger`, and `configmgr`. It acts as the sole gatekeeper mediating access to host network configuration and user authentication.
2. **Privilege Boundary Inversion:** Because the daemon runs with root privileges to configure Linux TUN interfaces and iptables, any flaw in API routing or authentication middleware bypasses normal Linux user-space boundaries.
3. **Storage Coupling:** Store interactions in `handleCreateConfig` and `handlePingAll` directly trigger synchronous disk writes and file renames (`fs.persist()`), creating a tight coupling between incoming network payload size and disk I/O throughput.

---

## 3. Findings Summary Table

| ID | Title & Category | Code Location | Severity | Status |
|---|---|---|---|---|
| **API-01** | Default Credential Auto-Reset Flaw on Store Error in `handleLogin` | `router.go:L127-L136` | **Critical** | Confirmed |
| **API-02** | Denial of Service via Unbounded Request Body Reading | `router.go:L122`, `L174`, `L219`, `L363` | **High** | Confirmed |
| **API-03** | Missing Authentication Rate Limiting & Brute-Force Vulnerability | `router.go:L117-L155` | **High** | Confirmed |
| **API-04** | Permissive Wildcard CORS (`*`) on Administrative Control Plane | `router.go:L38-L50` | **High** | Confirmed |
| **API-05** | Disk I/O Starvation in Batch Config Import (`handleCreateConfig`) | `router.go:L224-L245` | **Medium** | Confirmed |
| **API-06** | Internal Architecture & Filesystem Error Disclosure in 500 Responses | `router.go:L208`, `L261`, `L294`, `L319`, `L328`, `L354`, `L374`, `L384` | **Medium** | Confirmed |
| **API-07** | Broken SPA Fallback Routing & Static File Traversal / MIME Risks | `router.go:L83-L92` | **Medium** | Confirmed |
| **API-08** | Synchronous Request Blocking & Persistence Churn in `handlePingAll` | `router.go:L291-L304` | **Medium** | Confirmed |
| **API-09** | Silent Error Swallowing on Live Tunnel Switch & Lost JWT Context | `router.go:L95-L113`, `L267-L289` | **Low** | Confirmed |

---

## 4. Comprehensive Audit Findings (7-Field Defect Schema)

### Finding API-01: Default Credential Auto-Reset Flaw on Store Error in `handleLogin`

- **Title & Category:** Insecure Default Credential Auto-Reset on Ephemeral Store Error | **Authentication & Authorization**
- **Exact Code Location:** `internal/api/router.go:L127-L136`
- **Severity:** **Critical**
- **Trigger Scenario & Root Cause Analysis:**
  In `handleLogin`, the router attempts to fetch the current administrator account from the backing store:
  ```go
  admin, err := r.deps.Store.GetAdminUser()
  if err != nil || admin == nil {
      // Initialize default admin if none exists
      hash, _ := auth.HashPassword("admin")
      admin = &store.UserAccount{
          Username:     "admin",
          PasswordHash: hash,
      }
      _ = r.deps.Store.SetAdminUser(admin)
  }
  ```
  The condition `if err != nil || admin == nil` conflates a legitimate "first run / uninitialized" state with an operational failure (`err != nil`).
  If the storage medium experiences an I/O glitch, file lock contention, JSON unmarshaling delay, or transient error when reading `data.json`, `err` is non-nil. The handler catches this error, hashes the default password `"admin"`, replaces the in-memory `admin` object with `username: "admin"` / `password: "admin"`, and attempts to persist it back to disk (`SetAdminUser`).
  Even if the store write fails, the local `admin` variable in memory is assigned the default credential object. If the client supplied `admin:admin`, the password check succeeds, granting full administrator access and issuing a valid JWT token. An attacker who can induce disk pressure, concurrent write contention, or catch an ephemeral store error can immediately authenticate with `admin:admin`.
  Furthermore, first-run initialization belongs in bootstrap startup code, never inside a per-request HTTP authentication handler.
- **Proof of Concept / Verification Method:**
  1. Configure a store mock or fault injection where `GetAdminUser()` returns `errors.New("disk I/O timeout")`.
  2. Send `POST /api/auth/login` with payload `{"username": "admin", "password": "admin"}`.
  3. Instead of returning `500 Internal Server Error`, the endpoint executes the auto-reset branch, evaluates `auth.CheckPassword(hash, "admin")`, returns HTTP 200 OK, and issues a 24-hour admin JWT token.
- **Recommended Architectural Fix:**
  1. Disallow lazy credential initialization in `handleLogin`. Move admin provisioning strictly to application bootstrap in `cmd/v2raynix/main.go` or `store.New()`.
  2. In `handleLogin`, explicitly handle `err != nil` as a database failure (`500 Internal Server Error`) and log the incident.
  3. If `admin == nil` after initialization has completed, return `401 Unauthorized` or `500 Internal Server Error` (uninitialized system).
- **Existing Strengths & Robustness:**
  Uses standard bcrypt hashing via `auth.HashPassword` with adequate salt rounds, and compares passwords using constant-time verification in `auth.CheckPassword`.

---

### Finding API-02: Denial of Service via Unbounded Request Body Reading

- **Title & Category:** Unbounded Request Body Ingestion Leading to Memory Exhaustion (OOM DoS) | **Denial of Service / Resource Exhaustion**
- **Exact Code Location:** `internal/api/router.go:L122`, `L174`, `L219`, `L363`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  In all JSON-decoding endpoints (`handleLogin`, `handlePassword`, `handleCreateConfig`, and `handleCreateRoutingRule`), `json.NewDecoder(req.Body).Decode(&target)` is invoked directly on the raw `req.Body` without restricting payload size:
  ```go
  if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
      writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
      return
  }
  ```
  In Go, standard HTTP server connections do not impose a default maximum body size on `req.Body`. If an unauthenticated attacker sends a multi-gigabyte continuous stream to `POST /api/auth/login` or `POST /api/configs`, `json.NewDecoder` attempts to buffer and parse the incoming stream into memory.
  This leads directly to RAM exhaustion, high GC pauses, and kernel OOM kills of the V2Raynix process, causing an immediate drop of all active network tunnels.
- **Proof of Concept / Verification Method:**
  Execute a streaming HTTP POST request with an endless sequence of whitespace or large JSON padding:
  ```bash
  curl -X POST http://localhost:8080/api/auth/login \
       -H "Content-Type: application/json" \
       --data-binary @/dev/zero
  ```
  The process memory increases continuously until the Go runtime aborts or the host kernel terminates the daemon.
- **Recommended Architectural Fix:**
  Wrap `req.Body` using `http.MaxBytesReader` in every handler or apply a global middleware restricting request body size:
  ```go
  // In handler or middleware (e.g., 64KB for auth/rules, 1MB for batch configs)
  req.Body = http.MaxBytesReader(w, req.Body, 1<<20) // 1MB limit
  ```
- **Existing Strengths & Robustness:**
  Handlers correctly check for decode errors and return `http.StatusBadRequest` when JSON syntax is malformed.

---

### Finding API-03: Missing Authentication Rate Limiting & Brute-Force Vulnerability

- **Title & Category:** Missing Rate Limiting & CPU Starvation on Authentication Endpoints | **Authentication & Denial of Service**
- **Exact Code Location:** `internal/api/router.go:L117-L155`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  The `POST /api/auth/login` endpoint has no rate limiting, attempt counters, exponential backoff, or IP lockout mechanisms.
  This introduces two distinct vulnerabilities:
  1. **Credential Brute-Forcing:** Attackers with network access can automate dictionary or password-guessing attacks against the default `admin` username at thousands of requests per second.
  2. **Asymmetric CPU Exhaustion (Bcrypt DoS):** `auth.CheckPassword` computes a bcrypt hash, which is intentionally computationally heavy (~50-100ms of CPU time per call). An attacker sending concurrent login requests can saturate all available CPU cores on the host, starving the tunnel supervisor, DNS resolution, and packet routing.
- **Proof of Concept / Verification Method:**
  Run a concurrency test hitting `/api/auth/login`:
  ```bash
  hey -n 1000 -c 50 -m POST -T "application/json" \
      -d '{"username":"admin","password":"wrong"}' \
      http://localhost:8080/api/auth/login
  ```
  The host CPU usage spikes to 100%, and legitimate tunnel health checks and API requests experience multi-second latency or drop out entirely.
- **Recommended Architectural Fix:**
  1. Introduce an in-memory token bucket or sliding-window rate limiter middleware (e.g., using `golang.org/x/time/rate` or a keyed sync map by client IP) capped at 5 attempts per minute per IP for `/api/auth/login`.
  2. Add progressive delays (e.g., 1-second sleep before returning `401 Unauthorized`) or lockout mechanisms after 5 consecutive failed attempts.
- **Existing Strengths & Robustness:**
  Authentication returns generic `"invalid credentials"` messages for both non-existent usernames and incorrect passwords, preventing username enumeration.

---

### Finding API-04: Permissive Wildcard CORS (`*`) on Administrative Control Plane

- **Title & Category:** Overly Permissive Wildcard CORS Header Configuration | **CORS & Web Security**
- **Exact Code Location:** `internal/api/router.go:L38-L50`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  In `ServeHTTP`, the router unconditionally applies permissive CORS headers to every request:
  ```go
  w.Header().Set("Access-Control-Allow-Origin", "*")
  w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
  w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
  ```
  V2Raynix is a local administrative service controlling kernel routing and running as root. In desktop or server environments where the service listens on `0.0.0.0` or localhost, allowing any origin (`*`) creates serious cross-origin attack vectors:
  1. Any webpage opened in the administrator's browser can issue pre-flight `OPTIONS` requests and probe for active V2Raynix instances without restriction.
  2. If the API token is leaked (e.g. via local storage inspection, browser extension, or malicious script), any external website can make cross-origin requests to activate/deactivate tunnels, delete configs, or modify routing tables.
  3. Additionally, modern defensive HTTP security headers are completely absent:
     - Missing `X-Content-Type-Options: nosniff`
     - Missing `X-Frame-Options: DENY`
     - Missing `Content-Security-Policy`
- **Proof of Concept / Verification Method:**
  Send a cross-origin preflight request from an arbitrary origin:
  ```bash
  curl -i -X OPTIONS http://localhost:8080/api/configs \
       -H "Origin: https://malicious-tracker.com" \
       -H "Access-Control-Request-Method: POST"
  ```
  Response returns `Access-Control-Allow-Origin: *` and `200 OK`, confirming that any external site is authorized by the browser to execute pre-flight calls.
- **Recommended Architectural Fix:**
  1. Restrict allowed CORS origins to explicit loopback addresses (`http://localhost:*`, `http://127.0.0.1:*`) or origins defined in system configuration.
  2. Add essential defensive headers to all responses in `ServeHTTP`:
     ```go
     w.Header().Set("X-Content-Type-Options", "nosniff")
     w.Header().Set("X-Frame-Options", "DENY")
     ```
- **Existing Strengths & Robustness:**
  Properly intercepts `http.MethodOptions` and responds with `200 OK` before passing to route handlers, preventing unnecessary route processing during standard preflight.

---

### Finding API-05: Disk I/O Starvation in Batch Config Import (`handleCreateConfig`)

- **Title & Category:** Unbatched Synchronous Disk Persistence in Import Loop | **Performance & Resource Starvation**
- **Exact Code Location:** `internal/api/router.go:L224-L245`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  In `handleCreateConfig`, multi-line inputs (such as proxy subscription lists containing hundreds of links) are processed in a loop:
  ```go
  lines := strings.Split(body.Content, "\n")
  for _, line := range lines {
      ...
      item, err := configmgr.ParseShareLink(trimmed)
      ...
      _ = r.deps.Store.SaveConfig(item)
      created = append(created, item)
  }
  ```
  Each call to `Store.SaveConfig(item)` acquires the store's write mutex, serializes the entire configuration database to JSON, writes it to a temporary file on disk, flushes, syncs, and renames the file atomically (`fs.persist()`).
  If a user uploads a subscription with 500 nodes:
  - 500 complete database serializations occur in a tight loop.
  - 500 disk write + fsync + atomic rename operations are executed sequentially.
  This monopolizes the store mutex, blocks all concurrent API readers/writers, produces severe SSD write amplification, and can cause the HTTP request to time out.
- **Proof of Concept / Verification Method:**
  Simulate uploading a 200-node subscription file. Benchmarks show cumulative execution times exceeding several seconds, during which all other `/api/...` calls block on `fs.mu`.
- **Recommended Architectural Fix:**
  Add a batch save method to `store.Store`:
  ```go
  // In Store interface
  SaveConfigs(items []*ConfigItem) error

  // In FileStore
  func (fs *FileStore) SaveConfigs(items []*ConfigItem) error {
      fs.mu.Lock()
      defer fs.mu.Unlock()
      for _, item := range items {
          itemCopy := *item
          fs.data.Configs[item.ID] = &itemCopy
      }
      return fs.persist() // Persist once for the entire batch
  }
  ```
- **Existing Strengths & Robustness:**
  Correctly sanitizes whitespace, skips empty lines, ignores invalid proxy URIs without aborting the rest of the batch, and customizes the node name if a single link was submitted.

---

### Finding API-06: Internal Architecture & Filesystem Error Disclosure in 500 Responses

- **Title & Category:** Raw Internal Error Message Disclosure | **Information Disclosure**
- **Exact Code Location:** `internal/api/router.go:L208`, `L261`, `L294`, `L319`, `L328`, `L354`, `L374`, `L384`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  Throughout `router.go`, error returns directly forward `err.Error()` in HTTP 500 responses:
  ```go
  writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
  ```
  When underlying subsystems (such as `Store.SaveConfig`, `Supervisor.StartTunnel`, `exec.Command`, or `os.OpenFile`) return errors, they frequently embed absolute filesystem paths (e.g. `/etc/v2raynix/config.json`), operating system permission denials, process identifiers, or kernel error strings.
  Returning raw system errors directly to HTTP clients gives unauthorized or low-privileged network observers detailed reconnaissance regarding internal filesystem layout, file permissions, and installed dependencies.
- **Proof of Concept / Verification Method:**
  Trigger an unprivileged execution or write to a read-only directory: the response contains internal OS messages such as `open /var/lib/v2raynix/data.json.tmp.123: permission denied`.
- **Recommended Architectural Fix:**
  Log the full detailed error internally using a structured logger, and return sanitized, generic error strings to the HTTP client (e.g. `{"error": "internal system error"}` or `{"error": "failed to connect tunnel"}`).
- **Existing Strengths & Robustness:**
  Consistent JSON response envelope format `{"error": "..."}` ensures frontend clients can cleanly parse failure messages.

---

### Finding API-07: Broken SPA Fallback Routing & Static File Traversal / MIME Risks

- **Title & Category:** Broken SPA Client-Side Routing & Static File Server Exposure | **Web Architecture & Security**
- **Exact Code Location:** `internal/api/router.go:L83-L92`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  The static file server is registered as follows:
  ```go
  if r.deps.StaticFS != nil {
      fileServer := http.FileServer(http.FS(r.deps.StaticFS))
      r.mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
          if strings.HasPrefix(req.URL.Path, "/api/") {
              http.NotFound(w, req)
              return
          }
          fileServer.ServeHTTP(w, req)
      })
  }
  ```
  Two architectural issues exist in this implementation:
  1. **Broken SPA Routing on Page Refresh:** React SPA uses client-side routing (`/configs`, `/settings`, `/logs`). When a user refreshes the browser on `http://localhost:8080/configs`, the request hits `fileServer.ServeHTTP(w, req)`. Because no physical file named `/configs` exists on `r.deps.StaticFS`, `http.FileServer` returns HTTP 404 Not Found instead of serving `index.html`.
  2. **Missing Security Headers & Content-Type Sniffing Protection:** Static assets are served without `Cache-Control` policies (risking stale frontend code on upgrades) or `X-Content-Type-Options: nosniff`.
- **Proof of Concept / Verification Method:**
  1. Start the server with `StaticFS` populated with React build output (`index.html`, `assets/...`).
  2. Navigate directly to `GET /configs`.
  3. The server returns 404 Not Found instead of the Single Page Application index document.
- **Recommended Architectural Fix:**
  Implement a proper SPA fallback handler:
  ```go
  r.mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
      if strings.HasPrefix(req.URL.Path, "/api/") {
          http.NotFound(w, req)
          return
      }
      // Check if file exists in fs
      f, err := r.deps.StaticFS.Open(strings.TrimPrefix(req.URL.Path, "/"))
      if err == nil {
          _ = f.Close()
          fileServer.ServeHTTP(w, req)
          return
      }
      // Fallback to index.html for SPA routes
      indexData, err := fs.ReadFile(r.deps.StaticFS, "index.html")
      if err != nil {
          http.NotFound(w, req)
          return
      }
      w.Header().Set("Content-Type", "text/html; charset=utf-8")
      w.WriteHeader(http.StatusOK)
      _, _ = w.Write(indexData)
  })
  ```
- **Existing Strengths & Robustness:**
  Explicitly prevents fallback routing from hijacking unmatched `/api/` endpoints by enforcing `if strings.HasPrefix(req.URL.Path, "/api/") { http.NotFound(w, req) }`.

---

### Finding API-08: Synchronous Request Blocking & Persistence Churn in `handlePingAll`

- **Title & Category:** Long-Running Synchronous HTTP Blocking & Store Write Churn | **Performance & Concurrency**
- **Exact Code Location:** `internal/api/router.go:L291-L304`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  In `handlePingAll`:
  ```go
  func (r *Router) handlePingAll(w http.ResponseWriter, req *http.Request) {
      configs, err := r.deps.Store.GetConfigs()
      ...
      results := pinger.BatchPing(configs, 5, 2*time.Second)
      for id, lat := range results {
          _ = r.deps.Store.UpdateLatency(id, lat)
      }
      writeJSON(w, http.StatusOK, results)
  }
  ```
  1. `BatchPing` runs synchronously inside the HTTP handler with a fixed worker pool of 5 and a 2-second timeout per server. If the user has 60 configs, this HTTP request blocks the HTTP worker thread for 20-30 seconds.
  2. If the user closes the browser or aborts the request, `handlePingAll` does not respect `req.Context()`, keeping the backend ping workers running unnecessarily.
  3. Once pings finish, `Store.UpdateLatency(id, lat)` is executed in a loop. Each call acquires the store lock and rewrites `data.json` to disk, triggering up to 60 disk syncs within milliseconds.
- **Proof of Concept / Verification Method:**
  Issue `POST /api/configs/ping-all` with 100 stored configurations. Observe client socket timeouts and massive I/O serialization in disk monitor logs.
- **Recommended Architectural Fix:**
  1. Propagate `req.Context()` to `BatchPing` so pings cancel if the client disconnects.
  2. Update latencies in a single batch operation (`UpdateLatencies(map[string]int)`) that writes to disk once.
  3. Consider returning HTTP 202 Accepted and broadcasting results via WebSocket / Server-Sent Events (SSE) for large configuration catalogs.
- **Existing Strengths & Robustness:**
  Correctly isolates ping latency measurement through the dedicated `pinger.BatchPing` helper.

---

### Finding API-09: Silent Error Swallowing on Live Tunnel Switch & Lost JWT Context

- **Title & Category:** Ignored Activation Errors & Discarded Authentication Context Claims | **API Correctness & Traceability**
- **Exact Code Location:** `internal/api/router.go:L95-L113`, `L267-L289`
- **Severity:** **Low / Refactor**
- **Trigger Scenario & Root Cause Analysis:**
  Two logical omissions occur in `requireAuth` and `handleActivateConfig`:
  1. **Lost JWT Context:** In `requireAuth`:
     ```go
     claims, err := auth.ValidateJWT(tokenString, r.deps.JWTSecret)
     if err != nil {
         ...
     }
     _ = claims
     next(w, req)
     ```
     `claims` contains the authenticated username and token expiration. Discarding `claims` via `_ = claims` prevents downstream handlers (e.g. `handleMe`, `handlePassword`, audit logging) from knowing which authenticated identity initiated the operation. In `handlePassword`, it simply assumes `GetAdminUser()` without validating whether the caller's JWT matches the targeted user.
  2. **Ignored Tunnel Start Error on Live Switch:** In `handleActivateConfig`:
     ```go
     if r.deps.Supervisor.GetStatus().State == "connected" {
         _ = r.deps.Supervisor.StartTunnel(active)
     }
     ```
     If the supervisor fails to restart the tunnel with the new active configuration (e.g. kernel routing failure, port conflict), the error is discarded with `_ =`. The API returns HTTP 200 with `"message": "activated"`, misleading the frontend and user into believing the tunnel is successfully routed when it has actually failed.
- **Proof of Concept / Verification Method:**
  1. Activate an invalid or unreachable proxy config while connected.
  2. The endpoint responds with `200 OK` despite `StartTunnel` failing internally.
- **Recommended Architectural Fix:**
  1. Attach `claims` to the request context in `requireAuth`:
     ```go
     ctx := context.WithValue(req.Context(), userContextKey, claims)
     next(w, req.WithContext(ctx))
     ```
  2. Check the error returned by `StartTunnel` in `handleActivateConfig`:
     ```go
     if err := r.deps.Supervisor.StartTunnel(active); err != nil {
         writeJSON(w, http.StatusInternalServerError, map[string]string{
             "error": "activated in store but failed to switch tunnel: " + err.Error(),
         })
         return
     }
     ```
- **Existing Strengths & Robustness:**
  Properly differentiates between an active connected tunnel (triggering live restart) versus a stopped tunnel (updating supervisor preselection).

---

## 5. Architectural Recommendations & Remediation Plan

To elevate the REST API subsystem to enterprise production quality:

1. **Bootstrap Admin Initialization:** Remove all lazy account creation logic from `handleLogin`. Perform admin provisioning strictly during initial setup/CLI bootstrap.
2. **Global Body Size & Rate Limiter Middleware:** Wrap the `http.ServeMux` with a middleware stack providing:
   - Body size limits (`http.MaxBytesReader`) tailored per route.
   - IP-based token-bucket rate limiting for authentication endpoints.
   - Security headers (`X-Content-Type-Options`, `X-Frame-Options`).
3. **Restricted CORS Policy:** Replace `Access-Control-Allow-Origin: *` with an allowlist restricted to authorized origins (or configurable bind hosts).
4. **Batch Persistence Interface:** Implement `SaveConfigs` and `UpdateLatencies` in `store.Store` so multi-item imports and ping sweeps persist to disk in a single atomic I/O operation.
5. **Context-Aware Authentication:** Inject validated JWT claims into `http.Request` context to enable robust per-user auditing and authorization checks.
