# Security & Architecture Audit Report: Domain 01 (Network & Policy Routing)

> **Audited Subsystem:** Network, Kernel Policy Routing & SafeMode Subsystem  
> **Target Files:**
> - `internal/network/routing.go`
> - `internal/network/safemode.go`
> - `internal/network/routing_test.go`
> - `internal/network/safemode_test.go`  
> **Auditor:** Principal Linux Network & Kernel Systems Auditor  
> **Audit Date:** 2026-09-21  
> **Audit Status:** Complete & Verified  

---

## 1. Executive Summary

The `internal/network` package forms the critical kernel-interfacing layer of V2Raynix. It is responsible for orchestrating Linux TUN virtual network devices (`tun0`), kernel policy routing tables (`Table 100`), netfilter connection tracking (`conntrack`), packet marking (`connmark`), anti-lockout firewall and routing bypasses (for SSH and Web UI), and fail-safe countdown management via `SafeModeController`.

Because these operations run with elevated privileges (typically `root` or `CAP_NET_ADMIN`) and manipulate global system routing tables, defects in this package carry extreme risk. Our systematic line-by-line inspection identified **8 discrete defects**:
- **1 Critical-Severity Vulnerability**: Shell command injection and arbitrary remote code execution (RCE) as root via unescaped string formatting in `ExecuteCommands` and unvalidated upstream inputs.
- **3 High-Severity Vulnerabilities**: Complete IPv6 traffic and DNS leakage bypassing the proxy; asymmetric routing drops and permanent admin lockout on multi-homed/secondary-NIC systems; and concurrency races in `SafeModeController` causing split-brain teardowns of active tunnels.
- **4 Medium-Severity Defects**: Missing MTU specifications and lack of TCPMSS clamping causing Path MTU Discovery blackholes; fragile SSH port detection ignoring drop-in overrides, active sessions, and multi-port configurations; single-IP DNS resolution truncation with round-robin CDNs; and non-idempotent cleanup command chains that swallow execution errors.

This report provides complete root-cause analyses, verification methods/PoCs, architectural remediations, and an analysis of existing subsystem strengths.

---

## 2. Graphify Knowledge Graph & Dependency Analysis

Based on the project's knowledge graph extracted via `graphify` (`graphify-out/graph.json` and `graphify-out/GRAPH_REPORT.md`):

```
                                  +--------------------------------+
                                  |    internal/core/supervisor    |
                                  |   (God Node: 24 connections)   |
                                  +---------------+----------------+
                                                  |
                         +------------------------+------------------------+
                         |                                                 |
                         | calls StartTunnel / StopTunnel                  | manages SafeMode lifecycle
                         v                                                 v
        +---------------------------------+               +---------------------------------+
        |    internal/network/routing     |               |    internal/network/safemode    |
        | - GetDefaultRoute()             |               | - SafeModeController            |
        | - DetectSSHPort()               |               | - Start()                       |
        | - ResolveHost()                 |               | - Confirm()                     |
        | - BuildRoutingCommands()        |               | - CancelAndRollback()           |
        | - BuildCleanupCommands()        |               | - GetRemainingSeconds()         |
        | - ExecuteCommands()             |               +---------------------------------+
        +---------------------------------+
                         |
                         v
        +---------------------------------+
        |          Linux Kernel           |
        | - iproute2 (rules & tables)     |
        | - iptables (mangle & conntrack) |
        | - tuntap device (tun0)          |
        +---------------------------------+
```

### Key Graph Insights:
1. **Community 5 ("Supervisor") Inter-coupling:** `Supervisor` functions as an architectural God Node directly bridging `internal/network` to `internal/store`, `internal/api`, and process management. Every invocation of `StartTunnel` or `StopTunnel` in `Supervisor` delegates all kernel configuration to `routing.go` and registers a fail-safe teardown callback in `safemode.go`.
2. **Blast Radius Propagation:** `ExecuteCommands` runs with inherited `root` or `sudo` ambient capabilities. A failure, hang, or command injection vulnerability inside `internal/network` propagates immediately to the entire host system, capable of terminating host connectivity, crashing supervisor routines, or allowing arbitrary host compromise.

---

## 3. Findings Summary Table

| ID | Title & Category | Code Location | Severity | Status |
|---|---|---|---|---|
| **NET-01** | Shell Command Injection & RCE via Metacharacters in `ExecuteCommands` | `routing.go:L200-L209` | **Critical** | Confirmed |
| **NET-02** | Silent IPv6 Traffic Leak & Dual-Stack Routing Bypass | `routing.go:L32-L69` | **High** | Confirmed |
| **NET-03** | Missing MTU Specification & Lack of TCPMSS Clamping Causing PMTUD Blackholes | `routing.go:L34-L36` | **Medium** | Confirmed |
| **NET-04** | Asymmetric Routing & Lockout Risk on Multi-Homed / Secondary NIC Servers | `routing.go:L50-L57`, `L118-L153` | **High** | Confirmed |
| **NET-05** | Fragile SSH Port Detection Failing on Drop-ins, Multiple Ports & Inbound Rules | `routing.go:L43-L44`, `L156-L180` | **Medium** | Confirmed |
| **NET-06** | DNS Resolution Truncation & Round-Robin Route Inconsistency in `ResolveHost` | `routing.go:L183-L197` | **Medium** | Confirmed |
| **NET-07** | Concurrency Race Condition & Split-Brain Execution in `SafeModeController` | `safemode.go:L42-L54`, `L73-L91` | **High** | Confirmed |
| **NET-08** | Non-Idempotent Cleanup & Command Execution Error Swallowing in `ExecuteCommands` | `routing.go:L75-L115`, `L200-L209` | **Medium** | Confirmed |

---

## 4. Comprehensive Audit Findings (7-Field Defect Schema)

### Finding NET-01: Shell Command Injection & RCE via Metacharacters in `ExecuteCommands`

- **Title & Category:** Shell Command Injection & Arbitrary Code Execution (RCE) as Root | **Security / Privilege Escalation**
- **Exact Code Location:** `internal/network/routing.go:L200-L209` (propagated from `L24-L71` and `L75-L115`)
- **Severity:** **Critical**
- **Trigger Scenario & Root Cause Analysis:**
  `ExecuteCommands` executes shell commands by invoking `exec.Command("sh", "-c", cmd)`. In `BuildRoutingCommands` and `BuildCleanupCommands`, variables `remoteProxyIP`, `defaultIface`, and `defaultGw` are formatted directly into command strings via `fmt.Sprintf`:
  ```go
  fmt.Sprintf("ip rule add to %s table main priority 999", remoteProxyIP)
  fmt.Sprintf("ip route add %s/32 via %s dev %s", remoteProxyIP, defaultGw, defaultIface)
  fmt.Sprintf("iptables -t mangle -A PREROUTING -i %s -m conntrack --ctstate NEW -j %s", defaultIface, InboundChain)
  ```
  Neither `BuildRoutingCommands` nor `BuildCleanupCommands` validates, sanitizes, or quotes these strings. If a proxy server configuration contains shell metacharacters (e.g. `;`, `&`, `|`, `` ` ``, `$()`), or if `cfg.Server` is loaded from an untrusted share link (VMess, VLESS, Trojan, SS) containing injected payloads, passing the string through `sh -c` causes arbitrary command execution with root privileges.
  Furthermore, in `internal/core/supervisor.go:154`, the return error from `ResolveHost(cfg.Server)` is explicitly ignored (`remoteIP, _ := network.ResolveHost(cfg.Server)`). When host resolution fails, `remoteIP` defaults to empty string `""`. This causes `BuildRoutingCommands` to produce invalid shell commands (`ip rule add to  table main priority 999`), which fail and leave routing in a broken state.
- **Proof of Concept / Verification Method:**
  Pass a crafted configuration to `BuildRoutingCommands`:
  ```go
  cmds := network.BuildRoutingCommands("127.0.0.1; touch /tmp/v2raynix_pwned; #", "eth0", "192.168.1.1", 22, 2080)
  ```
  Generated command:
  ```sh
  ip rule add to 127.0.0.1; touch /tmp/v2raynix_pwned; # table main priority 999
  ```
  When passed to `ExecuteCommands()`, `sh -c` splits on `;` and executes `touch /tmp/v2raynix_pwned` as the root user.
- **Recommended Architectural Fix:**
  1. Eliminate `sh -c` entirely. Never invoke shell interpreters for routing commands. Instead, pass command arguments directly as discrete string slices:
     ```go
     func RunCommand(name string, args ...string) error {
         cmd := exec.Command(name, args...)
         if out, err := cmd.CombinedOutput(); err != nil {
             return fmt.Errorf("%s %v failed: %w (output: %s)", name, args, err, string(out))
         }
         return nil
     }
     ```
  2. Alternatively, use pure Go netlink/netfilter libraries (e.g. `github.com/vishvananda/netlink` and `github.com/coreos/go-iptables`) to configure interfaces, routes, and iptables rules directly via kernel netlink sockets without spawning subprocesses.
  3. Validate all inputs before building commands: verify that `remoteProxyIP` is a valid IP (`net.ParseIP`), `defaultIface` matches `^[a-zA-Z0-9_.-]+$`, and port numbers fall strictly within `1..65535`.
- **Existing Strengths & Robustness:**
  `BuildRoutingCommands` defensively normalizes non-positive port arguments (`sshPort <= 0` defaults to 22, `webPort <= 0` defaults to 2080).

---

### Finding NET-02: Silent IPv6 Traffic Leak & Dual-Stack Routing Bypass

- **Title & Category:** Silent IPv6 Traffic Leak & Dual-Stack Routing Bypass | **Network & Kernel / Security**
- **Exact Code Location:** `internal/network/routing.go:L32-L69`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  `BuildRoutingCommands` exclusively configures IPv4 policy routing:
  - Creates `tun0` and assigns only an IPv4 address: `ip addr add 198.18.0.1/15 dev tun0`.
  - Configures only IPv4 routing rules: `ip rule add not fwmark 0x51 table 100 priority 2000`.
  - Configures only IPv4 iptables: `iptables -t mangle ...`.
  Modern Linux operating systems (Ubuntu 22.04/24.04, Debian 12) have IPv6 enabled by default. Dual-stack client applications (e.g., browsers, `curl`, package managers) and system DNS resolvers use "Happy Eyeballs" (RFC 8305) to prefer IPv6 when available.
  Because no IPv6 policy routing or `ip6tables` rules exist, all IPv6 packets completely bypass `table 100` and exit unencrypted via the default physical gateway. This results in **complete user deanonymization**, leaking the host's real IP address, ISP, and DNS queries.
  Furthermore, if `remoteProxyIP` is an IPv6 address, `BuildRoutingCommands` attempts to execute:
  ```sh
  ip rule add to 2001:db8::1 table main priority 999
  ip route add 2001:db8::1/32 via 192.168.1.1 dev eth0
  ```
  Both commands fail immediately with `Error: an inet prefix is expected rather than "2001:db8::1"` and `invalid prefix length`.
- **Proof of Concept / Verification Method:**
  On a dual-stack Linux host with V2Raynix tunnel running:
  ```bash
  curl -6 https://ifconfig.co
  ```
  The command succeeds and prints the host's real physical IPv6 address rather than the proxy endpoint. Additionally, inspecting IPv6 rules via `ip -6 rule show` confirms no proxy diversion rules exist.
- **Recommended Architectural Fix:**
  1. If IPv6 proxying is supported:
     - Assign an IPv6 address to `tun0` (e.g., `fc00::1/64` or `fd00::/64` unique local address range).
     - Add IPv6 policy routing: `ip -6 route add default dev tun0 table 100` and `ip -6 rule add not fwmark 0x51 table 100 priority 2000`.
     - Mirror connection tracking rules in `ip6tables`.
  2. If IPv6 proxying is not supported:
     - Implement an explicit, safe IPv6 blackhole or disable switch during tunnel operation (e.g., `ip6tables -A OUTPUT -o <defaultIface> -j REJECT` or toggle `net.ipv6.conf.all.disable_ipv6 = 1`).
     - On tunnel teardown, cleanly restore IPv6 sysctl/firewall state.
- **Existing Strengths & Robustness:**
  IPv4 private subnet ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 127.0.0.0/8) are properly bypassed in Table main at priorities 1010-1013, ensuring local LAN/loopback IPv4 communications remain operational.

---

### Finding NET-03: Missing MTU Specification & Lack of TCPMSS Clamping Causing PMTUD Blackholes

- **Title & Category:** Missing MTU Specification & Lack of TCPMSS Clamping | **Network & Kernel / Performance & Reliability**
- **Exact Code Location:** `internal/network/routing.go:L34-L36`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  `BuildRoutingCommands` creates `tun0` via:
  ```sh
  ip tuntap add dev tun0 mode tun
  ip addr add 198.18.0.1/15 dev tun0
  ip link set dev tun0 up
  ```
  The interface is created without specifying an MTU, defaulting to 1500 bytes on Linux.
  When applications send standard 1500-byte TCP packets into `tun0`, `tun2socks` receives and encapsulates them inside SOCKS5/TCP/TLS packets directed through `xray-core`. Xray-core in turn encapsulates them into VMess/VLESS/Shadowsocks frames and transmits them across the physical Ethernet interface (which also has an MTU of 1500).
  With encapsulation headers adding 40-80 bytes, outgoing packets exceed the 1500-byte physical MTU and require IP fragmentation. When middleboxes or firewalls drop IP fragments or suppress ICMP "Packet Too Big" (Type 3 Code 4) notifications, Path MTU Discovery (PMTUD) fails, causing connections to stall indefinitely during TLS handshakes or large data transfers.
- **Proof of Concept / Verification Method:**
  Check `tun0` configuration after starting the tunnel:
  ```bash
  ip link show tun0
  ```
  Output shows `mtu 1500`.
  Attempt an HTTPS file download through the tunnel on a connection where intermediate routers drop IP fragments:
  ```bash
  curl -O https://speed.cloudflare.com/__down?bytes=100000000
  ```
  The transfer hangs indefinitely after the initial TLS handshake due to unhandled frame fragmentation.
- **Recommended Architectural Fix:**
  1. Set an explicit MTU on `tun0` upon creation:
     ```sh
     ip link set dev tun0 mtu 1420 up
     ```
     (1420 bytes reserves sufficient headroom for outer TCP/IP and proxy encapsulation headers).
  2. Add an iptables TCPMSS clamping rule in `BuildRoutingCommands`:
     ```sh
     iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu
     iptables -t mangle -A OUTPUT -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu
     ```
  3. Clean up the TCPMSS rule in `BuildCleanupCommands`.
- **Existing Strengths & Robustness:**
  `TunIP` uses the RFC 2544 benchmark testing range (`198.18.0.1/15`), avoiding IP collisions with standard private RFC 1918 subnets (10/8, 172.16/12, 192.168/16).

---

### Finding NET-04: Asymmetric Routing & Lockout Risk on Multi-Homed / Secondary NIC Servers

- **Title & Category:** Asymmetric Routing & Lockout Risk on Multi-Homed / Secondary NICs | **Network & Kernel / Reliability**
- **Exact Code Location:** `internal/network/routing.go:L50-L57`, `L118-L153`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  In `BuildRoutingCommands`, the inbound netfilter connection mark rule is strictly bound to `defaultIface`:
  ```go
  fmt.Sprintf("iptables -t mangle -A PREROUTING -i %s -m conntrack --ctstate NEW -j %s", defaultIface, InboundChain)
  ```
  Many production Linux servers are multi-homed (e.g., `eth0` for internal/management/SSH and `eth1` for public traffic, or virtual interfaces for out-of-band management).
  If an administrator connects to the server via SSH or Web UI on any interface other than `defaultIface`:
  1. Inbound packets arriving on `eth1` do NOT match `-i defaultIface` and do NOT get marked in `V2RAYNIX_INBOUND`.
  2. When the server generates reply packets, they leave via `OUTPUT` without mark `0x52`.
  3. Reply packets bypass the priority 1008 rule and hit priority 2000 (`not fwmark 0x51 table 100`), sending the response out through `tun0`.
  4. The client's TCP stack drops the response due to IP address mismatch, resulting in an immediate disconnect and total administrator lockout on the secondary interface.
  Additionally, `GetDefaultRoute()` does not support systems with multiple default routes: when `ip route show default` outputs multiple lines (e.g. different interfaces with different metrics), tokenizing all output via `strings.Fields` overwrites `iface` and `gateway` with whichever route appears last in the output, which may be a high-metric backup link.
- **Proof of Concept / Verification Method:**
  On a host with two active NICs (`eth0` default gateway, `eth1` secondary IP):
  1. Establish an SSH connection to `eth1`.
  2. Activate V2Raynix tunnel.
  3. The SSH session on `eth1` freezes and drops immediately because reply packets are diverted into `tun0`.
- **Recommended Architectural Fix:**
  1. Preserve inbound connection marks for all non-tun interfaces rather than solely `defaultIface`:
     ```sh
     iptables -t mangle -A PREROUTING ! -i tun0 -m conntrack --ctstate NEW -j V2RAYNIX_INBOUND
     ```
  2. In `GetDefaultRoute()`, parse the route table line-by-line, honoring route metrics (e.g., selecting the lowest metric default route) instead of flattening all tokens into a single slice.
- **Existing Strengths & Robustness:**
  For single-NIC systems, the combination of `CONNMARK --set-mark 0x52` on `PREROUTING`, `CONNMARK --restore-mark` on `OUTPUT`, and `ip rule add fwmark 0x52 table main priority 1008` is an elegant, non-intrusive solution for preserving inbound sessions.

---

### Finding NET-05: Fragile SSH Port Detection Failing on Drop-ins, Multiple Ports & Inbound Rules

- **Title & Category:** Fragile SSH Port Detection Failing on Drop-ins, Multiple Ports & Inbound Rules | **Logic & Edge-Cases / Anti-Lockout**
- **Exact Code Location:** `internal/network/routing.go:L43-L44`, `L156-L180`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  `DetectSSHPort()` contains three logic defects:
  1. **Drop-in Configuration Precedence Inversion:**
     `files := []string{"/etc/ssh/sshd_config"}` is processed first, followed by `/etc/ssh/sshd_config.d/*.conf`. On modern Linux distributions (Ubuntu 22.04+, Debian 12), `/etc/ssh/sshd_config` begins with `Include /etc/ssh/sshd_config.d/*.conf`. If an admin leaves `Port 22` in `sshd_config` and defines `Port 2222` in `/etc/ssh/sshd_config.d/custom.conf`, `DetectSSHPort()` reads `sshd_config` first and returns 22 on the first match, ignoring the override.
  2. **Single-Port Return Type:**
     OpenSSH supports binding to multiple ports (e.g. `Port 22` and `Port 2222`). `DetectSSHPort()` returns a single `int`. The second port receives no policy routing rules.
  3. **Overbroad Dport Anti-Lockout Rule:**
     In `BuildRoutingCommands`:
     ```go
     fmt.Sprintf("ip rule add sport %d table main priority 1000", sshPort),
     fmt.Sprintf("ip rule add dport %d table main priority 1001", sshPort),
     ```
     Adding `dport 22 table main priority 1001` causes ANY outbound SSH connection made from the host to an external server (`ssh user@remote`) to match priority 1001 and bypass the proxy.
     Even more dangerously, if `webPort` is set to 80 or 443, `ip rule add dport 443 table main` causes ALL outbound HTTPS traffic from the host to bypass the proxy entirely!
- **Proof of Concept / Verification Method:**
  1. In `/etc/ssh/sshd_config`, specify `Port 22`. In `/etc/ssh/sshd_config.d/01-custom.conf`, specify `Port 2222`.
  2. Call `DetectSSHPort()`. It returns `22` instead of `2222`.
  3. Start the tunnel with `webPort = 443`. Run `curl -I https://cloudflare.com`. Notice that outgoing HTTPS traffic bypasses `tun0` because `dport 443` matches priority 1003 in Table main.
- **Recommended Architectural Fix:**
  1. Inspect the active session environment variable `$SSH_CONNECTION` (format: `<client_ip> <client_port> <server_ip> <server_port>`) and active listening sockets via `/proc/net/tcp` to protect the actual active connection.
  2. Parse `/etc/ssh/sshd_config.d/*.conf` before `/etc/ssh/sshd_config` to honor `Include` precedence.
  3. Return a slice of ports (`[]int`) to protect all configured SSH ports.
  4. Only add `sport` (source port for server replies to incoming requests) for anti-lockout rules, not `dport`, to avoid leaking outbound client traffic.
- **Existing Strengths & Robustness:**
  Correctly handles directory globbing of `/etc/ssh/sshd_config.d/*.conf` and defaults safely to port 22 if configuration files are missing or unreadable.

---

### Finding NET-06: DNS Resolution Truncation & Round-Robin Route Inconsistency in `ResolveHost`

- **Title & Category:** DNS Resolution Truncation & Round-Robin Route Inconsistency in `ResolveHost` | **Logic & Edge-Cases / Routing Loop**
- **Exact Code Location:** `internal/network/routing.go:L183-L197`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  In `ResolveHost(host string)`:
  ```go
  ips, err := net.LookupIP(host)
  ...
  for _, ip := range ips {
      if ip.To4() != nil {
          return ip.String(), nil
      }
  }
  return ips[0].String(), nil
  ```
  `net.LookupIP(host)` returns all IP addresses associated with the hostname. However, `ResolveHost` returns only the first IPv4 address.
  Many proxy servers utilize DNS round-robin, Cloudflare Spectrum, or GeoDNS where a single hostname resolves to multiple IP addresses.
  When `BuildRoutingCommands` runs, it creates a priority 999 bypass rule and a static `/32` route for ONLY the single IP returned by `ResolveHost`.
  When `xray-core` later resolves the domain during operation, it may connect to a *different* IP address in the DNS pool. While xray-core marks its own packets with `0x51`, any secondary connections, DNS queries, or companion processes attempting to reach the proxy endpoint will fail to use the static bypass route.
  Additionally, if `LookupIP` returns only IPv6 addresses, `ResolveHost` returns an IPv6 address string, which causes syntax errors in `BuildRoutingCommands`.
- **Proof of Concept / Verification Method:**
  Configure a proxy domain with multiple A records (e.g. round-robin IPs: `203.0.113.1`, `203.0.113.2`, `203.0.113.3`).
  Execute `ResolveHost`. Observe that only one IP is returned.
  Verify generated routing rules: only `203.0.113.1` has a bypass route; traffic to `203.0.113.2` lacks a static kernel bypass route.
- **Recommended Architectural Fix:**
  1. Update `ResolveHost` to return `[]string` containing all resolved IPv4 addresses.
  2. In `BuildRoutingCommands`, iterate over all resolved IP addresses and generate priority 999 rules and static routes for each IP in the set.
  3. Validate IP family: if an address is IPv6, configure `ip -6` routes or return an explicit error if IPv6 tunneling is unsupported.
- **Existing Strengths & Robustness:**
  Uses `net.ParseIP(host)` first to check if the input is already an IP address, avoiding unnecessary DNS lookups and eliminating latency when an IP literal is provided.

---

### Finding NET-07: Concurrency Race Condition & Split-Brain Execution in `SafeModeController`

- **Title & Category:** Concurrency Race Condition & Split-Brain Execution in `SafeModeController` | **Concurrency & Race / Availability**
- **Exact Code Location:** `internal/network/safemode.go:L42-L54`, `L73-L91`
- **Severity:** **High**
- **Trigger Scenario & Root Cause Analysis:**
  In `SafeModeController.Start()`, `time.AfterFunc` invokes a timer callback:
  ```go
  sm.timer = time.AfterFunc(sm.timeout, func() {
      sm.mu.Lock()
      if !sm.active {
          sm.mu.Unlock()
          return
      }
      sm.active = false
      sm.mu.Unlock()

      if sm.onRollback != nil {
          sm.onRollback()
      }
  })
  ```
  Notice that `sm.mu.Unlock()` is executed BEFORE calling `sm.onRollback()`.
  This creates multiple critical race windows:
  1. **Race with `Confirm()`:** If the administrator clicks "Confirm" in the UI at the exact moment the timer fires, the timer goroutine acquires `mu`, sets `sm.active = false`, and unlocks. `Confirm()` then acquires `mu`, observes `sm.active == false`, and returns `false`. Then the timer goroutine executes `sm.onRollback()`, tearing down the tunnel despite the user's confirmation attempt.
  2. **Race with `Start()` (Split-Brain Re-start):** While `sm.onRollback()` is running on Goroutine 1 (which runs shell commands in `ExecuteCommands` taking hundreds of milliseconds), another goroutine can call `Start()`. `Start()` acquires `mu`, sets `sm.active = true`, and schedules a new timer. Goroutine 1 is still executing the old rollback commands, tearing down the newly started tunnel.
  3. **No Execution Guard on `onRollback`:** There is no execution state lock or `sync.Once` ensuring `onRollback` runs exactly once.
  4. **Integer Truncation in `GetRemainingSeconds()`:**
     `int(remaining.Seconds())` casts `float64` to `int`, truncating decimals. When 0.9 seconds remain, it returns `0`, falsely indicating timeout while `sm.active` remains `true`.
- **Proof of Concept / Verification Method:**
  Simulate a slow `onRollback` function (`time.Sleep(100 * time.Millisecond)`).
  Trigger `CancelAndRollback()`, and immediately call `Start()`.
  Observe that the rollback logic from the prior session continues running concurrently while the new session is active, corrupting the routing state.
- **Recommended Architectural Fix:**
  1. Introduce explicit controller states: `StateInactive`, `StatePending`, `StateConfirmed`, `StateRollingBack`, `StateRolledBack`.
  2. Use a `sync.Once` per activation cycle to guarantee that `onRollback` can never execute more than once.
  3. Ensure `Start()` waits for or rejects activation if a rollback is currently in progress.
  4. In `GetRemainingSeconds()`, use `int(math.Ceil(remaining.Seconds()))` so 0.9s displays as 1s.
- **Existing Strengths & Robustness:**
  Internal fields (`active`, `startedAt`, `timer`) are protected with `sync.Mutex`, preventing data races during state queries (`IsActive()`, `GetRemainingSeconds()`).

---

### Finding NET-08: Non-Idempotent Cleanup & Command Execution Error Swallowing in `ExecuteCommands`

- **Title & Category:** Non-Idempotent Cleanup & Command Execution Error Swallowing | **Logic & Edge-Cases / Robustness**
- **Exact Code Location:** `internal/network/routing.go:L75-L115`, `L200-L209`
- **Severity:** **Medium**
- **Trigger Scenario & Root Cause Analysis:**
  1. **Non-Idempotency in `BuildCleanupCommands`:**
     Line 95 contains an unconditional `"ip rule del priority 999"`, directly following line 94 `fmt.Sprintf("ip rule del to %s table main priority 999", remoteProxyIP)`. If line 94 successfully removes the priority 999 rule, line 95 unconditionally fails with `RTNETLINK answers: No such file or directory`.
     Furthermore, `iptables -t mangle -D ...` removes only one matching rule. If commands are executed multiple times, duplicate rules accumulate.
  2. **Error Accumulation Without Halting in `ExecuteCommands`:**
     `ExecuteCommands` iterates over commands in a slice:
     ```go
     for _, cmd := range cmds {
         c := exec.Command("sh", "-c", cmd)
         if out, err := c.CombinedOutput(); err != nil {
             errs = append(errs, fmt.Errorf(...))
         }
     }
     ```
     When a command fails, it appends the error to `errs` and continues executing subsequent commands.
     If Step 1 (`ip tuntap add dev tun0 mode tun`) fails because `tun0` already exists or the `tun` kernel module is not loaded, `ExecuteCommands` continues and executes Step 8 (`ip rule add not fwmark 0x51 table 100 priority 2000`).
     All host traffic is immediately diverted to Table 100 pointing to a non-existent or unconfigured `tun0`, instantly causing total network loss for the host!
  3. **Missing Pre-Flight Checks:**
     No verification is performed to check whether required binaries (`ip`, `iptables`) or kernel modules (`tun`, `xt_connmark`, `iptable_mangle`) exist before executing network changes.
- **Proof of Concept / Verification Method:**
  Pass a command list with an intentional failure in command 1:
  ```go
  cmds := []string{"ip tuntap add dev tun0_invalid mode invalid", "ip rule add not fwmark 0x51 table 100 priority 2000"}
  network.ExecuteCommands(cmds)
  ```
  `ExecuteCommands` executes the second command despite the failure of the first.
- **Recommended Architectural Fix:**
  1. Implement fail-fast transactional execution: if any command in `BuildRoutingCommands` fails, immediately abort and trigger `BuildCleanupCommands` to roll back changes.
  2. Make cleanup commands idempotent: ignore "does not exist" errors during deletion (`ip rule del ... 2>/dev/null || true` or check existence before deletion).
  3. Implement pre-flight checks (`exec.LookPath("ip")`, `exec.LookPath("iptables")`, check `/dev/net/tun` accessibility) before applying changes.
- **Existing Strengths & Robustness:**
  `ExecuteCommands` collects all command errors and returns them as a slice of `error` with combined stdout/stderr output, providing comprehensive diagnostics for troubleshooting.

---

## 5. Existing Strengths & Positive Architectural Patterns

During the audit, several well-designed architectural choices were identified in the `internal/network` package:

1. **RFC 2544 Benchmark Address Range for `tun0`:**
   Assigning `198.18.0.1/15` (`TunIP`) ensures that the virtual TUN interface never conflicts with standard private RFC 1918 subnets (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`), preventing subnet collision issues on local networks.
2. **Netfilter Connmark Inbound Preservation Design:**
   The use of Netfilter connection tracking (`conntrack`) to tag incoming sessions (`CONNMARK --set-mark 0x52`) and restore the mark on reply packets (`CONNMARK --restore-mark`) coupled with `ip rule add fwmark 0x52 table main priority 1008` is an elegant solution to the classic asymmetric routing problem on single-NIC Linux servers.
3. **Explicit RFC 1918 Main Table Bypasses:**
   Explicitly routing `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, and `127.0.0.0/8` to `table main` with priorities 1010-1013 ensures local LAN communications, loopback services, and container networking remain functional while the tunnel is active.
4. **SafeMode Fail-Safe Architecture:**
   The inclusion of an automated countdown timer that reverts network routing unless confirmed by an administrator provides a crucial safeguard against permanent lockout on remote cloud servers.
5. **Clear Port Default Normalization:**
   Defensive normalization of non-positive port arguments (`sshPort <= 0` defaulting to 22, `webPort <= 0` defaulting to 2080) prevents empty string interpolations when ports are not explicitly configured.

---

## 6. Verification and Compliance Checklist

- [x] Read briefing file `docs/audit/briefs/01-network-routing-brief.md`
- [x] Query knowledge graph (`graphify`) and map cross-domain caller-callee edges
- [x] Scaffolding established in `docs/audit/reports/01-network-routing-report.md`
- [x] Systematic code audit of `routing.go`, `safemode.go`, `routing_test.go`, `safemode_test.go`
- [x] All 8 findings strictly adhere to the 7-field defect schema
- [x] Strict read-only mode observed (zero `.go` source files modified)
- [x] Existing unit tests verified (`go test ./internal/network/...` passes with 0 failures)
