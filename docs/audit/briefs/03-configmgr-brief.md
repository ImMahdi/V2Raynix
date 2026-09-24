# Domain Briefing 03: Config Management & Xray Generator Subsystem

> **Subagent Role:** Principal Protocol Parsing & Proxy Configuration Auditor  
> **Audited Files:** `internal/configmgr/parser.go`, `internal/configmgr/generator.go`, `internal/configmgr/*_test.go`  
> **Output Report File:** `docs/audit/reports/03-configmgr-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Subsystem Architecture & Responsibilities
The `internal/configmgr` package handles:
- Ingestion and decoding of subscription links and proxy URI schemes: VMess (`vmess://`), VLESS (`vless://`), Shadowsocks (`ss://`), Trojan (`trojan://`).
- Serialization into full-fledged, valid Xray-core JSON configuration structures (Inbounds, Outbounds, Routing, DNS).

---

## 2. Critical Audit Directives & Failure Modes to Investigate

Auditor must systematically examine:
1. **Base64 Decoding Flaws & Panic Vulnerabilities:**
   - In `parseVMess` and `parseShadowsocks`: does base64 decoding handle missing padding, standard vs URL-safe alphabets, or non-base64 characters safely without panic?
2. **Type Assertion Hazards in VMess JSON:**
   - Are `port`, `aid`, and other fields parsed with unchecked interface type assertions (`v.(string)` vs `v.(float64)`) that could trigger runtime panics on malformed input?
3. **JSON Injection via URI Metadata:**
   - Are SNI, path, host, or camouflage headers inserted directly into JSON structures without proper escaping or schema enforcement?
4. **VLESS Reality / Flow Validation:**
   - Are Reality public keys, shortIds, and `xtls-rprx-vision` flow parameters validated strictly to prevent generating invalid Xray configurations that crash the core on boot?
5. **Inbound SOCKS/HTTP Port Collisions:**
   - Are local inbound ports (e.g. SOCKS 10808, HTTP 10809) safely managed, isolated, and verified against system port conflicts?
6. **DNS Leak Prevention in Outbound Configuration:**
   - Does generated Xray config enforce clean DNS resolution and prevent loopback DNS traps?

---

## 3. Subagent Execution Instructions
1. Run `python -m graphify.cli explain configmgr` or query graph to understand dependencies.
2. Progressively write findings to `docs/audit/reports/03-configmgr-report.md`.
3. Every finding MUST follow the 7-field defect schema (ID, Code Location, Severity, Trigger/Root Cause, PoC, Recommended Fix, Strengths).
4. STRICTLY READ-ONLY: Do not edit any code files.
