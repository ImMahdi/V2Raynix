# V2Raynix 🛡️

**V2Raynix** is a high-performance, single-binary Linux daemon and modern Web UI for V2Ray / Xray proxy configurations. It provides full-system network tunneling via `tun2socks`, zero-downtime multi-config switching, intelligent policy routing, and an automated 2-minute **Safe Mode** auto-rollback failsafe.

---

## Key Features

- **🚀 Full Server Tunneling:** Routes all Linux server network traffic through a virtual `tun0` interface via `tun2socks`.
- **🛡️ SSH & Web UI Anti-Lockout:** Automatically detects your SSH session, incoming IP, and local ports, routing them directly through the physical gateway so you never lose remote administrative access.
- **⏱️ Network Safe Mode (Commit Confirmed):** Whenever network routes change, a 120-second countdown begins. If not confirmed, changes automatically revert to an un-tunneled safe state.
- **⚡ Multi-Config Storage:** Store and switch between unlimited configurations (VLESS, VMess, Trojan, Shadowsocks, or custom Xray JSON) with one click.
- **📊 Real-Time Latency Testing:** Test individual or batch TCP pings and real HTTP handshake delays.
- **🌐 Custom Routing Policies:** Visual rule editor to direct, proxy, or block specific domains and IPs, with one-click presets for bypassing local services and Iranian banking/government domains.
- **📦 Single Binary Deployment:** The complete React Web UI is compiled directly into the Go executable (`go:embed`). No Node.js, Nginx, or external databases needed on the server.

---

## Quick Installation on Linux

Run the following command on your Debian, Ubuntu, CentOS, or Arch server as `root`:

```bash
curl -fsSL https://raw.githubusercontent.com/v2raynix/v2raynix/master/scripts/install.sh | bash
```

Once installed, navigate to:
```
http://<your-server-ip>:2080
```
- **Default Username:** `admin`
- **Default Password:** `admin` (Change this in the Settings tab immediately)

---

## Architecture

```
[ Incoming / Outgoing Linux Traffic ]
               │
               ▼
   [ Policy Routing Rules ] ──(SSH Port 22 / Web UI / Proxy IP)──> [ Physical Gateway eth0 ]
               │
          (All Else)
               │
               ▼
       [ Interface tun0 ]
               │
               ▼
         [ tun2socks ]
               │
               ▼
        [ Xray Core ] ────(VLESS / VMess / Trojan / SS)────> [ Remote Proxy Server ]
```

---

## Building from Source

### Prerequisites
- Go 1.22+
- Node.js 18+ and npm

### Build Steps

1. **Build Frontend:**
   ```bash
   cd web
   npm install
   npm run build
   cd ..
   ```

2. **Compile Go Binary:**
   ```bash
   go build -ldflags="-s -w" -o bin/v2raynix ./cmd/v2raynix
   ```

3. **Run Locally (Mock Mode):**
   ```bash
   ./bin/v2raynix -port 2080 -mock
   ```

---

## CLI Options

```
Usage of v2raynix:
  -port int
        Web UI listening port (default 2080)
  -data-dir string
        Path to data directory (default: /etc/v2raynix or ./data)
  -init-password string
        Set or reset the admin password
  -mock
        Run in simulation mode (without modifying system network interfaces)
  -version
        Show V2Raynix version and exit
```

---

## License
MIT License
