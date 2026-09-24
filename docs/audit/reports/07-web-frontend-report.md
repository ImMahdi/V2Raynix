# Security & Architecture Audit Report: Domain 07 (Web Frontend & UI State Subsystem)

> **Audited Subsystem:** Web Frontend, UI State Management, React Architecture & Client Security  
> **Target Files:**  
> - `web/src/App.jsx`  
> - `web/src/services/api.js`  
> - `web/src/main.jsx`  
> - `web/src/components/ErrorBoundary.jsx`  
> - `web/src/components/Navbar.jsx`  
> - `web/src/components/ConfigCard.jsx`  
> - `web/src/components/SafeModeModal.jsx`  
> - `web/src/components/UpdateCoreModal.jsx`  
> - `web/src/pages/DashboardPage.jsx`  
> - `web/src/pages/ConfigsPage.jsx`  
> - `web/src/pages/RoutingPage.jsx`  
> - `web/src/pages/LogsPage.jsx`  
> - `web/src/pages/SettingsPage.jsx`  
> - `web/src/pages/LoginPage.jsx`  
> **Auditor:** Principal Frontend Architecture & Web Security Auditor  
> **Audit Date:** 2026-09-24  
> **Audit Status:** Complete & Verified  

---

## 1. Executive Summary

The `web` directory provides the single-page management console and real-time operator interface for V2Raynix. Built upon React 18, Vite, Lucide icons, and modern glassmorphism styling, it allows administrators to control root-level system networking: engaging the full-system tunnel via `tun2socks` and `xray`, switching proxy server configurations, provisioning split-routing rules (Direct vs Proxy vs Block), inspecting live system logs, running real-delay network latency sweeps, and orchestrating core binary updates (`xray-core` and `tun2socks`).

Because V2Raynix operates with host root privileges (`iptables`, `ip rule`, `ip route`, policy routing tables), stability and security within the web dashboard are critical. Frontend state desynchronization, uncontrolled polling during network transitions, race conditions during master toggle transitions, unhandled runtime exceptions, or credential leakage can leave operators blind to underlying network state changes or cause invalid state transitions in the backend supervisor.

Recent improvements (commit `feddcb2`) mitigated several baseline issues by introducing basic exponential backoff with jitter in `App.jsx`, incorporating an `ErrorBoundary` component, capping log stream rendering to 200 elements, and adding an anti-spam `toggling` lock on the Master button. 

However, this in-depth, line-by-line audit identified significant remaining architectural and security vulnerabilities:
1. **Unbounded Fetch Hanging & Background Polling Overwrite During Active State Transitions (`WEB-01`):** `api.js` completely lacks request timeouts. When network interfaces restart or routes flap during tunnel activation, `fetch` calls hang indefinitely. Simultaneously, background polling continues firing every 2 seconds without pause during manual mutations, causing stale status to overwrite optimistic user actions.
2. **Persistent Administrative JWT Storage in `localStorage` (`WEB-02`):** Administrative session tokens are persistently stored in browser `localStorage`, exposing full host network control to any origin-level script or browser extension across browser restarts, without idle invalidation.
3. **Fragile Error Boundary Scope & Array Filter Crash Cascades (`WEB-03`):** Error boundaries do not encapsulate `Navbar`, `SafeModeModal`, or `UpdateCoreModal`. Furthermore, search filtering in `ConfigsPage.jsx` permits `null` objects when search queries are empty (`"".includes("")`), causing fatal `TypeError` during sorting that collapses the view into an unrecoverable reload loop.
4. **Master Toggle State De-synchronization & Unvalidated Zero-Config Tunneling (`WEB-04`):** The Master switch remains interactive when no configuration is active, causing predictable HTTP 400 rejections and intrusive blocking alerts. A race condition between delayed polling responses and manual toggle dispatch can revert the visual state back to `DISCONNECTED` while the tunnel is running.
5. **Asynchronous Lifecycle Memory Leaks on Component Unmount (`WEB-05`):** Async operations in `ConfigCard.jsx`, `UpdateCoreModal.jsx`, `LoginPage.jsx`, and routing rule actions do not check component mount status or catch unhandled promise rejections, triggering state updates on unmounted components.
6. **Clipboard API Crash on Plain HTTP Access & Synchronous `window.alert` Disrupting Timers (`WEB-06`):** `ConfigCard.jsx` invokes `navigator.clipboard.writeText` without checking `window.isSecureContext`, throwing unhandled exceptions on standard LAN HTTP setups. Widespread use of synchronous `window.alert` blocks JavaScript execution and stalls SafeMode countdowns.
7. **Core Engine Update Lifecycle Flaws (`WEB-07`):** `UpdateCoreModal` silently renders `null` when opened prior to data population, allows premature dismissal during live downloads, and permits updating binary engines while the proxy tunnel is actively running without warning.

---

## 2. Graphify Knowledge Graph & Dependency Analysis

The architectural topology and component call graph extracted via `graphify` (`graphify-out/graph.json`):

```
                                  +---------------------------------------+
                                  |           Browser / Client            |
                                  |              (main.jsx)               |
                                  +-------------------+-------------------+
                                                      |
                                                      | mounts (React 18 StrictMode)
                                                      v
                                  +---------------------------------------+
                                  |       Root ErrorBoundary.jsx          |
                                  +-------------------+-------------------+
                                                      |
                                                      v
                                  +---------------------------------------+
                                  |               App.jsx                 |
                                  |     (State Shell & Central Hub)       |
                                  +----+-------------+---------------+----+
                                       |             |               |
             +-------------------------+             |               +-------------------------+
             |                                       |                                         |
             v                                       v                                         v
+-------------------------+             +-------------------------+               +-------------------------+
|     UI Navigation       |             |   Tab ErrorBoundary     |               |     API Client Layer    |
| - Navbar.jsx            |             +------------+------------+               |     (services/api.js)   |
| - SafeModeModal.jsx     |                          |                            +------------+------------+
| - UpdateCoreModal.jsx   |                          v                                         |
+-------------------------+             +-------------------------+                            | HTTP fetch
                                        |       Active Pages      |                            | Bearer JWT
                                        | - DashboardPage.jsx     |                            v
                                        | - ConfigsPage.jsx       |               +-------------------------+
                                        | - RoutingPage.jsx       |               |    Go REST API Router   |
                                        | - LogsPage.jsx          |               |   (internal/api/router) |
                                        | - SettingsPage.jsx      |               +-------------------------+
                                        | - LoginPage.jsx         |                            |
                                        +-------------------------+                            v
                                                     |                            +-------------------------+
                                                     v                            |   Supervisor & Store    |
                                        +-------------------------+               |   (internal/core)       |
                                        |   Shared UI Components  |               +-------------------------+
                                        | - ConfigCard.jsx        |
                                        +-------------------------+
```

### Architectural Insights & Failure Surface:
1. **Centralized Hub Architecture (`App.jsx`):**
   `App.jsx` acts as the primary coordinator for data flow and authentication. Centralized state (`status`, `configs`, `routingRules`, `updateData`) is refreshed through an active polling loop and passed down via props. Any failure in `App.jsx`'s lifecycle cascades across all child views.
2. **Coupling with Network Reconfiguration:**
   The frontend communicates with 14 backend REST routes across `/api/auth/*`, `/api/configs/*`, `/api/tunnel/*`, `/api/routing/*`, and `/api/system/*`. When the backend alters network routing (applying `iptables` and policy routing rules for `tun0`), network latency spikes or transient drops directly impact `api.js` request execution. Without request timeouts, network stalling permanently freezes frontend promises.
3. **Absence of Shared Service Store:**
   State is managed through raw React state hooks without global store abstractions (like Redux or Zustand) or dedicated query-caching libraries (like TanStack Query / SWR). Consequently, caching, deduplication, backoff, optimistic updates, and cancellation must be handled explicitly within React component lifecycles.

---

## 3. Findings Summary Table

| ID | Title & Category | Code Location | Severity | Status |
|---|---|---|---|---|
| **WEB-01** | Missing Fetch Request Timeouts & Background Polling Overwrite During Active State Transitions | `web/src/services/api.js:L36-L53`<br>`web/src/App.jsx:L89-L113`<br>`web/src/App.jsx:L168-L181` | **High** | Confirmed |
| **WEB-02** | Persistent Administrative Session Storage in `localStorage` Lacking Inactivity Invalidation | `web/src/services/api.js:L3-L9`<br>`web/src/services/api.js:L22-L34`<br>`web/src/pages/LoginPage.jsx:L19-L22` | **High** | Confirmed |
| **WEB-03** | Insufficient Error Boundary Granularity & Array Filtering Vulnerability in Config/Routing Tables | `web/src/App.jsx:L254-L325`<br>`web/src/pages/ConfigsPage.jsx:L14-L36`<br>`web/src/pages/RoutingPage.jsx:L35-L40` | **High** | Confirmed |
| **WEB-04** | Missing Client-Side Active Config Preconditions & Polling Reversion Race Hazard in Master Switch | `web/src/App.jsx:L128-L144`<br>`web/src/pages/DashboardPage.jsx:L23-L32`<br>`web/src/services/api.js:L87` | **Medium** | Confirmed |
| **WEB-05** | Dangling Promises and Unmounted Component State Mutation in ConfigCard, UpdateCoreModal, and Routing Actions | `web/src/components/ConfigCard.jsx:L27-L40`<br>`web/src/components/UpdateCoreModal.jsx:L14-L29`<br>`web/src/pages/LoginPage.jsx:L12-L28`<br>`web/src/App.jsx:L224-L233` | **Medium** | Confirmed |
| **WEB-06** | Clipboard API Crash on Plain HTTP Access & Synchronous `window.alert` Disrupting Anti-Lockout Timers | `web/src/components/ConfigCard.jsx:L17-L25`<br>`web/src/App.jsx:L71,L140,L153,L163` | **Low / Refactor** | Confirmed |
| **WEB-07** | Core Engine Update Lifecycle Flaws: Silent Null Render, In-Flight Premature Dismissal, and Unchecked Active Tunnel Modification | `web/src/components/UpdateCoreModal.jsx:L10,L44-L57`<br>`web/src/pages/SettingsPage.jsx:L151-L165`<br>`web/src/App.jsx:L319-L324` | **Medium** | Confirmed |

---

## 4. In-Depth Defect Reports

---

### WEB-01: Missing Fetch Request Timeouts & Background Polling Overwrite During Active State Transitions

#### 1. ID & Title
`WEB-01`: Missing Fetch Request Timeouts & Background Polling Overwrite During Active State Transitions.

#### 2. Code Location
- `web/src/services/api.js:L36-L53`
- `web/src/App.jsx:L89-L113`
- `web/src/App.jsx:L168-L181`

#### 3. Severity
**High**

#### 4. Trigger & Root Cause
In `web/src/services/api.js`, the central `request()` function delegates directly to browser `fetch(url, options)` without enforcing a request timeout or AbortController:
```javascript
async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;
  const token = getToken();

  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(url, {
    ...options,
    headers,
  });
  ...
```
When V2Raynix activates or deactivates the master tunnel, host network interfaces and `iptables` mangle rules are flushed and recreated. If an outbound route hangs or a TCP socket stalls, `fetch` will wait indefinitely for the underlying OS socket timeout (which can exceed 120 seconds).

Furthermore, in `web/src/App.jsx`, the background polling loop (`poll()`) runs continuously every 2000ms:
```javascript
const poll = async () => {
  try {
    const [stat, cfgs, rls] = await Promise.all([
      api.getTunnelStatus(),
      api.getConfigs(),
      api.getRoutingRules(),
    ]);
    if (isCancelled) return;
    setStatus(stat);
    setConfigs(cfgs || []);
    setRoutingRules(rls || []);
...
```
When a user performs an explicit mutation (such as activating a configuration via `handleActivateConfig(id)`):
```javascript
const handleActivateConfig = async (id) => {
  setStatus(prev => (prev ? { ...prev, activeConfigId: id } : { activeConfigId: id }));
  try {
    await api.activateConfig(id);
    const [newStat, newCfgs] = await Promise.all([
      api.getTunnelStatus(),
      api.getConfigs(),
    ]);
    setStatus(newStat);
    setConfigs(newCfgs);
...
```
The optimistic state update immediately sets `activeConfigId: id`. However, if the background polling loop was already in flight, its delayed response returns the *pre-activation* status from the server and immediately overwrites `status` with stale data. Moments later, when `handleActivateConfig` completes, it overwrites it again. This causes visible UI flickering, temporary reversion of selected cards, and race conditions where outdated data displays on screen.

#### 5. PoC / Verification
1. Start V2Raynix and log into the Web UI.
2. Simulate a network interface stall or drop traffic on port 2080 via `iptables -A INPUT -p tcp --dport 2080 -j DROP`.
3. Observe Network tab in Chrome DevTools: `poll()` requests remain in `Pending` state indefinitely without timeout.
4. On `ConfigsPage`, rapidly click "Activate" on a new config while the 2000ms polling tick is scheduled.
5. In React DevTools, observe the `status.activeConfigId` change to the new ID, revert back to the old ID when `poll()` finishes, and then flip to the new ID when `handleActivateConfig` finishes.

#### 6. Recommended Fix
1. Add a default timeout (e.g. 10 seconds) using `AbortSignal.any` or `AbortSignal.timeout(10000)` in `services/api.js`.
2. Provide a pause/resume or in-flight suppression mechanism in `App.jsx` so background polling yields whenever an active mutation (`handleActivateConfig`, `handleToggleTunnel`) is executing:

```javascript
// web/src/services/api.js
async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;
  const token = getToken();

  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  // Default 12s timeout if no external signal provided
  const timeoutSignal = AbortSignal.timeout(options.timeout || 12000);
  const signal = options.signal 
    ? AbortSignal.any([options.signal, timeoutSignal])
    : timeoutSignal;

  const response = await fetch(url, {
    ...options,
    headers,
    signal,
  });
  ...
```

```javascript
// web/src/App.jsx
const isMutatingRef = useRef(false);

const handleActivateConfig = async (id) => {
  isMutatingRef.current = true;
  setStatus(prev => (prev ? { ...prev, activeConfigId: id } : { activeConfigId: id }));
  try {
    await api.activateConfig(id);
    const [newStat, newCfgs] = await Promise.all([
      api.getTunnelStatus(),
      api.getConfigs(),
    ]);
    setStatus(newStat);
    setConfigs(newCfgs);
  } catch (err) {
    alert(err.message || 'Failed to activate config');
  } finally {
    isMutatingRef.current = false;
  }
};

// In poll():
if (isMutatingRef.current) {
  // Yield to in-flight user mutation
  timerId = setTimeout(poll, 2000);
  return;
}
```

#### 7. Strengths
- Commit `feddcb2` successfully added exponential backoff (`Math.min(2000 * Math.pow(1.5, failureCount), 30000)`) and random jitter (0-500ms) to prevent server stampeding when the backend is completely offline.
- Polling cleans up properly upon component unmount or user logout via `clearTimeout(timerId)` and `isCancelled = true`.

---

### WEB-02: Persistent Administrative Session Storage in `localStorage` Lacking Inactivity Invalidation

#### 1. ID & Title
`WEB-02`: Persistent Administrative Session Storage in `localStorage` Lacking Inactivity Invalidation and Origin Partitioning.

#### 2. Code Location
- `web/src/services/api.js:L3-L9`
- `web/src/services/api.js:L22-L34`
- `web/src/pages/LoginPage.jsx:L19-L22`

#### 3. Severity
**High**

#### 4. Trigger & Root Cause
In `web/src/services/api.js`:
```javascript
export function getToken() {
  try {
    return sessionStorage.getItem('v2raynix_token') || localStorage.getItem('v2raynix_token');
  } catch (e) {
    return null;
  }
}

export function setToken(token) {
  try {
    if (token) {
      sessionStorage.setItem('v2raynix_token', token);
      localStorage.setItem('v2raynix_token', token);
    } else {
      sessionStorage.removeItem('v2raynix_token');
      localStorage.removeItem('v2raynix_token');
    }
  } catch (e) {
    console.warn('Storage operation failed:', e);
  }
}
```
Administrative JWT tokens grant total control over V2Raynix backend daemons, root-level iptables rules, routing policies, and arbitrary binary updates. 
Storing the token unconditionally in `localStorage` means:
1. The administrative credential persists indefinitely on the disk of the client workstation, surviving tab closure, browser closure, and machine reboots.
2. Any client-side script running on that origin (or an attacker with local file access / browser extension access) can extract `v2raynix_token` from `localStorage` at any time.
3. No client-side inactivity timer exists to expire the session if the operator walks away from an open terminal.
4. While `sessionStorage` is also written, the fallback to `localStorage` in `getToken()` completely negates the ephemeral security benefits of `sessionStorage`.

#### 5. PoC / Verification
1. Log into V2Raynix dashboard as `admin`.
2. Open Chrome DevTools -> Application -> Local Storage.
3. Inspect `v2raynix_token`: Observe the raw JWT string stored permanently in plain text.
4. Close all browser windows and tabs. Reopen browser and navigate directly to `http://localhost:2080`.
5. Observe that the user is immediately authenticated without entering credentials, regardless of elapsed time.

#### 6. Recommended Fix
1. Prefer `sessionStorage` exclusively for sensitive administrative sessions, avoiding `localStorage` persistence unless explicitly requested via a "Remember Me" checkbox.
2. Implement a client-side activity tracker (reset on `mousemove`, `keydown`, `touchstart`) that invalidates the session and calls `handleLogout()` after 30 minutes of idle time.
3. On the backend, support `HttpOnly; SameSite=Strict; Secure` session cookies to eliminate script access to tokens altogether.

```javascript
// web/src/services/api.js - Ephemeral Session-Only Storage
export function getToken() {
  try {
    return sessionStorage.getItem('v2raynix_token');
  } catch (e) {
    return null;
  }
}

export function setToken(token) {
  try {
    if (token) {
      sessionStorage.setItem('v2raynix_token', token);
    } else {
      sessionStorage.removeItem('v2raynix_token');
      localStorage.removeItem('v2raynix_token'); // Ensure legacy storage is cleared
    }
  } catch (e) {
    console.warn('Storage operation failed:', e);
  }
}
```

#### 7. Strengths
- Storage operations are wrapped in `try ... catch` blocks to avoid unhandled security exceptions in private/incognito browsing modes where `localStorage` access is blocked.
- When an API request returns HTTP 401 Unauthorized, `api.js` immediately clears tokens and triggers the `onUnauthorizedCallback` to return the user to the login screen.

---

### WEB-03: Insufficient Error Boundary Granularity & Array Filtering Vulnerability in Config/Routing Tables

#### 1. ID & Title
`WEB-03`: Insufficient Error Boundary Granularity & Array Filtering Vulnerability in Config/Routing Tables.

#### 2. Code Location
- `web/src/App.jsx:L254-L325`
- `web/src/pages/ConfigsPage.jsx:L14-L36`
- `web/src/pages/RoutingPage.jsx:L35-L40`

#### 3. Severity
**High**

#### 4. Trigger & Root Cause
There are two coupled vulnerabilities in error handling and data rendering:

**Vulnerability 1: Unprotected Shell Components Outside Error Boundary:**
In `web/src/App.jsx`:
```jsx
    <div className="app-container">
      <Navbar 
        activeTab={activeTab} 
        onSelectTab={setActiveTab} 
        user={user} 
        onLogout={handleLogout}
        hasUpdate={hasUpdate}
        onOpenUpdateModal={() => setIsUpdateModalOpen(true)}
      />

      <main className="main-content">
        <ErrorBoundary>
          {activeTab === 'dashboard' && ( ... )}
          {activeTab === 'configs' && ( ... )}
          ...
        </ErrorBoundary>
      </main>

      {/* Safe Mode Auto-Rollback Modal */}
      {status?.safeMode?.isActive && (
        <SafeModeModal ... />
      )}

      {/* Core Engine Update Modal */}
      <UpdateCoreModal ... />
    </div>
```
The inner `<ErrorBoundary>` wraps only the tab content inside `<main>`. `Navbar`, `SafeModeModal`, and `UpdateCoreModal` are located outside this boundary. If an unhandled rendering error occurs in any of these three components (e.g. malformed `safeMode` data from backend or corrupted `updateData`), the error bubbles past `App.jsx` to the root boundary in `main.jsx`. This unmounts the *entire* application, stripping the user of all navigation, tab switching, and logout capabilities, leaving only a "Reload View" button.

**Vulnerability 2: Filtering Flaw Permitting Null Objects to Cause Crash:**
In `web/src/pages/ConfigsPage.jsx`:
```javascript
  const filteredConfigs = (configs || []).filter(c => 
    (c?.name || '').toLowerCase().includes(searchTerm.toLowerCase()) ||
    (c?.server || '').toLowerCase().includes(searchTerm.toLowerCase()) ||
    (c?.protocol || '').toLowerCase().includes(searchTerm.toLowerCase())
  );
```
When `searchTerm` is empty string `""` (the default initial state):
If `configs` contains a `null` or `undefined` item (e.g., due to sparse JSON deserialization or empty store slots), `(c?.name || '').toLowerCase()` evaluates to `""`.
In JavaScript: `"".includes("")` evaluates to `true`!
Therefore, `null` passes through `filteredConfigs`!
Immediately following:
```javascript
  const sortedConfigs = [...filteredConfigs].sort((a, b) => {
    if (sortBy === 'ping') {
      const pA = (a.latencyMs > 0) ? a.latencyMs : 99999; // CRASH if a is null!
...
    // Default 'date': newest first
    const comp = (b.createdAt || '').localeCompare(a.createdAt || ''); // CRASH if b or a is null!
```
`sort()` throws `TypeError: Cannot read properties of null (reading 'createdAt')`.
The view crashes. The ErrorBoundary renders the fallback card. Clicking "Reload View" reloads the page, fetches the same config array from the server, and crashes again immediately, creating an unrecoverable infinite reload loop.

A similar vulnerability exists in `web/src/pages/RoutingPage.jsx:L35-L40`:
```javascript
  const sortedRules = [...(rules || [])].sort((a, b) => {
    if ((b.priority || 0) !== (a.priority || 0)) { // CRASH if a or b is null!
      return (b.priority || 0) - (a.priority || 0);
    }
    return (a.id || '').localeCompare(b.id || '');
  });
```

#### 5. PoC / Verification
1. Mock or inject a backend response from `GET /api/configs` containing `[null, {"id":"cfg1","name":"Test"}]`.
2. Navigate to the Configurations tab in V2Raynix.
3. Console logs: `Uncaught TypeError: Cannot read properties of null (reading 'createdAt')`.
4. The screen collapses into "Component Render Error".
5. Click "Reload View": The page reloads, fetches `[null, ...]`, and immediately collapses again. The dashboard is permanently bricked until the backend database is manually modified.

#### 6. Recommended Fix
1. Explicitly filter out falsy items in `filteredConfigs` and `sortedRules`: `filter(c => Boolean(c) && typeof c === 'object')`.
2. Wrap `SafeModeModal` and `UpdateCoreModal` in individual Error Boundaries or move them inside the main error boundary.
3. Provide an ErrorBoundary reset action that can clear cached client state rather than strictly forcing a browser reload:

```javascript
// web/src/pages/ConfigsPage.jsx
const filteredConfigs = (Array.isArray(configs) ? configs : [])
  .filter(c => c && typeof c === 'object')
  .filter(c => 
    (c.name || '').toLowerCase().includes(searchTerm.toLowerCase()) ||
    (c.server || '').toLowerCase().includes(searchTerm.toLowerCase()) ||
    (c.protocol || '').toLowerCase().includes(searchTerm.toLowerCase())
  );
```

```javascript
// web/src/pages/RoutingPage.jsx
const sortedRules = (Array.isArray(rules) ? rules : [])
  .filter(r => r && typeof r === 'object')
  .sort((a, b) => {
    const prioA = Number(a.priority) || 0;
    const prioB = Number(b.priority) || 0;
    if (prioB !== prioA) return prioB - prioA;
    return String(a.id || '').localeCompare(String(b.id || ''));
  });
```

#### 7. Strengths
- `ErrorBoundary.jsx` provides an informative fallback card with icons, clear typography, and a prominent "Reload View" button.
- Unhandled errors in any tab component are isolated from tearing down other un-rendered tabs when navigated.

---

### WEB-04: Missing Client-Side Active Config Preconditions & Polling Reversion Race Hazard in Master Switch

#### 1. ID & Title
`WEB-04`: Missing Client-Side Active Config Preconditions & Polling Reversion Race Hazard in Master Switch.

#### 2. Code Location
- `web/src/App.jsx:L128-L144`
- `web/src/pages/DashboardPage.jsx:L23-L32`
- `web/src/services/api.js:L87`

#### 3. Severity
**Medium**

#### 4. Trigger & Root Cause
In `web/src/pages/DashboardPage.jsx`:
```jsx
export default function DashboardPage({ status, configs, onToggleTunnel, onSelectTab, toggling }) {
  const isConnected = status?.state === 'connected';
  const isConnecting = status?.state === 'connecting';
  const activeConfig = configs.find(c => c.id === status?.activeConfigId);
...
  <button 
    className={`master-btn ${isConnected ? 'connected' : ''} ${(isConnecting || toggling) ? 'connecting' : ''}`}
    onClick={onToggleTunnel}
    disabled={isConnecting || toggling}
  >
```
1. **Unchecked Zero-Config Connection Attempt:**
   When no configuration is active (`activeConfig` is `undefined`), the master button remains enabled for connection. When clicked:
   In `App.jsx`:
   ```javascript
   const handleToggleTunnel = async () => {
     if (toggling) return;
     setToggling(true);
     try {
       if (status?.state === 'connected') {
         const newStat = await api.disconnectTunnel();
         setStatus(newStat);
       } else {
         const newStat = await api.connectTunnel();
         setStatus(newStat);
       }
     } catch (err) {
       alert(err.message || 'Tunnel operation failed');
     } finally {
       setToggling(false);
     }
   };
   ```
   `api.connectTunnel()` is dispatched to `/api/tunnel/connect`. The backend rejects the request with HTTP 400 (`{"error": "no active config selected"}`). The frontend triggers a blocking `alert('no active config selected')`. The UI should disallow clicking the button or guide the user directly to the Configs tab when no config is selected.

2. **Delayed Polling Reversion Race Condition:**
   `poll()` in `App.jsx` executes every 2000ms. If a background status request `req_poll` is dispatched at $t=0$, and the user clicks the Connect button at $t=100\text{ms}$:
   - `handleToggleTunnel` calls `api.connectTunnel()` which resolves at $t=1200\text{ms}$, updating `status.state` to `'connected'`.
   - However, if `req_poll` was delayed by network latency (or network interface creation on `tun0`) and resolves at $t=1250\text{ms}$:
   - `setStatus(stat)` in `poll()` executes with the *stale* status (`disconnected`) captured before the tunnel was started.
   - The UI snaps back from `CONNECTED` to `DISCONNECTED` even though the backend tunnel is active.

3. **API Client Signature Mismatch:**
   In `web/src/services/api.js`:
   `connectTunnel: (configId) => request('/tunnel/connect', { method: 'POST', body: JSON.stringify({ configId }) })`
   In `App.jsx`:
   `const newStat = await api.connectTunnel();`
   No `configId` is passed, serializing `{}` into the body. While the backend ignores the body and checks `Store.GetActiveConfig()`, the method signature creates confusion and disconnect in API contracts.

#### 5. PoC / Verification
1. Fresh install of V2Raynix without selecting an active configuration.
2. Navigate to Dashboard. Observe the large circular Power button is fully enabled with label `DISCONNECTED`.
3. Click the Power button: A synchronous browser `alert("no active config selected")` freezes the UI.
4. To verify the polling race: Add a 1500ms delay to `GET /api/tunnel/status` responses. Click the Master button: Observe the button switch to `CONNECTED` upon `POST /api/tunnel/connect` completion, and then flip back to `DISCONNECTED` when the delayed status response resolves.

#### 6. Recommended Fix
1. Disable the master button or show a warning prompt when `status?.state !== 'connected'` and `!status?.activeConfigId`.
2. Cancel or ignore in-flight polling ticks during and immediately after tunnel toggle operations.
3. Replace blocking `alert()` calls with inline toast or banner notifications.

```jsx
// web/src/pages/DashboardPage.jsx
const hasActiveConfig = Boolean(status?.activeConfigId && activeConfig);
const canToggle = isConnected || (hasActiveConfig && !isConnecting && !toggling);

<button 
  className={`master-btn ${isConnected ? 'connected' : ''} ${(isConnecting || toggling) ? 'connecting' : ''}`}
  onClick={onToggleTunnel}
  disabled={!canToggle}
  title={!hasActiveConfig && !isConnected ? 'Select an active config first' : ''}
>
  <Power size={48} strokeWidth={2.5} />
  <span style={{ fontSize: '0.85rem', fontWeight: 700, letterSpacing: '0.05em' }}>
    {toggling ? 'SWITCHING...' : isConnected ? 'CONNECTED' : isConnecting ? 'STARTING...' : !hasActiveConfig ? 'NO CONFIG' : 'DISCONNECTED'}
  </span>
</button>
```

#### 7. Strengths
- The `toggling` state lock prevents multiple rapid clicks on the master switch, eliminating immediate duplicate request dispatching.
- Uptime calculation and status formatting are clean, readable, and defensive against undefined inputs (`formatUptime`).

---

### WEB-05: Dangling Promises and Unmounted Component State Mutation in ConfigCard, UpdateCoreModal, and Routing Actions

#### 1. ID & Title
`WEB-05`: Dangling Promises and Unmounted Component State Mutation in ConfigCard, UpdateCoreModal, and Routing Actions.

#### 2. Code Location
- `web/src/components/ConfigCard.jsx:L27-L40`
- `web/src/components/UpdateCoreModal.jsx:L14-L29`
- `web/src/pages/LoginPage.jsx:L12-L28`
- `web/src/App.jsx:L224-L233`

#### 3. Severity
**Medium**

#### 4. Trigger & Root Cause
Multiple components execute asynchronous operations that update state without checking if the component is still mounted:

1. **`ConfigCard.jsx:L27-L40` (`handleTest`):**
   ```javascript
   const handleTest = async (e) => {
     e.stopPropagation();
     if (testing) return;
     setTesting(true);
     try {
       if (onTest) {
         await onTest(config.id);
       } else if (onPing) {
         await onPing(config.id);
       }
     } finally {
       setTesting(false); // CRASH / Memory Leak warning if unmounted!
     }
   };
   ```
   Testing latency invokes real TCP handshakes and takes 2-3 seconds. If the user switches tabs (e.g. to Dashboard or Logs) or deletes a config while testing is in progress, the component unmounts. `setTesting(false)` runs on the unmounted component, retaining the fiber node in memory.

2. **`UpdateCoreModal.jsx:L14-L29` (`handleUpdate`):**
   Downloading core engines can take 10-60 seconds. The modal close button `(X)` is enabled during updating. If the operator closes the modal, the modal unmounts. When `api.updateCore` finishes, `setLogMessage`, `setError`, and `setUpdatingCore` are invoked on an unmounted component.

3. **`LoginPage.jsx:L12-L28` (`handleSubmit`):**
   ```javascript
   try {
     const res = await api.login(username, password);
     if (res.token) {
       setToken(res.token);
       onLoginSuccess(res.user); // Triggers immediate parent unmount of LoginPage!
     }
   } catch (err) {
     setError(err.message || 'Invalid username or password');
   } finally {
     setLoading(false); // Fires after LoginPage is unmounted!
   }
   ```
   Calling `onLoginSuccess(res.user)` immediately sets `user` in `App.jsx`, which replaces `<LoginPage />` with the authenticated layout. The `finally` block then attempts to call `setLoading(false)` on the unmounted login page.

4. **`App.jsx:L224-L233` (Missing Catch Blocks in Routing Actions):**
   ```javascript
   const handleCreateRule = async (rule) => {
     await api.createRoutingRule(rule);
     const newRules = await api.getRoutingRules();
     setRoutingRules(newRules);
   };

   const handleDeleteRule = async (id) => {
     await api.deleteRoutingRule(id);
     setRoutingRules(routingRules.filter(r => r.id !== id));
   };
   ```
   Unlike config actions, `handleCreateRule` and `handleDeleteRule` omit `try ... catch` blocks entirely. If backend validation fails or the connection drops, an unhandled promise rejection is thrown.

#### 5. PoC / Verification
1. On `ConfigsPage`, click the test icon on a configuration card.
2. Immediately switch tabs to "Logs".
3. Check browser console: A React warning is logged: `Warning: Can't perform a React state update on an unmounted component. This is a no-op, but it indicates a memory leak in your application.`
4. On `RoutingPage`, attempt to add an invalid or malformed rule while offline: Notice an unhandled rejection in console without UI error notification.

#### 6. Recommended Fix
1. Introduce an `isMountedRef` or cancellation flag in `ConfigCard.jsx` and `UpdateCoreModal.jsx`.
2. Disable the close button on `UpdateCoreModal` while `updatingCore !== null`.
3. Wrap `handleCreateRule` and `handleDeleteRule` in `try ... catch` blocks.
4. In `LoginPage.jsx`, do not update `loading` state after `onLoginSuccess` has executed.

```javascript
// web/src/components/ConfigCard.jsx
const isMounted = useRef(true);
useEffect(() => {
  isMounted.current = true;
  return () => { isMounted.current = false; };
}, []);

const handleTest = async (e) => {
  e.stopPropagation();
  if (testing) return;
  setTesting(true);
  try {
    if (onTest) await onTest(config.id);
    else if (onPing) await onPing(config.id);
  } finally {
    if (isMounted.current) {
      setTesting(false);
    }
  }
};
```

#### 7. Strengths
- `LogsPage.jsx` sets an exemplary pattern: it uses `isMountedRef` and `controllerRef` with `AbortController` to abort in-flight requests and prevent unmounted state updates.
- `ConfigCard.jsx` properly cleans up its `copyTimeoutRef` on unmount.

---

### WEB-06: Clipboard API Crash on Plain HTTP Access & Synchronous `window.alert` Disrupting Anti-Lockout Timers

#### 1. ID & Title
`WEB-06`: Clipboard API Crash on Plain HTTP Access & Synchronous `window.alert` Disrupting Anti-Lockout Timers.

#### 2. Code Location
- `web/src/components/ConfigCard.jsx:L17-L25`
- `web/src/App.jsx:L71,L140,L153,L163,L180,L189,L210`

#### 3. Severity
**Low / Refactor**

#### 4. Trigger & Root Cause
1. **Clipboard API in Non-Secure Contexts:**
   In `web/src/components/ConfigCard.jsx`:
   ```javascript
   const copyToClipboard = (e) => {
     e.stopPropagation();
     navigator.clipboard.writeText(config.rawUrl);
     setCopied(true);
     if (copyTimeoutRef.current) {
       clearTimeout(copyTimeoutRef.current);
     }
     copyTimeoutRef.current = setTimeout(() => setCopied(false), 2000);
   };
   ```
   Under modern browser security specifications, `navigator.clipboard` is restricted to secure contexts (`https://` or `localhost`). V2Raynix is commonly deployed on headless Linux servers and accessed via local IP (e.g. `http://192.168.1.100:2080`).
   In this environment, `navigator.clipboard` is `undefined`. Clicking the copy button causes an unhandled exception: `TypeError: Cannot read properties of undefined (reading 'writeText')`. The action fails silently, leaving the user unable to copy configuration URLs.

2. **Synchronous `window.alert` Stalling SafeMode Timers:**
   Throughout `App.jsx`, error notifications are handled via `alert(err.message)`.
   Because `window.alert()` is modal and blocking, it halts the browser's JavaScript event loop. If an error occurs when a user confirms or rolls back SafeMode (or during a network drop), the alert modal blocks execution of `SafeModeModal.jsx`'s countdown timer:
   ```javascript
   // SafeModeModal.jsx
   useEffect(() => {
     if (localRemaining <= 0) return;
     const interval = setInterval(() => {
       setLocalRemaining(prev => Math.max(0, prev - 1));
     }, 1000);
     return () => clearInterval(interval);
   }, [localRemaining > 0]);
   ```
   While the alert is active on screen, the UI countdown freezes. Meanwhile, on the backend server, the Go supervisor's independent 120-second timer continues ticking. The user dismisses the alert thinking they have ample time remaining, only to find the connection rolled back unexpectedly.

#### 5. PoC / Verification
1. Access the V2Raynix dashboard over plain HTTP via a LAN IP (e.g. `http://<server-ip>:2080`).
2. Go to the Configs tab and click the copy icon on any config card.
3. Check browser console: Observe `Uncaught TypeError: Cannot read properties of undefined (reading 'writeText')`.
4. Trigger SafeMode, trigger a network error, and leave the resulting `alert()` dialog open for 30 seconds: Observe the countdown timer frozen in the background.

#### 6. Recommended Fix
1. Implement a fallback for clipboard operations that checks `navigator?.clipboard?.writeText` and falls back to a temporary `textarea` + `document.execCommand('copy')`.
2. Replace all instances of `alert()` in `App.jsx` with an asynchronous toast or notification banner that does not block the JavaScript event loop.

```javascript
// Clipboard helper with non-secure fallback
export async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    return navigator.clipboard.writeText(text);
  }
  // Fallback for HTTP access
  const textArea = document.createElement('textarea');
  textArea.value = text;
  textArea.style.position = 'fixed';
  textArea.style.opacity = '0';
  document.body.appendChild(textArea);
  textArea.select();
  try {
    document.execCommand('copy');
  } finally {
    document.body.removeChild(textArea);
  }
}
```

#### 7. Strengths
- The copied state properly gives visual feedback by switching to a green checkmark icon (`<Check size={16} color="#34d399" />`) for 2000ms.
- Timers for resetting the copy state are correctly cleared on unmount.

---

### WEB-07: Core Engine Update Lifecycle Flaws: Silent Null Render, In-Flight Premature Dismissal, and Unchecked Active Tunnel Modification

#### 1. ID & Title
`WEB-07`: Core Engine Update Lifecycle Flaws: Silent Null Render, In-Flight Premature Dismissal, and Unchecked Active Tunnel Modification.

#### 2. Code Location
- `web/src/components/UpdateCoreModal.jsx:L10,L44-L57`
- `web/src/pages/SettingsPage.jsx:L151-L165`
- `web/src/App.jsx:L319-L324`

#### 3. Severity
**Medium**

#### 4. Trigger & Root Cause
1. **Silent Null Render on Initial Click:**
   In `web/src/components/UpdateCoreModal.jsx:L10`:
   `if (!isOpen || !updateData) return null;`
   In `web/src/pages/SettingsPage.jsx:L153`:
   `<button type="button" onClick={onOpenUpdateModal} ...>`
   If `updateData` is null or has not yet resolved from the backend, clicking the update button calls `setIsUpdateModalOpen(true)` in `App.jsx`. However, because `updateData` is null, `UpdateCoreModal` returns `null`. Absolutely nothing renders on the screen, giving the operator the impression that the UI is unresponsive.

2. **Premature Dismissal During Critical Binary Writing:**
   In `web/src/components/UpdateCoreModal.jsx`:
   The modal exit button `(X)` at line 44 is not disabled when `updatingCore !== null`. The operator can close the modal while the backend is actively downloading, verifying hashes, or replacing `/usr/local/bin/xray` or `/usr/local/bin/tun2socks`. Closing the modal unmounts the component, losing the console log feedback and leaving the operator completely unaware of whether the binary installation completed successfully or failed hash verification.

3. **Unchecked Binary Replacement on Running Daemons:**
   Neither `UpdateCoreModal.jsx` nor `SettingsPage.jsx` checks `status?.state === 'connected'` before triggering `api.updateCore(coreName)`. If a user initiates an update on `xray` while traffic is actively routed through `tun0`, replacing or restarting the core daemon terminates all active connections, causes socket drops, and risks leaving routing policies pointing to an inactive daemon. The modal provides no warning or confirmation prompt to disconnect the tunnel prior to upgrading.

#### 5. PoC / Verification
1. Open V2Raynix in a fresh tab. Navigate directly to Settings before update metadata loads.
2. Click on "v... available ->": The modal does not render (`return null`).
3. Connect the master tunnel. Open the Update modal and click "Update" on `xray`.
4. While "Updating..." is spinning, click the `(X)` close button: The modal disappears with no way to monitor the installation progress.
5. In the backend, observe the active `xray` process terminated while `tun0` routing rules remain applied.

#### 6. Recommended Fix
1. Display a loading spinner in `UpdateCoreModal` when `isOpen` is true but `updateData` is null.
2. Disable the close button and background backdrop click when `updatingCore !== null`.
3. Check `status?.state === 'connected'` and display a warning banner requiring tunnel disconnection before applying core updates.

```jsx
// web/src/components/UpdateCoreModal.jsx
if (!isOpen) return null;

if (!updateData) {
  return (
    <div className="modal-overlay">
      <div className="modal-card" style={{ textAlign: 'center', padding: '2rem' }}>
        <RefreshCw size={24} className="animate-spin" style={{ margin: '0 auto 1rem', color: '#38bdf8' }} />
        <p>Loading core update information...</p>
      </div>
    </div>
  );
}

// Disable close during active update
<button
  onClick={onClose}
  disabled={updatingCore !== null}
  style={{ opacity: updatingCore !== null ? 0.3 : 1, cursor: updatingCore !== null ? 'not-allowed' : 'pointer' }}
>
  <X size={20} />
</button>
```

#### 7. Strengths
- Individual core update state is isolated (`updatingCore === c.name`), allowing clear visual distinction between cores.
- Post-update hook (`onRefreshUpdates`) automatically refreshes system update metadata upon successful completion.

---

## 5. Security & State Lifecycle Analysis

### 5.1 Token Lifecycle & XSS Attack Surface Evaluation
- **Token Security:** Currently, JWT tokens are placed in both `sessionStorage` and `localStorage`. In single-user administrative server applications, persistent storage in `localStorage` presents high risk if unauthorized users access the machine or if an XSS flaw exists. Transitioning to `sessionStorage` or HTTP-only cookies eliminates persistent credential exposure.
- **XSS Vector Analysis:** An audit of JSX markup across all components confirmed that React's native string escaping is used consistently. No usages of `dangerouslySetInnerHTML`, `innerHTML`, `outerHTML`, or `document.write` were found in `web/src`. Log messages in `LogsPage.jsx` and server configurations in `ConfigCard.jsx` are safely escaped as text nodes, preventing DOM XSS from malicious server names or log streams.

### 5.2 Concurrency & Race Hazard Model
The primary state hazard stems from uncontrolled concurrency between **background periodic polling** and **explicit user mutations**:

```
Timeline (Race Condition):
  t=0ms:    Background poll() dispatched (GET /api/tunnel/status) [Server state: DISCONNECTED]
  t=50ms:   User clicks "Connect" -> handleToggleTunnel() calls POST /api/tunnel/connect
  t=800ms:  POST /api/tunnel/connect succeeds -> setStatus('connected') [UI: CONNECTED]
  t=850ms:  Delayed poll() response returns -> setStatus('disconnected') [UI reverts: DISCONNECTED]
  Result:   UI displays DISCONNECTED while the proxy tunnel is actively running.
```

To eliminate this class of race hazard, the UI must implement request sequence versioning or pause background polling during user mutations.

---

## 6. Remediation Matrix & Prioritized Roadmap

| Priority | Defect ID | Remediation Task | Effort | Files Impacted |
|---|---|---|---|---|
| **P0 (Immediate)** | `WEB-03` | Add strict null/type guards in `ConfigsPage` and `RoutingPage` filters and sort functions to prevent fatal render crashes and reload loops. | 1 hr | `web/src/pages/ConfigsPage.jsx`<br>`web/src/pages/RoutingPage.jsx` |
| **P0 (Immediate)** | `WEB-01` | Add `AbortSignal.timeout(10000)` to `api.js` requests; pause background polling while mutations are executing. | 2 hrs | `web/src/services/api.js`<br>`web/src/App.jsx` |
| **P1 (High)** | `WEB-02` | Deprecate `localStorage` for JWT tokens; store strictly in `sessionStorage` and add idle timeout invalidation. | 2 hrs | `web/src/services/api.js`<br>`web/src/pages/LoginPage.jsx` |
| **P1 (High)** | `WEB-04` | Disable master switch when no active config is selected; cancel or ignore in-flight polling during master toggle. | 2 hrs | `web/src/App.jsx`<br>`web/src/pages/DashboardPage.jsx` |
| **P2 (Medium)** | `WEB-05` | Add mount guards (`isMountedRef`) in `ConfigCard` and `UpdateCoreModal`; wrap routing actions in `try/catch`. | 2 hrs | `web/src/components/ConfigCard.jsx`<br>`web/src/components/UpdateCoreModal.jsx`<br>`web/src/App.jsx` |
| **P2 (Medium)** | `WEB-07` | Implement loading state for `UpdateCoreModal`, lock modal dismissal during download, and require tunnel disconnection before core upgrades. | 2 hrs | `web/src/components/UpdateCoreModal.jsx` |
| **P3 (Low)** | `WEB-06` | Implement `copyText` fallback for non-secure HTTP contexts; replace blocking `window.alert` with non-blocking toast alerts. | 3 hrs | `web/src/components/ConfigCard.jsx`<br>`web/src/App.jsx` |

---

## 7. Sign-Off & Verification Verdict

The Domain 07 Web Frontend & UI State Subsystem audit is concluded. All 14 subsystem source files have been thoroughly inspected against client security, state concurrency, memory lifecycle, error boundaries, and API interaction patterns. No application source code was modified during this audit.

**Audit Verdict:** Complete & Documented. Seven distinct architectural defects (`WEB-01` through `WEB-07`) are fully detailed with concrete root causes, triggers, verification proofs, and recommended remediations.
