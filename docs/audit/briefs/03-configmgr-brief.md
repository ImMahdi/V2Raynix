# Domain Briefing 03: Config Management & Xray Subsystem

> **Subagent Role:** Principal Protocol Parsing & Xray Config Generation Auditor  
> **Audited Files:** `internal/configmgr/parser.go`, `internal/configmgr/generator.go`, `internal/configmgr/*_test.go`  
> **Output Report File:** `docs/audit/reports/03-configmgr-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Deep Knowledge Transfer & Architecture

The `configmgr` package is responsible for ingesting, validating, decoding, and translating proxy subscription links into valid, hardened Xray-core JSON configurations.

### Key Architectural Concepts:
1. **Share Link Parsing (`parser.go`):**
   - Dispatches based on URL scheme:
     - `vless://[uuid]@[host]:[port]?[query]#[name]`
     - `vmess://[base64-json]`
     - `trojan://[password]@[host]:[port]?[query]#[name]`
     - `ss://[base64-encoded-creds]@[host]:[port]#[name]` or SIP002 URI scheme
     - Raw JSON payloads (`{...}`)
   - Extracts network transport (`tcp`, `ws`, `grpc`, `h2`, `splithttp`), TLS settings (`tls`, `reality`), and camouflage headers.
2. **Xray Configuration Generation (`generator.go`):**
   - Generates two standard local inbounds:
     - SOCKS5 inbound on `127.0.0.1:10808` with traffic sniffing (`destOverride: ["http", "tls", "quic"]`).
     - HTTP inbound on `127.0.0.1:10809`.
   - Generates outbounds:
     - Primary proxy outbound (`tag: "proxy"`) with exact `streamSettings` matching the protocol and security requirements.
     - Direct outbound (`tag: "direct"`).
     - Block outbound (`tag: "block"` / `blackhole`).
   - Translates domain/IP routing rules into Xray routing table format.

---

## 2. Concerns, Pitfalls & Critical Warnings

When auditing this domain, pay hyper-vigilant attention to these failure modes:
1. **JSON Injection via Camouflage / SNI / Path Parameters:**
   In `generator.go`, values from `ConfigItem` or parsed query strings are assembled into JSON structs. If string concatenation or improper unmarshaling is used, can an adversary craft a subscription link that injects malicious outbounds or overrides Xray settings?
2. **Base64 Decoding Edge Cases in VMess & Shadowsocks:**
   VMess links in the wild frequently violate standard RFC 4648 padding (missing `=` padding, URL-safe base64 vs Standard base64, whitespace, newlines). Does `decodeBase64()` gracefully handle missing padding and both standard and URL-safe alphabets?
3. **Type-Assertion Panic Hazards in VMessJSON:**
   In `VMessJSON`, `Port`, `Aid`, and `V` are typed as `interface{}`. How are these unmarshaled? If a subscription provider delivers `"port": "443"` (string) instead of `443` (number), does the parser handle it cleanly or panic on unhandled type conversion?
4. **VLESS Reality / Flow Validation:**
   Reality requires specific parameters: `pbk` (public key), `sid` (short id), `sni` (server name), `fp` (fingerprint, e.g. `chrome`). If `flow=xtls-rprx-vision` is passed, does `generator.go` enforce TCP transport and vision flow requirements? What happens if `security=reality` but `pbk` is missing?
5. **Port Clashing & Inbound Hardcoding:**
   Are inbound ports `10808` and `10809` strictly verified not to conflict with system services before starting?
6. **DNS Leakage via Missing Xray DNS Configuration:**
   Does the generated Xray configuration include a dedicated `dns` block, or does Xray rely on system DNS resolution? If Xray performs remote DNS resolution without routing protection, can DNS requests loop or leak?

---

## 3. Lead Architect Directives & Step-by-Step Instructions

### Step 1: Mandatory Architectural Graph Exploration (`graphify`)
Before reading line-by-line, run:
```powershell
python -m graphify.cli explain configmgr
# or
python -m graphify.cli query "how is configmgr used across the application"
```
Check relationships between `configmgr`, `store.ConfigItem`, and `core.Supervisor`.

### Step 2: Initialize Report Scaffold
Immediately create `docs/audit/reports/03-configmgr-report.md` with standard sections.

### Step 3: Step-by-Step Systematic Code Inspection & Incremental Updates
Audit each area sequentially and update your report immediately:
1. **VMess & Shadowsocks Decoding (`parser.go:L97-L200`):** Audit base64 handling, JSON unmarshaling, type assertions, and invalid input resilience.
2. **VLESS & Trojan URL Parsing (`parser.go:L62-L95`, `L201-L274`):** Audit query parameter decoding, URL parsing flaws, and default fallback logic.
3. **Outbound Configuration Builder (`generator.go:L23-L200`):** Audit `buildProxyOutbound`, Reality streamSettings, WebSocket headers, and gRPC service names.
4. **Routing Rules Translation (`generator.go:L68-L150`):** Audit IP CIDR validation, domain rule syntax (`geosite:`, `domain:`), and action tagging (`direct`, `proxy`, `block`).

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
