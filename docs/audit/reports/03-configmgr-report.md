# Security & Architecture Audit Report: Domain 03 (Config Management & Xray Subsystem)

> **Audited Subsystem:** Share Link Parsing, Xray Configuration Generation & Routing Synthesis  
> **Target Files:**  
> - `internal/configmgr/parser.go`  
> - `internal/configmgr/generator.go`  
> - `internal/configmgr/parser_test.go`  
> - `internal/configmgr/generator_test.go`  
> **Auditor:** Principal Protocol Parsing & Proxy Configuration Auditor (Subagent Domain 03)  
> **Audit Date:** 2026-09-24  
> **Audit Status:** Completed (Comprehensive Line-by-Line Inspection)  

---

## 1. Executive Summary

The `internal/configmgr` package is the protocol ingestion and translation gateway of V2Raynix. It is responsible for parsing heterogenous, real-world proxy share links (`vless://`, `vmess://`, `trojan://`, `ss://`, and raw JSON) into normalized `store.ConfigItem` entities, and subsequently synthesizing hardened JSON configuration specifications executed directly by the `xray-core` child process.

Because Xray configuration generation dictates network encapsulation, cipher negotiation, camouflage headers, TLS handshakes, anti-loop socket marks, and split-tunnel routing, defects in this domain have immediate and critical repercussions on network stability, user privacy, and daemon survival.

Our systematic inspection of `internal/configmgr/parser.go`, `internal/configmgr/generator.go`, and their accompanying test suites reveals significant architectural, protocol, and security defects across the 6 audit dimensions mandated in `docs/audit/briefs/03-configmgr-brief.md`:

1. **VMess Generator Fragment Splitting Omission (CFG-D3-01):** `parseVMess` in `parser.go` strips `#` fragments from raw links, but `buildProxyOutbound` in `generator.go` does not. When `GenerateXrayConfig` is invoked on any VMess item containing a remark fragment (e.g. `#Remark`), base64 decoding fails with `illegal base64 data`, causing tunnel activation to fail completely.
2. **Missing Xray DNS Block Causing Loopback Traps & DNS Leakage (CFG-D3-02):** `generator.go` sets `routing.domainStrategy = "IPIfNonMatch"` and enables inbound sniffing, but generates zero `"dns"` configuration. When domain routing rules do not match, Xray falls back to the host OS resolver (`net.LookupIP`), which lacks `sockopt.mark = 81`. In TUN mode, this causes an infinite DNS routing loop or leaks DNS queries in cleartext to local ISP resolvers.
3. **Inbound SOCKS/HTTP Port Collision and Out-of-Range Acceptance (CFG-D3-03):** `GenerateXrayConfig` accepts `localSocksPort` and `localHttpPort` without checking for identical port numbers (`localSocksPort == localHttpPort`) or validating port boundaries (`1 <= port <= 65535`), generating configurations that crash `xray-core` on socket bind.
4. **Raw Custom JSON Ingestion Bypassing Inbounds & Anti-Loop Policies (CFG-D3-04):** When `Protocol == "custom_json"`, `generator.go` outputs the raw JSON directly, bypassing the injection of local SOCKS/HTTP inbounds, user routing rules, and `sockopt: { mark: 81 }`, breaking `tun2socks` and triggering infinite packet routing loops.
5. **Broken IPv6 and Password Delimiter Parsing in Shadowsocks (CFG-D3-05):** In `parseShadowsocks`, host and port are split via `strings.SplitN(hostPort, ":", 2)`. IPv6 addresses (e.g. `[2001:db8::1]:8388`) are split on internal colons, causing port conversion to fail and rejecting valid IPv6 endpoints.
6. **Trojan Transport Camouflage Dropping and SNI Fallback Omission (CFG-D3-06):** In `buildProxyOutbound`, Trojan configurations silently drop WebSocket and gRPC transport parameters (`type=ws`, `path`, `host`, `serviceName`), falling back to TCP and causing connection failures on CDN/WSS endpoints. Missing `sni` also fails to default to the server hostname.
7. **Reality Security Transport Incompatibility & Parameter Validation Gaps (CFG-D3-07):** `generator.go` permits `security=reality` with incompatible transports such as WebSocket or gRPC, crashing Xray on boot. Furthermore, public keys (`pbk`), short IDs (`sid`), and uTLS fingerprints (`fp`) are not validated against schema constraints.
8. **Suboptimal Routing Rule Auto-Detection & Target Splitting (CFG-D3-08):** Routing rules with unspecified target types default to `domain` rules even when the target is an IP address or CIDR block, and comma-separated target strings are not parsed into individual list items.
9. **Missing Percent-Encoding Handling in Base64 Decoding (CFG-D3-09):** `decodeBase64` does not unescape URL-encoded characters (`%2B`, `%2F`, `%3D`), failing on web-encoded subscription links.

---

## 2. Graphify Knowledge Graph & Dependency Analysis

A graph query (`graphify query "configmgr parser generator"`) and architectural mapping reveal the critical placement of `configmgr` within the V2Raynix architecture:

```
                                  +---------------------------------------+
                                  |         internal/api/router           |
                                  |     - /api/configs (POST/PUT)         |
                                  +-------------------+-------------------+
                                                      |
                                                      | invokes
                                                      v
                                  +---------------------------------------+
                                  |         internal/configmgr            |
                                  |               parser.go               |
                                  |   - ParseShareLink()                  |
                                  |     * parseVLESS()                    |
                                  |     * parseVMess()                    |
                                  |     * parseTrojan()                   |
                                  |     * parseShadowsocks()              |
                                  |     * parseRawJSON()                  |
                                  +-------------------+-------------------+
                                                      |
                                                      | returns *store.ConfigItem
                                                      v
                                  +---------------------------------------+
                                  |         internal/store                |
                                  |   - ConfigItem / RoutingRule          |
                                  +-------------------+-------------------+
                                                      |
                                                      | consumed by
                                                      v
                                  +---------------------------------------+
                                  |        internal/core/supervisor       |
                                  |     (Supervisor / Tunnel Lifecycle)   |
                                  +-------------------+-------------------+
                                                      |
                                                      | calls
                                                      v
                                  +---------------------------------------+
                                  |         internal/configmgr            |
                                  |              generator.go             |
                                  |   - GenerateXrayConfig()              |
                                  |     * buildProxyOutbound()            |
                                  |     * socks-in (10808) / http-in      |
                                  |     * routing rules synthesis         |
                                  +-------------------+-------------------+
                                                      |
                                                      | writes xray-active.json
                                                      v
                                  +---------------------------------------+
                                  |      External Daemon: xray-core       |
                                  |   xray run -c data/xray-active.json   |
                                  +---------------------------------------+
```

### Architectural Dependency Insights:
1. **The Re-parsing Split-Brain Hazard:**
   `parser.go` ingests the share link into `store.ConfigItem`, but `generator.go` re-parses the raw URL string (`cfg.RawURL`). Any discrepancy between how `parser.go` sanitizes a link and how `generator.go` parses it creates a split-brain condition: links are validated and stored by the API, but crash the supervisor during tunnel startup.
2. **Coupling to Linux Policy Routing & TUN:**
   `generator.go` produces configuration that operates under the host policy routing configured by `internal/network/routing.go`. Because `routing.go` sets up `fwmark 81` lookup tables to exempt proxy traffic from `tun0`, `generator.go` must ensure that all network-bound traffic originating from Xray (outbound proxy, freedom direct, and DNS resolution) is marked with `mark: 81`.

---

## 3. Findings Summary Table

| Finding ID | Title | Severity | Impact Area | File & Line Range |
| :--- | :--- | :--- | :--- | :--- |
| **CFG-D3-01** | VMess Generator Fragment Splitting Omission | **HIGH** | Outbound Generation / Base64 | `internal/configmgr/generator.go:248-251` |
| **CFG-D3-02** | Missing Xray DNS Block Causing Loopback Traps & DNS Leakage | **CRITICAL** | DNS Resolution / Leak Prevention | `internal/configmgr/generator.go:120-130` |
| **CFG-D3-03** | Inbound SOCKS/HTTP Port Collision & Range Validation Omission | **HIGH** | Inbound Ports / Daemon Stability | `internal/configmgr/generator.go:14, 29-53` |
| **CFG-D3-04** | Raw Custom JSON Ingestion Bypassing Inbounds & Anti-Loop Policy | **HIGH** | Configuration Integrity / TUN | `internal/configmgr/generator.go:19-22` |
| **CFG-D3-05** | Broken IPv6 and Password Delimiter Parsing in Shadowsocks | **MEDIUM** | Protocol Ingestion / Shadowsocks | `internal/configmgr/parser.go:198-202, 216-220` |
| **CFG-D3-06** | Trojan Transport Camouflage Dropping & SNI Fallback Omission | **MEDIUM** | Protocol Generation / Trojan | `internal/configmgr/generator.go:331-354` |
| **CFG-D3-07** | Reality Security Transport Incompatibility & Validation Gaps | **MEDIUM** | VLESS Reality / Flow Validation | `internal/configmgr/generator.go:178-211` |
| **CFG-D3-08** | Suboptimal Routing Rule Auto-Detection & Target Splitting | **LOW** | Routing Rule Synthesis | `internal/configmgr/generator.go:96-115` |
| **CFG-D3-09** | Missing Percent-Encoding Handling in Base64 Decoding | **LOW** | Protocol Ingestion / Base64 | `internal/configmgr/parser.go:270-292` |

---

## 4. Detailed Defect Analysis

### CFG-D3-01: VMess Generator Fragment Splitting Omission

- **Code Location:** `internal/configmgr/generator.go:248-251` (contrast with `internal/configmgr/parser.go:99-102`)
- **Severity:** **HIGH**
- **Trigger & Root Cause:**  
  When parsing a VMess link in `parser.go`, lines 99-102 sanitize the link by stripping remark fragments:
  ```go
  b64Data := strings.TrimPrefix(link, "vmess://")
  if idx := strings.IndexAny(b64Data, "#?"); idx != -1 {
      b64Data = b64Data[:idx]
  }
  decoded, err := decodeBase64(b64Data)
  ```
  However, in `generator.go`, `buildProxyOutbound` omits this sanitization:
  ```go
  case "vmess":
      b64 := strings.TrimPrefix(cfg.RawURL, "vmess://")
      decoded, err := decodeBase64(b64)
  ```
  If a VMess link contains a fragment (`#Remark`), `ParseShareLink` successfully imports the link into the database, preserving the full link in `cfg.RawURL`. When the user activates the tunnel, `GenerateXrayConfig` passes `b64` (which still contains `#Remark`) to `decodeBase64`. Because `#` is not a valid base64 character in standard or URL-safe alphabets, `decodeBase64` returns an error, and `GenerateXrayConfig` fails, preventing the tunnel from starting.
- **PoC / Verification:**  
  ```go
  link := "vmess://eyJ2IjoiMiIsInBzIjoidGVzdCIsImFkZCI6IjEuMi4zLjQiLCJwb3J0IjoiNDQzIiwiaWQiOiJhNmM0ZDdiMi01MjBlLTRiNjktOGNlMi00ZTBkNGM4MmI5NTIiLCJhaWQiOiIwIiwic2N5IjoiYXV0byIsIm5ldCI6IndzIiwidHlwZSI6Im5vbmUiLCJob3N0IjoiZXhhbXBsZS5jb20iLCJwYXRoIjoiL3dzIiwidGxzIjoiIn0=#RemarkFragment"
  cfg, err := configmgr.ParseShareLink(link) // Succeeds
  _, err = configmgr.GenerateXrayConfig(cfg, nil, 10808, 10809)
  // Fails with: "failed to build proxy outbound: illegal base64 data at input byte ..."
  ```
- **Recommended Fix:**  
  Strip URL fragments and query strings in `generator.go` prior to base64 decoding:
  ```go
  case "vmess":
      b64 := strings.TrimPrefix(cfg.RawURL, "vmess://")
      if idx := strings.IndexAny(b64, "#?"); idx != -1 {
          b64 = b64[:idx]
      }
      decoded, err := decodeBase64(b64)
      if err != nil {
          return nil, fmt.Errorf("failed to decode vmess base64: %w", err)
      }
  ```
- **Strengths:**  
  `parser.go` properly implements fragment and query stripping during the initial parse pass, and `decodeBase64` cleans whitespace characters (`\r`, `\n`, `\t`, `' '`).

---

### CFG-D3-02: Missing Xray DNS Block Causing Loopback Traps & DNS Leakage

- **Code Location:** `internal/configmgr/generator.go:120-130`
- **Severity:** **CRITICAL**
- **Trigger & Root Cause:**  
  `GenerateXrayConfig` configures `routing.domainStrategy = "IPIfNonMatch"` and enables sniffing on the SOCKS inbound:
  ```go
  "routing": map[string]interface{}{
      "domainStrategy": "IPIfNonMatch",
      "rules":          xRoutingRules,
  },
  ```
  However, the generated Xray JSON completely omits a `"dns"` top-level configuration object and contains no DNS outbound (`tag: "dns-out"`).
  When incoming connections require domain-to-IP resolution to evaluate `IPIfNonMatch` routing rules, Xray-core falls back to the host operating system's standard resolver (`net.LookupIP`).
  In a full-tunnel setup using `tun2socks` and Linux policy routing:
  1. The host OS resolver issues queries via standard UDP/53 sockets that lack `sockopt.mark = 81`.
  2. These packets are captured by the `tun0` interface routing table, routed back to `tun2socks`, forwarded to `socks-in`, and handed to Xray.
  3. Xray triggers another `net.LookupIP` to match routing rules, creating an infinite resolution loop that freezes DNS queries.
  4. If host DNS traffic is somehow exempt from TUN, the queries exit unencrypted to local ISP nameservers, causing severe DNS leaks.
- **PoC / Verification:**  
  Inspect generated JSON from `TestGenerateXrayConfig`. The output contains `"log"`, `"inbounds"`, `"outbounds"`, and `"routing"`, but `"dns"` is absent. In an active TUN deployment, querying an un-cached domain results in resolution timeout or cleartext UDP packets on the physical interface.
- **Recommended Fix:**  
  Inject a dedicated `"dns"` configuration block and DNS routing rule in `GenerateXrayConfig`:
  ```go
  config := map[string]interface{}{
      "log": map[string]interface{}{
          "loglevel": "warning",
      },
      "dns": map[string]interface{}{
          "servers": []interface{}{
              "https://1.1.1.1/dns-query",
              "8.8.8.8",
              "localhost",
          },
          "queryStrategy": "UseIP",
      },
      "inbounds":  inbounds,
      "outbounds": outbounds,
      "routing": map[string]interface{}{
          "domainStrategy": "IPIfNonMatch",
          "rules":          xRoutingRules,
      },
  }
  ```
  Additionally, add a DNS outbound or ensure freedom/proxy outbounds route DNS traffic with `mark: 81`.
- **Strengths:**  
  `inbounds` sniffing is enabled for `http`, `tls`, and `quic`, allowing domain extraction from encrypted payloads.

---

### CFG-D3-03: Inbound SOCKS/HTTP Port Collision and Out-of-Range Acceptance

- **Code Location:** `internal/configmgr/generator.go:14, 29-53`
- **Severity:** **HIGH**
- **Trigger & Root Cause:**  
  `GenerateXrayConfig` takes `localSocksPort` and `localHttpPort` as integer arguments:
  ```go
  func GenerateXrayConfig(activeConfig *store.ConfigItem, rules []*store.RoutingRule, localSocksPort, localHttpPort int) ([]byte, error)
  ```
  The function does not validate:
  1. Port range validity: ports `<= 0` or `> 65535` are accepted without error.
  2. Port collision: if `localSocksPort == localHttpPort` (e.g. both set to 10808), both inbounds are generated on the exact same port.
  When `xray-core` boots with conflicting ports or invalid port numbers, it exits immediately with an address binding error (`bind: address already in use` or `invalid port`), aborting the supervisor.
- **PoC / Verification:**  
  ```go
  cfg := &store.ConfigItem{Protocol: "vless", Server: "1.1.1.1", Port: 443, RawURL: "vless://uuid@1.1.1.1:443"}
  data, err := configmgr.GenerateXrayConfig(cfg, nil, 10808, 10808) // Same port
  // err is nil! Generated JSON has two inbounds on 10808.
  ```
- **Recommended Fix:**  
  Add strict port validation at the start of `GenerateXrayConfig`:
  ```go
  if localSocksPort <= 0 || localSocksPort > 65535 {
      return nil, fmt.Errorf("invalid localSocksPort: %d (must be 1-65535)", localSocksPort)
  }
  if localHttpPort <= 0 || localHttpPort > 65535 {
      return nil, fmt.Errorf("invalid localHttpPort: %d (must be 1-65535)", localHttpPort)
  }
  if localSocksPort == localHttpPort {
      return nil, fmt.Errorf("localSocksPort and localHttpPort cannot be identical (%d)", localSocksPort)
  }
  ```
- **Strengths:**  
  Inbounds are strictly bound to loopback (`"listen": "127.0.0.1"`), preventing accidental exposure to the local network or public interfaces.

---

### CFG-D3-04: Raw Custom JSON Ingestion Bypassing Inbounds & Anti-Loop Policy

- **Code Location:** `internal/configmgr/generator.go:19-22`, `internal/configmgr/parser.go:250-268`
- **Severity:** **HIGH**
- **Trigger & Root Cause:**  
  When an active configuration has `Protocol == "custom_json"`, `generator.go` returns the raw content directly:
  ```go
  // If it's already a raw custom JSON, return its bytes directly
  if activeConfig.Protocol == "custom_json" {
      return []byte(activeConfig.RawURL), nil
  }
  ```
  This creates multiple architectural failures:
  1. The required local SOCKS inbound (`127.0.0.1:localSocksPort`) and HTTP inbound are not injected. If the user's custom JSON does not define a SOCKS inbound on that exact port, `tun2socks` cannot connect to Xray, breaking the system proxy.
  2. Outbounds do not receive the anti-loop socket mark (`streamSettings.sockopt.mark = 81`). All proxy and direct traffic exiting Xray matches the Table 100 policy routing rule and is recirculated into `tun0`, causing an immediate system network freeze.
  3. Routing rules configured in V2Raynix are ignored.
- **PoC / Verification:**  
  Import a standard Xray config JSON that does not define SOCKS port 10808 or `mark: 81`. Start tunnel. `tun2socks` logs connection refused, or host traffic loops indefinitely.
- **Recommended Fix:**  
  Parse the custom JSON into a structured map, validate and ensure `inbounds` contain the required SOCKS and HTTP listeners, and ensure all outbounds have `sockopt: { mark: 81 }`:
  ```go
  if activeConfig.Protocol == "custom_json" {
      var customMap map[string]interface{}
      if err := json.Unmarshal([]byte(activeConfig.RawURL), &customMap); err != nil {
          return nil, fmt.Errorf("invalid custom json: %w", err)
      }
      // Inject mandatory inbounds and ensure mark: 81 on outbounds
      ensureRequiredInbounds(customMap, localSocksPort, localHttpPort)
      ensureAntiLoopMarks(customMap, 81)
      return json.MarshalIndent(customMap, "", "  ")
  }
  ```
- **Strengths:**  
  `parseRawJSON` validates that the input is syntactically valid JSON before storing it.

---

### CFG-D3-05: Broken IPv6 and Password Delimiter Parsing in Shadowsocks Links

- **Code Location:** `internal/configmgr/parser.go:198-202, 216-220`
- **Severity:** **MEDIUM**
- **Trigger & Root Cause:**  
  In `parseShadowsocks`, host and port are split using:
  ```go
  hp := strings.SplitN(hostPort, ":", 2)
  if len(hp) != 2 {
      return nil, ErrMalformedLink
  }
  server = hp[0]
  portStr := hp[1]
  ```
  For an IPv6 endpoint such as `ss://base64@[2001:db8::1]:8388#Node`, `hostPort` is `[2001:db8::1]:8388`.
  `SplitN(hostPort, ":", 2)` cuts on the first colon, producing:
  - `hp[0]` = `"[2001"`
  - `hp[1]` = `"db8::1]:8388"`
  `strconv.Atoi(portStr)` returns an error, causing `parseShadowsocks` to fail with `ErrMalformedLink`.
  Furthermore, in legacy Shadowsocks decoding (`base64(method:password@host:port)`), if a password contains an `@` symbol, splitting on the first `@` truncates the password.
- **PoC / Verification:**  
  ```go
  // Valid SIP002 link with IPv6 server
  link := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ=@%5B2001:db8::1%5D:8388#IPv6-Node"
  _, err := configmgr.ParseShareLink(link)
  // Fails with ErrMalformedLink
  ```
- **Recommended Fix:**  
  Use `net.SplitHostPort` or search for the last colon to separate host and port, and use `strings.LastIndex` for the `@` separator:
  ```go
  host, portStr, err := net.SplitHostPort(hostPort)
  if err != nil {
      // Fallback or error
      return nil, ErrMalformedLink
  }
  server = strings.Trim(host, "[]")
  port, err := strconv.Atoi(portStr)
  ```
- **Strengths:**  
  Supports both SIP002 (`ss://base64(method:password)@host:port`) and Legacy format (`ss://base64(method:password@host:port)`).

---

### CFG-D3-06: Trojan Transport Camouflage Dropping & SNI Fallback Omission

- **Code Location:** `internal/configmgr/generator.go:331-354`
- **Severity:** **MEDIUM**
- **Trigger & Root Cause:**  
  In `generator.go`, `buildProxyOutbound` for Trojan constructs the outbound:
  ```go
  outbound["streamSettings"] = map[string]interface{}{
      "security": "tls",
      "tlsSettings": map[string]interface{}{
          "serverName": q.Get("sni"),
      },
  }
  ```
  1. **Transport Camouflage Ignored:** Modern Trojan links (Trojan-Go, Xray-Trojan) often use WebSocket (`type=ws&path=/path&host=domain`) or gRPC (`type=grpc&serviceName=svc`). `generator.go` does not check `q.Get("type")`, `q.Get("path")`, or `q.Get("host")` for Trojan. All transport camouflage is silently dropped, defaulting to TCP. Connections to CDN-fronted or WebSocket-based Trojan endpoints fail immediately.
  2. **Empty SNI:** If `sni` is not explicitly set in the query string, `serverName` is set to `""`. Standard practice requires falling back to `cfg.Server` (the hostname) to ensure valid TLS SNI during the handshake.
- **PoC / Verification:**  
  Generate config for `trojan://pass@example.com:443?type=ws&path=/trojan-ws&host=example.com`. The resulting JSON contains no `wsSettings` and network is set to default TCP.
- **Recommended Fix:**  
  Add transport handling for Trojan mirroring VLESS, and fallback `sni` to `cfg.Server`:
  ```go
  sni := q.Get("sni")
  if sni == "" {
      sni = cfg.Server
  }
  tlsMap := map[string]interface{}{
      "serverName": sni,
  }
  // Add wsSettings / grpcSettings based on q.Get("type")
  ```
- **Strengths:**  
  Password and server details are correctly mapped to Xray's `servers` schema.

---

### CFG-D3-07: Reality Security Transport Incompatibility & Validation Gaps

- **Code Location:** `internal/configmgr/generator.go:178-211`
- **Severity:** **MEDIUM**
- **Trigger & Root Cause:**  
  1. **Incompatible Transports for Reality:** `generator.go` validates that `xtls-rprx-vision` requires TCP transport, but fails to validate that `security=reality` itself requires TCP (`netType == "tcp"`). If a link specifies `type=ws&security=reality`, `generator.go` generates both `realitySettings` and `wsSettings`. Xray-core cannot run Reality over WebSocket, causing startup termination.
  2. **Missing Schema Validation:** Neither `parser.go` nor `generator.go` validates:
     - `pbk`: Must be a valid 32-byte base64 Curve25519 public key (43-44 characters).
     - `sid`: Must be a hexadecimal string of up to 16 hex digits (or empty).
     - `fp`: Must be a supported uTLS fingerprint (`chrome`, `firefox`, `safari`, `ios`, `android`, `edge`, `360`, `qq`, `random`, `randomized`).
     Invalid values cause Xray to fail during configuration load or TLS handshakes.
- **PoC / Verification:**  
  Provide `vless://uuid@1.1.1.1:443?security=reality&pbk=invalid_key&type=ws&sni=example.com`. `GenerateXrayConfig` generates a config that Xray crashes on with transport/cipher errors.
- **Recommended Fix:**  
  Enforce that `security=reality` requires `netType == "tcp"`, and validate public key and fingerprint values:
  ```go
  if sec == "reality" {
      if netType != "tcp" {
          return nil, fmt.Errorf("vless reality is only supported over tcp transport, got: %s", netType)
      }
      // Validate pbk length and base64 encoding
  }
  ```
- **Strengths:**  
  Enforces non-empty `pbk` and `sni` for Reality, and checks `xtls-rprx-vision` compatibility with TCP and TLS/Reality.

---

### CFG-D3-08: Suboptimal Routing Rule Auto-Detection & Target Splitting

- **Code Location:** `internal/configmgr/generator.go:96-115`
- **Severity:** **LOW**
- **Trigger & Root Cause:**  
  In routing rule synthesis:
  1. When `TargetType` is unspecified or set to `default`, the logic checks:
     ```go
     if strings.HasPrefix(target, "geoip:") {
         ruleMap["ip"] = []string{target}
     } else {
         ruleMap["domain"] = []string{target}
     }
     ```
     If a user inputs a CIDR or IP (e.g. `10.0.0.0/8` or `192.168.1.1`) without selecting `TargetType: "ip"`, it is auto-detected as a domain: `"domain": ["10.0.0.0/8"]`. This rule will never match network traffic.
  2. If a user enters comma-separated targets (e.g. `google.com, youtube.com`), the string is not split, producing `"domain": ["google.com, youtube.com"]`, which is invalid.
- **PoC / Verification:**  
  Create a rule with `Target: "10.0.0.0/8"` and empty `TargetType`. Inspect generated JSON: `ruleMap["domain"]` is populated instead of `ruleMap["ip"]`.
- **Recommended Fix:**  
  Parse IP/CIDR using `net.ParseIP` or `net.ParseCIDR` to differentiate IPs from domains, and split targets by comma.
- **Strengths:**  
  Correctly filters out disabled rules, normalizes action tags (`direct`, `proxy`, `block`), and maps to Xray's `field` routing rule format.

---

### CFG-D3-09: Missing Percent-Encoding Handling in Base64 Decoding

- **Code Location:** `internal/configmgr/parser.go:270-292`
- **Severity:** **LOW**
- **Trigger & Root Cause:**  
  `decodeBase64` removes whitespace (`\r`, `\n`, `\t`, `' '`), but does not call `url.QueryUnescape`. Subscription feeds and web query links frequently include percent-encoded base64 payloads (e.g. `%2B` instead of `+`, `%2F` instead of `/`, `%3D` instead of `=`). When passed to `decodeBase64`, all four standard decoders fail on `%` characters.
- **PoC / Verification:**  
  `vmess://eyJ2Ijoi...%3D%3D` fails with `invalid base64 in vmess`.
- **Recommended Fix:**  
  Unescape percent-encoding before cleaning whitespace and decoding:
  ```go
  func decodeBase64(s string) ([]byte, error) {
      if unescaped, err := url.QueryUnescape(s); err == nil {
          s = unescaped
      }
      clean := strings.Map(...)
  ```
- **Strengths:**  
  Attempts all four permutations: Standard, RawStandard, URL, and RawURL encoding.

---

## 5. Architectural Recommendations & Hardening Plan

1. **Eliminate Parser / Generator Split-Brain:**  
   Refactor `generator.go` to construct outbounds directly from strongly-typed fields stored on `store.ConfigItem` or an intermediate protocol struct, rather than re-parsing `cfg.RawURL` from scratch.
2. **Comprehensive DNS Architecture in Xray:**  
   Add a full DNS subsystem to `generator.go` with encrypted DoH (`https://1.1.1.1/dns-query`) and `sockopt.mark = 81` on DNS outbounds to permanently prevent DNS leakage and routing loops.
3. **Inbound Port Safety & Conflict Detection:**  
   Validate that `localSocksPort` and `localHttpPort` are in the valid range `[1024, 65535]` and distinct. Ensure the supervisor performs pre-flight socket binding checks before launching Xray.
4. **Custom JSON Normalization:**  
   When importing custom JSON, deserialize into a DOM/map, verify presence of essential inbounds, inject `sockopt: { mark: 81 }` into all non-loopback outbounds, and validate syntax with `xray -test`.

---

## 6. Conclusion & Audit Sign-Off

The `internal/configmgr` subsystem demonstrates clean modular structure and covers key proxy protocols (`vless`, `vmess`, `trojan`, `shadowsocks`). However, critical vulnerabilities exist in DNS resolution (loop hazard and DNS leak), inbound port collision handling, custom JSON safety, and VMess fragment re-parsing.

Implementing the defensive remediations outlined above will ensure robust protocol parsing, zero-leak DNS resolution, and rock-solid daemon execution.

**Audit Status:** Completed. Source code remains strictly unchanged.
