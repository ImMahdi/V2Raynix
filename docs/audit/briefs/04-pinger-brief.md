# Domain Briefing 04: Pinger & Latency Diagnostics Subsystem

> **Subagent Role:** Principal Network Diagnostics & High-Concurrency Systems Auditor  
> **Audited Files:** `internal/pinger/pinger.go`, `internal/pinger/pinger_test.go`  
> **Output Report File:** `docs/audit/reports/04-pinger-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Deep Knowledge Transfer & Architecture

The `pinger` package provides network diagnostic and latency measurement capabilities to assess proxy node quality before or during active connection.

### Key Architectural Concepts:
1. **TCP Handshake Latency (`TCPPing`):**
   - Measures pure Layer-4 TCP SYN/ACK round-trip time directly to `host:port` using `net.DialTimeout`.
   - Fast, low overhead, but does not test proxy protocol handshake or Internet transit through the proxy.
2. **End-to-End HTTP Latency (`RealHTTPDelay`):**
   - Tests genuine outbound browsing latency through a local SOCKS5 proxy (`127.0.0.1:10808`) by fetching a lightweight captive portal probe (`http://cp.cloudflare.com`).
   - Uses `golang.org/x/net/proxy.SOCKS5` dialer and custom `http.Transport`.
3. **Batch Concurrent Diagnostics (`BatchPing`):**
   - Concurrently tests a collection of `store.ConfigItem` nodes with configurable concurrency (default 5) and timeout (default 2s).
   - Collects results into a thread-safe map `results[c.ID] = latencyMs`.

---

## 2. Concerns, Pitfalls & Critical Warnings

When auditing this domain, pay hyper-vigilant attention to these failure modes:
1. **Unbounded Goroutine Spawning Hazard:**
   In `BatchPing(configs, concurrency, timeout)`:
   ```go
   for _, cfg := range configs {
       wg.Add(1)
       go func(c *store.ConfigItem) {
           defer wg.Done()
           sem <- struct{}{}
           defer func() { <-sem }()
           ...
       }(cfg)
   }
   ```
   Notice that the goroutine is spawned **BEFORE** acquiring the semaphore! If a user imports a subscription with 5,000 or 10,000 configs, `BatchPing` immediately spawns 10,000 concurrent goroutines, exhausting memory and OS thread resources. A true worker pool or pre-spawn semaphore acquisition is required.
2. **Missing `context.Context` Support (Uncancelable Operations):**
   Neither `TCPPing`, `RealHTTPDelay`, nor `BatchPing` accept a `context.Context`. If the user leaves the UI, cancels the test, or the HTTP request aborts, background network dials continue running until their full timeout.
3. **DNS Resolution Latency Distortion:**
   `net.DialTimeout("tcp", addr, timeout)` resolves domain names synchronously. If the system DNS is slow, DNS resolution time is counted as proxy latency or can exceed the dial timeout before any TCP packet is transmitted.
4. **HTTP Transport Resource Leaks:**
   In `RealHTTPDelay()`, a new `http.Transport` and `http.Client` are instantiated on every single call. In Go, `http.Transport` creates internal connection pools and dialers. While `DisableKeepAlives: true` is set, creating transports per request incurs unnecessary garbage collection overhead and potential socket lingering.
5. **Target Probe Reliability & Captive Portal Redirection:**
   `http://cp.cloudflare.com` is plaintext HTTP. In restrictive network environments or national firewalls (like Iran), unencrypted HTTP requests to Cloudflare may trigger ISP HTTP injection, SNI tampering, or captive portal false positives (returning HTTP 200 with ISP landing page).

---

## 3. Lead Architect Directives & Step-by-Step Instructions

### Step 1: Mandatory Architectural Graph Exploration (`graphify`)
Before reading line-by-line, run:
```powershell
python -m graphify.cli explain pinger
# or
python -m graphify.cli query "how does pinger interact with store and api"
```
Check callers of `BatchPing` and `RealHTTPDelay` across the codebase.

### Step 2: Initialize Report Scaffold
Immediately create `docs/audit/reports/04-pinger-report.md` with standard sections.

### Step 3: Step-by-Step Systematic Code Inspection & Incremental Updates
Audit each area sequentially and update the report immediately upon finding defects:
1. **Concurrency Architecture (`pinger.go:L65-L105`):** Audit goroutine lifecycle, channel buffer size, memory scaling, and mutex contention.
2. **Context & Cancellation Support (`pinger.go:L1-L105`):** Audit how cancellation is handled across all dialers.
3. **HTTP Transport & Body Management (`pinger.go:L28-L64`):** Audit transport creation, response body reading/draining, and socket reuse.
4. **Error Differentiation:** Check how network errors (DNS failure, Connection Refused, Connection Timed Out) are distinguished and reported.

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
