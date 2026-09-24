# Domain Briefing 04: Pinger & Latency Diagnostics Subsystem

> **Subagent Role:** Principal High-Performance Networking & Diagnostics Auditor  
> **Audited Files:** `internal/pinger/pinger.go`, `internal/pinger/*_test.go`  
> **Output Report File:** `docs/audit/reports/04-pinger-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Subsystem Architecture & Responsibilities
The `internal/pinger` package executes active latency probes against proxy endpoints and internet targets:
- TCP Handshake latency measurement (`TCPPing`).
- Real end-to-end HTTP/HTTPS delay testing through proxies (`RealHTTPDelay`).
- Batch concurrent ping execution across multi-node server lists (`BatchPing`).

---

## 2. Critical Audit Directives & Failure Modes to Investigate

Auditor must systematically examine:
1. **Unbounded Goroutine Spawning & Semaphore Starvation:**
   - In `BatchPing()`: are goroutines launched before acquiring the concurrency semaphore, causing mass goroutine spikes under large server lists (e.g. 500+ nodes)?
2. **Context Cancellation & Timeout Propagation:**
   - Do all network dials respect `context.Context` cancellation properly, or do abandoned pings linger in background socket wait states?
3. **HTTP Transport Lifecycle & Connection Pooling:**
   - In `RealHTTPDelay()`: are response bodies explicitly drained (`io.Copy(io.Discard, ...)`) and closed (`resp.Body.Close()`)?
   - Is `http.Transport` properly closed (`CloseIdleConnections`) or reused to prevent TCP socket exhaustion?
4. **DNS Resolution Latency Distortion:**
   - In `TCPPing()`, does the measured latency inadvertently include the host DNS resolution time rather than pure TCP handshake delay?
5. **Captive Portal & False Positive Probes:**
   - What endpoint is targeted for delay measurement? Does it handle redirects, captive portals, and TLS handshake timeouts reliably?

---

## 3. Subagent Execution Instructions
1. Run `python -m graphify.cli explain pinger` or query graph to understand dependencies.
2. Progressively write findings to `docs/audit/reports/04-pinger-report.md`.
3. Every finding MUST follow the 7-field defect schema (ID, Code Location, Severity, Trigger/Root Cause, PoC, Recommended Fix, Strengths).
4. STRICTLY READ-ONLY: Do not edit any code files.
