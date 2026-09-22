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
