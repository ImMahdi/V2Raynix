package network

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	TunDevice       = "tun0"
	TunIP           = "198.18.0.1/15"
	TableID         = 100
	FwmarkValue     = "0x51"
	InboundFwmark   = "0x52"
	InboundPriority = 1008
	InboundChain    = "V2RAYNIX_INBOUND"
)

// BuildRoutingCommands produces the ordered list of Linux ip commands to setup tun0 and safe policy routing
func BuildRoutingCommands(remoteProxyIP, defaultIface, defaultGw string, sshPort int, webPort int) []string {
	if sshPort <= 0 {
		sshPort = 22
	}
	if webPort <= 0 {
		webPort = 2080
	}

	cmds := []string{
		// 1. Create and configure tun interface
		fmt.Sprintf("ip tuntap add dev %s mode tun", TunDevice),
		fmt.Sprintf("ip addr add %s dev %s", TunIP, TunDevice),
		fmt.Sprintf("ip link set dev %s up", TunDevice),

		// 2. Anti-lockout bypass for remote proxy server IP (priority 999 rule + direct route)
		fmt.Sprintf("ip rule add to %s table main priority 999", remoteProxyIP),
		fmt.Sprintf("ip route add %s/32 via %s dev %s", remoteProxyIP, defaultGw, defaultIface),

		// 3. Anti-lockout policy rules for SSH
		fmt.Sprintf("ip rule add sport %d table main priority 1000", sshPort),
		fmt.Sprintf("ip rule add dport %d table main priority 1001", sshPort),

		// 4. Anti-lockout policy rules for Web UI
		fmt.Sprintf("ip rule add sport %d table main priority 1002", webPort),
		fmt.Sprintf("ip rule add dport %d table main priority 1003", webPort),

		// 5. Inbound Connection Preservation via Netfilter Connmark
		fmt.Sprintf("iptables -t mangle -N %s", InboundChain),
		fmt.Sprintf("iptables -t mangle -F %s", InboundChain),
		fmt.Sprintf("iptables -t mangle -A PREROUTING -i %s -m conntrack --ctstate NEW -j %s", defaultIface, InboundChain),
		fmt.Sprintf("iptables -t mangle -A %s -j CONNMARK --set-mark %s", InboundChain, InboundFwmark),
		fmt.Sprintf("iptables -t mangle -A OUTPUT -m connmark --mark %s -j CONNMARK --restore-mark", InboundFwmark),
		fmt.Sprintf("ip rule add fwmark %s table main priority %d", InboundFwmark, InboundPriority),

		// 6. Bypass RFC1918 private IP subnets
		"ip rule add to 10.0.0.0/8 table main priority 1010",
		"ip rule add to 172.16.0.0/12 table main priority 1011",
		"ip rule add to 192.168.0.0/16 table main priority 1012",
		"ip rule add to 127.0.0.0/8 table main priority 1013",

		// 7. Policy route table 100 through tun0
		fmt.Sprintf("ip route add default dev %s table %d", TunDevice, TableID),

		// 8. Divert non-marked traffic to table 100
		fmt.Sprintf("ip rule add not fwmark %s table %d priority 2000", FwmarkValue, TableID),
	}

	return cmds
}

// BuildCleanupCommands produces the commands to cleanly tear down routes and restore default system networking
func BuildCleanupCommands(remoteProxyIP, defaultIface, defaultGw string, sshPort int, webPort int) []string {
	if sshPort <= 0 {
		sshPort = 22
	}
	if webPort <= 0 {
		webPort = 2080
	}

	cmds := []string{
		// Remove policy rules
		fmt.Sprintf("ip rule del not fwmark %s table %d priority 2000", FwmarkValue, TableID),
		"ip rule del to 10.0.0.0/8 table main",
		"ip rule del to 172.16.0.0/12 table main",
		"ip rule del to 192.168.0.0/16 table main",
		"ip rule del to 127.0.0.0/8 table main",
		fmt.Sprintf("ip rule del sport %d table main", sshPort),
		fmt.Sprintf("ip rule del dport %d table main", sshPort),
		fmt.Sprintf("ip rule del sport %d table main", webPort),
		fmt.Sprintf("ip rule del dport %d table main", webPort),
		fmt.Sprintf("ip rule del to %s table main priority 999", remoteProxyIP),
		"ip rule del priority 999",

		// Remove conntrack rules and custom chain
		fmt.Sprintf("iptables -t mangle -D PREROUTING -i %s -m conntrack --ctstate NEW -j %s", defaultIface, InboundChain),
		fmt.Sprintf("iptables -t mangle -D OUTPUT -m connmark --mark %s -j CONNMARK --restore-mark", InboundFwmark),
		fmt.Sprintf("iptables -t mangle -F %s", InboundChain),
		fmt.Sprintf("iptables -t mangle -X %s", InboundChain),
		fmt.Sprintf("ip rule del fwmark %s table main", InboundFwmark),

		// Flush custom table
		fmt.Sprintf("ip route flush table %d", TableID),

		// Delete bypass route
		fmt.Sprintf("ip route del %s/32 via %s dev %s", remoteProxyIP, defaultGw, defaultIface),

		// Delete tun interface
		fmt.Sprintf("ip tuntap del dev %s mode tun", TunDevice),
	}

	return cmds
}

// GetDefaultRoute discovers the current default network interface and gateway
func GetDefaultRoute() (iface, gateway string, err error) {
	out, err := exec.Command("ip", "route", "get", "1.1.1.1").Output()
	if err == nil {
		fields := strings.Fields(string(out))
		for i := 0; i < len(fields)-1; i++ {
			if fields[i] == "via" {
				gateway = fields[i+1]
			}
			if fields[i] == "dev" {
				iface = fields[i+1]
			}
		}
		if iface != "" && gateway != "" {
			return iface, gateway, nil
		}
	}

	// Fallback to "ip route show default"
	out, err = exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get default route: %w", err)
	}
	fields := strings.Fields(string(out))
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] == "via" {
			gateway = fields[i+1]
		}
		if fields[i] == "dev" {
			iface = fields[i+1]
		}
	}
	if iface == "" || gateway == "" {
		return "", "", fmt.Errorf("could not parse default route: %s", string(out))
	}
	return iface, gateway, nil
}

// DetectSSHPort inspects sshd config and environment to protect the active SSH port
func DetectSSHPort() int {
	files := []string{"/etc/ssh/sshd_config"}
	matches, _ := filepath.Glob("/etc/ssh/sshd_config.d/*.conf")
	files = append(files, matches...)

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "port ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					if p, err := strconv.Atoi(parts[1]); err == nil && p > 0 {
						return p
					}
				}
			}
		}
	}
	return 22
}

// ResolveHost resolves domain to IP string (or returns IP directly)
func ResolveHost(host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return host, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("cannot resolve host %s: %w", host, err)
	}
	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.String(), nil
		}
	}
	return ips[0].String(), nil
}

// Valid network interface name pattern (e.g. eth0, ens3, wlan0, tun0, bond0.100)
var ifaceRegex = regexp.MustCompile(`^[a-zA-Z0-9_.:-]+$`)

// ValidateRoutingParams validates network parameters before constructing or applying policy routing
func ValidateRoutingParams(remoteProxyIP, defaultIface, defaultGw string, sshPort int, webPort int) error {
	if net.ParseIP(remoteProxyIP) == nil {
		return fmt.Errorf("invalid remote proxy IP: %q", remoteProxyIP)
	}
	if net.ParseIP(defaultGw) == nil {
		return fmt.Errorf("invalid default gateway IP: %q", defaultGw)
	}
	if !ifaceRegex.MatchString(defaultIface) {
		return fmt.Errorf("invalid network interface name: %q", defaultIface)
	}
	if sshPort <= 0 || sshPort > 65535 {
		return fmt.Errorf("invalid SSH port: %d (must be between 1 and 65535)", sshPort)
	}
	if webPort <= 0 || webPort > 65535 {
		return fmt.Errorf("invalid Web port: %d (must be between 1 and 65535)", webPort)
	}
	return nil
}

// ExecuteCommands runs a list of system commands directly without invoking a shell interpreter
func ExecuteCommands(cmds []string) []error {
	var errs []error
	for _, cmd := range cmds {
		trimmed := strings.TrimSpace(cmd)
		if trimmed == "" {
			continue
		}
		parts := strings.Fields(trimmed)
		if len(parts) == 0 {
			continue
		}
		c := exec.Command(parts[0], parts[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			errs = append(errs, fmt.Errorf("command '%s' failed: %v, output: %s", cmd, err, strings.TrimSpace(string(out))))
		}
	}
	return errs
}

