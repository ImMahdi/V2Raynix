# Domain Briefing 07: Web Frontend & UI State Management Subsystem

> **Subagent Role:** Principal Frontend Architecture & Web Security Auditor  
> **Audited Files:** `web/src/App.jsx`, `web/src/services/api.js`, `web/src/pages/*`, `web/src/components/*`  
> **Output Report File:** `docs/audit/reports/07-web-frontend-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Subsystem Architecture & Responsibilities
The `web` directory contains the modern React 18 single-page application (SPA):
- Real-time tunnel connection monitoring and master toggle.
- Configuration item listing, subscription import, QR code generation, and ping latency visualization.
- Routing rule configuration (Direct vs Proxy vs Block).
- Live log streaming with severity filtering.
- System settings, admin credential management, and core update modal/notifications.

---

## 2. Critical Audit Directives & Failure Modes to Investigate

Auditor must systematically examine:
1. **Network Disconnect Polling Stampede & Exponential Backoff:**
   - In `App.jsx` and `pages/`: when the backend goes offline or restarts, do polling intervals continue hammering the server without backoff?
2. **Missing React Error Boundaries & Blank Screen Crashes:**
   - If an unexpected API response (malformed JSON, undefined property in config/ping) occurs, does it crash the entire React DOM into a blank white screen?
3. **Session Token Storage & Cross-Site Scripting (XSS):**
   - Is JWT token stored in `localStorage`? How are user-supplied configuration names, proxy addresses, and log streams sanitized when rendered in the DOM?
4. **Master Connect/Disconnect Concurrency & Optimistic State Races:**
   - What happens if the user rapidly clicks the Master Connect/Disconnect switch while an in-flight API request is pending? Does it trigger race conditions or duplicate tunnel initialization?
5. **Memory Leaks on Component Lifecycle:**
   - Are `setInterval`, `setTimeout`, and AbortController event listeners properly cleaned up on component unmount?
6. **Core Update Notification & Modal Interactions:**
   - How does the UI handle core update triggering? Does it handle timeout gracefully if backend restarts mid-update?

---

## 3. Subagent Execution Instructions
1. Run `python -m graphify.cli explain web` or query graph to understand dependencies.
2. Progressively write findings to `docs/audit/reports/07-web-frontend-report.md`.
3. Every finding MUST follow the 7-field defect schema (ID, Code Location, Severity, Trigger/Root Cause, PoC, Recommended Fix, Strengths).
4. STRICTLY READ-ONLY: Do not edit any code files.
