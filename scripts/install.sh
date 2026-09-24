#!/usr/bin/env bash
set -e

# ==============================================================================
# V2Raynix One-Line Linux Installer
# ==============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${CYAN}======================================================${NC}"
echo -e "${CYAN}           V2Raynix Linux Suite Installer            ${NC}"
echo -e "${CYAN}======================================================${NC}"

if [ "$(id -u)" != "0" ]; then
   echo -e "${RED}Error: This script must be run as root.${NC}" 1>&2
   exit 1
fi

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

echo -e "${GREEN}* Detected system architecture: $GOARCH${NC}"

# Ensure unzip & curl are installed
if ! command -v unzip >/dev/null 2>&1 || ! command -v curl >/dev/null 2>&1; then
    echo -e "${YELLOW}* Installing required tools (curl, unzip)...${NC}"
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update -y && apt-get install -y curl unzip
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y curl unzip
    elif command -v yum >/dev/null 2>&1; then
        yum install -y curl unzip
    elif command -v pacman >/dev/null 2>&1; then
        pacman -Sy --noconfirm curl unzip
    fi
fi

# Directory setup
mkdir -p /etc/v2raynix
mkdir -p /usr/local/bin
mkdir -p /usr/local/share/xray

# Download helper with mirror fallback
fetch_asset() {
    local primary="$1"
    local mirror="https://ghproxy.net/${primary}"
    local dest="$2"

    echo -e "${CYAN}* Downloading: ${primary}...${NC}"
    if curl -fsSL --connect-timeout 10 -m 90 "$primary" -o "$dest"; then
        return 0
    else
        echo -e "${YELLOW}Primary download failed. Attempting anti-censorship mirror...${NC}"
        curl -fsSL --connect-timeout 15 -m 180 "$mirror" -o "$dest"
    fi
}

ensure_xray() {
    if command -v xray >/dev/null 2>&1; then
        echo -e "${GREEN}* Xray core is already installed: $(xray -version 2>/dev/null | head -n1 || echo 'detected')${NC}"
        return 0
    fi

    echo -e "${YELLOW}* Xray core not found. Installing latest official release...${NC}"
    local tmp_zip="/tmp/xray.zip"
    local asset_name=""

    case "$GOARCH" in
        amd64) asset_name="Xray-linux-64.zip" ;;
        arm64) asset_name="Xray-linux-arm64-v8a.zip" ;;
        *) echo -e "${RED}Unsupported arch for Xray: $GOARCH${NC}"; return 1 ;;
    esac

    local download_url="https://github.com/XTLS/Xray-core/releases/latest/download/${asset_name}"
    fetch_asset "$download_url" "$tmp_zip"

    mkdir -p /tmp/xray_unpack
    unzip -o -q "$tmp_zip" -d /tmp/xray_unpack
    cp -f /tmp/xray_unpack/xray /usr/local/bin/xray
    chmod +x /usr/local/bin/xray
    if [ -f "/tmp/xray_unpack/geosite.dat" ]; then
        cp -f /tmp/xray_unpack/geosite.dat /usr/local/share/xray/
        cp -f /tmp/xray_unpack/geosite.dat /usr/local/bin/
    fi
    if [ -f "/tmp/xray_unpack/geoip.dat" ]; then
        cp -f /tmp/xray_unpack/geoip.dat /usr/local/share/xray/
        cp -f /tmp/xray_unpack/geoip.dat /usr/local/bin/
    fi
    rm -rf "$tmp_zip" /tmp/xray_unpack
    echo -e "${GREEN}* Xray successfully installed: $(xray -version 2>/dev/null | head -n1 || echo 'ok')${NC}"
}

ensure_tun2socks() {
    if command -v tun2socks >/dev/null 2>&1; then
        echo -e "${GREEN}* tun2socks is already installed.${NC}"
        return 0
    fi

    echo -e "${YELLOW}* tun2socks not found. Installing latest release...${NC}"
    local tmp_zip="/tmp/tun2socks.zip"
    local asset_name="tun2socks-linux-${GOARCH}.zip"
    local download_url="https://github.com/xjasonlyu/tun2socks/releases/latest/download/${asset_name}"

    fetch_asset "$download_url" "$tmp_zip"
    mkdir -p /tmp/tun2socks_unpack
    unzip -o -q "$tmp_zip" -d /tmp/tun2socks_unpack
    if [ -f "/tmp/tun2socks_unpack/tun2socks" ]; then
        cp -f /tmp/tun2socks_unpack/tun2socks /usr/local/bin/tun2socks
    else
        cp -f /tmp/tun2socks_unpack/tun2socks-linux* /usr/local/bin/tun2socks
    fi
    chmod +x /usr/local/bin/tun2socks
    rm -rf "$tmp_zip" /tmp/tun2socks_unpack
    echo -e "${GREEN}* tun2socks successfully installed.${NC}"
}

configure_firewall() {
    local port="${1:-2080}"
    if command -v ufw >/dev/null 2>&1 && ufw status | grep -qw "active"; then
        echo -e "${CYAN}* Allowing port ${port} in UFW...${NC}"
        ufw allow "${port}/tcp" >/dev/null 2>&1 || true
    elif command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld; then
        echo -e "${CYAN}* Allowing port ${port} in firewalld...${NC}"
        firewall-cmd --add-port="${port}/tcp" --permanent >/dev/null 2>&1 || true
        firewall-cmd --reload >/dev/null 2>&1 || true
    fi
}

# Install dependencies
ensure_xray
ensure_tun2socks
configure_firewall 2080

# Install v2raynix binary
ensure_v2raynix() {
    if [ -f "./bin/v2raynix" ]; then
        echo -e "${GREEN}* Installing from local ./bin/v2raynix...${NC}"
        cp -f ./bin/v2raynix /usr/local/bin/v2raynix
    elif [ -f "./v2raynix" ]; then
        echo -e "${GREEN}* Installing from local ./v2raynix...${NC}"
        cp -f ./v2raynix /usr/local/bin/v2raynix
    else
        echo -e "${YELLOW}* Fetching precompiled V2Raynix binary for ${GOARCH}...${NC}"
        local release_base="https://github.com/ImMahdi/V2Raynix/releases/latest/download"
        local archive_name="v2raynix-linux-${GOARCH}.tar.gz"
        local direct_bin_name="v2raynix-linux-${GOARCH}"
        local tmp_archive="/tmp/${archive_name}"
        local tmp_bin="/tmp/${direct_bin_name}"
        local installed=false

        # Attempt 1: Compressed tarball
        if fetch_asset "${release_base}/${archive_name}" "$tmp_archive" 2>/dev/null && [ -s "$tmp_archive" ]; then
            if tar -tzf "$tmp_archive" >/dev/null 2>&1; then
                tar -xzf "$tmp_archive" -C /tmp/
                if [ -f "/tmp/v2raynix" ]; then
                    cp -f /tmp/v2raynix /usr/local/bin/v2raynix
                    installed=true
                fi
            fi
            rm -f "$tmp_archive"
        fi

        # Attempt 2: Direct standalone binary
        if [ "$installed" = false ]; then
            if fetch_asset "${release_base}/${direct_bin_name}" "$tmp_bin" 2>/dev/null && [ -s "$tmp_bin" ]; then
                cp -f "$tmp_bin" /usr/local/bin/v2raynix
                installed=true
            fi
            rm -f "$tmp_bin"
        fi

        # Attempt 3: Build from source if go is installed
        if [ "$installed" = false ]; then
            if command -v go >/dev/null 2>&1; then
                echo -e "${YELLOW}* Precompiled binary not yet published; compiling with Go...${NC}"
                go build -ldflags="-s -w" -o /usr/local/bin/v2raynix ./cmd/v2raynix
                installed=true
            fi
        fi

        if [ "$installed" = false ]; then
            echo -e "${RED}Error: Could not install V2Raynix binary. Please check release availability or install Go.${NC}"
            exit 1
        fi
    fi
    chmod +x /usr/local/bin/v2raynix
    echo -e "${GREEN}* V2Raynix binary successfully installed to /usr/local/bin/v2raynix${NC}"
}

ensure_v2raynix

# Install systemd service
cat << 'EOF' > /etc/systemd/system/v2raynix.service
[Unit]
Description=V2Raynix Linux Proxy and Full Tunnel Manager
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=/etc/v2raynix
ExecStart=/usr/local/bin/v2raynix -port 2080 -data-dir /etc/v2raynix
Restart=always
RestartSec=3
LimitNOFILE=65536
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable v2raynix
systemctl restart v2raynix

SERVER_IP=$(curl -s4 icanhazip.com || echo "your-server-ip")

echo -e "\n${GREEN}======================================================${NC}"
echo -e "${GREEN}      V2Raynix successfully installed & started!      ${NC}"
echo -e "${GREEN}======================================================${NC}"
echo -e "Web UI Address:  ${CYAN}http://${SERVER_IP}:2080${NC}"
echo -e "Default User:    ${YELLOW}admin${NC}"
echo -e "Default Pass:    ${YELLOW}admin${NC} (Please change upon first login!)"
echo -e "Service Status:  ${CYAN}systemctl status v2raynix${NC}"
echo -e "Service Logs:    ${CYAN}journalctl -u v2raynix -f${NC}"
echo -e "${GREEN}======================================================${NC}"
