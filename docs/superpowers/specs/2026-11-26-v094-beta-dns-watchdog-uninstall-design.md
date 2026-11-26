# Specification: V2Raynix v0.9.4-beta Improvements & Architectural Hardening

- **Status:** APPROVED & IN SPECIFICATION
- **Date:** 2026-11-26
- **Target Release:** `v0.9.4-beta`
- **Authors:** Lead Architect & Controller

---

## 1. Executive Summary

Release `v0.9.4-beta` addresses five critical operational, network, and installation challenges identified during real-world deployments on Linux servers:

1. **DNS-over-Tunnel & Censorship Poisoning Bypass:** Filtered applications (such as Telegram) fail because Iranian ISP DNS middleboxes forge responses to private RFC1918 addresses (`10.x.x.x` / `127.0.0.1`), which bypass `tun0` due to RFC1918 policy routing rules. V2Raynix will route system DNS queries through the encrypted tunnel (`1.1.1.1`, `8.8.8.8`) using `resolvectl` and `/etc/resolv.conf` management, configurable via Web UI and TUI (Default: Enabled).
2. **Watchdog Decoupling & Safe Mode Trap Elimination:** Watchdog auto-recovery previously re-armed the 120-second Safe Mode countdown timer on internal restarts. Since restarts happen unattended in the background, Safe Mode would time out and terminate the tunnel. Internal Watchdog restarts will now strictly bypass Safe Mode, probe timeouts will be increased, and DNS resolution during reconnects will be hardened.
3. **In-Flight Web Connection Protection (Zero-Timeout Connect):** During `POST /api/tunnel/connect`, packet flow from existing connections was previously intercepted by the newly activated `table 100` before receiving connection marks, causing the browser to timeout until refreshed. Netfilter conntrack rules will now protect `ESTABLISHED,RELATED` states and reverse-proxy ports (80/443), and the Web API client timeout will be expanded.
4. **Installer Modernization & PATH Parity:** Fix the `/dev/fd/63: No such file or directory` error caused by running `sudo bash <(curl ...)` with process substitution by adopting `curl ... | sudo bash`, automatically creating universal `/usr/bin/` symlinks for `v2raynix`, `xray`, and `tun2socks`, and resolving absolute executable paths via `os.Executable()`.
5. **Universal Uninstall Engine:** Introduce a fail-safe uninstallation capability across CLI (`v2raynix uninstall [--purge]`), installer (`install.sh --uninstall`), and TUI (Menu 6 option 4), guaranteeing Zero-Lockout network restoration (dropping priority 2000 first), systemd service removal, core cleanup, and optional configuration retention.

---

## 2. Subsystem Specifications

### 2.1 DNS-over-Tunnel & Anti-Poisoning Architecture

#### 2.1.1 Root Cause & Mechanics
When an application on the host requests a domain name (e.g. `api.telegram.org`), the OS queries the resolvers listed in `/etc/resolv.conf`. In typical Linux installations, these resolvers are local (`127.0.0.53`, `192.168.x.x`, or `10.x.x.x`). 
Because V2Raynix installs high-priority bypass rules for RFC1918 subnets (priorities 1010–1013 in `table main`), DNS queries travel out of the unencrypted physical interface. The ISP intercepts UDP port 53 and injects a poisoned IP (e.g. `10.10.34.34`). When the application tries to establish a TCP handshake to `10.10.34.34`, rule 1010 forces the connection to the domestic network, bypassing `tun0` and Xray completely.

#### 2.1.2 Solution Design
A new package module `internal/network/dns.go` will manage host DNS lifecycle during tunnel transitions:

- **Systemd-Resolved (Modern Linux / Debian / Ubuntu / Fedora / Arch):**
  When the tunnel activates and `DNSOverTunnel == true`:
  ```bash
  # 1. Direct tun0 to secure remote DNS servers
  resolvectl dns tun0 1.1.1.1 8.8.8.8

  # 2. Assign routing domain "~." to force all system lookups through tun0
  resolvectl domain tun0 "~."

  # 3. Designate tun0 as the default DNS route
  resolvectl default-route tun0 yes

  # 4. Flush stale / poisoned resolver caches
  resolvectl flush-caches
  ```
  Upon tunnel disconnect or rollback:
  ```bash
  resolvectl revert tun0
  resolvectl flush-caches
  ```

- **Static Resolv.conf Fallback (Alpine / Minimal / Non-systemd):**
  - On start: Backup `/etc/resolv.conf` to `/etc/v2raynix/resolv.conf.bak` and write:
    ```text
    nameserver 1.1.1.1
    nameserver 8.8.8.8
    ```
  - On stop: Restore `/etc/resolv.conf` from backup and remove backup file.

- **Data Model (`internal/store/models.go`):**
  ```go
  type SystemSettings struct {
      WebPort                    int    `json:"webPort"`
      SafeModeSeconds            int    `json:"safeModeSeconds"`
      AutoStartTunnel            bool   `json:"autoStartTunnel"`
      HealthCheckIntervalMinutes int    `json:"healthCheckIntervalMinutes"`
      HealthCheckURL             string `json:"healthCheckURL"`
      DNSOverTunnel              bool   `json:"dnsOverTunnel"`              // Default: true
      RemoteDNS                  string `json:"remoteDNS"`                  // Default: "1.1.1.1,8.8.8.8"
  }
  ```

- **User Control:**
  - **Web UI:** Toggle in Settings view ("DNS & Security" section) to enable/disable DNS Over Tunnel and specify custom upstream DNS resolvers.
  - **TUI:** Menu 5 (Settings) includes sub-option to toggle DNS Over Tunnel and update remote resolvers.

---

### 2.2 Watchdog Decoupling & Safe Mode Trap Elimination

#### 2.2.1 Root Cause & Mechanics
Currently in `internal/core/supervisor.go:280`, `StartTunnel()` unconditionally calls `startSafeModeTimerLocked()`.
When `internal/core/watchdog.go` experiences 2 consecutive probe failures (5-second timeout, 3-second retry), it invokes `s.RestartTunnel()`.
`RestartTunnel()` stops the tunnel and calls `StartTunnel()`, which starts a fresh 120-second Safe Mode countdown in the background. Because this restart is automated and unattended, no user confirms it in the UI. At $t = 120\text{s}$, the timer fires `StopTunnel()`, permanently killing the connection.

#### 2.2.2 Solution Design
1. **Parameterize Safe Mode in `StartTunnel`:**
   Modify `StartTunnel` to accept a boolean flag or provide a dedicated method:
   ```go
   func (s *Supervisor) StartTunnel(cfg *store.ConfigItem) error {
       return s.startTunnelInternal(cfg, true) // arm Safe Mode for user-initiated starts
   }

   func (s *Supervisor) RestartTunnel() error {
       // Auto-restart MUST NOT re-arm Safe Mode!
       return s.startTunnelInternal(cfg, false)
   }
   ```
2. **Watchdog Sensitivity Hardening:**
   - Probe timeout expanded from 5s to **10s**.
   - Number of retries expanded from 2 to **3**.
   - Exponential backoff retry delays: 5s, 10s.
   - Support secondary probe endpoint if primary fails (e.g. `http://www.google.com/generate_204` or `http://cp.cloudflare.com/generate_204`).
3. **Resilient Reconnection Loop:**
   - In `watchdog.go`, do not permanently exit on failed `RestartTunnel()`. If the initial restart attempt fails (because the local network is still flapping), enter an exponential backoff loop (up to 5 attempts: 5s, 10s, 20s, 30s) before transitioning state to `disconnected`.
4. **Proxy IP Caching:**
   - When resolving `cfg.Server` on tunnel start, cache the resolved IP (`s.cachedProxyIP`). During auto-reconnects when external DNS may be temporarily uncontactable, use `s.cachedProxyIP` to prevent invalid `/32` routes that trigger routing loops.
5. **Headless CLI Parity:**
   - In `internal/cli/commands.go`, `v2raynix connect <node>` will automatically confirm safe mode unless an explicit `--safe-mode` flag is provided, preventing unattended CLI connections from rolling back at 120s.

---

### 2.3 Web Connect In-Flight Response Protection

#### 2.3.1 Root Cause & Mechanics
When a user clicks "Connect" in the Web UI:
1. Browser opens an HTTP TCP socket and sends `POST /api/tunnel/connect`.
2. Inside `StartTunnel()`, `BuildRoutingCommands()` runs:
   ```bash
   iptables -t mangle -A PREROUTING -i eth0 -m conntrack --ctstate NEW -j V2RAYNIX_INBOUND
   iptables -t mangle -A OUTPUT -m connmark --mark 0x52 -j CONNMARK --restore-mark
   ip rule add not fwmark 0x51 table 100 priority 2000
   ```
3. Because `V2RAYNIX_INBOUND` only matches `--ctstate NEW`, the pre-existing HTTP connection (which is in `ESTABLISHED` state) does not receive mark `0x52`.
4. The server generates the HTTP 200 response. The outgoing packet has `sport 2080`, but lacks mark `0x52`. If accessed via reverse proxy (ports 80/443), it matches `not fwmark 0x51 table 100 priority 2000` and is routed into `tun0` instead of back to the browser!
5. Browser times out after 15s. Upon manual refresh, a `NEW` connection is opened, which receives mark `0x52`, hits `priority 1008 table main`, and loads properly.

#### 2.3.2 Solution Design
1. **Netfilter Inbound Protection for Established Connections:**
   In `internal/network/routing.go`, update the conntrack rule to mark both `NEW` and `ESTABLISHED,RELATED` inbound traffic on physical interface:
   ```bash
   iptables -t mangle -A PREROUTING -i <iface> -m conntrack --ctstate NEW,ESTABLISHED,RELATED -j V2RAYNIX_INBOUND
   ```
2. **Reverse Proxy Port Inbound Rules:**
   Add explicit bypass priority rules for common reverse-proxy ports (80 and 443) if detected, ensuring web server responses are never diverted into `tun0`.
3. **Frontend Request Timeout:**
   In `web/src/services/api.js`, increase timeout for `connectTunnel` and `disconnectTunnel` from default 15s to **30s**.

---

### 2.4 Installer Hardening, `/dev/fd/63` Elimination & PATH Parity

#### 2.4.1 Root Cause & Mechanics
1. Running `sudo bash <(curl -Ls ...)` causes Bash to use process substitution via `/dev/fd/63`. `sudo` is SetUID and closes non-standard file descriptors (`closefrom(3)`). In the root bash child process, `/dev/fd/63` does not exist in `/proc/self/fd/`, causing `bash: /dev/fd/63: No such file or directory`.
2. Binaries (`v2raynix`, `xray`, `tun2socks`) were only installed to `/usr/local/bin/`. On Debian non-login shells and systems with strict `secure_path` in `/etc/sudoers`, `/usr/local/bin` is excluded from `$PATH`.
3. In `cmd/v2raynix/main.go`, self-elevation executed `sudo v2raynix`, passing the bare relative string rather than the resolved executable path (`os.Executable()`).

#### 2.4.2 Solution Design
1. **README & Installation Pipe:**
   Standardize the official installation command across all documentation to:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/ImMahdi/V2Raynix/main/scripts/install.sh | sudo bash
   ```
2. **Auto-Elevation in `scripts/install.sh`:**
   If executed as non-root from a file on disk, auto-elevate with `exec sudo -E bash "$0" "$@"`. If piped from stdin without root, print the exact `curl ... | sudo bash` command and exit cleanly.
3. **Universal `/usr/bin` Symlinks:**
   In `scripts/install.sh`:
   ```bash
   mkdir -p /usr/bin
   ln -sf /usr/local/bin/v2raynix /usr/bin/v2raynix
   ln -sf /usr/local/bin/xray /usr/bin/xray
   ln -sf /usr/local/bin/tun2socks /usr/bin/tun2socks
   ```
4. **Self-Elevation Fix in `cmd/v2raynix/main.go`:**
   Resolve `exePath, err := os.Executable()` and pass `exePath` as `argv[0]` to `sudo`.
5. **Non-Interactive Package Management:**
   Use `export DEBIAN_FRONTEND=noninteractive` and redirect stdin `</dev/null` for `apt-get` / `dnf` in `install.sh` to prevent stealing pipe input.

---

### 2.5 Universal Uninstall Engine

#### 2.5.1 Subsystem Design
V2Raynix must provide a clean, complete uninstallation mechanism that guarantees zero SSH lockouts.

1. **CLI Subcommand:**
   ```bash
   v2raynix uninstall [flags]
   ```
   **Flags:**
   - `--purge`: Delete all persistent configuration (`/etc/v2raynix`), core binaries (`xray`, `tun2socks`), and firewall rules.
   - `--keep-data`: Retain `/etc/v2raynix/v2raynix.json` (Default non-interactive mode).
   - `--remove-cores`: Remove `/usr/local/bin/xray`, `/usr/local/bin/tun2socks`, `/usr/local/share/xray`.
   - `--remove-firewall`: Remove web port from `ufw` / `firewalld`.
   - `--yes`, `-y`: Non-interactive auto-confirm.

2. **Installer Switch:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/ImMahdi/V2Raynix/main/scripts/install.sh | sudo bash -s -- --uninstall [--purge]
   # Or locally:
   sudo ./scripts/install.sh --uninstall [--purge]
   ```
   If `/usr/local/bin/v2raynix` exists, delegates to `v2raynix uninstall "$@"`. If binary is missing, executes standalone bash cleanup.

3. **TUI Menu Integration:**
   In `internal/cli/menu_service.go` (Menu 6: Core Engines & Service Management):
   Option `[4] Uninstall V2Raynix from System` with confirmation prompts.

#### 2.5.2 Safe Teardown Execution Order (Zero-Lockout)
```
[Start Uninstall]
       │
       ▼
1. Drop priority 2000 rule (`ip rule del priority 2000`)
   --> General traffic immediately restores to default physical gateway!
       │
       ▼
2. Flush custom table 100 (`ip route flush table 100`)
       │
       ▼
3. Delete tun0 device (`ip tuntap del dev tun0 mode tun` / `ip link delete tun0`)
       │
       ▼
4. Delete bypass priority rules (999, 1000-1003, 1008, 1010-1013)
       │
       ▼
5. Flush and delete iptables mangle chain `V2RAYNIX_INBOUND`
       │
       ▼
6. Revert DNS settings (`resolvectl revert tun0` or restore `/etc/resolv.conf`)
       │
       ▼
7. Stop & disable systemd service (`systemctl stop/disable v2raynix`)
       │
       ▼
8. Terminate child processes (`pkill -f xray-active.json`, `pkill -f tun2socks`)
       │
       ▼
9. Remove service file (`/etc/systemd/system/v2raynix.service`) & `systemctl daemon-reload`
       │
       ▼
10. Data handling: Purge `/etc/v2raynix` OR retain `v2raynix.json` and clean temporary files
       │
       ▼
11. Core handling: If opted-in, remove `xray`, `tun2socks`, `geosite.dat`, `geoip.dat`
       │
       ▼
12. Firewall: If opted-in, remove port 2080 from UFW / firewalld
       │
       ▼
13. Delete binaries & symlinks: `/usr/local/bin/v2raynix`, `/usr/bin/v2raynix`, `/tmp/v2raynix*`
       │
       ▼
[Exit 0: Complete]
```

---

## 3. Implementation Plan & File Touch Points

| File | Subsystem | Action |
|---|---|---|
| `internal/store/models.go` | Store | Add `DNSOverTunnel bool`, `RemoteDNS string` to `SystemSettings` |
| `internal/store/store.go` | Store | Set defaults (`DNSOverTunnel: true`, `RemoteDNS: "1.1.1.1,8.8.8.8"`) |
| `internal/network/dns.go` (new) | DNS | Implement `ApplyDNSOverTunnel`, `RevertDNSOverTunnel` using `resolvectl` & `/etc/resolv.conf` |
| `internal/network/dns_test.go` (new) | DNS | Unit tests for DNS command construction and failure resilience |
| `internal/core/supervisor.go` | Core | Decouple Safe Mode from internal restarts; invoke DNS manager on connect/disconnect; cache proxy IP |
| `internal/core/watchdog.go` | Watchdog | Expand probe timeout to 10s, 3 retries, exponential backoff, do not re-arm Safe Mode |
| `internal/core/supervisor_watchdog_test.go` | Watchdog | Unit test verifying auto-restart does NOT re-arm Safe Mode timer |
| `internal/network/routing.go` | Routing | Include `ESTABLISHED,RELATED` in conntrack rules; add reverse-proxy ports protection |
| `internal/network/cleanup.go` (new) | Cleanup | Standalone unconditional kernel route & iptables purge helper |
| `internal/cli/uninstall.go` (new) | CLI | Native uninstaller routine with interactive and flag modes |
| `internal/cli/uninstall_test.go` (new) | CLI | Unit tests for uninstall options and flags |
| `internal/cli/commands.go` | CLI | Wire `uninstall` subcommand; auto-confirm safe mode on `connect` unless `--safe-mode` |
| `internal/cli/commands_test.go` | CLI | Unit tests for `v2raynix uninstall` subcommand |
| `internal/cli/menu_service.go` | TUI | Add option `[4] Uninstall V2Raynix` |
| `internal/cli/menu_settings.go` | TUI | Add DNS-over-Tunnel toggle & configuration |
| `web/src/services/api.js` | Web API | Expand `connectTunnel` timeout to 30s |
| `web/src/views/SettingsView.jsx` | Web UI | Add "DNS & Security" section with toggle and custom DNS input |
| `cmd/v2raynix/main.go` | Binary | Self-elevation with `os.Executable()`; register `uninstall` |
| `scripts/install.sh` | Installer | Symlink `/usr/bin`, fix non-root elevation, non-interactive apt, `--uninstall` switch |
| `README*.md` (all 4) | Docs | Update install one-liner, document `uninstall` and DNS settings |

---

## 4. Verification & Testing Strategy

1. **DNS over Tunnel Testing:**
   - Unit test `internal/network/dns_test.go` verifying correct `resolvectl` and `/etc/resolv.conf` command sequences.
   - Live server verification: verify `resolvectl status tun0` displays `1.1.1.1 8.8.8.8` and domain `~.` while connected; verify Telegram connects and DNS leaks are eliminated.
2. **Watchdog Decoupling Testing:**
   - Unit test simulating failed probes and verifying `RestartTunnel()` restores tunnel state without starting `safeMode.timer`.
   - Verify tunnel remains connected beyond 120 seconds after an automated restart.
3. **In-Flight Connect Response Testing:**
   - Measure `POST /api/tunnel/connect` HTTP response time; verify browser receives response immediately without timeout or hanging.
4. **Installer & PATH Testing:**
   - Test execution of `install.sh` under non-root user with `sudo`.
   - Verify `which v2raynix` returns `/usr/bin/v2raynix` and `v2raynix status` runs in non-login subshells.
5. **Uninstall Verification:**
   - Execute `v2raynix uninstall --keep-data --yes` and verify:
     - Active SSH connection is maintained without interruption.
     - `ip rule` shows no residual table 100 or V2Raynix fwmark rules.
     - `systemctl status v2raynix` confirms service removed.
     - `/etc/v2raynix/v2raynix.json` is preserved while runtime logs are deleted.
     - Binary is unlinked cleanly.
