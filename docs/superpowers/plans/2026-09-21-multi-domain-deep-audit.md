# Multi-Domain Deep Code Audit & Gap Analysis Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Conduct an exhaustive, systematic, read-only architectural audit and gap analysis across all 8 subsystems of V2Raynix without modifying any source code. Each domain will have an expert knowledge-transfer briefing document, be independently audited by an isolated subagent using graphify and incremental reporting, and produce a standardized, evidence-backed defect ledger.

**Architecture:** The Lead Architect generates 8 dedicated domain briefing specifications (`docs/audit/briefs/*.md`) with deep knowledge transfer, risks/warnings, and step-by-step audit directives. The Lead Architect then dispatches 8 dedicated subagents via `$null | agy --add-dir ...`. Each subagent uses `graphify` (`graphify query` / `graphify explain` or `graphify-out/wiki/`) to understand package relationships, progressively updates its designated report (`docs/audit/reports/*.md`) at each step, and follows a strict 7-field defect schema. Finally, the Lead Architect compiles the master executive synthesis (`docs/audit/FINAL_COMPREHENSIVE_AUDIT_REPORT.md`).

**Tech Stack:** Go 1.22+, React 18, Vite, Linux Networking (iptables/iproute2/tun2socks/xray), Antigravity CLI (`agy`), Graphify Knowledge Graph (`graphify-out/`).

**Spec:** Multi-Domain Comprehensive Code Audit Protocol & Subagent Operational Directives.

---

## Global Constraints

- **STRICTLY READ-ONLY:** No source code modifications (`.go`, `.jsx`, `.css`, `.json`) will be performed in this phase. Code is only inspected and audited.
- **DOMAIN ISOLATION:** Each subagent is strictly forbidden from drifting outside its assigned domain files.
- **GRAPHIFY INTEGRATION:** Each subagent must utilize the existing graphify knowledge graph (`graphify query` / `graphify explain` or `graphify-out/wiki/`) to trace inbound and outbound dependencies before auditing lines.
- **INCREMENTAL WRITING:** Each subagent must create its designated report file (`docs/audit/reports/<domain>-report.md`) immediately with scaffolding, and progressively append/update findings step-by-step as each inspection area is analyzed, rather than dumping all findings at the end.
- **STANDARDIZED DEFECT SCHEMA:** Every reported issue must contain:
  1. Title & Category (Security / Concurrency & Race / Resource Leaks / Network & Kernel / Logic & Edge-Cases)
  2. Exact Code Location (file path & line numbers `Lxx-Lyy`)
  3. Severity (`Critical`, `High`, `Medium`, `Low / Refactor`)
  4. Trigger Scenario & Root Cause Analysis
  5. Proof of Concept / Verification Method (test scenario, script, or CLI command)
  6. Recommended Architectural Fix
  7. Existing Strengths & Robustness in that area

---

## Review Focus

1. **Scope Leakage:** Subagent modifies code or wanders outside its domain files.
   *Mitigation:* Prompt specifies `--effort medium`, explicitly forbids file edits outside `docs/audit/reports/`, and bounds target files.
2. **Graphify Omission:** Subagent relies solely on shallow grep without understanding the dependency graph.
   *Mitigation:* Task prompt requires running `graphify explain <subsystem>` or `graphify query` as Step 1.
3. **Report Batch Dump:** Subagent dumps everything at the very end or fails mid-run with no saved progress.
   *Mitigation:* Briefing instructs subagent to write report skeleton first, then append findings iteratively after each step.
4. **Windows Stdin Hang in `agy`:** Background or foreground execution blocks indefinitely if stdin is left open.
   *Mitigation:* All invocations must use `$null | agy --add-dir "c:\MaadZone\Github Projects\V2Raynix" ...`.
5. **Vague or Unsubstantiated Findings:** LLM hallucinated issues with no line references or real triggers.
   *Mitigation:* Enforce strict 7-field schema; any finding lacking line ranges (`Lxx-Lyy`) is rejected.

---

## The 8 Architectural Domains & Target Files

| # | Domain Name | Target Code Files | Brief Path | Report Path |
| :- | :--- | :--- | :--- | :--- |
| **1** | **Network & Policy Routing** | `internal/network/routing.go`, `safemode.go`, `routing_test.go` | `docs/audit/briefs/01-network-routing-brief.md` | `docs/audit/reports/01-network-routing-report.md` |
| **2** | **Core Process Supervisor** | `internal/core/supervisor.go`, `supervisor_test.go` | `docs/audit/briefs/02-core-supervisor-brief.md` | `docs/audit/reports/02-core-supervisor-report.md` |
| **3** | **Config Management & Xray** | `internal/configmgr/parser.go`, `generator.go`, `*_test.go` | `docs/audit/briefs/03-configmgr-brief.md` | `docs/audit/reports/03-configmgr-report.md` |
| **4** | **Pinger & Diagnostics** | `internal/pinger/pinger.go`, `pinger_test.go` | `docs/audit/briefs/04-pinger-brief.md` | `docs/audit/reports/04-pinger-report.md` |
| **5** | **Auth & Session Security** | `internal/auth/auth.go`, `auth_test.go` | `docs/audit/briefs/05-auth-security-brief.md` | `docs/audit/reports/05-auth-security-report.md` |
| **6** | **Atomic Store & Persistence** | `internal/store/store.go`, `store_test.go` | `docs/audit/briefs/06-store-brief.md` | `docs/audit/reports/06-store-report.md` |
| **7** | **REST API & Router** | `internal/api/router.go`, `handlers.go`, `router_test.go` | `docs/audit/briefs/07-api-brief.md` | `docs/audit/reports/07-api-report.md` |
| **8** | **Web Frontend & UI State** | `web/src/App.jsx`, `web/src/pages/*`, `web/src/components/*` | `docs/audit/briefs/08-web-frontend-brief.md` | `docs/audit/reports/08-web-frontend-report.md` |

---

### Task 1: Generate Domain Briefing Specs for Core Backend (Domains 1 to 4)

**Files:**
- Create: `docs/audit/briefs/01-network-routing-brief.md`
- Create: `docs/audit/briefs/02-core-supervisor-brief.md`
- Create: `docs/audit/briefs/03-configmgr-brief.md`
- Create: `docs/audit/briefs/04-pinger-brief.md`

- [ ] **Step 1: Write Domain 1 Brief (Network & Policy Routing)**
  Include complete knowledge transfer on `tun0`, `connmark 0x52`, `ip rule`, table 100, anti-lockout, concerns/risks, and step-by-step audit directives.
- [ ] **Step 2: Write Domain 2 Brief (Core Process Supervisor)**
  Include process supervision lifecycle for `xray` and `tun2socks`, signal handling, safe mode cancellation, zombie process prevention, and audit directives.
- [ ] **Step 3: Write Domain 3 Brief (Config Management & Xray)**
  Include parser specifics for VLESS/VMess/Trojan/SS, Xray streamSettings generator, post-quantum encryption handling, injection vulnerabilities, and audit directives.
- [ ] **Step 4: Write Domain 4 Brief (Pinger & Diagnostics)**
  Include concurrent worker pool, timeout handling, TCP ping edge cases, HTTP proxy pinging, and audit directives.
- [ ] **Step 5: Verify briefing documents exist and are populated**

---

### Task 2: Generate Domain Briefing Specs for Services & Frontend (Domains 5 to 8)

**Files:**
- Create: `docs/audit/briefs/05-auth-security-brief.md`
- Create: `docs/audit/briefs/06-store-brief.md`
- Create: `docs/audit/briefs/07-api-brief.md`
- Create: `docs/audit/briefs/08-web-frontend-brief.md`

- [ ] **Step 1: Write Domain 5 Brief (Auth & Session Security)**
  Include bcrypt hashing, JWT construction/validation, algorithm confusion risks, clock skew, and audit directives.
- [ ] **Step 2: Write Domain 6 Brief (Atomic Store & Persistence)**
  Include RWMutex locking, atomic tempfile write/rename, JSON serialization, corruption resilience, and audit directives.
- [ ] **Step 3: Write Domain 7 Brief (REST API & Router)**
  Include Go 1.22+ routing patterns, auth middleware, request validation, command injection in shell endpoints, error leaking, and audit directives.
- [ ] **Step 4: Write Domain 8 Brief (Web Frontend & UI State)**
  Include React 18 state management, infinite re-render hazards, token storage, API error handling, UX responsiveness, and audit directives.
- [ ] **Step 5: Commit all 8 briefing documents**
  `git add docs/audit/briefs/*.md && git commit -m "docs(audit): create 8 architectural briefing specifications"`

---

### Task 3: Dispatch Isolated Subagents for Backend Domains (Domains 1 to 4)

**Files:**
- Output: `docs/audit/reports/01-network-routing-report.md`
- Output: `docs/audit/reports/02-core-supervisor-report.md`
- Output: `docs/audit/reports/03-configmgr-report.md`
- Output: `docs/audit/reports/04-pinger-report.md`

- [ ] **Step 1: Dispatch Subagent 1 (Network & Policy Routing)**
  Command:
  `$null | agy --add-dir "c:\MaadZone\Github Projects\V2Raynix" -p "Read docs/audit/briefs/01-network-routing-brief.md. Run graphify explain network or graphify query network routing. Audit internal/network/routing.go and safemode.go step-by-step. Progressively write your findings to docs/audit/reports/01-network-routing-report.md following the 7-field schema. Do NOT edit any source code." --dangerously-skip-permissions --effort medium`
- [ ] **Step 2: Dispatch Subagent 2 (Core Process Supervisor)**
  Command:
  `$null | agy --add-dir "c:\MaadZone\Github Projects\V2Raynix" -p "Read docs/audit/briefs/02-core-supervisor-brief.md. Run graphify explain core supervisor. Audit internal/core/supervisor.go step-by-step. Progressively write your findings to docs/audit/reports/02-core-supervisor-report.md following the 7-field schema. Do NOT edit any source code." --dangerously-skip-permissions --effort medium`
- [ ] **Step 3: Dispatch Subagent 3 (Config Management & Xray)**
  Command:
  `$null | agy --add-dir "c:\MaadZone\Github Projects\V2Raynix" -p "Read docs/audit/briefs/03-configmgr-brief.md. Run graphify explain configmgr. Audit internal/configmgr/parser.go and generator.go step-by-step. Progressively write your findings to docs/audit/reports/03-configmgr-report.md following the 7-field schema. Do NOT edit any source code." --dangerously-skip-permissions --effort medium`
- [ ] **Step 4: Dispatch Subagent 4 (Pinger & Diagnostics)**
  Command:
  `$null | agy --add-dir "c:\MaadZone\Github Projects\V2Raynix" -p "Read docs/audit/briefs/04-pinger-brief.md. Run graphify explain pinger. Audit internal/pinger/pinger.go step-by-step. Progressively write your findings to docs/audit/reports/04-pinger-report.md following the 7-field schema. Do NOT edit any source code." --dangerously-skip-permissions --effort medium`
- [ ] **Step 5: Review & Validate Reports 1-4 Quality**
  Check that each report contains real line numbers, root cause, reproduction, and recommended fix.

---

### Task 4: Dispatch Isolated Subagents for Services & Frontend (Domains 5 to 8)

**Files:**
- Output: `docs/audit/reports/05-auth-security-report.md`
- Output: `docs/audit/reports/06-store-report.md`
- Output: `docs/audit/reports/07-api-report.md`
- Output: `docs/audit/reports/08-web-frontend-report.md`

- [ ] **Step 1: Dispatch Subagent 5 (Auth & Session Security)**
  Command:
  `$null | agy --add-dir "c:\MaadZone\Github Projects\V2Raynix" -p "Read docs/audit/briefs/05-auth-security-brief.md. Run graphify explain auth. Audit internal/auth/auth.go step-by-step. Progressively write your findings to docs/audit/reports/05-auth-security-report.md following the 7-field schema. Do NOT edit any source code." --dangerously-skip-permissions --effort medium`
- [ ] **Step 2: Dispatch Subagent 6 (Atomic Store & Persistence)**
  Command:
  `$null | agy --add-dir "c:\MaadZone\Github Projects\V2Raynix" -p "Read docs/audit/briefs/06-store-brief.md. Run graphify explain store. Audit internal/store/store.go step-by-step. Progressively write your findings to docs/audit/reports/06-store-report.md following the 7-field schema. Do NOT edit any source code." --dangerously-skip-permissions --effort medium`
- [ ] **Step 3: Dispatch Subagent 7 (REST API & Router)**
  Command:
  `$null | agy --add-dir "c:\MaadZone\Github Projects\V2Raynix" -p "Read docs/audit/briefs/07-api-brief.md. Run graphify explain api. Audit internal/api/router.go and handlers.go step-by-step. Progressively write your findings to docs/audit/reports/07-api-report.md following the 7-field schema. Do NOT edit any source code." --dangerously-skip-permissions --effort medium`
- [ ] **Step 4: Dispatch Subagent 8 (Web Frontend & UI State)**
  Command:
  `$null | agy --add-dir "c:\MaadZone\Github Projects\V2Raynix" -p "Read docs/audit/briefs/08-web-frontend-brief.md. Run graphify explain web. Audit web/src/App.jsx, pages, and components step-by-step. Progressively write your findings to docs/audit/reports/08-web-frontend-report.md following the 7-field schema. Do NOT edit any source code." --dangerously-skip-permissions --effort medium`
- [ ] **Step 5: Review & Validate Reports 5-8 Quality**
  Check that each report contains real line numbers, root cause, reproduction, and recommended fix.

---

### Task 5: Master Synthesis & Comprehensive Audit Ledger

**Files:**
- Create: `docs/audit/FINAL_COMPREHENSIVE_AUDIT_REPORT.md`

- [ ] **Step 1: Aggregate and cross-reference all 8 domain reports**
  Analyze inter-domain dependencies, deduplicate overlapping findings, and compile severity breakdown (Critical, High, Medium, Low).
- [ ] **Step 2: Formulate prioritized remediation roadmap**
  Group findings into actionable phases (Phase 1: Critical Security & System Stability; Phase 2: Functional Bugs; Phase 3: Performance & Refactoring).
- [ ] **Step 3: Commit all audit reports and final synthesis**
  `git add docs/audit/reports/*.md docs/audit/FINAL_COMPREHENSIVE_AUDIT_REPORT.md && git commit -m "docs(audit): finalize comprehensive 8-domain audit reports and master synthesis"`
