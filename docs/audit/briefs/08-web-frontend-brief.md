# Domain Briefing 08: Web Frontend & UI State Subsystem

> **Subagent Role:** Principal Frontend Architecture & Web Security Auditor  
> **Audited Files:** `web/src/App.jsx`, `web/src/services/api.js`, `web/src/pages/*`, `web/src/components/*`  
> **Output Report File:** `docs/audit/reports/08-web-frontend-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Deep Knowledge Transfer & Architecture

The `web` package provides the responsive, real-time web management dashboard for V2Raynix built with React 18 and Vite, styled with custom Vanilla CSS utilizing dark theme glassmorphism.

### Key Architectural Concepts:
1. **Application Shell & Navigation (`App.jsx`):**
   - Manages top-level state: `token`, `currentUser`, `activeTab` (`dashboard`, `configs`, `routing`, `logs`, `settings`), `status`, `configs`.
   - Polls `/api/tunnel/status` periodically to update connection state, uptime, and SafeMode countdown.
2. **API Communication Layer (`services/api.js`):**
   - Centralizes `fetch` calls, adds Bearer token header, handles 401 Unauthorized token revocation.
   - Saves JWT in `localStorage` under `v2raynix_token`.
3. **Core Pages:**
   - `DashboardPage.jsx`: Master toggle button for tunnel, stats grid, uptime display, SafeMode banner.
   - `ConfigsPage.jsx`: Import modal, node list, protocol badges, latency tester trigger, active config selector.
   - `RoutingPage.jsx`: Rule creation, preset filters (Direct, Proxy, Block), target domains/IPs.
   - `LogsPage.jsx`: Real-time system log stream, severity filter.
   - `SettingsPage.jsx`: Password updates, safe mode duration setting, web port info.

---

## 2. Concerns, Pitfalls & Critical Warnings

When auditing this domain, pay hyper-vigilant attention to these failure modes:
1. **Polling Stampede & Missing Backoff During Server Disconnect:**
   When the tunnel connects or disconnects, or if the server goes down, how does the frontend poll? If `setInterval` polls every 1-2 seconds without exponential backoff upon network failure, the frontend spams the server with failed network requests, preventing recovery.
2. **Missing React Error Boundaries (Blank Screen Crashes):**
   If an unexpected response format is received from an API endpoint (e.g. `null` instead of an array for configs or rules), does an unhandled JavaScript error crash the entire React component tree to a blank white screen?
3. **Token Storage & XSS Vulnerability:**
   Storing administrative JWT tokens in `localStorage` makes them permanently accessible to any JavaScript running in the origin. If any user-supplied content (such as config names, node remarks, or log messages) is rendered unsafely, tokens can be stolen.
4. **Race Conditions in Master Toggle & State Inconsistency:**
   If a user rapidly clicks the Master Connect/Disconnect button before the previous HTTP response resolves, can overlapping `connect` and `disconnect` API calls trigger invalid state transitions in the Supervisor?
5. **Memory Leaks on Component Unmount:**
   Are all timers, event listeners, and pending fetch requests properly aborted or cleared in `useEffect` cleanup functions?

---

## 3. Lead Architect Directives & Step-by-Step Instructions

### Step 1: Mandatory Architectural Graph Exploration (`graphify`)
Before reading line-by-line, run:
```powershell
python -m graphify.cli explain web
# or
python -m graphify.cli query "how does web frontend interact with api endpoints"
```
Check `graphify-out/wiki/` for frontend service structures.

### Step 2: Initialize Report Scaffold
Immediately create `docs/audit/reports/08-web-frontend-report.md` with standard sections.

### Step 3: Step-by-Step Systematic Code Inspection & Incremental Updates
Audit each area sequentially and update the report immediately upon discovering defects:
1. **API Client & Authentication Lifecycle (`api.js:L1-L83`):** Audit token persistence, error parsing, and unauthorized callback loops.
2. **State Management & Polling in Shell (`App.jsx`):** Audit `useEffect` intervals, cleanup handlers, and state synchronization.
3. **Master Switch & SafeMode UX (`DashboardPage.jsx`):** Audit button disabling, transition states, and countdown timer accuracy.
4. **Config & Routing Management (`ConfigsPage.jsx`, `RoutingPage.jsx`):** Audit form validation, batch link pasting, and error banners.
5. **Logs Stream Rendering (`LogsPage.jsx`):** Audit DOM rendering performance and memory growth during continuous log updates.

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
