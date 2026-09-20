# System Architecture & Design Specification: V2Raynix

**Document ID:** 2026-09-20-v2raynix-design  
**Date:** 2026-09-20  
**Status:** Approved by User  
**Target:** Linux Servers & Workstations (Native Systemd Service / Standalone Binary)

---

## 1. Executive Summary & Goals

**V2Raynix** is a high-performance, single-binary Linux management suite and Web UI for V2Ray / Xray proxy configurations. It provides:
1. **Full-system network tunneling** via `tun2socks` and a virtual `tun0` interface.
2. **Multi-configuration management** with instant, zero-downtime switching among stored configs without manual file edits.
3. **Dual-layer intelligent routing** with permanent SSH/WebUI anti-lockout protection and custom user-defined domain/IP rules (`Direct`, `Proxy`, `Block`).
4. **Network Safe Mode (Commit Confirmed):** A failsafe 2-minute countdown timer for network and routing modifications that automatically rolls back if the user does not explicitly confirm stability.
5. **Real-time diagnostics:** TCP ping and real HTTP handshake latency measurement per configuration.
6. **Embedded Modern Web UI:** A responsive React (Vite) interface embedded directly inside the Go executable via `go:embed`.

---

## 2. Technology Stack & Runtime Architecture

### 2.1 Backend Daemon (Go / Golang)
- **Language:** Go 1.22+
- **Distribution Model:** Single standalone binary bundling backend HTTP server, core orchestrator, and embedded Web UI static assets.
- **Embedded Web UI:** Served directly from memory via Go's `embed.FS` (`//go:embed web/dist/*`).
- **Data Persistence:** Embedded SQLite or file-based key-value store stored securely at `/etc/v2raynix/v2raynix.db` (permissions `0600`).
- **Process Orchestration:** Manages `xray` and `tun2socks` as supervised child processes with health checks, crash detection, and automatic failsafe cleanup.

### 2.2 Frontend (React + Vite)
- **Framework:** React 18+ with Vite
- **Styling:** Modern Dark Mode, CSS design system with glassmorphism touches, responsive cards, and clean typography.
- **Icons:** Lucide Icons.
- **State Management & Communication:** REST API + Server-Sent Events (SSE) or polling for live pings, system logs, and traffic throughput.

---

## 3. Directory & Package Structure

```
v2raynix/
├── cmd/
│   └── v2raynix/
│       └── main.go                 # Application entrypoint & CLI flags
├── internal/
│   ├── api/                        # HTTP REST API routes & controllers
│   │   ├── auth_handler.go         # Login, logout, password change
│   │   ├── config_handler.go       # CRUD configs, activate, batch import
│   │   ├── routing_handler.go      # Custom routing rules & presets
│   │   ├── system_handler.go       # Service status, logs, safe-mode confirm
│   │   └── router.go               # HTTP routes registration & SPA static file server
│   ├── auth/                       # JWT token generation, bcrypt password verification, middleware
│   ├── configmgr/                  # Parsers for vless://, vmess://, trojan://, ss:// and raw Xray JSON
│   ├── core/                       # Xray process runner, config generator, test validator
│   ├── network/                    # tun2socks manager, tun0 interface, ip route / ip rule policy engine
│   ├── pinger/                     # TCP ping & proxy HTTP handshake delay tester
│   └── store/                      # SQLite / JSON DB layer for configs, rules, user auth
├── web/                            # React + Vite frontend source code
│   ├── src/
│   │   ├── components/             # Dashboard cards, ConfigCard, PingBadge, SafeModeModal
│   │   ├── pages/                  # Dashboard, Configs, Routing, Logs, Settings
│   │   ├── services/               # API client
│   │   └── App.jsx
│   ├── package.json
│   └── vite.config.js
├── docs/
│   └── superpowers/
│       └── specs/
│           └── 2026-09-20-v2raynix-design.md
├── scripts/
│   ├── install.sh                  # One-line installer script for Linux systems
│   └── v2raynix.service            # Systemd service unit definition
├── go.mod
└── AGENTS.md
```

---

## 4. Network, tun2socks & Anti-Lockout Routing Design

### 4.1 System Network Provisioning
When the user toggles **Connect** in the Web UI:
1. **Network Discovery:** 
   - Detects the default outbound network interface (e.g. `eth0`) and default gateway IP via `/proc/net/route` or `ip route show default`.
   - Detects current active SSH connection IP/ports to prevent dropping remote administrative sessions.
2. **Interface Creation:**
   - Allocates virtual TUN device `tun0` with internal subnet (e.g. `198.18.0.1/15`).
3. **Anti-Lockout Policy Routing:**
   - Creates static routes via the default gateway for:
     - The active SSH incoming remote IP & local SSH listening port (22).
     - The Web UI listening port.
     - The remote V2Ray proxy server's destination IP (to eliminate proxy loops).
     - Private subnets (RFC 1918: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `127.0.0.0/8`).
4. **Policy Routing Table:**
   - Adds custom routing table (e.g. table `100`) routing `0.0.0.0/0` via `dev tun0`.
   - Configures `ip rule add not fwmark <MARK> table 100` so local daemon traffic and Xray outbound remain unmarked and route via default gateway.
5. **tun2socks Initiation:**
   - Spawns `tun2socks -device tun0 -proxy socks5://127.0.0.1:<xray_socks_port>`.

### 4.2 Network Safe Mode (Commit Confirmed)
- Whenever a network tunnel connection or routing rule change is initiated, the backend activates a **Safe Mode Timer** (default: 120 seconds).
- The Web UI displays a countdown modal: *"Network changes applied. Please confirm stability within 120s or changes will automatically revert."*
- If the user clicks **"Confirm & Keep"**, the timer is cleared and changes persist.
- If the countdown reaches zero or the server loses contact with the browser session, the daemon triggers `network.Rollback()`, tearing down `tun0`, restoring routing tables, and reverting to safe un-tunneled mode.
- Signal handlers for `SIGINT`, `SIGTERM`, and process crashes ensure `network.Cleanup()` is always executed.

---

## 5. Configuration Management & Parsing

### 5.1 Supported Formats
1. **VLESS (`vless://...`)** - Supports reality, tls, ws, grpc, tcp, flow.
2. **VMess (`vmess://...`)** - Base64 encoded JSON standard links.
3. **Trojan (`trojan://...`)** - Supports tls, ws, grpc.
4. **Shadowsocks (`ss://...`)** - Standard base64 SIP002 URIs.
5. **Raw Xray JSON** - Full custom Xray configurations.

### 5.2 Multi-Config Management
- Configs are stored in the database with metadata: ID, Name, Protocol, Server, Port, Latency, Active status, CreatedAt.
- One-click **Switch Active**: Re-generates Xray runtime config and performs soft restart / reload in < 500ms.
- **Batch Ping Engine:** Concurrently tests multiple configs using worker pools and updates latency metrics in real-time.

---

## 6. Web UI & User Experience Specifications

### 6.1 Views
1. **Dashboard:**
   - Big Master Switch (Connect / Disconnect).
   - Current Active Config summary badge with protocol and latency indicator.
   - Real-time throughput (Up/Down speed graph & total bytes).
   - Safe Mode countdown banner/modal during trial period.
2. **Configs Screen:**
   - Import modal (paste multiple links or drag & drop JSON).
   - Config cards list with one-click "Activate", "Ping", "Copy Link", "View QR", "Delete".
   - "Test All Latencies" button.
3. **Routing Rules Screen:**
   - Table of custom rules with columns: Target (Domain/IP/CIDR), Action (`Direct`, `Proxy`, `Block`), Toggle Status.
   - Preset buttons: "Bypass Iran Websites & IPs", "Bypass Banking & Government Services".
4. **Live Logs:**
   - Streaming terminal-like log viewer with severity color coding and text search.
5. **Settings:**
   - Web UI Port & Listen Address.
   - Admin credentials (Username, Password change).
   - Safe Mode timeout configuration (30s, 60s, 120s, 300s, or disabled).
   - System Service Control (Restart Service, Reset Network).

---

## 7. Error Handling, Reliability & TDD Strategy

### 7.1 Resilience Controls
- **Config Pre-validation:** Every config is validated using `xray -test -config ...` before execution.
- **Process Supervisor:** Monitors child processes; if `tun2socks` or `xray` dies, immediately cleans up network routes to prevent internet deadlocks.
- **Brute-Force Rate Limiting:** Enforces maximum 5 failed login attempts per IP per 10 minutes.

### 7.2 Verification & Testing Strategy
All development will follow strict **Test-Driven Development (TDD)**:
1. `configmgr_test.go`: Unit tests covering parser edge cases (vless, vmess, trojan, ss, malformed inputs).
2. `network_test.go`: Unit tests for routing command generation, Safe Mode timer triggers, and rollback validation.
3. `auth_test.go`: Unit tests for password hashing, JWT generation, validation, and expiration.
4. `api_test.go`: Integration tests verifying endpoints with authorized/unauthorized states.
5. End-to-end build verification ensuring the single Go binary builds cleanly with embedded UI assets.
