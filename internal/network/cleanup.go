package network

import (
	"os/exec"
	"strings"
)

// BuildForceTeardownCommands returns the ordered, fail-safe Zero-Lockout
// command sequence to unconditionally tear down V2Raynix network state.
func BuildForceTeardownCommands(sshPort, webPort int) []string {
	_ = sshPort
	_ = webPort

	return []string{
		// 1. Drop priority 2000 FIRST to prevent lockout.
		"ip rule del not fwmark 0x51 table 100 priority 2000",
		"ip rule del priority 2000",

		// 2. Flush routing table 100.
		"ip route flush table 100",

		// 3. Delete tun0 interface.
		"ip tuntap del dev tun0 mode tun",
		"ip link delete tun0",

		// 4. Remove bypass priority rules.
		"ip rule del priority 1000",
		"ip rule del priority 1001",
		"ip rule del priority 1002",
		"ip rule del priority 1003",
		"ip rule del priority 1004",
		"ip rule del priority 1005",
		"ip rule del priority 1006",
		"ip rule del priority 1007",
		"ip rule del priority 1008",
		"ip rule del priority 1010",
		"ip rule del priority 1011",
		"ip rule del priority 1012",
		"ip rule del priority 1013",
		"ip rule del priority 999",

		// 5. Unlink, flush, and delete iptables mangle V2RAYNIX_INBOUND rules.
		"iptables -t mangle -D PREROUTING -j V2RAYNIX_INBOUND",
		"iptables -t mangle -D OUTPUT -m connmark --mark 0x51 -j CONNMARK --restore-mark",
		"iptables -t mangle -F V2RAYNIX_INBOUND",
		"iptables -t mangle -X V2RAYNIX_INBOUND",
	}
}

// ForceTeardownNetwork executes the unconditional Zero-Lockout teardown
// sequence resiliently, ignoring errors for missing rules or interfaces,
// and reverts host DNS configuration.
func ForceTeardownNetwork(sshPort, webPort int) error {
	for _, cmdStr := range BuildForceTeardownCommands(sshPort, webPort) {
		parts := strings.Fields(cmdStr)
		if len(parts) == 0 {
			continue
		}
		_ = exec.Command(parts[0], parts[1:]...).Run()
	}

	return RevertDNSOverTunnel()
}
