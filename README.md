<h1 align="center">V2Raynix</h1>

<p align="center">
  <img src="repo_assets/v2raynix-banner.png" alt="V2Raynix Banner" width="720" />
</p>

<p align="center">
  <b>Next-Generation Full-System Linux Network Tunnel & Smart Proxy Manager</b><br/>
  <i>Engineered with Go, Xray-core, tun2socks, Kernel Policy Routing, and an Embedded React Web Dashboard</i>
</p>

<p align="center">
  <a href="https://github.com/v2raynix/v2raynix/releases"><img src="https://img.shields.io/badge/version-v0.9.0--beta-orange.svg?style=flat-square" alt="Version"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/go-%3E%3D1.23-blue.svg?style=flat-square" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green.svg?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/platform-Linux%20(amd64%20%7C%20arm64)-purple.svg?style=flat-square" alt="Platform">
  <img src="https://img.shields.io/badge/status-Active%20Beta-success.svg?style=flat-square" alt="Status">
</p>

---

## 📑 Table of Contents

- [⚡ Single-Command Installation](#-single-command-installation)
- [💡 Why V2Raynix?](#-why-v2raynix)
- [🚀 Key Features](#-key-features)
- [🏛️ System Architecture](#-system-architecture)
- [🖥️ Interactive Terminal TUI](#-interactive-terminal-tui)
- [🌐 Modern Web Dashboard](#-web-dashboard)
- [⚙️ CLI Reference](#-cli-reference)
- [🛠️ Building from Source (Optional)](#-building-from-source-optional)
- [💖 Support & Donations](#-support--donations)
- [🔒 Security & Disclaimers](#-security--disclaimers)
- [📄 License](#-license)

---

## ⚡ Single-Command Installation

Install and start V2Raynix on any Linux server (**Ubuntu**, **Debian**, **CentOS**, **Fedora**, or **Arch Linux**) with a single command — **no Go, no Node.js, and zero build steps required**:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/v2raynix/v2raynix/master/scripts/install.sh)
```

### 🪄 What the Installer Does Automatically:
1. **Detects Architecture:** Identifies `x86_64` (`amd64`) or `aarch64` (`arm64`) automatically.
2. **Installs System Dependencies:** Ensures `curl`, `unzip`, `iptables`, `iproute2`, and `ca-certificates` are installed.
3. **Deploys Core Engines:** Downloads and configures the latest official **Xray-core** and **tun2socks** binaries alongside up-to-date **GeoIP** and **GeoSite** routing databases.
4. **Installs Standalone Binary:** Installs the pre-built `v2raynix` binary with embedded web dashboard directly into `/usr/local/bin/v2raynix`.
5. **Configures Firewall:** Opens web management port `2080` in `ufw` or `firewalld`.
6. **Creates & Starts Service:** Sets up the `v2raynix.service` systemd daemon and immediately starts it.

### 🌐 Accessing the Panel:
Once installation finishes, open your browser:
```text
http://<your-server-ip>:2080
```
- **Default Username:** `admin`
- **Default Password:** `admin` *(You will be prompted to update credentials upon first login or via the terminal menu)*

---

## 💡 Why V2Raynix?

Managing proxy clients on headless Linux servers has traditionally been cumbersome, brittle, and dangerous. Standard command-line tools often require writing complex JSON configs manually, lack unified TUN device management, or worse—alter default network routing tables in ways that **sever active SSH sessions**, locking administrators out of their own cloud instances.

**V2Raynix** solves these challenges by combining:
1. **Zero-Lockout Kernel Policy Routing:** Automatic detection of SSH ports, incoming administration IPs, and default gateways ensures your management connections remain entirely untouched.
2. **120-Second Network Safe Mode:** Route alterations automatically initiate a countdown failsafe; if unconfirmed, routing rolls back cleanly to prevent network blackholes.
3. **Single-Binary Zero-Dependency Deployment:** The full React administration panel is compiled directly into the Go executable (`go:embed`). No Node.js runtime, Nginx reverse proxy, or external databases are required on your server.
4. **Interactive Terminal Setup (TUI):** A color-coded, border-aligned ANSI terminal console for quick credential management (with Linux `sudo`-style silent password masking and confirmation), port reconfigurations, and systemd supervision.

---

## 🚀 Key Features

- **🌐 Global Server Tunneling (`tun2socks`):** Routes all outgoing Linux TCP and UDP traffic transparently through a virtual `tun0` adapter into your active proxy node.
- **🛡️ 2-Tier Anti-Lockout Defense:** Automatic policy routing rules ensure SSH (port 22/custom), local web management traffic, and direct connections bypass `tun0` directly to the physical gateway.
- **⏱️ Automated Safe Mode Failsafe:** A 120-second commit-confirmation timer guards against misconfigured routes, automatically rolling back changes before access is lost.
- **⚡ Multi-Protocol Support:**
  - **VLESS:** Supports `xhttp`, `reality`, `xtls-rprx-vision`, `ws`, `grpc`, and `tcp`.
  - **VMess:** Full AEAD support with WebSocket, TCP, and camouflage headers.
  - **Trojan:** Standard TLS and WebSocket configurations.
  - **Shadowsocks:** Modern 2022 AEAD ciphers as well as legacy stream methods.
- **🧠 Smart Policy Routing & GeoData Normalization:** Visual domain and IP rule routing (Direct / Proxy / Block). Automatically normalizes aliases (e.g., `geosite:ir` / `geosite:iran` $\to$ `geosite:category-ir`, `geoip:ir`) to guarantee compatibility with official `geosite.dat` and `geoip.dat` assets without engine syntax crashes.
- **📊 Real-Time Node Diagnostics:** Dual-metric latency analysis supporting both raw TCP pings and true HTTP end-to-end handshake delays.
- **🖥️ Silent-Input Terminal TUI:** Interactive terminal setup menu (`v2raynix setup`) with silent password entry (hidden characters without echo) and double-confirmation checks.
- **🔄 In-Panel Core Engine Updater:** Inspect and update upstream `Xray-core` and `tun2socks` binaries directly from the web interface with automated checksum verification and atomic rollbacks.

---

## 🏛️ System Architecture

```text
[ Outgoing Server Applications & System Traffic ]
                      │
                      ▼
   ┌─────────────────────────────────────┐
   │    Kernel Policy Routing Rules      │
   └──────────────────┬──────────────────┘
                      │
       ┌──────────────┴──────────────┐
       │ (SSH / Web Port / Dest IP)  │ (All Default Traffic)
       ▼                             ▼
┌──────────────┐              ┌──────────────┐
│ Physical Eth │              │   tun0 Net   │
│   Gateway    │              │  Interface   │
└──────────────┘              └──────┬───────┘
                                     │
                                     ▼
                              ┌──────────────┐
                              │  tun2socks   │
                              └──────┬───────┘
                                     │ (SOCKS5 127.0.0.1:10808)
                                     ▼
                              ┌──────────────┐
                              │  Xray Core   │
                              └──────┬───────┘
                                     │ (VLESS / VMess / Trojan / SS)
                                     ▼
                        [ Remote Proxy Server ]
```

---

## 🖥️ Interactive Terminal TUI

Need to reset your credentials, change listening ports, or check service health directly from SSH without opening a browser? Simply run:

```bash
sudo v2raynix setup
```

```text
┌────────────────────────────────────────────────────────────┐
│                    V2RAYNIX SERVER SETUP                   │
├────────────────────────────────────────────────────────────┤
│  Service Status : ● Active (Running)                       │
│  Web Management : http://127.0.0.1:2080                    │
├────────────────────────────────────────────────────────────┤
│  [1] Change / Reset Admin Credentials                      │
│  [2] Change Web Panel Listening Port                       │
│  [3] Manage V2Raynix Service (Start / Stop / Restart)      │
│  [4] View Server Status & Diagnostics                      │
│  [0] Exit Setup                                            │
└────────────────────────────────────────────────────────────┘
```

> **Security Note:** In option `[1]`, password inputs are completely masked (silent input with no characters or asterisks echoed to the screen, exactly like Linux `sudo`), and a confirmation re-entry is required before any changes are written.

---

## 🌐 Web Dashboard

The embedded web interface provides a reactive, modern dashboard with dark-mode glassmorphic aesthetics:

- **Dashboard:** Monitor real-time system network transfer speeds, active connection uptime, virtual TUN adapter status, and safe mode countdown.
- **Configurations:** Import nodes via standard share links (`vless://`, `vmess://`, `trojan://`, `ss://`) or raw JSON. Run batch pings and real HTTP latency tests across all nodes.
- **Smart Routing:** Create granular policy rules (Domain / IP / CIDR) targeting Direct, Proxy, or Block outbounds. Includes one-click presets for bypassing domestic services (`geosite:category-ir`, `geoip:ir`) and blocking ad trackers (`geosite:category-ads-all`).
- **Live Logs:** Real-time log inspector streaming kernel routing events, daemon state transitions, and Xray core output.
- **Settings & Core Updater:** Configure listening ports, Safe Mode timeouts, admin passwords, and update underlying Xray / tun2socks cores with single-click zero-downtime execution.

---

## ⚙️ CLI Reference

```text
Usage: v2raynix [flags]
       v2raynix setup [subcommand flags]

Flags:
  -port int
        Web UI and REST API listening port (default: 2080)
  -data-dir string
        Path to state directory (default: /etc/v2raynix or ./data)
  -init-password string
        Directly initialize or override admin password
  -mock
        Run in simulation mode without altering kernel interfaces or iptables
  -version
        Print V2Raynix version and exit
```

---

## 🛠️ Building from Source (Optional)

<details>
<summary><b>Click here to view compilation instructions (for developers)</b></summary>
<br/>

If you prefer compiling directly from the source code rather than using the automated 1-command installer:

### Prerequisites:
- **Go:** Version 1.23 or newer
- **Node.js & npm:** Version 18+ (for compiling the embedded React frontend)
- **Make / Bash:** Standard build utilities

### Step-by-Step Compilation:

```bash
# 1. Clone the repository
git clone https://github.com/v2raynix/v2raynix.git
cd v2raynix

# 2. Build the React SPA frontend
cd web
npm install
npm run build
cd ..

# 3. Compile the single standalone Go binary
go build -ldflags="-s -w" -o bin/v2raynix ./cmd/v2raynix

# 4. Run locally in simulation (mock) mode
./bin/v2raynix -port 2080 -mock
```
</details>

---

## 💖 Support & Donations

V2Raynix is an independent, free, and open-source project dedicated to internet freedom and open communication. If this tool helps you maintain reliable, secure connectivity, please consider supporting future maintenance and development!

### 📋 Wallet Addresses

| Network / Cryptocurrency | Address |
| :--- | :--- |
| **BNB Smart Chain (BEP20)** | `0x726524eF2Bf606f12829C7724a37196E5fE00F44` |
| **Tron (TRC20)** | `TYkdrBjmJEMxXbxS6pwCujHvB18AS9WZ57` |
| **Bitcoin (BTC)** | `bc1qjyjat944wz466l3pz27953gfl69ey2wf66r2x4` |
| **Solana (SOL)** | `GevJAdW3x8Y8sqgDmYQf6VXGsoNny3hh8gFJthknC61W` |
| **Ethereum (ERC20)** | `0x726524eF2Bf606f12829C7724a37196E5fE00F44` |

<br/>

### 📱 Scan to Donate (QR Codes)

<div align="center">

| **BNB (BEP20)** | **Tron (TRC20)** | **Bitcoin (BTC)** | **Solana (SOL)** | **Ethereum (ERC20)** |
| :---: | :---: | :---: | :---: | :---: |
| <a href="repo_assets/qr-bnb.png"><img src="repo_assets/qr-bnb.png" width="95" alt="BNB" /></a> | <a href="repo_assets/qr-trx.png"><img src="repo_assets/qr-trx.png" width="95" alt="TRX" /></a> | <a href="repo_assets/qr-btc.png"><img src="repo_assets/qr-btc.png" width="95" alt="BTC" /></a> | <a href="repo_assets/qr-sol.png"><img src="repo_assets/qr-sol.png" width="95" alt="SOL" /></a> | <a href="repo_assets/qr-eth.png"><img src="repo_assets/qr-eth.png" width="95" alt="ETH" /></a> |
| `0x7265...0F44` | `TYkd...WZ57` | `bc1q...r2x4` | `GevJ...C61W` | `0x7265...0F44` |

</div>

---

## 🔒 Security & Disclaimers

- **Root Privileges:** Full-system network routing (`tun0`, `iptables`, `ip route`) requires administrative privileges (`CAP_NET_ADMIN`, `CAP_NET_BIND_SERVICE`). The background daemon drops unnecessary capabilities where applicable.
- **Disclaimer:** This software is provided "as is", without warranty of any kind. Users are responsible for complying with local telecommunication regulations and laws when deploying proxy tunnels.

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
Feel free to contribute, open issues, and submit pull requests!
