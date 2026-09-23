# V2Raynix Task Backlog & Improvements

This document tracks planned architectural, backend, and UI enhancements scheduled for subsequent development phases.

---

### [TASK-NEXT-01] Deterministic Ordering & Stable UI Rendering

**Problem Statement:**
During the 2-second background polling loop in the Web UI, configuration cards and routing rules dynamically jump and shuffle positions on the screen ([video reference](file:///feedback/Recording%202026-09-21%20232358.mp4)). 

**Root Causes:**
1. **Backend Go Map Randomization:** `fs.data.Configs` and `fs.data.RoutingRules` in [`internal/store/store.go`](file:///internal/store/store.go) are Go hash maps. In the Go language runtime, iteration order over map keys is randomized on every execution. Because `GetConfigs()` and `GetRoutingRules()` iterate the map without sorting before returning slices, every `GET /api/configs` call produces a different, shuffled array.
2. **Frontend Periodic Polling:** [`web/src/App.jsx`](file:///web/src/App.jsx) queries `api.getConfigs()` every 2000ms. React sets the randomized array into state, causing [`web/src/pages/ConfigsPage.jsx`](file:///web/src/pages/ConfigsPage.jsx) to re-render the card grid in the newly shuffled sequence.

**Planned Actions:**
- **Backend Fix (`internal/store/store.go`):**
  - Sort `GetConfigs()` deterministically using `sort.Slice` (e.g., by `CreatedAt` ascending or descending).
  - Sort `GetRoutingRules()` deterministically by `Priority` descending, then `ID`.
  - Add unit tests verifying stable ordering across consecutive invocations.
- **Frontend Fix (`web/src/pages/ConfigsPage.jsx`):**
  - Enforce a stable client-side sort order to protect against unordered API payloads.
  - Add sort control options for the user (sort by ping/latency, sort by name, sort by date added).

---

### [TASK-NEXT-02] Real Xray Proxy Delay Testing & Individual Config Test Action

**Problem Statement:**
Currently, latency testing relies strictly on a raw TCP Handshake to `Server:Port`. If an invalid/garbage configuration uses a popular or live CDN/IP (e.g. `1.1.1.1`, `cloudflare.com`, `google.com`), TCP ping succeeds with low latency (e.g., `10ms`), giving the user a false impression of a working node. Furthermore, users cannot test an individual configuration without running a full batch ping across all configs.

**Planned Architecture & Solution:**
1. **Real Proxy Delay via Xray Core (`internal/pinger` & `internal/core`):**
   - Implement real end-to-end testing through Xray core rather than raw TCP handshake.
   - Run a test request through the proxy outbound (e.g., HTTP GET `http://cp.cloudflare.com` or `generate_204`) using `pinger.RealHTTPDelay`.
   - Validate full handshake, encryption, UUID, and path authentication. If handshake fails or HTTP status is erroneous, register as failed (`-1`).
2. **Granular REST Endpoints (`internal/api`):**
   - `POST /api/configs/{id}/test`: Tests a single specific configuration on-demand and returns its real HTTP delay.
   - `POST /api/configs/test-all`: Runs concurrent real delay testing across configs with bounded concurrency.
3. **UI / UX Overhaul (`web/src`):**
   - Rename global action button from **"Ping All"** to **"Test All"** (with `Zap` or `Gauge` icon).
   - Add a dedicated **"Test" action button on each `ConfigCard`** (with dedicated icon, e.g., `Zap` / `Play`), featuring an isolated per-card loading/spinning state so users can test single configs with one click.
   - Keep latency badges visually distinct (e.g., Real HTTP Delay vs. Untested / Failed).
