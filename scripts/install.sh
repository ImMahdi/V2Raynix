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

# Directory setup
mkdir -p /etc/v2raynix
mkdir -p /usr/local/bin

# Build or install binary
if [ -f "./bin/v2raynix" ]; then
    cp ./bin/v2raynix /usr/local/bin/v2raynix
elif [ -f "./v2raynix" ]; then
    cp ./v2raynix /usr/local/bin/v2raynix
else
    echo -e "${YELLOW}Building V2Raynix binary from source...${NC}"
    if command -v go >/dev/null 2>&1; then
        go build -o /usr/local/bin/v2raynix ./cmd/v2raynix
    else
        echo -e "${RED}Error: go compiler not found. Please install go or pre-build bin/v2raynix.${NC}"
        exit 1
    fi
fi

chmod +x /usr/local/bin/v2raynix

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
