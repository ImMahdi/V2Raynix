# Security & Architecture Audit Report: Domain 08 (Web Frontend & UI State Subsystem)

> **Audited Subsystem:** Web Frontend, UI State Management, React Architecture & Client Security  
> **Target Files:**  
> - `web/src/App.jsx`  
> - `web/src/services/api.js`  
> - `web/src/main.jsx`  
> - `web/src/pages/DashboardPage.jsx`  
> - `web/src/pages/ConfigsPage.jsx`  
> - `web/src/pages/RoutingPage.jsx`  
> - `web/src/pages/LogsPage.jsx`  
> - `web/src/pages/SettingsPage.jsx`  
> - `web/src/pages/LoginPage.jsx`  
> - `web/src/components/ConfigCard.jsx`  
> - `web/src/components/Navbar.jsx`  
> - `web/src/components/SafeModeModal.jsx`  
> **Auditor:** Principal Frontend Architecture & Web Security Auditor  
> **Audit Date:** 2026-09-21  
> **Audit Status:** Complete & Verified  

---

## 1. Executive Summary

The `web` package provides the real-time management console and graphical user interface for V2Raynix. Built on React 18, Vite, and Lucide icons with custom Vanilla CSS glassmorphism styling, it serves as the primary administrative dashboard through which operators monitor tunnel health, import proxy subscription configs, configure routing policy rules, inspect live service logs, trigger latency ping sweeps, and manage the anti-lockout SafeMode fail-safe state machine.

Because V2Raynix controls root-level host networking (`iptables`, `ip route`, policy routing rules, and proxy daemons `xray` and `tun2socks`), stability and security within the web dashboard are critical. Frontend state desynchronization, uncontrolled polling, race conditions during toggle actions, or unhandled exceptions can leave administrators blind to underlying network state changes or cause invalid state transitions in the backend supervisor.

Our comprehensive, line-by-line inspection of the web frontend subsystem identified several critical architectural and operational defects:
- **Polling Stampede & Missing Backoff:** `App.jsx` and `LogsPage.jsx` unconditionally poll `/api/tunnel/status`, `/api/configs`, `/api/routing/rules`, and `/api/system/logs` via fixed `setInterval` timers with no exponential backoff or network failure detection. When the server goes down or restarts during network reconfiguration, the client hammers the backend with unthrottled requests.
- **Missing React Error Boundaries:** Neither `main.jsx` nor `App.jsx` implements React Error Boundaries. Unhandled runtime errors (e.g., calling `.toLowerCase()` on undefined config fields in `ConfigsPage.jsx`) unmount the entire component root, collapsing the dashboard into a blank white screen.
- **Insecure Token Storage in `localStorage`:** JWT tokens are stored indefinitely in browser `localStorage`, exposing administrative credentials with full system proxy control to any script running within the browser origin.
- **Race Hazards in Master Toggle:** Rapid clicks on the Master Connect/Disconnect switch dispatch overlapping in-flight API requests because `isConnecting` disablement relies solely on delayed server state rather than local request mutexing.
- **Memory Leaks on Component Unmount:** In-flight fetch calls and component timers (`LogsPage.jsx`, `ConfigCard.jsx`) lack `AbortController` cancellation or unmount guards, triggering state updates on unmounted components.
- **Unbounded DOM Growth in Log Streaming:** `LogsPage.jsx` renders hundreds of log lines keyed by array index without virtualization or line capping, risking browser tab degradation and UI freezes during heavy log output.

A full summary of findings and detailed 7-field defect analyses are presented below.

---

## 2. Graphify Knowledge Graph & Dependency Analysis

Based on the project's knowledge graph extracted via `graphify` (`graphify-out/graph.json` and `graphify-out/GRAPH_REPORT.md`):

```
                                  +---------------------------------------+
                                  |           Browser / Client            |
                                  |              (main.jsx)               |
                                  +-------------------+-------------------+
                                                      |
                                                      | mounts (React 18)
                                                      v
                                  +---------------------------------------+
                                  |               App.jsx                 |
                                  |     (State Shell & Root Router)       |
                                  +----+-------------+---------------+----+
                                       |             |               |
             +-------------------------+             |               +-------------------------+
             |                                       |                                         |
             v                                       v                                         v
+-------------------------+             +-------------------------+               +-------------------------+
|     UI Navigation       |             |       Active Pages      |               |     API Client Layer    |
| - Navbar.jsx            |             | - DashboardPage.jsx     |               |     (services/api.js)   |
| - SafeModeModal.jsx     |             | - ConfigsPage.jsx       |               +------------+------------+
+-------------------------+             | - RoutingPage.jsx       |                            |
                                        | - LogsPage.jsx          |                            | HTTP fetch
                                        | - SettingsPage.jsx      |                            | Bearer JWT
                                        | - LoginPage.jsx         |                            v
                                        +-------------------------+               +-------------------------+
                                                     |                            |    Go REST API Router   |
                                                     v                            |   (internal/api/router) |
                                        +-------------------------+               +-------------------------+
                                        |   Shared UI Components  |                            |
                                        | - ConfigCard.jsx        |                            v
                                        +-------------------------+               +-------------------------+
                                                                                  |   Supervisor & Store    |
                                                                                  |   (internal/core)       |
                                                                                  +-------------------------+
```

### Key Graph Insights:
1. **Frontend Hub (`App.jsx` - Community 14):**
   `App.jsx` serves as the central hub connecting authentication lifecycle (`services/api.js`), five primary views (`DashboardPage`, `ConfigsPage`, `RoutingPage`, `LogsPage`, `SettingsPage`), and global modal overlays (`SafeModeModal`). Centralized state (`status`, `configs`, `routingRules`) is refreshed on an active polling loop and passed down via props.
2. **Coupling with Backend Endpoints:**
   The frontend communicates with 14 backend REST routes across `/api/auth/*`, `/api/configs/*`, `/api/tunnel/*`, `/api/routing/*`, and `/api/system/*`. When the backend restarts the network subsystem (applying `iptables` and policy routing rules for `tun0`), network latency spikes or transient drops directly impact `api.js` request execution.
3. **Absence of Shared Service Store:**
   State is managed through raw React state hooks without global store abstractions (like Redux or Zustand) or dedicated query-caching libraries (like TanStack Query / SWR). Consequently, caching, deduplication, backoff, and cancellation must be handled explicitly within React component lifecycles.

---

## 3. Findings Summary Table

| ID | Title & Category | Code Location | Severity | Status |
|---|---|---|---|---|
| **WEB-01** | Polling Stampede & Missing Exponential Backoff During Server Disconnect | `web/src/App.jsx:L51-L72`, `web/src/pages/LogsPage.jsx:L21-L25` | **High** | Confirmed |
| **WEB-02** | Total Application Crash via Missing React Error Boundaries | `web/src/main.jsx:L6-L10`, `web/src/pages/ConfigsPage.jsx:L13-L17` | **High** | Confirmed |
| **WEB-03** | Insecure Administrative JWT Storage in `localStorage` & XSS Exposure Surface | `web/src/services/api.js:L3-L20`, `web/src/pages/LoginPage.jsx:L18-L21` | **High** | Confirmed |
| **WEB-04** | Race Conditions & Duplicate Tunnel Transitions in Master Toggle | `web/src/App.jsx:L80-L92`, `web/src/pages/DashboardPage.jsx:L23-L27` | **Medium** | Confirmed |
| **WEB-05** | Asynchronous Memory Leaks & Dangling State Updates on Component Unmount | `web/src/pages/LogsPage.jsx:L9-L25`, `web/src/components/ConfigCard.jsx:L8-L13` | **Medium** | Confirmed |
| **WEB-06** | Unbounded DOM Growth & Array Index Keying in Real-Time Log Stream | `web/src/pages/LogsPage.jsx:L68-L81` | **Low / Refactor** | Confirmed |
| **WEB-07** | SafeMode Countdown Desynchronization & Unprotected Action Dispatch | `web/src/components/SafeModeModal.jsx:L4-L10`, `web/src/App.jsx:L95-L113` | **Medium** | Confirmed |
| **WEB-08** | Hardcoded System Configuration Parameters Bypassing Server State | `web/src/pages/SettingsPage.jsx:L120-L134` | **Low / Refactor** | Confirmed |

---

## 4. Comprehensive Audit Findings (7-Field Defect Schema)

---

### WEB-01: Polling Stampede & Missing Exponential Backoff During Server Disconnect

- **Title & Category:** Network Polling Stampede & Missing Exponential Backoff | UI State & Network Reliability
- **Code Location:** `web/src/App.jsx:L51-L72`, `web/src/pages/LogsPage.jsx:L21-L25`
- **Severity:** High
- **Trigger Scenario & Root Cause Analysis:**
  In `App.jsx`, when an authenticated session is active, a `setInterval` timer triggers every 2,000 milliseconds (2 seconds) executing `refreshData()`:
  ```javascript
  const refreshData = async () => {
    try {
      const [stat, cfgs, rls] = await Promise.all([
        api.getTunnelStatus(),
        api.getConfigs(),
        api.getRoutingRules(),
      ]);
      setStatus(stat);
      setConfigs(cfgs || []);
      setRoutingRules(rls || []);
    } catch (err) {
      console.error('Error refreshing state:', err);
    }
  };

  refreshData();
  const interval = setInterval(refreshData, 2000);
  return () => clearInterval(interval);
  ```
  This creates three critical operational failure modes:
  1. **Fixed Timer Pileup (No In-Flight Check):** `setInterval(..., 2000)` fires every 2,000ms regardless of whether the previous `Promise.all` invocation resolved. If network latency spikes to 2.5 seconds (common when host routing rules are being applied or flushed by the backend), subsequent ticks spawn overlapping HTTP requests, queueing multiple pending requests in the browser socket pool.
  2. **No Exponential Backoff on Disconnection / Restart:** If the V2Raynix backend service restarts or the network link drops, `refreshData` catches the network error, logs to console, and immediately repeats 2 seconds later. Every 2 seconds, 3 HTTP requests (`/api/tunnel/status`, `/api/configs`, `/api/routing/rules`) are spammed at the offline server indefinitely.
  3. **Multiplied by LogsPage Poller:** If the user is on the "Logs" tab, `LogsPage.jsx` concurrently executes its own `setInterval(fetchLogs, 3000)`, adding another unthrottled request every 3 seconds.
  
  During a daemon restart or crash, a single open browser tab bombards the host with ~100 failed HTTP connection attempts every minute, consuming server sockets and complicating graceful service recovery.
- **Proof of Concept / Verification Method:**
  1. Open the V2Raynix Web UI in Chrome or Firefox with Developer Tools Network tab open.
  2. Stop the backend server process (`pkill v2raynix` or close daemon).
  3. Observe the Network tab: every 2,000ms, three red failed requests appear continuously with 0ms delay extension, continuing unabated for minutes without backoff.
- **Recommended Architectural Fix:**
  Replace naive `setInterval` with a recursive `setTimeout` loop incorporating exponential backoff and jitter upon consecutive failures:
  ```javascript
  useEffect(() => {
    if (!user) return;
    let timerId = null;
    let isCancelled = false;
    let failureCount = 0;

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
        failureCount = 0;
      } catch (err) {
        if (isCancelled) return;
        failureCount++;
      } finally {
        if (!isCancelled) {
          // Normal delay 2s; on error, exponential backoff up to 30s with jitter
          const delay = failureCount === 0 
            ? 2000 
            : Math.min(2000 * Math.pow(1.5, failureCount), 30000) + Math.random() * 500;
          timerId = setTimeout(poll, delay);
        }
      }
    };

    poll();
    return () => {
      isCancelled = true;
      if (timerId) clearTimeout(timerId);
    };
  }, [user]);
  ```
- **Existing Strengths & Robustness:**
  `Promise.all` bundles the three queries into a single concurrency group rather than cascading sequential `await` statements, minimizing idle time when the backend is responsive.

---

### WEB-02: Total Application Crash via Missing React Error Boundaries

- **Title & Category:** Complete UI Crash via Missing React Error Boundaries | Frontend Resilience & Fault Tolerance
- **Code Location:** `web/src/main.jsx:L6-L10`, `web/src/App.jsx:L184-L249`, `web/src/pages/ConfigsPage.jsx:L13-L17`
- **Severity:** High
- **Trigger Scenario & Root Cause Analysis:**
  In React 18, unhandled exceptions during rendering or in component lifecycle methods unmount the entire component tree below the nearest Error Boundary.
  
  In `web/src/main.jsx`:
  ```javascript
  ReactDOM.createRoot(document.getElementById('root')).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>
  );
  ```
  There is no React `ErrorBoundary` class or hook wrapping `<App />` or individual tab pages.
  
  In `web/src/pages/ConfigsPage.jsx:L13-L17`:
  ```javascript
  const filteredConfigs = configs.filter(c => 
    c.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    c.server.toLowerCase().includes(searchTerm.toLowerCase()) ||
    c.protocol.toLowerCase().includes(searchTerm.toLowerCase())
  );
  ```
  If a subscription or custom configuration imported into the store possesses an empty, null, or undefined `name`, `server`, or `protocol` attribute (e.g. from an incomplete raw JSON import or unexpected backend schema mutation), executing `.toLowerCase()` throws:
  `TypeError: Cannot read properties of undefined (reading 'toLowerCase')`.
  
  Because no Error Boundary exists, this error propagates all the way to `ReactDOM.createRoot`, unmounting the entire V2Raynix dashboard and leaving the user with a completely blank screen and no recovery option other than clearing browser storage.
- **Proof of Concept / Verification Method:**
  1. In the database or via API, introduce a config item with `{ id: "test-bad", server: "1.1.1.1", protocol: "vless", name: undefined }`.
  2. Navigate to the "Configs" tab or type into the search box.
  3. Observe the browser console: `Uncaught TypeError: Cannot read properties of undefined (reading 'toLowerCase')`.
  4. The dashboard DOM vanishes entirely; the user is locked out with a blank screen.
- **Recommended Architectural Fix:**
  1. Implement a top-level `ErrorBoundary` component in `src/components/ErrorBoundary.jsx`:
     ```javascript
     import React from 'react';

     export class ErrorBoundary extends React.Component {
       state = { hasError: false, error: null };

       static getDerivedStateFromError(error) {
         return { hasError: true, error };
       }

       componentDidCatch(error, errorInfo) {
         console.error('ErrorBoundary caught:', error, errorInfo);
       }

       render() {
         if (this.state.hasError) {
           return (
             <div className="glass-card" style={{ maxWidth: 500, margin: '4rem auto', textAlign: 'center', padding: '2rem' }}>
               <h3 style={{ color: '#fb7185', marginBottom: '1rem' }}>Dashboard Encountered an Error</h3>
               <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginBottom: '1.5rem' }}>
                 {this.state.error?.message || 'An unexpected rendering error occurred.'}
               </p>
               <button className="btn btn-primary" onClick={() => window.location.reload()}>
                 Reload Dashboard
               </button>
             </div>
           );
         }
         return this.props.children;
       }
     }
     ```
  2. Wrap `<App />` in `main.jsx` and each tab page in `App.jsx` with `<ErrorBoundary>`.
  3. Harden property access in `ConfigsPage.jsx` using optional chaining and nullish fallbacks:
     ```javascript
     const filteredConfigs = (configs || []).filter(c => 
       (c.name || '').toLowerCase().includes(searchTerm.toLowerCase()) ||
       (c.server || '').toLowerCase().includes(searchTerm.toLowerCase()) ||
       (c.protocol || '').toLowerCase().includes(searchTerm.toLowerCase())
     );
     ```
- **Existing Strengths & Robustness:**
  `App.jsx:L62-L63` handles nullish arrays from API responses with `setConfigs(cfgs || [])` and `setRoutingRules(rls || [])`, avoiding crashes when endpoints return empty payloads.

---

### WEB-03: Insecure Administrative JWT Storage in `localStorage` & XSS Exposure Surface

- **Title & Category:** Insecure Administrative JWT Storage & XSS Vulnerability Surface | Authentication & Web Security
- **Code Location:** `web/src/services/api.js:L3-L20`, `web/src/pages/LoginPage.jsx:L18-L21`
- **Severity:** High
- **Trigger Scenario & Root Cause Analysis:**
  In `web/src/services/api.js`:
  ```javascript
  export function getToken() {
    return localStorage.getItem('v2raynix_token');
  }

  export function setToken(token) {
    if (token) {
      localStorage.setItem('v2raynix_token', token);
    } else {
      localStorage.removeItem('v2raynix_token');
    }
  }
  ```
  Administrative authentication tokens generated upon login are written directly to `window.localStorage`.
  
  In a web application controlling system-level networking and daemon execution:
  1. **Persistence Across Sessions:** `localStorage` persists indefinitely across browser restarts until explicitly cleared.
  2. **No `HttpOnly` or Security Protection:** `localStorage` is accessible to any JavaScript running within the page context (`document.domain`). If an XSS vulnerability exists anywhere in the web app or in any loaded third-party dependency, an attacker can execute `localStorage.getItem('v2raynix_token')` and exfiltrate the full administrator JWT.
  3. **Absence of Content Security Policy (CSP):** Inspection of `web/index.html` and `internal/api/router.go` confirms there is no `Content-Security-Policy` header or meta tag defined. External scripts or data URIs can be evaluated without browser-level restriction.
  4. **Broad CORS Policy:** In `internal/api/router.go:L40`, the backend sets `Access-Control-Allow-Origin: *`. Combined with `localStorage` token management, this increases susceptibility to cross-origin abuse if the administrative port is accessible.
- **Proof of Concept / Verification Method:**
  1. Log into the V2Raynix dashboard.
  2. Open the browser Developer Tools console and run:
     `console.log(localStorage.getItem('v2raynix_token'))`
  3. The raw, unsigned administrative JWT token is printed immediately. Any script running in this origin has full read access to issue authenticated REST requests to `/api/tunnel/connect`, `/api/configs`, or `/api/auth/password`.
- **Recommended Architectural Fix:**
  1. Migrate session authentication from client-accessible `localStorage` to server-set `HttpOnly`, `SameSite=Strict`, `Secure` session cookies:
     ```http
     Set-Cookie: v2raynix_session=<jwt_token>; Path=/; HttpOnly; SameSite=Strict; Secure
     ```
  2. When using `HttpOnly` cookies, JavaScript cannot access or exfiltrate the token; the browser automatically attaches the cookie to `/api/*` requests.
  3. If maintaining token-based authentication via headers, store the token only in memory (within React state/context) and implement an `HttpOnly` refresh token mechanism.
  4. Add a strict `Content-Security-Policy` in `index.html` or in Go's `ServeHTTP` middleware:
     ```http
     Content-Security-Policy: default-src 'self'; style-src 'self' 'unsafe-inline' fonts.googleapis.com; font-src 'self' fonts.gstatic.com; connect-src 'self';
     ```
- **Existing Strengths & Robustness:**
  `services/api.js:L39-L45` correctly monitors for `401 Unauthorized` responses and cleans up the stored token via `setToken(null)`, terminating the local session if the backend revokes the token.

---

### WEB-04: Race Conditions & Duplicate Tunnel Transitions in Master Toggle

- **Title & Category:** Master Toggle Race Condition & Duplicate In-Flight Requests | Concurrency & State Synchronization
- **Code Location:** `web/src/App.jsx:L80-L92`, `web/src/pages/DashboardPage.jsx:L23-L27`
- **Severity:** Medium
- **Trigger Scenario & Root Cause Analysis:**
  In `DashboardPage.jsx:L23-L27`:
  ```jsx
  <button 
    className={`master-btn ${isConnected ? 'connected' : ''} ${isConnecting ? 'connecting' : ''}`}
    onClick={onToggleTunnel}
    disabled={isConnecting}
  >
  ```
  The button disablement evaluates `disabled={isConnecting}`, where `isConnecting = status?.state === 'connecting'`.
  
  In `App.jsx:L80-L92`:
  ```javascript
  const handleToggleTunnel = async () => {
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
    }
  };
  ```
  Notice that `handleToggleTunnel` does not maintain or set an immediate local `isSubmitting` / `isToggling` state:
  1. When the user clicks the Master Toggle button, `status?.state` is still `'disconnected'` (or `'connected'`).
  2. The async request `api.connectTunnel()` is dispatched over HTTP. Because the backend supervisor must spawn `xray`, spawn `tun2socks`, and run Linux routing commands, this call takes between 500ms and 2,500ms to resolve.
  3. Throughout this entire window, `disabled` is FALSE.
  4. If the user double-clicks or rapidly clicks the toggle, multiple concurrent `POST /api/tunnel/connect` requests are dispatched in parallel.
  5. On the backend, this can trigger race hazards in `Supervisor.Start()`, where two goroutines attempt to configure `tun0` and bind sockets simultaneously, potentially leading to port collisions or inconsistent supervisor states.
- **Proof of Concept / Verification Method:**
  1. On the Dashboard page with the tunnel disconnected, rapidly double-click the large Master Power button.
  2. Inspect the Network tab: two simultaneous `POST /api/tunnel/connect` requests are initiated concurrently.
  3. The first request starts the supervisor; the second request returns an error or conflicts with the in-progress startup.
- **Recommended Architectural Fix:**
  Introduce an immediate local `toggling` state lock in `App.jsx` or `DashboardPage.jsx` that disables the button immediately before awaiting the API call:
  ```javascript
  const [toggling, setToggling] = useState(false);

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
  And update `DashboardPage.jsx` to disable on `disabled={isConnecting || toggling}`.
- **Existing Strengths & Robustness:**
  `DashboardPage.jsx` visualizes the connecting state with animation classes (`connecting`) and provides clear textual feedback (`STARTING...`, `CONNECTED`, `DISCONNECTED`).

---

### WEB-05: Asynchronous Memory Leaks & Dangling State Updates on Component Unmount

- **Title & Category:** Asynchronous Memory Leaks & Dangling State Updates on Unmount | Component Lifecycle
- **Code Location:** `web/src/pages/LogsPage.jsx:L9-L25`, `web/src/components/ConfigCard.jsx:L8-L13`, `web/src/services/api.js:L7-L11`
- **Severity:** Medium
- **Trigger Scenario & Root Cause Analysis:**
  Three distinct unmount lifecycle defects exist in the frontend:
  1. **Unaborted Fetch in `LogsPage.jsx`:**
     ```javascript
     const fetchLogs = async () => {
       setLoading(true);
       try {
         const data = await api.getLogs();
         setLogs(data || []);
       } catch (err) {
         console.error(err);
       } finally {
         setLoading(false);
       }
     };

     useEffect(() => {
       fetchLogs();
       const interval = setInterval(fetchLogs, 3000);
       return () => clearInterval(interval);
     }, []);
     ```
     `clearInterval(interval)` clears the timer, but any pending `api.getLogs()` fetch request remains in flight. If the user navigates away from the Logs tab (e.g. clicks "Dashboard"), the fetch finishes after unmount and calls `setLogs()` and `setLoading(false)` on an unmounted component, causing React memory leak warnings and retaining detached DOM references.
  2. **Dangling Timeout in `ConfigCard.jsx:L8-L13`:**
     ```javascript
     const copyToClipboard = (e) => {
       e.stopPropagation();
       navigator.clipboard.writeText(config.rawUrl);
       setCopied(true);
       setTimeout(() => setCopied(false), 2000);
     };
     ```
     If the user copies a link and immediately deletes the config card or switches tabs within 2 seconds, the anonymous `setTimeout` callback executes `setCopied(false)` on the unmounted `ConfigCard`.
  3. **Module Singleton Callback in `services/api.js:L7-L11`:**
     `let onUnauthorizedCallback = null;` holds a global closure to `setUser(null)` without an unsubscribe function, leading to potential memory leaks during hot module reloading or test environments.
- **Proof of Concept / Verification Method:**
  1. Open the Logs tab in Chrome with React DevTools active.
  2. Simulate a slow 3G network connection in DevTools.
  3. Click "Refresh" or wait for a poll to begin, then immediately switch to the "Settings" tab.
  4. Check the browser console: React emits a warning: `Can't perform a React state update on an unmounted component`.
- **Recommended Architectural Fix:**
  1. Utilize `AbortController` in `fetchLogs` and cancel pending requests in the `useEffect` cleanup handler:
     ```javascript
     useEffect(() => {
       const controller = new AbortController();
       let isMounted = true;

       const fetchLogs = async () => {
         setLoading(true);
         try {
           const res = await fetch('/api/system/logs', {
             headers: { Authorization: `Bearer ${getToken()}` },
             signal: controller.signal,
           });
           const data = await res.json();
           if (isMounted) setLogs(data || []);
         } catch (err) {
           if (err.name !== 'AbortError' && isMounted) console.error(err);
         } finally {
           if (isMounted) setLoading(false);
         }
       };

       fetchLogs();
       const interval = setInterval(fetchLogs, 3000);
       return () => {
         isMounted = false;
         clearInterval(interval);
         controller.abort();
       };
     }, []);
     ```
  2. Store timer references in `ConfigCard` and clear them in a cleanup effect.
- **Existing Strengths & Robustness:**
  `App.jsx:L71` and `LogsPage.jsx:L24` consistently provide cleanup return functions for `setInterval` instances, preventing runaway interval accumulation across re-renders.

---

### WEB-06: Unbounded DOM Growth & Array Index Keying in Real-Time Log Stream

- **Title & Category:** Unbounded DOM Growth & Index-Based Keying in Log Streaming | Performance & DOM Optimization
- **Code Location:** `web/src/pages/LogsPage.jsx:L68-L81`
- **Severity:** Low / Refactor
- **Trigger Scenario & Root Cause Analysis:**
  In `web/src/pages/LogsPage.jsx:L68-L81`:
  ```jsx
  {logs.map((log, idx) => (
    <div key={idx} style={{ display: 'flex', gap: '0.75rem', marginBottom: '0.25rem' }}>
      <span style={{ color: 'var(--text-muted)', flexShrink: 0 }}>
        {log.timestamp?.substring(11, 19) || '00:00:00'}
      </span>
      <span style={{ color: getLevelColor(log.level), fontWeight: 600, textTransform: 'uppercase', width: 55, flexShrink: 0 }}>
        [{log.level}]
      </span>
      <span style={{ color: 'var(--text-primary)', wordBreak: 'break-all' }}>
        {log.message}
      </span>
    </div>
  ))}
  ```
  Two performance issues arise here:
  1. **Anti-Pattern `key={idx}` on Dynamic Collections:**
     Because logs roll in continuously, using the array index `idx` as the React `key` breaks React's virtual DOM reconciliation algorithm. When new logs are prepended or pruned, React fails to match identical DOM nodes, forcing complete re-rendering and layout recalculations for every log row every 3 seconds.
  2. **Unbounded DOM Node Count:**
     The backend `Supervisor.GetLogs()` returns up to 100 entries per request. If this is increased or logs accumulate in the client, rendering hundreds of complex flexbox `<div>` elements without windowing or virtualization causes scrolling stutter and high memory usage in long-running browser tabs.
- **Proof of Concept / Verification Method:**
  1. Open Chrome DevTools Performance tab.
  2. Record a 15-second trace while viewing the Logs tab.
  3. Inspect the timeline: every 3,000ms, a forced layout and full DOM recalculation occurs for all log elements due to `key={idx}` reconciliation mismatch.
- **Recommended Architectural Fix:**
  1. Use a stable unique composite key combining timestamp and log index/hash: `key={`${log.timestamp}-${idx}`}` or a backend-assigned log ID.
  2. Limit the rendered log slice to the most recent 150-200 lines:
     ```javascript
     const displayedLogs = logs.slice(-200);
     ```
  3. For large log streams, integrate lightweight virtualized list rendering (e.g. `@tanstack/react-virtual` or CSS `content-visibility: auto`).
- **Existing Strengths & Robustness:**
  `wordBreak: 'break-all'` ensures long URLs or base64 payloads in log messages do not overflow horizontally or break dashboard card layouts.

---

### WEB-07: SafeMode Countdown Desynchronization & Unprotected Action Dispatch

- **Title & Category:** SafeMode Countdown Desynchronization & Duplicate Dispatch Hazard | State Synchronization & Safety UX
- **Code Location:** `web/src/components/SafeModeModal.jsx:L4-L10`, `web/src/App.jsx:L95-L113`
- **Severity:** Medium
- **Trigger Scenario & Root Cause Analysis:**
  The SafeMode anti-lockout countdown modal serves as the critical safeguard against SSH lockout when routing changes are applied.
  
  In `SafeModeModal.jsx:L4-L10`:
  ```javascript
  export default function SafeModeModal({ remainingSeconds, onConfirm, onRollback }) {
    if (remainingSeconds <= 0) return null;

    const minutes = Math.floor(remainingSeconds / 60);
    const seconds = remainingSeconds % 60;
    const timeFormatted = `${minutes}:${seconds < 10 ? '0' : ''}${seconds}`;
  ```
  And in `App.jsx:L51-L72`, status is refreshed only via the 2,000ms polling interval.
  
  This creates two operational problems:
  1. **Choppy / Desynchronized Countdown:** Because `remainingSeconds` only updates when the backend status poll completes (every 2 seconds), the countdown timer ticks down in irregular 2-second jumps (`120` -> `118` -> `116`) rather than a smooth 1-second cadence. If an HTTP poll experiences latency, the timer freezes momentarily.
  2. **Unprotected Action Dispatch (No Loading State):**
     In `SafeModeModal.jsx:L55-L71`:
     ```jsx
     <button className="btn btn-danger" onClick={onRollback}>
       <RotateCcw size={16} /> Revert Now
     </button>
     <button className="btn btn-success" onClick={onConfirm}>
       <CheckCircle2 size={16} /> Confirm & Keep
     </button>
     ```
     Neither button has a disabled state or loading spinner upon click. If an operator clicks "Confirm & Keep", `handleConfirmSafeMode` in `App.jsx` issues `await api.confirmSafeMode()`. While that request travels across the network, the modal remains fully interactive. An anxious user double-clicking dispatches duplicate confirm/rollback requests, which can trigger errors or race conditions in the backend `SafeModeController`.
- **Proof of Concept / Verification Method:**
  1. Trigger SafeMode by connecting the tunnel.
  2. Observe the countdown timer: notice the number counts down in 2-second jumps rather than every second.
  3. Click "Confirm & Keep" multiple times rapidly: multiple HTTP POST requests to `/api/tunnel/safe-mode/confirm` are dispatched before the modal unmounts.
- **Recommended Architectural Fix:**
  1. Implement a local 1-second ticking state inside `SafeModeModal` that smoothly interpolates between backend syncs:
     ```javascript
     const [localRemaining, setLocalRemaining] = useState(remainingSeconds);

     useEffect(() => {
       setLocalRemaining(remainingSeconds);
     }, [remainingSeconds]);

     useEffect(() => {
       if (localRemaining <= 0) return;
       const timer = setInterval(() => {
         setLocalRemaining(prev => Math.max(0, prev - 1));
       }, 1000);
       return () => clearInterval(timer);
     }, []);
     ```
  2. Add an `isSubmitting` state to `SafeModeModal` to disable both buttons and display a spinner as soon as either action is triggered.
- **Existing Strengths & Robustness:**
  The modal features high-visibility warning iconography, clear explanatory text detailing the auto-reversion behavior, and a color-coded countdown indicator (`#f59e0b`).

---

### WEB-08: Hardcoded System Configuration Parameters Bypassing Server State

- **Title & Category:** Hardcoded System Configuration Parameters in Settings View | Architectural Maintainability
- **Code Location:** `web/src/pages/SettingsPage.jsx:L120-L134`
- **Severity:** Low / Refactor
- **Trigger Scenario & Root Cause Analysis:**
  In `web/src/pages/SettingsPage.jsx:L120-L134`:
  ```jsx
  <div style={{ display: 'flex', justifyContent: 'space-between', ... }}>
    <span>Web UI Port:</span>
    <strong>2080</strong>
  </div>
  <div style={{ display: 'flex', justifyContent: 'space-between', ... }}>
    <span>Local SOCKS5 Port:</span>
    <strong>10808</strong>
  </div>
  <div style={{ display: 'flex', justifyContent: 'space-between', ... }}>
    <span>Safe Mode Timeout:</span>
    <strong>120 seconds</strong>
  </div>
  <div style={{ display: 'flex', justifyContent: 'space-between', ... }}>
    <span>Virtual TUN Interface:</span>
    <strong>tun0</strong>
  </div>
  ```
  These operational parameters are hardcoded statically into the JSX markup rather than queried from the backend server.
  
  If V2Raynix is launched with custom CLI flags (such as `--port 8080`, `--socks-port 10809`, `--tun-interface tun1`, or `--safe-timeout 60s`), the Web UI displays inaccurate system information to the operator, creating confusion during diagnostics and network troubleshooting.
- **Proof of Concept / Verification Method:**
  1. Launch V2Raynix with custom CLI parameters: `v2raynix --port 8080 --socks-port 9999`.
  2. Access the Web UI and navigate to the "Settings" tab.
  3. Inspect the "System Configuration" card: it still displays "Web UI Port: 2080" and "Local SOCKS5 Port: 10808".
- **Recommended Architectural Fix:**
  1. Expose a `/api/system/settings` or `/api/system/info` GET endpoint in the Go backend returning active runtime configurations.
  2. In `SettingsPage.jsx`, fetch and display dynamic values with fallback defaults:
     ```jsx
     <span>Web UI Port:</span>
     <strong>{systemInfo?.webPort || window.location.port || '2080'}</strong>
     ```
- **Existing Strengths & Robustness:**
  The Settings page implements solid client-side password validation, confirming that `newPassword === confirmPassword` before dispatching the update request to the server.

---

## 5. Security Posture & Architecture Review Summary

### Overall Subsystem Assessment:
The V2Raynix Web Frontend provides a responsive, intuitive, and clean user experience with thoughtful visual feedback (latency badges, animated status indicators, modal confirmation dialogs). However, from a production stability and security standpoint, it currently relies on naive frontend assumptions:
1. **Resilience to Backend Fluctuations:** The frontend assumes the backend is always reachable. When the backend undergoes network-induced pauses, the lack of backoff, in-flight mutexing, and error boundaries creates client-side errors and request flooding.
2. **Credential Hygiene:** Storing root-level management JWT tokens in `localStorage` presents an unnecessary security risk for a host network management tool. Migrating to `HttpOnly` cookies or memory-based tokens will align the project with OWASP security best practices.
3. **State Mutexing:** Critical lifecycle operations (connecting the tunnel, toggling SafeMode) require immediate local client locking to prevent operator double-clicks from causing backend race conditions.

Addressing these 8 findings will transform the V2Raynix web console into an enterprise-grade, resilient management interface.
