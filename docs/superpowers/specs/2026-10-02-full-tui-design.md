# Specification: V2Raynix v0.9.2 Full Terminal User Interface (TUI)

**Topic:** Full Terminal User Interface (TUI) mirroring all Web Dashboard capabilities  
**Date:** 2026-10-02  
**Status:** Approved by User  
**Target Version:** v0.9.2-beta  

---

## 1. Executive Summary

This specification defines the comprehensive Terminal User Interface (TUI) subsystem for V2Raynix v0.9.2. The goal is to provide a complete, standalone command-line environment inside headless SSH sessions that mirrors 100% of the Web Dashboard's capabilities—allowing full system administration, node management, policy routing, live logging, health monitoring, and core updating without opening a web browser.

---

## 2. Architecture & Design Principles

### 2.1 Invocation & Terminal Detection
1. **Interactive Root Command:**
   - Running `v2raynix` without flags inside a TTY (detected via `golang.org/x/term.IsTerminal`) automatically opens the Full TUI.
   - Subcommands `v2raynix menu` and `v2raynix setup` open the same comprehensive TUI.
2. **Backward-Compatible Non-Interactive Flags:**
   - If daemon flags are provided (`-port`, `-data-dir`, `-init-password`, `-mock`, `-version`), V2Raynix runs as a daemon/CLI tool without launching the TUI.
   - Non-interactive flags for `v2raynix setup` (`--user`, `--pass`, `--port`, `--service`) remain functional for automated shell scripts and installers.

### 2.2 Dual-Mode Engine Coordination
The TUI operates in one of two modes:
1. **Daemon-Connected Mode (Default when `v2raynix.service` is active):**
   - Detects the local daemon on the configured web port (`http://127.0.0.1:<port>`).
   - Obtains or uses an authenticated local API session.
   - Routes actions (tunnel connection, disconnection, safe mode commit/rollback, health check, ping testing, live log streaming) through the running daemon's REST API.
   - Guarantees zero state desynchronization or database locking conflicts between web and terminal.
2. **Direct-Store Mode (When daemon is inactive or stopped):**
   - Operates directly on the JSON data store (`/etc/v2raynix/v2raynix.json`).
   - Allows importing/deleting configs, editing routing rules, changing ports and passwords, and provides one-click commands to start the systemd daemon.

### 2.3 Visual Design & ANSI Palette
- Pure ANSI escape codes without external CGO curses dependencies, ensuring 100% compatibility with PuTTY, Termius, OpenSSH, xterm, and Windows Terminal.
- Standard Box-Drawing characters (`┌`, `─`, `┐`, `│`, `└`, `┘`, `├`, `┤`).
- Unified navigation rules:
  - Entering `0` or `b` / `B` returns to the parent menu.
  - Entering `q` / `Q` exits the TUI cleanly.
  - Clear screen on menu transitions with preserved error/status banners.

---

## 3. Detailed Menu Hierarchy & Functional Specifications

### 3.1 Main Dashboard & Status Header
Every view renders a dynamic, live status card at the top:
- **Service State:** Active (Running) / Inactive (Stopped) with colored status bullet.
- **Tunnel State:** Connected (Config Name & Protocol) / Disconnected / Connecting.
- **Real-Time Traffic:** Live Upload & Download speeds (Bps / KB/s / MB/s).
- **Uptime & Safe Mode:** Duration of active tunnel session; Safe Mode remaining seconds or "Inactive".
- **Health State:** Healthy (latency in ms) / Unhealthy / Not verified.
- **Web UI URL:** `http://<ip>:<port>`.

### 3.2 Menu [1]: Tunnel & Connectivity Operations
- **[1] Connect Config:** Displays numbered list of configs; user selects number to initiate tunnel.
- **[2] Disconnect Tunnel:** Cleanly tears down active routing and tun0.
- **[3] Safe Mode Controls:**
  - If Safe Mode is active: Option to **[C] Confirm Changes** (make permanent) or **[R] Rollback Now** (emergency restore).
- **[4] Quick Health Check:** Sends an immediate HTTP probe and displays latency / status.

### 3.3 Menu [2]: Proxy Configurations Management
- **[1] List Configurations:** Renders formatted table:
  ```text
  ┌────┬──────────────────────┬──────────┬────────────────────────┬─────────┬──────────┬────────┐
  │ #  │ Name                 │ Protocol │ Address:Port           │ Ping    │ HTTP Lat │ Status │
  ├────┼──────────────────────┼──────────┼────────────────────────┼─────────┼──────────┼────────┤
  │ 1  │ EU - High Speed      │ vless    │ eu.server.com:443      │ 42ms    │ 128ms    │ ACTIVE │
  │ 2  │ US - Fallback        │ vmess    │ us.server.com:8443     │ 180ms   │ -        │        │
  └────┴──────────────────────┴──────────┴────────────────────────┴─────────┴──────────┴────────┘
  ```
- **[2] Import from Share Link:** Prompts for `vless://`, `vmess://`, `trojan://`, or `ss://` URI with validation.
- **[3] Import from JSON File:** Prompts for file path to raw config JSON.
- **[4] Run Batch Latency Test:** Tests TCP ping and HTTP real delay across all configs concurrently with a progress indicator.
- **[5] Test Single Config:** Select by number to run isolated ping/HTTP test.
- **[6] Delete Config:** Prompts for config number with confirmation.

### 3.4 Menu [3]: Smart Policy Routing Rules
- **[1] View Active Rules:** Lists all routing rules sorted by priority (Target, Type, Outbound: Direct / Proxy / Block).
- **[2] Add Routing Rule:** Interactive wizard (Type: Domain / IP / CIDR; Target string; Outbound).
- **[3] Apply Recommended Presets:**
  - One-click install: Bypass Domestic Services (`geosite:category-ir`, `geoip:ir` $\to$ Direct).
  - One-click install: Block Ads & Trackers (`geosite:category-ads-all` $\to$ Block).
- **[4] Delete / Toggle Rule:** Select rule by number to remove or enable/disable.

### 3.5 Menu [4]: Real-Time System & Engine Logs
- **[1] View Recent Logs:** Prints the last 50 log lines with colorized timestamps, components (`[CORE]`, `[TUNNEL]`, `[WATCHDOG]`), and levels (`INFO`, `WARN`, `ERROR`).
- **[2] Live Log Stream (Tail):** Continuously streams new log lines to terminal in real time until the user presses Enter or `q`.

### 3.6 Menu [5]: System Settings & Core Maintenance
- **[1] Change Admin Credentials:** Silent password input with verification (no characters echoed).
- **[2] Change Web Panel Port:** Validates port range (1-65535) and updates configuration.
- **[3] Configure Safe Mode Timeout:** Sets failsafe duration in seconds (e.g. 60-300s).
- **[4] Configure Health Check Watchdog:** Configures interval in minutes and probe URL.
- **[5] Core Engine Updater:**
  - Checks GitHub for latest releases of `Xray-core` and `tun2socks`.
  - Displays current vs latest version.
  - Option to trigger one-click in-place update with checksum validation.

### 3.7 Menu [6]: Service Supervisor
- **[1] Service Status:** Runs `systemctl status v2raynix` and prints output.
- **[2] Start Service:** Launches background daemon.
- **[3] Stop Service:** Stops background daemon.
- **[4] Restart Service:** Restarts background daemon.
- **[5] Enable on Boot:** Ensures `systemctl enable v2raynix` is configured.

---

## 4. Testing & Verification

1. **Unit Testing:**
   - Test menu rendering, table formatting, and input parsing (`internal/cli/tui_test.go`).
   - Test non-interactive backward compatibility (`internal/cli/setup_test.go`).
   - Test local daemon API client integration (`internal/cli/client_test.go`).
2. **Interactive Simulation:**
   - Run `v2raynix` in mock mode and verify menu navigation, back shortcuts (`0`/`b`), and exit (`q`).
3. **Build & Version Verification:**
   - Build standalone binary and bump version to `v0.9.2-beta`.
