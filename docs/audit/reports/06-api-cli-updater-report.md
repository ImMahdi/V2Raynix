# Security & Architecture Audit Report: Domain 06 (REST API, Admin CLI, Installer & Core Updater Subsystem)

> **Audited Subsystem:** REST API, Terminal Setup CLI, Core Updater & Systems Deployment Suite  
> **Audited Files:**
> - `internal/api/router.go`
> - `internal/api/api_test.go`
> - `internal/cli/setup.go`
> - `internal/cli/setup_test.go`
> - `internal/updater/updater.go`
> - `internal/updater/updater_test.go`
> - `cmd/v2raynix/main.go`
> - `scripts/install.sh`
> - `scripts/v2raynix.service`
>
> **Subagent Role:** Principal API Security, CLI Management & Systems Deployment Auditor  
> **Audit Date:** 2026-09-24  
> **Audit Status:** Complete & Verified  

---

## 1. Executive Summary

This report delivers an exhaustive, line-by-line security and systems architecture audit of **Domain 06: REST API, Admin CLI, Installer & Core Updater Subsystem** of V2Raynix. 

The audited subsystem forms the management and operational control plane of V2Raynix. It encompasses:
1. **REST Management API (`internal/api`)**: Go 1.22+ routing layer providing authenticated endpoints for configuration import, proxy switching, safe-mode watchdog controls, latency measurement, system logs, and core engine binary updates.
2. **Terminal Administration CLI (`internal/cli/setup.go`)**: Interactive console and scriptable CLI (`v2raynix setup`) enabling headless operators to reset credentials, change web panel listening ports, and manage the systemd daemon.
3. **Core Updater Engine (`internal/updater`)**: Automated subsystem responsible for checking upstream GitHub releases for `Xray-core` and `tun2socks`, downloading release bundles, and updating binaries.
4. **Daemon Entrypoint (`cmd/v2raynix/main.go`)**: Main process lifecycle orchestrator, CLI flag parser, store initiator, and HTTP listener binder.
5. **Production Installer & Service Definitions (`scripts/install.sh`, `scripts/v2raynix.service`)**: Shell installer provisioning systemd services, downloading external core dependencies, and managing local firewall policies.

Because V2Raynix operates with elevated Linux host privileges (`root` and `CAP_NET_ADMIN` / `CAP_NET_BIND_SERVICE` / `CAP_NET_RAW`) to manipulate kernel network routing tables, iptables chains, and virtual TUN adapters, vulnerabilities in this subsystem directly expose the host operating system to complete compromise, denial of service, or severe operational failure.

### Summary of Audit Findings
Our inspection identified **14 discrete defects** categorized across the severity spectrum:
- **1 Critical-Severity Vulnerability**:
  - Unauthenticated remote code execution risk via unverified external core downloads and third-party mirror fallback without checksum validation in both `updater.go` and `install.sh`.
- **4 High-Severity Vulnerabilities**:
  - Architectural web port discrepancy between CLI setup, store settings, daemon flags, and systemd `ExecStart`.
  - Rate limiter client IP spoofing and trivial brute-force bypass via unverified `X-Forwarded-For` / `X-Real-IP` headers.
  - Cross-device link invalidation (`EXDEV`) across `/tmp` (`tmpfs`) and `/usr/local/bin` (`rootfs`) causing total core update failure on standard Linux distributions.
  - Missing pre-execution verification and asymmetric rollback in `updateTun2socks`.
- **5 Medium-Severity Defects**:
  - Unbounded memory leak and DoS in `loginRateLimiter` via IP state table poisoning.
  - Synchronous disk I/O thrashing and write amplification in batch ping and test endpoints.
  - Permissive wildcard CORS (`Access-Control-Allow-Origin: *`) on administrative endpoints.
  - Uncontrolled core updater concurrency lacking coordination with `core.Supervisor`.
  - Installer creation of world-readable `/etc/v2raynix` directory and unhardened systemd service unit.
- **4 Low-Severity Defects**:
  - Sensitive internal environment and filesystem path disclosure in HTTP 500 error responses.
  - Broken SPA subroute navigation due to lack of HTML5 pushState fallback in static file serving.
  - Ephemeral in-memory JWT secret causing total user session invalidation across daemon restarts.
  - Silent error swallowing on live tunnel switch and lost JWT claim context in middleware.

---

## 2. Subsystem Architecture & Dependency Analysis

Based on the architectural analysis and dependency graph of Domain 06:

```
                                  +---------------------------------------+
                                  |         Web Browser / Frontend        |
                                  |         React 18 SPA (web/dist)       |
                                  +-------------------+-------------------+
                                                      |
                                        HTTP/JSON     |  (Wildcard CORS: *)
                                                      v
+-----------------------+         +---------------------------------------+
|  scripts/install.sh   |         |        internal/api (Router)          |
|  - Insecure Mirrors   |         |  - ServeHTTP (CORS: *)                |
|  - Missing Checksums  |         |  - requireAuth (JWT extraction)       |
|  - World-Readable Perm|         |  - loginRateLimiter (IP-spoofable)    |
+-----------+-----------+         |  - decodeJSON (MaxBytesReader)        |
            |                     +---+-------------------------------+---+
            | Provisions Service      |                               |
            v                         | Store Ops                     | Process & Core Ops
+-----------------------+             v                               v
|  systemd Service Unit | +-----------------------+       +-----------------------+
|  ExecStart hardcodes  | | internal/store (Store)|       | internal/core (Sup)   |
|  "-port 2080"         | | - v2raynix.json       |       | - Start/Stop Tunnel   |
+-----------+-----------+ | - Settings.WebPort    |       | - SafeMode Watchdog   |
            |             +-----------+-----------+       +-----------+-----------+
            | Launches                ^                               ^
            v                         | Read/Write                    |
+-----------------------+             |                               |
|  cmd/v2raynix/main.go |-------------+                               |
|  - Ignores WebPort!   |                                             |
|  - Random JWT Secret  |                                             |
+-----------------------+                                             |
            ^                                                         |
            | Invokes                                                 | Uncoordinated
+-----------+-----------+                                             |
| internal/cli/setup.go |                                 +-----------+-----------+
| - ApplyWebPort (DB)   |                                 | internal/updater      |
| - Restarts Service    |                                 | - EXDEV rename bug    |
+-----------------------+                                 | - No checksums        |
                                                          | - Third-party mirror  |
                                                          +-----------------------+
```

### Architectural Divergences & System Blind Spots:
1. **The WebPort Divergence Triad:** Three separate authorities claim ownership of the listening port:
   - `internal/cli/setup.go` modifies `settings.WebPort` in the JSON store.
   - `scripts/v2raynix.service` hardcodes `-port 2080` in `ExecStart`.
   - `cmd/v2raynix/main.go` parses `-port` flag (defaulting to 2080) and never consults `settings.WebPort`.
   This guarantees that port modifications made via setup CLI or web settings are completely ignored by the running daemon.
2. **Filesystem Boundary Ignorance in Core Updater:** `internal/updater` assumes `os.Rename` works universally across `/tmp` and `/usr/local/bin`. On standard Linux server installations where `/tmp` is mounted on `tmpfs`, the `rename(2)` syscall fails with `EXDEV`, permanently preventing core updates.
3. **Supply Chain Blind Spot:** Both the shell installer and Go updater download executable binary zip archives from an unauthenticated third-party proxy mirror (`ghproxy.net`) without any cryptographic checksum or signature verification, and immediately execute the unpacked binaries as root.

---

## 3. Findings Matrix

| ID | Title & Category | Code Location | Severity | Status |
|---|---|---|---|---|
| **DEFECT-06-01** | Web Port Architectural Discrepancy (CLI Setup vs Main vs Systemd) | `setup.go:L40-L52`, `main.go:L44, L126`, `v2raynix.service:L10` | **High** | Confirmed |
| **DEFECT-06-02** | Rate Limiter IP Spoofing & Brute-Force Bypass via Untrusted Headers | `router.go:L141-L145, L511-L524` | **High** | Confirmed |
| **DEFECT-06-03** | Unbounded Memory Leak & DoS in `loginRateLimiter` via Table Poisoning | `router.go:L526-L587` | **Medium** | Confirmed |
| **DEFECT-06-04** | Cross-Device Link Invalidation (`EXDEV`) Causing Total Core Update Failure | `updater.go:L301-L328, L356-L379` | **High** | Confirmed |
| **DEFECT-06-05** | Missing Checksum Validation & Unverified Third-Party Mirror Code Execution | `updater.go:L125, L201, L309`, `install.sh:L55, L84, L112` | **Critical** | Confirmed |
| **DEFECT-06-06** | Asymmetric Rollback & Missing Pre/Post Verification in `updateTun2socks` | `updater.go:L346-L383` | **High** | Confirmed |
| **DEFECT-06-07** | Uncontrolled Core Updater Concurrency & Lack of Supervisor Coordination | `updater.go:L251-L280`, `router.go:L606-L629` | **Medium** | Confirmed |
| **DEFECT-06-08** | Permissive Wildcard CORS (`*`) on Administrative Control Plane | `router.go:L54-L66` | **Medium** | Confirmed |
| **DEFECT-06-09** | Internal Filesystem & System Environment Disclosure in 500 Responses | `router.go:L250, L306, L396, L467, L600, L622` | **Low** | Confirmed |
| **DEFECT-06-10** | Broken SPA Subroute Navigation Due to Missing Static Fallback | `router.go:L106-L115` | **Low** | Confirmed |
| **DEFECT-06-11** | Synchronous Disk I/O Thrashing in Batch Ping & Test Endpoints | `router.go:L343-L348, L375-L379`, `store.go:L327-L337` | **Medium** | Confirmed |
| **DEFECT-06-12** | Silent Error Swallowing on Live Tunnel Switch & Discarded JWT Context | `router.go:L133, L321-L328` | **Low** | Confirmed |
| **DEFECT-06-13** | World-Readable Configuration Directory & Unhardened Systemd Service | `install.sh:L48, L161-L179`, `v2raynix.service:L6-L15` | **Medium** | Confirmed |
| **DEFECT-06-14** | Ephemeral JWT Secret Generation Causes Session Eviction on Daemon Restart | `main.go:L99-L102` | **Low** | Confirmed |

---

## 4. Comprehensive Audit Findings (7-Field Defect Schema)

### Finding DEFECT-06-01: Web Port Architectural Discrepancy (CLI Setup vs Main vs Systemd)

- **ID & Title:** `DEFECT-06-01`: Web Management Port Configuration Discrepancy & De-Synchronization | **System Architecture & Lifecycle**
- **Exact Code Location:**
  - `internal/cli/setup.go:L40-L52`, `L117-L123`, `L186-L199`
  - `cmd/v2raynix/main.go:L44`, `L104-L108`, `L126-L132`
  - `scripts/v2raynix.service:L10`
  - `scripts/install.sh:L171`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  1. An administrator uses the terminal utility `v2raynix setup` to change the web management port to `8080` (e.g. running non-interactively `v2raynix setup --port 8080` or via interactive menu option `[2]`).
  2. `ApplyWebPort(st, 8080)` saves `settings.WebPort = 8080` into `/etc/v2raynix/v2raynix.json` and runs `RestartService()`, which triggers `systemctl restart v2raynix`.
  3. However, `systemd` invokes `ExecStart` defined in `/etc/systemd/system/v2raynix.service`:
     ```ini
     ExecStart=/usr/local/bin/v2raynix -port 2080 -data-dir /etc/v2raynix
     ```
  4. In `cmd/v2raynix/main.go`, `port := flag.Int("port", 2080, ...)` parses `-port 2080`.
  5. In `main.go:L104`, the daemon loads `settings, _ := st.GetSettings()`, checks `settings.SafeModeSeconds`, but **completely ignores `settings.WebPort`**!
  6. The HTTP server binds unconditionally to `addr := fmt.Sprintf("0.0.0.0:%d", *port)` (`0.0.0.0:2080`).
  7. **Consequence:** The web panel remains bound to port 2080. Meanwhile, `setup.go` tells the administrator:
     ```
     [OK] Port updated to 8080. Restarting service to apply...
     Panel URL: http://<your-server-ip>:8080
     ```
     The administrator opens port 8080 in their external firewall, attempts to connect to `http://<ip>:8080`, and is met with `Connection Refused`. The system is in an unrecoverable state of de-synchronization.
- **PoC / Verification:**
  1. Initialize store with `WebPort: 8080`.
  2. Launch `v2raynix` binary with default flags as provisioned by systemd: `v2raynix -port 2080 -data-dir /etc/v2raynix`.
  3. Execute `ss -tlpn | grep v2raynix` or `curl -I http://127.0.0.1:8080`.
  4. Observed output: Port 8080 is closed; port 2080 is listening. Port setting in store is completely disregarded.
- **Recommended Fix:**
  1. In `cmd/v2raynix/main.go`, resolve the web port with clear precedence:
     - If the user explicitly provided `-port` on the command line (detect via `flag.CommandLine.Visit` or by defaulting flag to 0), use the CLI flag.
     - Otherwise, if `settings.WebPort > 0`, use `settings.WebPort`.
     - Otherwise, fallback to default `2080`.
  2. In `scripts/v2raynix.service` and `scripts/install.sh`, remove the hardcoded `-port 2080` argument from `ExecStart`, allowing the daemon to load the port directly from the store:
     ```ini
     ExecStart=/usr/local/bin/v2raynix -data-dir /etc/v2raynix
     ```
  3. In `cmd/v2raynix/main.go`:
     ```go
     portFlag := flag.Int("port", 0, "Web UI listening port (default: from store settings or 2080)")
     // ...
     flag.Parse()
     // ...
     listenPort := 2080
     if settings != nil && settings.WebPort > 0 {
         listenPort = settings.WebPort
     }
     if *portFlag > 0 {
         listenPort = *portFlag
     }
     addr := fmt.Sprintf("0.0.0.0:%d", listenPort)
     ```
- **Strengths / Existing Mitigations:**
  `ApplyWebPort` correctly validates port ranges (`port < 1 || port > 65535`), persists cleanly to `store.Store`, and initiates a service restart.

---

### Finding DEFECT-06-02: Rate Limiter IP Spoofing & Brute-Force Bypass via Untrusted Headers

- **ID & Title:** `DEFECT-06-02`: Client IP Header Spoofing & Rate Limiter Bypass | **Authentication & Access Control**
- **Exact Code Location:** `internal/api/router.go:L141-L145`, `L511-L524`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  1. `handleLogin` enforces brute-force rate limiting by querying `r.loginLimiter.isBlocked(clientIP)`.
  2. `getClientIP(req)` resolves the client IP as follows:
     ```go
     func getClientIP(req *http.Request) string {
         if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
             parts := strings.Split(xff, ",")
             return strings.TrimSpace(parts[0])
         }
         if xrip := req.Header.Get("X-Real-IP"); xrip != "" {
             return strings.TrimSpace(xrip)
         }
         host, _, err := net.SplitHostPort(req.RemoteAddr)
         if err == nil {
             return host
         }
         return req.RemoteAddr
     }
     ```
  3. `getClientIP` blindly trusts `X-Forwarded-For` and `X-Real-IP` without checking whether `req.RemoteAddr` is a trusted reverse proxy (e.g. `127.0.0.1` or a configured gateway).
  4. V2Raynix binds `0.0.0.0:2080` directly to public/LAN traffic. An attacker directly connecting to the daemon can forge `X-Forwarded-For: 10.0.0.X` or generate a random IP header on every request.
  5. **Consequence:** The rate limiter treats each attempt as coming from a different client IP. An attacker can launch high-speed dictionary and credential-stuffing attacks without ever triggering the 5-attempt limit (`HTTP 429`). Furthermore, each attempt triggers `auth.CheckPassword` (bcrypt cost 10), allowing an attacker to pin CPU utilization at 100%.
- **PoC / Verification:**
  Send 10 sequential failed login requests from the same client machine, incrementing `X-Forwarded-For` header:
  ```bash
  for i in $(seq 1 10); do
    curl -s -o /dev/null -w "%{http_code}\n" -X POST http://127.0.0.1:2080/api/auth/login \
      -H "Content-Type: application/json" \
      -H "X-Forwarded-For: 198.51.100.$i" \
      -d '{"username":"admin","password":"badpassword"}'
  done
  ```
  Observed result: All 10 requests return `HTTP 401 Unauthorized`. The rate limiter is never engaged (`HTTP 429` is never returned) despite exceeding the 5-failure threshold.
- **Recommended Fix:**
  1. Do NOT trust proxy headers by default when running directly bound to public sockets. Only extract `X-Forwarded-For` or `X-Real-IP` if `req.RemoteAddr` matches a strictly configured list of trusted proxies (e.g. `127.0.0.1`).
  2. Fallback to `req.RemoteAddr` as the definitive, un-spoofable client IP address for direct connections:
     ```go
     func getClientIP(req *http.Request, trustProxy bool) string {
         host, _, err := net.SplitHostPort(req.RemoteAddr)
         if err != nil {
             host = req.RemoteAddr
         }
         if !trustProxy || (host != "127.0.0.1" && host != "::1") {
             return host
         }
         if xrip := req.Header.Get("X-Real-IP"); xrip != "" {
             return strings.TrimSpace(xrip)
         }
         if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
             parts := strings.Split(xff, ",")
             return strings.TrimSpace(parts[0])
         }
         return host
     }
     ```
- **Strengths / Existing Mitigations:**
  The `loginRateLimiter` design uses a sliding window (`time.Minute`) and resets counters on successful authentication (`r.loginLimiter.reset(clientIP)`), which works correctly when the IP cannot be spoofed.

---

### Finding DEFECT-06-03: Unbounded Memory Leak & DoS in `loginRateLimiter` via Table Poisoning

- **ID & Title:** `DEFECT-06-03`: Unbounded Memory Leak in Rate Limiter via IP Table Poisoning | **Denial of Service**
- **Exact Code Location:** `internal/api/router.go:L526-L587`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  1. `loginRateLimiter` stores failure timestamps in a map: `attempts map[string][]time.Time`.
  2. In `recordFailure(ip)`:
     ```go
     func (rl *loginRateLimiter) recordFailure(ip string) {
         rl.mu.Lock()
         defer rl.mu.Unlock()
         // filters expired entries for this IP only...
         valid = append(valid, now)
         rl.attempts[ip] = valid
     }
     ```
  3. Notice that `delete(rl.attempts, ip)` is ONLY invoked inside `isBlocked(ip)` when an IP's history drops to 0, or inside `reset(ip)` on successful login.
  4. There is **no background eviction goroutine**, **no global TTL sweep**, and **no upper bound on map size**.
  5. When coupled with DEFECT-06-02 (IP header spoofing) or distributed internet scanning from dynamic IPs, an attacker can send millions of login attempts with randomized IP addresses.
  6. Each request creates a new map entry `rl.attempts[ip]` that is never visited again.
  7. **Consequence:** The `rl.attempts` hash table continuously expands in memory, consuming heap space until the Go runtime triggers an Out-Of-Memory (OOM) panic or the Linux OOM killer terminates `v2raynix`.
- **PoC / Verification:**
  Simulate memory growth by repeatedly issuing failed login requests with generated IP keys:
  ```go
  rl := newLoginRateLimiter(5, time.Minute)
  for i := 0; i < 1_000_000; i++ {
      rl.recordFailure(fmt.Sprintf("10.%d.%d.%d", (i>>16)&255, (i>>8)&255, i&255))
  }
  // Observe rl.attempts map has 1,000,000 retained entries occupying tens of megabytes indefinitely.
  ```
- **Recommended Fix:**
  1. Introduce a maximum capacity limit (e.g., 10,000 active tracking IPs) with LRU eviction.
  2. Implement a periodic background cleanup ticker (e.g., every 5 minutes) to prune stale entries across the entire map:
     ```go
     func (rl *loginRateLimiter) startJanitor(ctx context.Context, interval time.Duration) {
         ticker := time.NewTicker(interval)
         go func() {
             for {
                 select {
                 case <-ticker.C:
                     rl.mu.Lock()
                     cutoff := time.Now().Add(-rl.window)
                     for ip, timestamps := range rl.attempts {
                         if len(timestamps) == 0 || timestamps[len(timestamps)-1].Before(cutoff) {
                             delete(rl.attempts, ip)
                         }
                     }
                     rl.mu.Unlock()
                 case <-ctx.Done():
                     ticker.Stop()
                     return
                 }
             }
         }()
     }
     ```
- **Strengths / Existing Mitigations:**
  Individual IP slices are pruned in-place (`valid := timestamps[:0]`) during evaluation, preventing memory leaks for recurring single-IP requests.

---

### Finding DEFECT-06-04: Cross-Device Link Invalidation (`EXDEV`) Causing Total Core Update Failure

- **ID & Title:** `DEFECT-06-04`: Cross-Device Link Invalidation (`EXDEV`) on Binary Atomic Swap | **Core Updater & System Integrity**
- **Exact Code Location:** `internal/updater/updater.go:L301-L328`, `L356-L379`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  1. In `updateXray`:
     ```go
     tmpBin := filepath.Join(os.TempDir(), "xray.new")
     // ... extracts binary to tmpBin ...
     targetBin := "/usr/local/bin/xray"
     backupBin := targetBin + ".bak"
     if _, err := os.Stat(targetBin); err == nil {
         _ = os.Rename(targetBin, backupBin)
     }
     if err := os.Rename(tmpBin, targetBin); err != nil {
         // Rollback ...
         return fmt.Errorf("failed to swap xray binary: %w", err)
     }
     ```
  2. On standard Linux distributions (Ubuntu 20.04/22.04/24.04, Debian 11/12, Arch Linux, Fedora, RHEL), `os.TempDir()` maps to `/tmp`.
  3. In standard Linux systemd installations, `/tmp` is mounted as an independent `tmpfs` in-memory filesystem, whereas `/usr/local/bin` is located on the persistent root filesystem (`/` on `ext4` or `xfs`).
  4. The Linux kernel `rename(2)` syscall **cannot rename files across filesystem or mount boundaries**. When attempted across different mount points, it immediately returns `EXDEV: Invalid cross-device link`.
  5. Go's `os.Rename` directly wraps `rename(2)` and does not implement cross-filesystem copy-and-unlink fallbacks.
  6. **Consequence:** In production Linux environments, invoking the core update via Web UI (`POST /api/system/update-core`) or CLI will **always fail** with:
     ```
     failed to swap xray binary: rename /tmp/xray.new /usr/local/bin/xray: invalid cross-device link
     ```
     The same defect exists in `updateTun2socks` (`L373`). Core updates are completely non-functional in standard Linux deployments.
- **PoC / Verification:**
  On any standard Linux VM where `/tmp` is a `tmpfs` mount:
  1. Create a dummy file `/tmp/test.bin`.
  2. Attempt `os.Rename("/tmp/test.bin", "/usr/local/bin/test.bin")`.
  3. Observe error: `rename /tmp/test.bin /usr/local/bin/test.bin: invalid cross-device link`.
- **Recommended Fix:**
  Create temporary files and backup files **within the same directory as the target binary** (or same filesystem, e.g. `/usr/local/bin/xray.tmp.PID`), ensuring the rename operation remains strictly on the same filesystem mount:
  ```go
  targetDir := filepath.Dir(targetBin)
  tmpBin := filepath.Join(targetDir, fmt.Sprintf(".%s.new.%d", filepath.Base(targetBin), os.Getpid()))
  defer os.Remove(tmpBin)
  ```
  Alternatively, implement a fallback copy-and-rename utility:
  ```go
  func safeAtomicSwap(src, dst string) error {
      if err := os.Rename(src, dst); err == nil {
          return nil
      }
      // Cross-device fallback: copy src to dst.tmp in dst dir, then rename
      dstTmp := dst + ".tmp"
      if err := copyFile(src, dstTmp); err != nil {
          return err
      }
      return os.Rename(dstTmp, dst)
  }
  ```
- **Strengths / Existing Mitigations:**
  The code attempts rollback (`os.Rename(backupBin, targetBin)`) if the swap fails, preventing the existing executable from being completely lost when the swap errors.

---

### Finding DEFECT-06-05: Missing Checksum Validation & Unverified Third-Party Mirror Code Execution

- **ID & Title:** `DEFECT-06-05`: Remote Code Execution Risk via Unverified Downloads & Third-Party Mirrors | **Supply Chain & Cryptographic Integrity**
- **Exact Code Location:**
  - `internal/updater/updater.go:L125-L133`, `L201-L206`, `L293-L312`
  - `scripts/install.sh:L53-L65`, `L83-L89`, `L111-L121`
- **Severity:** **Critical**
- **Trigger Scenario & Root Cause Analysis:**
  1. Both `scripts/install.sh` and `internal/updater/updater.go` automatically fall back to downloading release binaries from `https://ghproxy.net/`:
     ```go
     // internal/updater/updater.go:L201
     mirrorURL := "https://ghproxy.net/" + url
     resp, err = client.Get(mirrorURL)
     ```
     ```bash
     # scripts/install.sh:L55
     local mirror="https://ghproxy.net/${primary}"
     ```
  2. `ghproxy.net` is an unauthenticated, third-party public reverse proxy operated outside the control of the V2Raynix project.
  3. **Neither the installer nor the updater performs ANY cryptographic integrity verification** (e.g. comparing SHA-256 digests against GitHub release checksums or verifying GPG/Cosign signatures).
  4. In `updater.go:L309-L312`, immediately after unpacking the archive, the daemon executes:
     ```go
     testCmd := exec.Command(tmpBin, "-version")
     if err := testCmd.Run(); err != nil { ... }
     ```
  5. Because the daemon runs as `root`, executing `testCmd.Run()` executes the unverified downloaded binary **with root privileges**.
  6. **Consequence:** If `ghproxy.net` is compromised, DNS-spoofed, or serves a malicious binary archive, or if an attacker performs a TLS interception on unpinned connections, arbitrary executable code is downloaded and executed directly as `root` on the user's host.
- **PoC / Verification:**
  1. Inspect network traffic during `install.sh` or `POST /api/system/update-core`.
  2. Notice that the SHA256 checksums file (`Xray-linux-64.zip.dgst` or `tun2socks...checksums.txt`) published on GitHub releases is never requested, parsed, or verified.
  3. Any zip archive containing a binary named `xray` will be executed directly via `exec.Command(tmpBin, "-version").Run()`.
- **Recommended Fix:**
  1. Require SHA-256 checksum verification before unpacking or executing any downloaded binary. Download the official `dgst` / checksum file directly from GitHub releases API.
  2. Calculate `sha256.Sum256` of `tmpZip` and compare against the release digest before proceeding:
     ```go
     func verifySHA256(filePath, expectedHex string) error {
         f, err := os.Open(filePath)
         if err != nil {
             return err
         }
         defer f.Close()
         h := sha256.New()
         if _, err := io.Copy(h, f); err != nil {
             return err
         }
         actualHex := hex.EncodeToString(h.Sum(nil))
         if !strings.EqualFold(actualHex, strings.TrimSpace(expectedHex)) {
             return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHex, actualHex)
         }
         return nil
     }
     ```
  3. In `scripts/install.sh`, fetch the official checksum file and execute `sha256sum -c` prior to unpacking.
- **Strengths / Existing Mitigations:**
  HTTPS is enforced for primary downloads (`https://github.com/...`).

---

### Finding DEFECT-06-06: Asymmetric Rollback & Missing Pre/Post Verification in `updateTun2socks`

- **ID & Title:** `DEFECT-06-06`: Missing Pre/Post-Verification and Premature Backup Deletion in `updateTun2socks` | **Core Updater & System Integrity**
- **Exact Code Location:** `internal/updater/updater.go:L346-L383`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  1. In `updateXray`, the updater runs verification steps:
     - Pre-swap check: `exec.Command(tmpBin, "-version").Run()`
     - Post-swap check: `exec.Command(targetBin, "-version").Run()`
     - Automated rollback if post-swap check fails: `os.Rename(backupBin, targetBin)`.
  2. However, in `updateTun2socks` (`L356-L382`):
     - **No pre-swap check** is performed on `tmpBin`.
     - `targetBin` is moved to `backupBin`.
     - `os.Rename(tmpBin, targetBin)` is attempted.
     - **No post-swap check** is performed on `targetBin`.
     - `os.Remove(backupBin)` is called immediately without verifying that `tun2socks` can execute!
  3. **Consequence:** If the downloaded `tun2socks` binary is truncated, corrupted, or compiled against an incompatible C library version (glibc/musl), the working backup is deleted immediately. The tunnel manager is left with a broken executable, permanently incapacitating the system's VPN tunneling capabilities.
- **PoC / Verification:**
  Compare `updateXray` (`L309-L342`) with `updateTun2socks` (`L364-L382`):
  `updateTun2socks` lacks any invocation of `exec.Command(tmpBin, "-version")` or `exec.Command(targetBin, "-v")` and unconditionally removes `backupBin` on line 380.
- **Recommended Fix:**
  Add pre-swap testing, post-swap verification, and automated rollback to `updateTun2socks`:
  ```go
  // Pre-swap test
  if err := exec.Command(tmpBin, "-version").Run(); err != nil {
      if err := exec.Command(tmpBin, "-v").Run(); err != nil {
          return fmt.Errorf("verification of new tun2socks binary failed: %w", err)
      }
  }
  // Post-swap test & rollback
  if err := exec.Command(targetBin, "-version").Run(); err != nil {
      if _, bErr := os.Stat(backupBin); bErr == nil {
          _ = os.Rename(backupBin, targetBin)
      }
      return fmt.Errorf("tun2socks verification failed after swap, rolled back: %w", err)
  }
  _ = os.Remove(backupBin)
  ```
- **Strengths / Existing Mitigations:**
  `updateXray` implements complete two-stage verification and rollback, proving the architecture supports safe swaps when properly replicated.

---

### Finding DEFECT-06-07: Uncontrolled Core Updater Concurrency & Lack of Supervisor Coordination

- **ID & Title:** `DEFECT-06-07`: Uncontrolled Core Replacement During Active Tunnel Execution | **Concurrency & Process Safety**
- **Exact Code Location:** `internal/updater/updater.go:L251-L280`, `internal/api/router.go:L606-L629`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  1. The `Updater` struct does not hold a reference to `core.Supervisor`.
  2. When `/api/system/update-core` is called, `handleUpdateCore` passes the request directly to `r.deps.Updater.UpdateCore(body.Core)` without checking the state of the active tunnel.
  3. If the tunnel is currently `connected`, `xray` and `tun2socks` are executing child processes managed by `core.Supervisor`.
  4. The updater moves the running binary (`targetBin` -> `backupBin`) and places the new binary at `targetBin`. While Linux allows unlinking running executables from directory entries, the running process continues executing with the old binary in memory.
  5. Concurrently, geo asset databases (`/usr/local/share/xray/geosite.dat`, `geoip.dat`) are overwritten in-place while Xray may be actively reading them for domain/IP routing resolution.
  6. If the supervisor restarts the process, or if the safe-mode watchdog triggers a rollback or reconnect, the new binary is launched abruptly without proper lifecycle transitions.
- **PoC / Verification:**
  1. Start tunnel via `POST /api/tunnel/connect`. Verify state is `connected`.
  2. Trigger `POST /api/system/update-core` with `{"core":"all"}`.
  3. Observe that update proceeds concurrently while the child process PID is active, without any coordination or graceful stop/start from `core.Supervisor`.
- **Recommended Fix:**
  1. Provide `core.Supervisor` to `Updater` or coordinate inside `handleUpdateCore`:
  2. If `Supervisor.GetStatus().State == "connected"`:
     - Temporarily pause or stop the tunnel gracefully.
     - Perform the atomic binary swap.
     - Restart the tunnel with the updated core.
     - If the new core fails to start, automatically revert to the backup binary.
- **Strengths / Existing Mitigations:**
  `Updater.UpdateCore` enforces an internal mutex lock (`u.status.IsUpdating`) to prevent multiple concurrent update operations from colliding.

---

### Finding DEFECT-06-08: Permissive Wildcard CORS (`*`) on Administrative Control Plane

- **ID & Title:** `DEFECT-06-08`: Overly Permissive Wildcard CORS on Root Control Plane | **Network Security & CORS**
- **Exact Code Location:** `internal/api/router.go:L54-L66`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  1. In `ServeHTTP`:
     ```go
     w.Header().Set("Access-Control-Allow-Origin", "*")
     w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
     w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
     ```
  2. Wildcard CORS (`*`) is applied indiscriminately to all incoming HTTP requests and preflight `OPTIONS` requests across the entire API router.
  3. Because V2Raynix manages system network configurations and operates with root privileges, exposing `Access-Control-Allow-Origin: *` allows any malicious website running in an administrator's browser to execute preflight probes and interact with endpoints on localhost (`http://127.0.0.1:2080`) or local intranet routers (`http://192.168.1.1:2080`).
  4. Any unauthenticated endpoints or future endpoints with ambient credentials (cookies, basic auth) become vulnerable to Cross-Origin Resource Sharing abuses and CSRF.
- **PoC / Verification:**
  Send an OPTIONS preflight request with an arbitrary foreign origin:
  ```bash
  curl -i -X OPTIONS http://127.0.0.1:2080/api/configs \
    -H "Origin: https://malicious-tracker.com" \
    -H "Access-Control-Request-Method: POST"
  ```
  Response contains:
  ```http
  Access-Control-Allow-Origin: *
  Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
  ```
- **Recommended Fix:**
  1. Remove wildcard CORS. If the Web UI is served directly from the embedded static filesystem on the same origin, CORS headers are entirely unnecessary.
  2. If cross-origin access is required for remote management, validate the `Origin` header against an explicit whitelist configured in store settings:
     ```go
     origin := req.Header.Get("Origin")
     if origin != "" && isAllowedOrigin(origin, allowedOrigins) {
         w.Header().Set("Access-Control-Allow-Origin", origin)
         w.Header().Set("Vary", "Origin")
     }
     ```
- **Strengths / Existing Mitigations:**
  Protected endpoints require a valid `Authorization: Bearer <token>` header, mitigating basic browser requests that do not possess the JWT token.

---

### Finding DEFECT-06-09: Internal Filesystem & System Environment Disclosure in 500 Responses

- **ID & Title:** `DEFECT-06-09`: Verbose Internal System & Path Information Disclosure | **Information Disclosure**
- **Exact Code Location:** `internal/api/router.go:L250`, `L306`, `L339`, `L371`, `L396`, `L405`, `L432`, `L467`, `L477`, `L600`, `L622`
- **Severity:** **Low**
- **Trigger Scenario & Root Cause Analysis:**
  Across numerous API endpoints, internal Go errors are passed directly to the HTTP response payload via `err.Error()`:
  - `L250`: `writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})`
  - `L396`: `writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})`
  - `L622`: `writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})`
  These errors frequently contain absolute server filesystem paths (`/etc/v2raynix/v2raynix.json`, `/tmp/xray-update.zip`), operating system syscall outputs, iptables command errors, and supervisor process states.
- **PoC / Verification:**
  Trigger an internal failure (e.g. simulate permission failure on data directory or invoke update with offline network):
  The response body reveals internal system paths:
  `{"error": "open /etc/v2raynix/v2raynix.json: permission denied"}` or shell command failures.
- **Recommended Fix:**
  Log the detailed internal error to server logs via `log.Printf`, but return sanitized, generic error descriptions to the API client:
  ```go
  log.Printf("[API ERROR] %s failed: %v", req.URL.Path, err)
  writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "an internal system error occurred"})
  ```
- **Strengths / Existing Mitigations:**
  Authentication failures on `/api/auth/login` already return sanitized messages (`invalid credentials`), preventing user enumeration during standard login.

---

### Finding DEFECT-06-10: Broken SPA Subroute Navigation Due to Missing Static Fallback

- **ID & Title:** `DEFECT-06-10`: 404 Not Found on SPA Subroute Direct Access & Page Refresh | **Web Routing & Client Usability**
- **Exact Code Location:** `internal/api/router.go:L106-L115`
- **Severity:** **Low**
- **Trigger Scenario & Root Cause Analysis:**
  1. Static SPA assets are registered on the root path:
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
  2. The frontend web UI is a React Single Page Application utilizing HTML5 pushState client routing (e.g. `/configs`, `/settings`, `/logs`, `/tunnel`).
  3. When an operator navigates to `http://<ip>:2080/settings` directly or refreshes the page with `F5`, `fileServer.ServeHTTP` attempts to find a file named `dist/settings` in the embedded filesystem.
  4. Because no such physical file exists, Go's `http.FileServer` returns `404 page not found`.
- **PoC / Verification:**
  Access `http://localhost:2080/settings` in a browser or via `curl -i http://localhost:2080/settings`.
  Result: HTTP 404 Not Found, breaking client navigation upon browser reload.
- **Recommended Fix:**
  Implement an SPA fallback that checks if the requested file exists in `StaticFS`; if not, and the request does not have a static file extension (e.g. `.js`, `.css`, `.png`), serve `index.html`:
  ```go
  r.mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
      if strings.HasPrefix(req.URL.Path, "/api/") {
          http.NotFound(w, req)
          return
      }
      path := strings.TrimPrefix(req.URL.Path, "/")
      if path != "" {
          if f, err := r.deps.StaticFS.Open(path); err == nil {
              _ = f.Close()
              fileServer.ServeHTTP(w, req)
              return
          }
      }
      // SPA Fallback: serve index.html
      req.URL.Path = "/"
      fileServer.ServeHTTP(w, req)
  })
  ```
- **Strengths / Existing Mitigations:**
  Requests to `/api/*` are guarded from falling into the static file server (`http.NotFound(w, req)`).

---

### Finding DEFECT-06-11: Synchronous Disk I/O Thrashing in Batch Ping & Test Endpoints

- **ID & Title:** `DEFECT-06-11`: Synchronous Disk Write Amplification in Batch Ping Endpoints | **Performance & Disk I/O**
- **Exact Code Location:**
  - `internal/api/router.go:L343-L348`, `L375-L379`
  - `internal/store/store.go:L327-L337`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  1. In `handlePingAll`:
     ```go
     results := pinger.BatchPingContext(req.Context(), configs, 5, 2*time.Second)
     for id, lat := range results {
         _ = r.deps.Store.UpdateLatency(id, lat)
     }
     ```
  2. In `internal/store/store.go`:
     ```go
     func (fs *FileStore) UpdateLatency(id string, latencyMs int) error {
         fs.mu.Lock()
         defer fs.mu.Unlock()
         // ...
         cfg.LatencyMs = latencyMs
         return fs.persist()
     }
     ```
  3. Each call to `UpdateLatency` acquires `fs.mu.Lock()`, serializes the entire JSON database, writes it to a temporary file, executes `Sync()`, and renames the file.
  4. If an operator has imported 300 configs, `handlePingAll` or `handleTestAll` executes **300 sequential disk writes and syncs** back-to-back.
  5. **Consequence:** This induces heavy disk write amplification, wears flash storage (especially on low-cost VPS or SD cards on embedded routers), and blocks the store mutex, starving concurrent requests such as `/api/tunnel/status`.
- **PoC / Verification:**
  Import 100 proxy nodes. Call `POST /api/configs/ping-all`. Monitor filesystem write activity with `iotop` or `inotifywait -m /etc/v2raynix`.
  Observed output: `/etc/v2raynix/v2raynix.json` is recreated and rewritten 100 consecutive times within a few seconds.
- **Recommended Fix:**
  Add a batch latency update method in `store.Store` (`UpdateLatenciesBatch(map[string]int)`) that updates memory records and calls `fs.persist()` **once**:
  ```go
  func (fs *FileStore) UpdateLatenciesBatch(latencies map[string]int) error {
      fs.mu.Lock()
      defer fs.mu.Unlock()
      for id, lat := range latencies {
          if cfg, ok := fs.data.Configs[id]; ok {
              cfg.LatencyMs = lat
          }
      }
      return fs.persist()
  }
  ```
- **Strengths / Existing Mitigations:**
  Pinging itself is non-blocking and concurrency-controlled via `BatchPingContext` with a worker limit of 5.

---

### Finding DEFECT-06-12: Silent Error Swallowing on Live Tunnel Switch & Discarded JWT Context

- **ID & Title:** `DEFECT-06-12`: Silent Tunnel Error Swallowing & Lost JWT Context | **Error Handling & Middleware**
- **Exact Code Location:** `internal/api/router.go:L133`, `L321-L328`
- **Severity:** **Low**
- **Trigger Scenario & Root Cause Analysis:**
  1. In `handleActivateConfig`:
     ```go
     if r.deps.Supervisor != nil {
         if r.deps.Supervisor.GetStatus().State == "connected" {
             _ = r.deps.Supervisor.StartTunnel(active)
         } else {
             r.deps.Supervisor.SetActiveConfig(active)
         }
     }
     writeJSON(w, http.StatusOK, map[string]interface{}{"message": "activated", ...})
     ```
  2. If the tunnel is currently running and the operator selects a newly added proxy node that is invalid or unreachable, `StartTunnel(active)` fails.
  3. The error is silently discarded with `_ = `. The endpoint returns `200 OK` with `"message": "activated"`.
  4. The operator believes the new node is active, but the tunnel has failed or disconnected, leaving the system in a stalled network state.
  5. In `requireAuth`, the extracted JWT claims are discarded (`_ = claims`), failing to inject user identity into `req.Context()`.
- **PoC / Verification:**
  Connect tunnel. Call `POST /api/configs/<broken-id>/activate`.
  Observe response is `HTTP 200 OK` despite `StartTunnel` encountering an error.
- **Recommended Fix:**
  1. Check the error returned by `StartTunnel`:
     ```go
     if r.deps.Supervisor.GetStatus().State == "connected" {
         if err := r.deps.Supervisor.StartTunnel(active); err != nil {
             writeJSON(w, http.StatusInternalServerError, map[string]string{
                 "error": fmt.Sprintf("failed to switch tunnel: %v", err),
             })
             return
         }
     }
     ```
  2. Inject validated JWT claims into `req.Context()`.
- **Strengths / Existing Mitigations:**
  Inactive tunnel selection cleanly persists the active ID to store without side effects.

---

### Finding DEFECT-06-13: World-Readable Configuration Directory & Unhardened Systemd Service

- **ID & Title:** `DEFECT-06-13`: Insecure File Permissions & Missing Systemd Service Hardening | **OS & Service Hardening**
- **Exact Code Location:**
  - `scripts/install.sh:L48`, `L161-L179`, `L185`
  - `scripts/v2raynix.service:L6-L15`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  1. In `scripts/install.sh:L48`:
     `mkdir -p /etc/v2raynix`
     No explicit permissions are specified. Under standard Linux default umask (`022`), `/etc/v2raynix` is created with mode `0755` (world-readable).
  2. The store file `/etc/v2raynix/v2raynix.json` contains:
     - Admin bcrypt password hash.
     - VLESS/VMess/Trojan UUIDs, private keys, subscription tokens, and proxy server IPs.
  3. Any unprivileged local user or process on the host can read `/etc/v2raynix/v2raynix.json`.
  4. In `v2raynix.service`, the daemon runs as `User=root` without modern systemd security restrictions (`ProtectSystem`, `ProtectHome`, `PrivateTmp`, `NoNewPrivileges`).
  5. `scripts/install.sh:L185` queries external IP over unencrypted HTTP: `curl -s4 icanhazip.com`.
- **PoC / Verification:**
  Run `scripts/install.sh` on a Linux host. Log in as an unprivileged user (e.g. `nobody` or a guest account). Execute:
  `cat /etc/v2raynix/v2raynix.json`
  The entire configuration database, including password hashes and proxy keys, is readable.
- **Recommended Fix:**
  1. Enforce strict permissions (`0700` for directory, `0600` for configuration files) in `scripts/install.sh`:
     ```bash
     mkdir -p /etc/v2raynix
     chmod 700 /etc/v2raynix
     ```
  2. Harden `scripts/v2raynix.service`:
     ```ini
     [Service]
     ProtectSystem=full
     ProtectHome=true
     PrivateTmp=true
     NoNewPrivileges=true
     ```
  3. In `install.sh`, use HTTPS for IP queries: `curl -fsSL https://icanhazip.com`.
- **Strengths / Existing Mitigations:**
  `AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW` is correctly defined, laying the groundwork for non-root execution.

---

### Finding DEFECT-06-14: Ephemeral JWT Secret Generation Causes Session Eviction on Daemon Restart

- **ID & Title:** `DEFECT-06-14`: Ephemeral In-Memory JWT Secret Causes Total User Session Eviction | **Authentication & Lifecycle**
- **Exact Code Location:** `cmd/v2raynix/main.go:L99-L102`
- **Severity:** **Low**
- **Trigger Scenario & Root Cause Analysis:**
  1. In `cmd/v2raynix/main.go`:
     ```go
     // Random JWT secret
     jwtSecret := make([]byte, 32)
     _, _ = rand.Read(jwtSecret)
     ```
  2. A new cryptographically random JWT signing key is generated in-memory on every process startup.
  3. Whenever `v2raynix` restarts (e.g. system reboot, service restart by `v2raynix setup`, or core update), the signing key changes.
  4. **Consequence:** All existing JWT tokens issued to logged-in web administrators become invalid. Active administrator sessions are abruptly terminated with `HTTP 401: invalid or expired token`, requiring re-authentication.
- **PoC / Verification:**
  Log in to Web UI to obtain a JWT token. Execute `systemctl restart v2raynix`. Attempt to make any request with the previously valid token. Request returns `401 Unauthorized`.
- **Recommended Fix:**
  Persist a securely generated 32-byte JWT secret in the store (`settings.JWTSecret`) or in a dedicated key file (`/etc/v2raynix/jwt.key` with `0600` permissions). Load the existing key on boot, generating a new key only if none exists.
- **Strengths / Existing Mitigations:**
  Using `crypto/rand` ensures that secrets are cryptographically strong and never statically hardcoded in source code.

---

## 5. Architectural Strengths & Existing Mitigations

Our audit noted several positive engineering practices and security controls:
1. **Request Body Size Limits (`http.MaxBytesReader`):**
   Unlike earlier iterations, `router.go` now wraps JSON decoding in `decodeJSON` with strict body limits (`defaultMaxBodyBytes = 2MB`, `configMaxBodyBytes = 10MB`), preventing naive HTTP body memory exhaustion attacks.
2. **Batch Configuration Persistence (`SaveConfigsBatch`):**
   `handleCreateConfig` uses `Store.SaveConfigsBatch(created)` rather than calling per-record disk writes, mitigating write amplification on config imports.
3. **Database Integrity on Login Store Failure:**
   `handleLogin` properly verifies `errors.Is(err, store.ErrNotFound)` before attempting default user initialization, preventing transient I/O errors from wiping credentials.
4. **Standard Go 1.22+ ServeMux Routing:**
   Clean routing structure utilizing standard library patterns without unnecessary third-party router bloat.
5. **Interactive CLI Safeguards:**
   `internal/cli/setup.go` validates port ranges (`1-65535`) and strictly limits service actions to `start`, `stop`, `restart`, `status`.

---

## 6. Remediation & Hardening Roadmap

### Priority 1: Critical & High Severity (Immediate Hotfixes)
- [ ] **Fix EXDEV Cross-Device Rename (DEFECT-06-04):** Move temporary update staging directory to the target binary's parent directory (`/usr/local/bin/`).
- [ ] **Enforce SHA-256 Checksums (DEFECT-06-05):** Validate downloaded release archives against upstream SHA-256 digests before extracting and executing.
- [ ] **Resolve WebPort Discrepancy (DEFECT-06-01):** Remove `-port 2080` from `ExecStart` in `v2raynix.service` and `install.sh`. Update `main.go` to prioritize store `settings.WebPort` when `-port` flag is not explicitly passed.
- [ ] **Fix Rate Limiter IP Spoofing (DEFECT-06-02):** Restrict `X-Forwarded-For` trust to verified local loopback proxies.
- [ ] **Add Tun2socks Rollback (DEFECT-06-06):** Add pre-swap and post-swap verification with automatic rollback to `updateTun2socks`.

### Priority 2: Medium Severity (Next Release Cycle)
- [ ] **Implement Rate Limiter LRU / Eviction (DEFECT-06-03):** Cap tracking map size and add a background cleanup routine.
- [ ] **Batch Latency Persistence (DEFECT-06-11):** Implement `UpdateLatenciesBatch` in `store.Store` to eliminate disk thrashing during ping tests.
- [ ] **Tighten CORS Headers (DEFECT-06-08):** Restrict or remove wildcard `*` CORS from the administrative API.
- [ ] **Harden Installer Permissions & Service (DEFECT-06-13):** Apply `chmod 700 /etc/v2raynix` and enable systemd security isolation flags.
- [ ] **Coordinate Updater with Supervisor (DEFECT-06-07):** Ensure active tunnels are paused or safely transitioned during core binary upgrades.

### Priority 3: Low Severity (Polish & Operational Hygiene)
- [ ] **Sanitize HTTP 500 Responses (DEFECT-06-09):** Mask internal filesystem paths from client error responses.
- [ ] **Implement SPA Fallback (DEFECT-06-10):** Serve `index.html` on direct subroute browser reloads.
- [ ] **Persist JWT Secret (DEFECT-06-14):** Save JWT secret to `/etc/v2raynix/jwt.key` so sessions survive daemon restarts.
- [ ] **Propagate Errors & JWT Claims (DEFECT-06-12):** Return error on failed live tunnel switch and attach claims to request context.
