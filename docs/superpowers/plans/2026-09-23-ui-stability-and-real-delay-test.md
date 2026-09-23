# UI Stability & Real Xray Proxy Delay Testing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the non-deterministic UI card/row swapping on Configs and Routing pages ([TASK-NEXT-01](file:///docs/backlog.md)) and replace raw TCP ping with real end-to-end proxy delay testing using Xray core and dedicated single/batch test actions ([TASK-NEXT-02](file:///docs/backlog.md)).

**Architecture:**
1. **Backend Storage Layer:** Enhance [`internal/store/store.go`](file:///internal/store/store.go) so that `GetConfigs()` and `GetRoutingRules()` return deterministically sorted slices using `sort.Slice` instead of returning unsorted Go map iterations.
2. **Diagnostics Engine (`internal/pinger`):** Add `TestConfigRealDelay` using an ephemeral test Xray outbound / SOCKS dialer and `RealHTTPDelay` (`http://cp.cloudflare.com` or `generate_204`) to verify full protocol handshake, UUID, and path authentication. Add bounded concurrency worker pool for `BatchRealTestContext`.
3. **REST API Layer (`internal/api`):** Expose `POST /api/configs/{id}/test` (single config test) and `POST /api/configs/test-all` (batch real test).
4. **Web UI Layer (`web/src`):**
   - Enforce client-side stable sort in [`web/src/pages/ConfigsPage.jsx`](file:///web/src/pages/ConfigsPage.jsx) and [`web/src/pages/RoutingPage.jsx`](file:///web/src/pages/RoutingPage.jsx).
   - Add user sort controls (Date, Ping, Name) on Configs page.
   - Rename global "Ping All" button to "Test All" (icon: `Zap`).
   - Add an individual "Test" button on [`web/src/components/ConfigCard.jsx`](file:///web/src/components/ConfigCard.jsx) with an isolated spinner.

**Tech Stack:** Go 1.22+, React 18, Vite, Xray-Core 1.8+, SOCKS5 proxy protocol, Vitest / Go testing.

---

## File Structure & Responsibilities

| File Path | Responsibility | Action |
| :--- | :--- | :--- |
| [`internal/store/store.go`](file:///internal/store/store.go) | Deterministic sorting for `GetConfigs` and `GetRoutingRules` | Modify |
| [`internal/store/store_test.go`](file:///internal/store/store_test.go) | Unit test validating consecutive ordering stability | Modify |
| [`internal/pinger/pinger.go`](file:///internal/pinger/pinger.go) | Implement real Xray proxy delay test and batch runner | Modify |
| [`internal/pinger/pinger_test.go`](file:///internal/pinger/pinger_test.go) | Unit tests for real delay test and error handling | Modify |
| [`internal/api/router.go`](file:///internal/api/router.go) | Register `/api/configs/{id}/test` and `/api/configs/test-all` | Modify |
| [`internal/api/api_test.go`](file:///internal/api/api_test.go) | Integration tests for single and batch test endpoints | Modify |
| [`web/src/services/api.js`](file:///web/src/services/api.js) | Client API methods for `testConfig` and `testAll` | Modify |
| [`web/src/components/ConfigCard.jsx`](file:///web/src/components/ConfigCard.jsx) | Add dedicated Test button and per-card loading state | Modify |
| [`web/src/pages/ConfigsPage.jsx`](file:///web/src/pages/ConfigsPage.jsx) | Add sort controls, "Test All" button, and stable sort | Modify |
| [`web/src/pages/RoutingPage.jsx`](file:///web/src/pages/RoutingPage.jsx) | Enforce stable client-side sorting for rules table | Modify |
| [`web/src/App.jsx`](file:///web/src/App.jsx) | Handlers for single-card test and batch test | Modify |

---

## Tasks

### Task 1: Deterministic Storage Sorting (Backend TDD)

- [ ] **Step 1: Write failing test in `internal/store/store_test.go`**
  - Add `TestFileStore_DeterministicOrdering(t *testing.T)`.
  - Insert 5 configs with distinct timestamps/IDs and 5 routing rules with varying priorities.
  - Call `GetConfigs()` 20 times in a loop and assert that the returned slice order is 100% identical on every call.
  - Call `GetRoutingRules()` 20 times in a loop and assert that the returned slice order is 100% identical on every call.
- [ ] **Step 2: Run test to confirm failure**
  - Command: `go test -v -run TestFileStore_DeterministicOrdering ./internal/store`
  - Expected: Failure due to randomized Go map iteration order.
- [ ] **Step 3: Implement deterministic sorting in `internal/store/store.go`**
  - In `GetConfigs()`:
    ```go
    sort.Slice(items, func(i, j int) bool {
        if items[i].CreatedAt != items[j].CreatedAt {
            return items[i].CreatedAt > items[j].CreatedAt // Newest first
        }
        return items[i].ID < items[j].ID
    })
    ```
  - In `GetRoutingRules()`:
    ```go
    sort.Slice(items, func(i, j int) bool {
        if items[i].Priority != items[j].Priority {
            return items[i].Priority > items[j].Priority // Highest priority first
        }
        return items[i].ID < items[j].ID
    })
    ```
- [ ] **Step 4: Run test to verify success**
  - Command: `go test -v -run TestFileStore_DeterministicOrdering ./internal/store`
  - Expected: PASS.
- [ ] **Step 5: Commit changes**
  - Command: `git commit -am "fix(store): ensure deterministic ordering for configs and routing rules"`

---

### Task 2: Real Xray Proxy Delay Testing Engine (Backend TDD)

- [ ] **Step 1: Write failing tests in `internal/pinger/pinger_test.go`**
  - Test real HTTP delay using a mock SOCKS5 test server:
    - Test successful HTTP 204/200 response -> returns latency in ms (> 0).
    - Test unreachable or invalid auth proxy -> returns `-1` without hanging.
    - Test context cancellation stops test immediately.
- [ ] **Step 2: Run test to confirm failure**
  - Command: `go test -v -run TestRealHTTPDelay ./internal/pinger`
- [ ] **Step 3: Implement real proxy delay & batch tester in `internal/pinger/pinger.go`**
  - Implement `TestConfigRealDelay(ctx context.Context, cfg *store.ConfigItem, timeout time.Duration) (int, error)`.
    - If active proxy / tunneled Xray is running, route test through local SOCKS5 (`127.0.0.1:10808`).
    - If evaluating unselected configs, run an ephemeral single-outbound Xray test instance on a dynamic port or execute `RealHTTPDelay`.
    - Returns measured RTT in milliseconds, or `-1` if connection/authentication fails.
  - Implement `BatchRealTestContext(ctx context.Context, configs []*store.ConfigItem, concurrency int, timeout time.Duration) map[string]int` with bounded worker pool (default 5 workers).
- [ ] **Step 4: Run tests to verify success**
  - Command: `go test -v ./internal/pinger`
  - Expected: PASS.
- [ ] **Step 5: Commit changes**
  - Command: `git commit -am "feat(pinger): add real proxy delay testing with Xray and worker pool"`

---

### Task 3: REST API Test Endpoints (Backend TDD)

- [ ] **Step 1: Write failing integration tests in `internal/api/api_test.go`**
  - Test `POST /api/configs/{id}/test` returns HTTP 200 with `{"id": id, "latencyMs": ...}`.
  - Test `POST /api/configs/{id}/test` with non-existent ID returns HTTP 404.
  - Test `POST /api/configs/test-all` returns HTTP 200 with map of latencies.
- [ ] **Step 2: Run test to confirm failure**
  - Command: `go test -v -run TestConfigTestEndpoints ./internal/api`
- [ ] **Step 3: Implement route handlers in `internal/api/router.go`**
  - Register:
    - `POST /api/configs/{id}/test` -> `r.handleTestConfig`
    - `POST /api/configs/test-all` -> `r.handleTestAll`
  - Handler updates `store.UpdateLatency` upon receiving result and writes JSON response.
- [ ] **Step 4: Run test to verify success**
  - Command: `go test -v ./internal/api`
  - Expected: PASS.
- [ ] **Step 5: Commit changes**
  - Command: `git commit -am "feat(api): add endpoints for individual and batch real proxy delay testing"`

---

### Task 4: Frontend UI Stability & Client-Side Sorting Controls

- [ ] **Step 1: Update `web/src/pages/RoutingPage.jsx`**
  - Add client-side stable sort before mapping:
    ```javascript
    const sortedRules = [...rules].sort((a, b) => (b.priority || 0) - (a.priority || 0) || a.id.localeCompare(b.id));
    ```
  - Map over `sortedRules` instead of raw `rules`.
- [ ] **Step 2: Update `web/src/pages/ConfigsPage.jsx` with sorting controls**
  - Add state `sortBy` ("date", "ping", "name").
  - Add sort control UI (dropdown / toggle buttons) next to the search bar.
  - Apply deterministic sorting on `filteredConfigs`:
    ```javascript
    const sortedConfigs = [...filteredConfigs].sort((a, b) => {
      if (sortBy === 'ping') {
        const pA = a.latencyMs > 0 ? a.latencyMs : 99999;
        const pB = b.latencyMs > 0 ? b.latencyMs : 99999;
        return pA - pB;
      }
      if (sortBy === 'name') return (a.name || '').localeCompare(b.name || '');
      return (b.createdAt || '').localeCompare(a.createdAt || '') || a.id.localeCompare(b.id);
    });
    ```
- [ ] **Step 3: Verify visually in web bundle**
  - Command: `npm --prefix web run build`
  - Expected: Clean build with zero TypeScript / JSX lint errors.
- [ ] **Step 4: Commit changes**
  - Command: `git commit -am "feat(web): add stable client-side sorting and user sort controls"`

---

### Task 5: Frontend Single-Config & Batch Real Test Integration

- [ ] **Step 1: Update `web/src/services/api.js`**
  - Add `testConfig: (id) => request(`/configs/${id}/test`, { method: 'POST' })`.
  - Add `testAll: () => request('/configs/test-all', { method: 'POST' })`.
- [ ] **Step 2: Update `web/src/components/ConfigCard.jsx`**
  - Add dedicated Test button with `Zap` icon:
    ```jsx
    <button 
      className="btn btn-icon" 
      title="Test Real Proxy Delay" 
      onClick={handleTest}
      disabled={testing}
    >
      <Zap size={16} className={testing ? 'animate-spin' : ''} style={{ color: 'var(--accent-amber)' }} />
    </button>
    ```
  - Manage isolated `testing` state per card so only the clicked card shows spinning animation.
- [ ] **Step 3: Update `web/src/pages/ConfigsPage.jsx` & `web/src/App.jsx`**
  - Change "Ping All" button to "Test All" with `Zap` icon.
  - Wire `onTestConfig={(id) => handleTestConfig(id)}` to update only that card's latency.
  - Wire `onTestAll={() => handleTestAll()}` to update all configs.
- [ ] **Step 4: Test production Vite build**
  - Command: `npm --prefix web run build`
  - Expected: Vite build completes successfully in `web/dist`.
- [ ] **Step 5: Commit changes**
  - Command: `git commit -am "feat(web): integrate individual card test and Test All actions"`

---

### Task 6: Cross-Compilation & Live Server Verification

- [ ] **Step 1: Cross-compile Linux binary**
  - Command: `$env:GOOS='linux'; $env:GOARCH='amd64'; go build -o v2raynix-linux ./cmd/v2raynix`
- [ ] **Step 2: Deploy updated binary to remote test server (`192.168.254.80`)**
  - Upload via SFTP to `/usr/local/bin/v2raynix` and restart `v2raynix.service`.
- [ ] **Step 3: Live verification on server**
  - Verify `POST /api/configs/{id}/test` returns real latency on working proxy, and `-1` on broken/fake proxy.
  - Verify Web UI at `http://192.168.254.80:2080`:
    - Confirm zero card jumping during 2s polling loops.
    - Confirm zero routing table row swapping.
    - Click individual "Test" button on a card and verify isolated spinner and badge update.
    - Click "Test All" and verify batch progress.
- [ ] **Step 4: Update Knowledge Graph (`graphify update .`)**
- [ ] **Step 5: Final review and documentation walkthrough**
