# Security & Architecture Audit Report: Domain 03 (Config Management & Xray Subsystem)

> **Audited Subsystem:** Share Link Parsing, Xray Configuration Generation & Routing Synthesis  
> **Target Files:**  
> - `internal/configmgr/parser.go`  
> - `internal/configmgr/generator.go`  
> - `internal/configmgr/parser_test.go`  
> - `internal/configmgr/generator_test.go`  
> **Auditor:** Principal Protocol Parsing & Xray Config Generation Auditor  
> **Audit Date:** 2026-09-21  
> **Audit Status:** Completed (Comprehensive Line-by-Line Inspection)  

---

## 1. Executive Summary

The `internal/configmgr` package is the protocol ingestion and translation gateway of V2Raynix. It is responsible for parsing heterogenous, real-world proxy share links (`vless://`, `vmess://`, `trojan://`, `ss://`, and raw JSON) into normalized `store.ConfigItem` entities, and subsequently synthesizing hardened JSON configuration specifications executed directly by the `xray-core` child process.

Because Xray configuration generation dictates network encapsulation, cipher negotiation, camouflage headers, TLS handshakes, anti-loop socket marks, and split-tunnel routing, defects in this domain have immediate and critical repercussions on network stability, user privacy, and daemon survival.

Our systematic inspection of `internal/configmgr/parser.go`, `internal/configmgr/generator.go`, and their accompanying test suites has revealed 10 significant architectural, protocol, and security defects:

1. **Infinite Packet Routing Loops on Direct Outbounds (CFG-01):** Anti-loop `sockopt.mark = 81` is applied exclusively to the `proxy` outbound; direct (`freedom`) traffic matches host policy routing rules and loops infinitely back into `tun0`.
2. **Critical VMess Camouflage & Header Dropping (CFG-02):** The outbound generator for VMess completely omits `wsSettings` (dropping path and Host headers), `grpcSettings`, and `tlsSettings.serverName`, causing immediate connection failures on CDN and TLS-camouflaged endpoints.
3. **Broken Legacy Shadowsocks Credential Synthesis (CFG-03):** Legacy format `ss://base64(method:password@host:port)` is accepted by `parser.go` but fails in `generator.go`, emitting empty `method` and `password` fields that crash Xray.
4. **Subscription Link Base64 & Fragment Fragility (CFG-04):** `parseVMess` fails to strip URL fragments (`#name`), and `decodeBase64` does not sanitize MIME line breaks, whitespace, or percent-encodings, causing parser rejections on valid subscription links.
5. **Type Assertion & Value Boundary Risks in VMessJSON (CFG-05):** Port boundary checks omit the upper limit (`> 65535`), `int`/`int64` unmarshaling cases are absent, and `alterId` is hardcoded to 0, breaking compatibility with legacy VMess nodes.
6. **Missing Reality & Vision Flow Compatibility Validation (CFG-06):** Reality public keys (`pbk`) and SNI are not validated for presence, and `xtls-rprx-vision` is allowed on unsupported transports (WebSocket, gRPC), crashing Xray at startup.
7. **DNS Leakage & Loop Hazard via Missing Dedicated DNS Configuration (CFG-07):** Xray is configured with `domainStrategy: "IPIfNonMatch"` but contains zero `dns` block configuration, forcing DNS queries onto the host OS resolver where they loop or leak in cleartext to local ISPs.
8. **Inbound Port Collisions & Hardcoding (CFG-08):** Inbound ports 10808 (SOCKS5) and 10809 (HTTP) are hardcoded without pre-flight socket binding checks or user configuration options.
9. **Raw JSON Ingestion Bypassing Inbounds & Anti-Loop Policies (CFG-09):** `custom_json` configurations bypass the injection of local inbounds (10808/10809) and anti-loop socket marks (`mark: 81`), leading to broken tunnel forwarding.
10. **Wildcard Catch-All Routing Rule Synthesis on Malformed Targets (CFG-10):** Routing rules with invalid or empty `TargetType` generate empty field criteria matching 100% of network traffic.

---

## 2. Graphify Knowledge Graph & Dependency Analysis

The architectural relationship between `configmgr` and adjacent subsystems was mapped via `graphify` (`graphify-out/graph.json`):

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
                                                      | returns
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
                                  |   (God Node: Central Controller)      |
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
1. **Critical Pipeline Decoupling:**
   `parser.go` ingests the share link into `store.ConfigItem`, but `generator.go` re-parses the raw URL string (`cfg.RawURL`). Any discrepancy between how `parser.go` validates a link and how `generator.go` interprets it creates a split-brain vulnerability where invalid links are accepted into persistent storage but subsequently crash the core supervisor on tunnel activation.
2. **Coupling to Linux Policy Routing:**
   `generator.go` must generate configuration that coexists with the Linux routing table rules created by `internal/network/routing.go`. Specifically, because `routing.go` sets up `fwmark 81` lookup tables to exempt proxy traffic from `tun0`, `generator.go` must ensure that **all** traffic originating from Xray (both `proxy` and `direct`) bears `mark 81`.

---

## 3. Findings Summary Table

| ID | Title & Category | Code Location | Severity | Status |
|---|---|---|---|---|
| **CFG-01** | Infinite Routing Loop on Direct Outbound Traffic via Missing `sockopt.mark` | `generator.go:L56-L60`, `L300-L308` | **High** | Confirmed |
| **CFG-02** | VMess Outbound Omission of WebSocket, gRPC, and TLS Camouflage Settings | `generator.go:L231-L241` | **High** | Confirmed |
| **CFG-03** | Broken Legacy Shadowsocks Outbound Credential Extraction & Silent Error Discard | `generator.go:L272-L290`, `parser.go:L187-L211` | **High** | Confirmed |
| **CFG-04** | Base64 Fragility in VMess and Shadowsocks Ingestion (Fragments & Line Breaks) | `parser.go:L98-L103`, `L175-L211`, `L254-L269` | **Medium** | Confirmed |
| **CFG-05** | Type Assertion Gap, Port Range Omission, and AlterId Dropping in VMessJSON | `parser.go:L21-L35`, `L109-L120`, `generator.go:L221` | **Medium** | Confirmed |
| **CFG-06** | Missing Reality Public Key and Incompatible Flow Parameter Validation | `generator.go:L135-L168` | **Medium** | Confirmed |
| **CFG-07** | Absence of Dedicated Xray DNS Block Causing DNS Leaks and Loop Deadlocks | `generator.go:L93-L103` | **Medium** | Confirmed |
| **CFG-08** | Inbound Port Collision Exposure via Hardcoded Ports 10808 and 10809 | `generator.go:L28-L52`, `internal/core/supervisor.go:L135` | **Medium** | Confirmed |
| **CFG-09** | Raw Custom JSON Pass-Through Bypasses Local Inbounds and Anti-Loop Protection | `generator.go:L19-L21`, `parser.go:L234-L252` | **Medium** | Confirmed |
| **CFG-10** | Unmatched Wildcard Catch-All Routing Rule Synthesis on Invalid TargetType | `generator.go:L68-L91` | **Low / Refactor** | Confirmed |

---

## 4. Comprehensive Audit Findings (7-Field Defect Schema)

---

### CFG-01: Infinite Routing Loop on Direct Outbound Traffic via Missing `sockopt.mark`

- **Title & Category:** Network Routing Loop / Policy Routing Mark Omission
- **Code Location:** `internal/configmgr/generator.go:L56-L60`, `L300-L308`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  In Linux transparent proxying environments (as implemented in `internal/network/routing.go`), host policy routing redirects all host traffic into the virtual interface `tun0` unless the packet possesses the firewall mark `81` (`from all fwmark 81 lookup main priority 100`).
  In `generator.go`, `buildProxyOutbound` sets `sockopt: {"mark": 81}` on the `proxy` outbound (`L305-L307`). However, the `direct` outbound (`protocol: "freedom"`) defined at `L56-L60` has empty settings:
  ```go
  {
      "tag":      "direct",
      "protocol": "freedom",
      "settings": map[string]interface{}{},
  }
  ```
  When a user defines routing rules to bypass domestic domains or IP ranges (e.g. `geosite:ir`, `10.0.0.0/8`, or local LAN CIDRs with `action: "direct"`), Xray evaluates the routing table and routes those packets through the `direct` outbound. Because the direct outbound lacks `streamSettings.sockopt.mark = 81`, the kernel routes the egress packets directly back into `tun0`. `tun2socks` intercepts the packets and forwards them again to Xray's SOCKS5 inbound (10808). This triggers an immediate, CPU-saturating infinite packet routing loop, rendering all direct/bypassed traffic completely unreachable.
- **Proof of Concept / Verification Method:**
  1. Initialize a configuration with a direct routing rule for a destination IP (e.g. `198.51.100.50`).
  2. Generate configuration via `GenerateXrayConfig(cfg, rules, 10808, 10809)`.
  3. Inspect the resulting JSON structure for the outbound with `"tag": "direct"`.
  4. Confirm that `streamSettings.sockopt.mark` is completely absent from the direct outbound.
  5. Under Linux policy routing with `tun0`, sending traffic to `198.51.100.50` causes routing loop packet counters in iptables/nftables to increment exponentially until buffer exhaustion.
- **Recommended Architectural Fix:**
  Ensure that all outbounds capable of initiating egress network traffic (`proxy` and `direct`) include the anti-loop `sockopt` mark:
  ```go
  freedomOutbound := map[string]interface{}{
      "tag":      "direct",
      "protocol": "freedom",
      "settings": map[string]interface{}{},
      "streamSettings": map[string]interface{}{
          "sockopt": map[string]interface{}{
              "mark": 81,
          },
      },
  }
  ```
- **Existing Strengths & Robustness:**
  `buildProxyOutbound` reliably attaches mark 81 to the primary proxy outbound, successfully preventing loops on proxy-bound egress traffic.

---

### CFG-02: VMess Outbound Omission of WebSocket, gRPC, and TLS Camouflage Settings

- **Title & Category:** Protocol Ingestion & Configuration Synthesis / Functional Omission
- **Code Location:** `internal/configmgr/generator.go:L231-L241`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  Real-world VMess nodes extensively use WebSocket or gRPC transports combined with CDN reverse proxies (e.g., Cloudflare, Fastly). When parsing a VMess link, `parser.go` defines `VMessJSON` with fields `Net`, `Host`, `Path`, `TLS`, and `Sni`.
  However, in `generator.go` (`L231-L241`), the generator only constructs:
  ```go
  network := vmess.Net
  if network == "" {
      network = "tcp"
  }
  streamSettings := map[string]interface{}{
      "network": network,
  }
  if vmess.TLS == "tls" {
      streamSettings["security"] = "tls"
  }
  outbound["streamSettings"] = streamSettings
  ```
  `generator.go` completely fails to evaluate `vmess.Path`, `vmess.Host`, `vmess.Sni`, or `vmess.Type`. If `vmess.Net == "ws"`, no `wsSettings` object is produced; the WebSocket connection is sent without HTTP Host headers or custom request paths, resulting in an immediate HTTP 400/404 from edge CDNs. If `vmess.TLS == "tls"`, no `tlsSettings` object is constructed, dropping `serverName` (SNI); the TLS handshake will fail or present the raw IP, which CDNs reject immediately.
- **Proof of Concept / Verification Method:**
  1. Pass the test VMess link from `parser_test.go:L38` (which specifies `net: "ws"`, `host: "example.com"`, `path: "/ws"`) into `GenerateXrayConfig`.
  2. Parse the generated JSON and inspect `outbounds[0]["streamSettings"]`.
  3. Notice `wsSettings` is `nil` and `tlsSettings` is `nil`. The generated configuration is functionally non-viable for any CDN-fronted or WebSocket-based VMess server.
- **Recommended Architectural Fix:**
  Add comprehensive transport and TLS mapping for VMess in `generator.go`:
  ```go
  if vmess.TLS == "tls" {
      streamSettings["security"] = "tls"
      tlsMap := map[string]interface{}{}
      if vmess.Sni != "" {
          tlsMap["serverName"] = vmess.Sni
      } else if vmess.Host != "" {
          tlsMap["serverName"] = vmess.Host
      }
      streamSettings["tlsSettings"] = tlsMap
  }
  if network == "ws" {
      wsMap := map[string]interface{}{}
      if vmess.Path != "" {
          wsMap["path"] = vmess.Path
      }
      if vmess.Host != "" {
          wsMap["headers"] = map[string]string{"Host": vmess.Host}
      }
      streamSettings["wsSettings"] = wsMap
  } else if network == "grpc" {
      grpcMap := map[string]interface{}{}
      if vmess.Path != "" {
          grpcMap["serviceName"] = vmess.Path
      }
      streamSettings["grpcSettings"] = grpcMap
  }
  ```
- **Existing Strengths & Robustness:**
  `buildProxyOutbound` correctly implements full `wsSettings`, `grpcSettings`, and `xhttpSettings` for the VLESS protocol (`L170-L198`).

---

### CFG-03: Broken Legacy Shadowsocks Outbound Credential Extraction & Silent Error Discard

- **Title & Category:** Protocol Ingestion & Credential Loss
- **Code Location:** `internal/configmgr/generator.go:L272-L290`, `internal/configmgr/parser.go:L187-L211`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  Shadowsocks URIs exist in two common formats:
  1. **SIP002:** `ss://base64(method:password)@hostname:port#tag`
  2. **Legacy:** `ss://base64(method:password@hostname:port)#tag`
  In `parser.go` (`L197-L211`), both formats are supported during initial ingestion.
  However, in `generator.go` (`L272-L290`), `buildProxyOutbound` assumes **only** SIP002 format:
  ```go
  var method, password string
  if strings.Contains(mainPart, "@") {
      sub := strings.SplitN(mainPart, "@", 2)
      decoded, err := decodeBase64(sub[0])
      if err == nil {
          mp := strings.SplitN(string(decoded), ":", 2)
          if len(mp) == 2 {
              method = mp[0]
              password = mp[1]
          }
      }
  }
  ```
  For legacy format links, `mainPart` is raw base64 and does not contain `@`. Therefore, `strings.Contains(mainPart, "@")` evaluates to `false`, leaving `method` and `password` as empty strings `""`.
  Furthermore, if `sub[0]` base64 decoding fails or produces an unexpected string format, the error is swallowed by `if err == nil`, leaving credentials empty. The resulting outbound has `"method": ""` and `"password": ""`, causing Xray to reject the configuration on launch.
- **Proof of Concept / Verification Method:**
  1. Create a `store.ConfigItem` with `RawURL: "ss://YmYtY2ZiOnRlc3RAMTkyLjE2OC4xMDAuMTo4ODg4#LegacyNode"`.
  2. Call `GenerateXrayConfig(cfg, nil, 10808, 10809)`.
  3. Inspect `outbounds[0]["settings"]["servers"][0]`.
  4. Verify that `"method": ""` and `"password": ""` are generated, resulting in an unusable Xray configuration.
- **Recommended Architectural Fix:**
  Mirror the decoding logic of `parser.go` within `generator.go` or, preferably, extract `method` and `password` during parsing and store them as structured fields in `store.ConfigItem` instead of repeatedly re-parsing raw URLs:
  ```go
  if strings.Contains(mainPart, "@") {
      // SIP002 format
      sub := strings.SplitN(mainPart, "@", 2)
      decoded, err := decodeBase64(sub[0])
      if err != nil {
          return nil, fmt.Errorf("invalid base64 credentials in shadowsocks URL: %w", err)
      }
      mp := strings.SplitN(string(decoded), ":", 2)
      if len(mp) != 2 {
          return nil, fmt.Errorf("invalid method:password format in shadowsocks URL")
      }
      method, password = mp[0], mp[1]
  } else {
      // Legacy format
      decoded, err := decodeBase64(mainPart)
      if err != nil {
          return nil, fmt.Errorf("invalid base64 in legacy shadowsocks URL: %w", err)
      }
      decStr := string(decoded)
      sub := strings.SplitN(decStr, "@", 2)
      if len(sub) == 2 {
          mp := strings.SplitN(sub[0], ":", 2)
          if len(mp) == 2 {
              method, password = mp[0], mp[1]
          }
      }
  }
  if method == "" || password == "" {
      return nil, fmt.Errorf("empty method or password in shadowsocks configuration")
  }
  ```
- **Existing Strengths & Robustness:**
  `parser.go` correctly extracts the server hostname and port for both SIP002 and legacy Shadowsocks URL formats.

---

### CFG-04: Base64 Fragility in VMess and Shadowsocks Ingestion (Fragments & Line Breaks)

- **Title & Category:** Input Parsing & RFC 4648 Compliance
- **Code Location:** `internal/configmgr/parser.go:L98-L103`, `L175-L211`, `L254-L269`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  In real-world subscription feeds:
  1. VMess share links frequently include a fragment suffix for server remarks (e.g. `vmess://eyJ2Ijoi...#Frankfurt-01`). In `parseVMess` (`L98`), the code executes `b64Data := strings.TrimPrefix(link, "vmess://")` without stripping `#`. When `decodeBase64` runs on this string, the trailing `#` causes standard and URL-safe base64 decoders to fail with `illegal base64 data at input byte ...`.
  2. Subscription payloads often wrap base64 strings across multiple lines with `\r\n` or contain internal whitespace. Standard Go `base64.DecodeString` fails on any unstripped whitespace.
  3. URL query parameters in Shadowsocks (such as SIP003 plugin options `ss://...:8388?plugin=obfs-local#Tag`) are not stripped before calling `strconv.Atoi(hp[1])` (`L195`), causing port conversion to fail and discarding legitimate links.
- **Proof of Concept / Verification Method:**
  1. Attempt to parse `vmess://eyJ2IjoiMiIsInBzIjoidGVzdCIsImFkZCI6IjEuMi4zLjQiLCJwb3J0Ijo0NDMsImlkIjoiOTZjNGQ3YjItNTIwZS00YjY5LThjZTItNGUwZDRjODJiOTUyIn0=#RemarkFragment`.
  2. `ParseShareLink` returns `malformed configuration link: invalid base64 in vmess`.
  3. Attempt to parse a base64 string with embedded newline `\n`.
  4. `decodeBase64` returns an error rather than sanitizing whitespace.
- **Recommended Architectural Fix:**
  1. Strip fragments and query parameters from VMess links before base64 decoding:
     ```go
     b64Data := strings.TrimPrefix(link, "vmess://")
     if idx := strings.IndexAny(b64Data, "#?"); idx != -1 {
         b64Data = b64Data[:idx]
     }
     ```
  2. Sanitize whitespace in `decodeBase64`:
     ```go
     func decodeBase64(s string) ([]byte, error) {
         clean := strings.Map(func(r rune) rune {
             if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
                 return -1
             }
             return r
         }, s)
         if b, err := base64.StdEncoding.DecodeString(clean); err == nil {
             return b, nil
         }
         if b, err := base64.RawStdEncoding.DecodeString(clean); err == nil {
             return b, nil
         }
         if b, err := base64.URLEncoding.DecodeString(clean); err == nil {
             return b, nil
         }
         return base64.RawURLEncoding.DecodeString(clean)
     }
     ```
- **Existing Strengths & Robustness:**
  `decodeBase64` attempts multiple encodings in cascade (standard with padding, standard raw, URL-safe with padding, and URL-safe raw).

---

### CFG-05: Type Assertion Gap, Port Range Omission, and AlterId Dropping in VMessJSON

- **Title & Category:** Data Model Deserialization & Boundary Validation
- **Code Location:** `internal/configmgr/parser.go:L21-L35`, `L109-L120`, `internal/configmgr/generator.go:L221`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  1. In `VMessJSON`, `Port`, `Aid`, and `V` are typed as `interface{}` (`L22`, `L25`, `L27`). In `parseVMess`, `Port` is extracted using a type switch covering `float64` and `string` (`L110-L115`). However, `int` and `int64` are omitted. Furthermore, the validation check (`L117`) only verifies `port <= 0`. It fails to enforce the upper port boundary (`port > 65535`), allowing invalid port numbers (e.g. 70000 or 99999) to be persisted.
  2. `Aid` (alterId) in `VMessJSON` is unmarshaled as `interface{}` but is completely discarded in `generator.go:L221`, where `"alterId": 0` is unconditionally hardcoded. While modern VMess deployments use AEAD (`alterId = 0`), older servers in certain enterprise or legacy subscriptions strictly require non-zero alterId (e.g., 64). Hardcoding 0 without honoring the configured `aid` breaks connection establishment for these servers.
  3. `vmess.ID` is never checked for non-empty string or UUID compliance in `parseVMess`, allowing empty user IDs to be accepted into storage.
- **Proof of Concept / Verification Method:**
  1. Supply a VMess JSON payload with `"port": 70000` and `"aid": 64`.
  2. `ParseShareLink` succeeds and persists `Port: 70000`.
  3. `GenerateXrayConfig` emits a configuration with `port: 70000` and `alterId: 0`, dropping the user's `aid`.
- **Recommended Architectural Fix:**
  1. Validate port upper bound and add type coverage:
     ```go
     port := 0
     switch p := vmess.Port.(type) {
     case float64:
         port = int(p)
     case int:
         port = p
     case string:
         port, _ = strconv.Atoi(strings.TrimSpace(p))
     }
     if vmess.Add == "" || port <= 0 || port > 65535 || strings.TrimSpace(vmess.ID) == "" {
         return nil, ErrMalformedLink
     }
     ```
  2. Extract and honor `alterId` in `generator.go`:
     ```go
     alterID := 0
     switch a := vmess.Aid.(type) {
     case float64:
         alterID = int(a)
     case int:
         alterID = a
     case string:
         alterID, _ = strconv.Atoi(strings.TrimSpace(a))
     }
     ```
- **Existing Strengths & Robustness:**
  `parseVLESS` and `parseTrojan` strictly validate `port <= 0 || port > 65535`.

---

### CFG-06: Missing Reality Public Key and Incompatible Flow Parameter Validation

- **Title & Category:** Protocol Validation & Daemon Startup Crash Hazard
- **Code Location:** `internal/configmgr/generator.go:L135-L168`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  VLESS Reality protocol has strict cryptographic and transport prerequisites:
  1. **Public Key (`publicKey` / `pbk`):** When `security=reality`, Xray-core mandates a valid base64url-encoded Curve25519 public key. If `pbk` query parameter is missing or empty, `generator.go` emits `"publicKey": ""`. Xray-core fails to initialize and terminates with an error: `failed to build reality config: empty public key`.
  2. **XTLS Vision Flow (`xtls-rprx-vision`):** The vision flow algorithm is exclusively supported over raw TCP transport with TLS or Reality encryption (`network: "tcp"`). If a share link contains `flow=xtls-rprx-vision` alongside `type=ws`, `type=grpc`, or unencrypted transport, `generator.go` blindly injects `"flow": "xtls-rprx-vision"` into `users`. Xray-core terminates on startup with an invalid flow configuration error.
- **Proof of Concept / Verification Method:**
  1. Construct a VLESS link with `security=reality` but without `pbk`:
     `vless://uuid@1.2.3.4:443?type=tcp&security=reality&sni=example.com`
  2. Pass to `GenerateXrayConfig`.
  3. The generated config contains `"publicKey": ""`. Executing `xray -test -c xray-active.json` results in immediate configuration rejection.
  4. Construct a VLESS link with `type=ws` and `flow=xtls-rprx-vision`.
  5. The resulting config combines WebSocket with vision flow, causing Xray startup failure.
- **Recommended Architectural Fix:**
  Add strict validation in `buildProxyOutbound`:
  ```go
  if q.Get("security") == "reality" {
      pbk := q.Get("pbk")
      if pbk == "" {
          return nil, fmt.Errorf("vless reality requires a non-empty public key (pbk)")
      }
      sni := q.Get("sni")
      if sni == "" {
          return nil, fmt.Errorf("vless reality requires serverName (sni)")
      }
      streamSettings["realitySettings"] = map[string]interface{}{
          "serverName":  sni,
          "publicKey":   pbk,
          "shortId":     q.Get("sid"),
          "fingerprint": q.Get("fp"),
      }
  }

  flow := q.Get("flow")
  if flow == "xtls-rprx-vision" {
      sec := q.Get("security")
      if netType != "tcp" || (sec != "reality" && sec != "tls") {
          return nil, fmt.Errorf("xtls-rprx-vision is only supported with TCP transport and TLS/Reality security")
      }
  }
  ```
- **Existing Strengths & Robustness:**
  `generator.go` correctly maps `fp` (uTLS fingerprint) and `sid` (shortId) into `realitySettings`.

---

### CFG-07: Absence of Dedicated Xray DNS Block Causing DNS Leaks and Loop Deadlocks

- **Title & Category:** Network Privacy & DNS Leakage / Loop Deadlock Hazard
- **Code Location:** `internal/configmgr/generator.go:L93-L103`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  In `generator.go`, `GenerateXrayConfig` configures the routing engine with `"domainStrategy": "IPIfNonMatch"` (`L100`), but completely omits a top-level `"dns"` configuration block:
  ```go
  config := map[string]interface{}{
      "log": map[string]interface{}{
          "loglevel": "warning",
      },
      "inbounds":  inbounds,
      "outbounds": outbounds,
      "routing": map[string]interface{}{
          "domainStrategy": "IPIfNonMatch",
          "rules":          xRoutingRules,
      },
  }
  ```
  When an application initiates a connection by domain name, Xray inspects domain routing rules. If no domain rule matches, `IPIfNonMatch` compels Xray to resolve the domain to an IP address in order to test IP-based routing rules (e.g. `10.0.0.0/8` direct bypass).
  Because no internal Xray DNS servers or DNS outbounds are specified, Xray delegates domain resolution to Go's internal standard resolver or host OS resolver (`/etc/resolv.conf`) via plain UDP port 53.
  This introduces two critical failures:
  1. **DNS Leakage:** Domain queries generated during proxy routing are dispatched in unencrypted plain text to local ISP DNS servers, exposing user browsing metadata.
  2. **Resolution Deadlock/Loop:** If host DNS traffic is redirected into `tun0` by policy routing without mark 81, Xray's resolution queries loop back into `tun2socks`, hanging connection establishment indefinitely.
- **Proof of Concept / Verification Method:**
  1. Generate an Xray configuration with `GenerateXrayConfig`.
  2. Unmarshal the JSON and check for the `"dns"` key.
  3. Observe that `"dns"` is absent.
  4. Inspect routing rules: Observe that no `outboundTag: "dns-out"` or DNS redirection rules exist.
- **Recommended Architectural Fix:**
  Add a secure, encrypted DNS block and corresponding routing rules to `generator.go`:
  ```go
  "dns": map[string]interface{}{
      "servers": []interface{}{
          "https://1.1.1.1/dns-query",
          "https://8.8.8.8/dns-query",
          map[string]interface{}{
              "address": "1.1.1.1",
              "domains": []string{"geosite:cn", "geosite:ir"},
              "expectIPs": []string{},
              "skipFallback": true,
          },
      },
      "queryStrategy": "UseIPv4",
  }
  ```
  Ensure outbound DNS queries are routed either through the `proxy` outbound or direct with `sockopt.mark = 81`.
- **Existing Strengths & Robustness:**
  SOCKS5 inbound sniffing is enabled for `"http"`, `"tls"`, and `"quic"` (`generator.go:L38-L41`), extracting destination domain names directly from TLS ClientHello and HTTP request headers.

---

### CFG-08: Inbound Port Collision Exposure via Hardcoded Ports 10808 and 10809

- **Title & Category:** System Resilience & Port Conflict
- **Code Location:** `internal/configmgr/generator.go:L28-L52`, `internal/core/supervisor.go:L135`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  In `internal/core/supervisor.go:L135`, configuration generation is invoked with hardcoded port values:
  ```go
  rawJSON, err := configmgr.GenerateXrayConfig(cfg, rules, 10808, 10809)
  ```
  Neither `configmgr` nor `supervisor`:
  1. Verifies that `localSocksPort` (10808) and `localHttpPort` (10809) are not already bound by existing local processes (such as a system-wide v2ray/xray daemon, Tor, Clash, or a lingering orphaned daemon from an earlier crash).
  2. Verifies that `localSocksPort != localHttpPort`.
  3. Exposes configuration settings for these inbound ports in `store.SystemSettings`.
  If port 10808 or 10809 is in use, Xray fails to bind on startup (`bind: address already in use`). Because `supervisor.go` starts Xray asynchronously and applies network routing rules without confirming socket readiness, host routing directs traffic to `tun0` while no listening proxy daemon exists, causing total host network disconnection.
- **Proof of Concept / Verification Method:**
  1. Bind a dummy socket on localhost port 10808 (`nc -l 10808` or equivalent).
  2. Trigger tunnel activation via `Supervisor.StartTunnel()`.
  3. `GenerateXrayConfig` emits inbounds with port 10808 without error.
  4. Xray fails to bind and exits; the host network is severed.
- **Recommended Architectural Fix:**
  1. Add pre-flight TCP port binding verification utility in `configmgr` or `supervisor` before config generation:
     ```go
     func CheckPortAvailable(port int) error {
         ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
         if err != nil {
             return fmt.Errorf("inbound port %d is already in use: %w", port, err)
         }
         _ = ln.Close()
         return nil
     }
     ```
  2. Expose `SocksPort` and `HttpPort` in `store.SystemSettings` with default fallbacks.
- **Existing Strengths & Robustness:**
  Inbounds are strictly bound to loopback `127.0.0.1` (`L32`, `L46`), preventing external network access to local proxy entry points.

---

### CFG-09: Raw Custom JSON Pass-Through Bypasses Local Inbounds and Anti-Loop Protection

- **Title & Category:** Architectural Bypass & Tunnel Failure
- **Code Location:** `internal/configmgr/generator.go:L19-L21`, `internal/configmgr/parser.go:L234-L252`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  In `generator.go`:
  ```go
  // If it's already a raw custom JSON, return its bytes directly
  if activeConfig.Protocol == "custom_json" {
      return []byte(activeConfig.RawURL), nil
  }
  ```
  `parseRawJSON` (`parser.go:L234-L252`) allows users to import arbitrary Xray configuration files. When `GenerateXrayConfig` processes a `custom_json` item, it returns the raw JSON directly, bypassing:
  1. Generation of local SOCKS5 (`127.0.0.1:10808`) and HTTP (`127.0.0.1:10809`) inbounds required by `tun2socks`.
  2. Injection of anti-loop socket marks (`streamSettings.sockopt.mark = 81`) into outbounds.
  3. Incorporation of application routing rules configured in `store.RoutingRule`.
  If a user imports a standard server configuration that listens on external interfaces or lacks `mark: 81`, `tun2socks` will fail to communicate with Xray, and outbound proxy traffic will loop infinitely under host policy routing.
- **Proof of Concept / Verification Method:**
  1. Import a raw custom JSON configuration missing inbounds on 10808:
     `{"outbounds": [{"protocol": "freedom"}]}`
  2. Call `GenerateXrayConfig(cfg, rules, 10808, 10809)`.
  3. The returned JSON is identical to the input payload. Ports 10808/10809 and anti-loop mark 81 are absent.
- **Recommended Architectural Fix:**
  Instead of raw byte pass-through, parse the custom JSON into a structured map, validate or merge required local inbounds (10808/10809), and enforce `sockopt.mark = 81` on all outbounds:
  ```go
  if activeConfig.Protocol == "custom_json" {
      var customMap map[string]interface{}
      if err := json.Unmarshal([]byte(activeConfig.RawURL), &customMap); err != nil {
          return nil, fmt.Errorf("invalid custom json payload: %w", err)
      }
      injectRequiredInbounds(customMap, localSocksPort, localHttpPort)
      injectOutboundSockopts(customMap, 81)
      return json.MarshalIndent(customMap, "", "  ")
  }
  ```
- **Existing Strengths & Robustness:**
  `parseRawJSON` validates that the payload is valid JSON before accepting it into the configuration store.

---

### CFG-10: Unmatched Wildcard Catch-All Routing Rule Synthesis on Invalid TargetType

- **Title & Category:** Routing Rule Synthesis & Security Policy Enforcement
- **Code Location:** `internal/configmgr/generator.go:L68-L91`
- **Severity:** **Low / Refactor**
- **Trigger Scenario & Root Cause Analysis:**
  In `generator.go`, routing rules are converted into Xray rule specifications:
  ```go
  ruleMap := map[string]interface{}{
      "type":        "field",
      "outboundTag": r.Action, // "direct", "proxy", "block"
  }
  if r.Action == "proxy" {
      ruleMap["outboundTag"] = "proxy"
  }
  if r.TargetType == "domain" {
      ruleMap["domain"] = []string{r.Target}
  } else if r.TargetType == "ip" {
      ruleMap["ip"] = []string{r.Target}
  }
  xRoutingRules = append(xRoutingRules, ruleMap)
  ```
  If a rule in the database has an empty or unrecognized `TargetType` (e.g. corrupted storage entry or future unsupported type), neither `ruleMap["domain"]` nor `ruleMap["ip"]` is populated. The resulting object is:
  ```json
  {
      "type": "field",
      "outboundTag": "direct"
  }
  ```
  In Xray-core routing syntax, a field rule with no matching criteria (no domain, ip, port, or network specified) acts as an unconditional **wildcard rule** that matches 100% of network traffic. If the action is `"block"` or `"direct"`, all subsequent routing rules are shadowed and all user traffic is misrouted.
- **Proof of Concept / Verification Method:**
  1. Add a routing rule with `TargetType: ""` and `Action: "block"`.
  2. Call `GenerateXrayConfig`.
  3. Inspect `routing.rules`. Observe an empty criteria rule matching everything.
- **Recommended Architectural Fix:**
  Validate `TargetType` and ensure rules without valid match conditions are ignored or rejected:
  ```go
  if r.TargetType == "domain" && strings.TrimSpace(r.Target) != "" {
      ruleMap["domain"] = []string{r.Target}
  } else if r.TargetType == "ip" && strings.TrimSpace(r.Target) != "" {
      ruleMap["ip"] = []string{r.Target}
  } else {
      // Discard invalid/empty rule to prevent wildcard catch-all behavior
      continue
  }
  ```
- **Existing Strengths & Robustness:**
  The routing builder correctly checks `if !r.IsEnabled { continue }` to skip disabled rules, and sets `domainStrategy: "IPIfNonMatch"` to support combined domain and IP routing.

---

## 5. Architectural Recommendations & Remediation Roadmap

1. **Unify Anti-Loop Socket Marks Across All Outbounds:**
   Immediately modify `GenerateXrayConfig` to ensure that both `proxy` and `direct` (`freedom`) outbounds are configured with `streamSettings.sockopt.mark = 81`.
2. **Complete VMess and Trojan Camouflage Generation:**
   Extend `buildProxyOutbound` to construct `wsSettings`, `grpcSettings`, and `tlsSettings` for VMess and Trojan, preventing connection drops when communicating with CDN-proxied endpoints.
3. **Parse Once, Store Structured Configurations:**
   Refactor `store.ConfigItem` to store parsed parameters (UUID, cipher method, password, transport type, path, SNI, alterId) in structured fields rather than serializing to `RawURL` and repeatedly parsing via string splits and regex.
4. **Harden Share Link Base64 Decoding:**
   Cleanse URL fragments (`#...`), query strings, and whitespace before decoding base64 in `parseVMess` and `parseShadowsocks`.
5. **Implement Comprehensive Test Coverage:**
   Add unit tests covering edge cases: VMess with WebSocket and path, base64 links with fragments and newlines, legacy Shadowsocks links, Reality links with missing public keys, and direct outbound anti-loop mark verification.
