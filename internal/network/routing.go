package network

import (
	"fmt"
)

const (
	TunDevice   = "tun0"
	TunIP       = "198.18.0.1/15"
	TableID     = 100
	FwmarkValue = "0x51"
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
		fmt.Sprintf("ip link add dev %s type tun", TunDevice),
		fmt.Sprintf("ip addr add %s dev %s", TunIP, TunDevice),
		fmt.Sprintf("ip link set dev %s up", TunDevice),

		// 2. Anti-lockout bypass for remote proxy server IP (prevent routing loop)
		fmt.Sprintf("ip route add %s/32 via %s dev %s", remoteProxyIP, defaultGw, defaultIface),

		// 3. Anti-lockout policy rules for SSH
		fmt.Sprintf("ip rule add sport %d table main priority 1000", sshPort),
		fmt.Sprintf("ip rule add dport %d table main priority 1001", sshPort),

		// 4. Anti-lockout policy rules for Web UI
		fmt.Sprintf("ip rule add sport %d table main priority 1002", webPort),
		fmt.Sprintf("ip rule add dport %d table main priority 1003", webPort),

		// 5. Bypass RFC1918 private IP subnets
		"ip rule add to 10.0.0.0/8 table main priority 1010",
		"ip rule add to 172.16.0.0/12 table main priority 1011",
		"ip rule add to 192.168.0.0/16 table main priority 1012",
		"ip rule add to 127.0.0.0/8 table main priority 1013",

		// 6. Policy route table 100 through tun0
		fmt.Sprintf("ip route add default dev %s table %d", TunDevice, TableID),

		// 7. Divert non-marked traffic to table 100
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
		fmt.Sprintf("ip rule del sport %d table main", sshPort),
		fmt.Sprintf("ip rule del dport %d table main", sshPort),
		fmt.Sprintf("ip rule del sport %d table main", webPort),
		fmt.Sprintf("ip rule del dport %d table main", webPort),

		// Flush custom table
		fmt.Sprintf("ip route flush table %d", TableID),

		// Delete bypass route
		fmt.Sprintf("ip route del %s/32 via %s dev %s", remoteProxyIP, defaultGw, defaultIface),

		// Delete tun interface
		fmt.Sprintf("ip link del %s", TunDevice),
	}

	return cmds
}
