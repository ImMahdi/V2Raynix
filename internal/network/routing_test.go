package network_test

import (
	"strings"
	"testing"

	"github.com/v2raynix/v2raynix/internal/network"
)

func TestBuildRoutingCommands(t *testing.T) {
	remoteProxyIP := "198.51.100.1"
	defaultIface := "eth0"
	defaultGw := "192.168.1.1"
	sshPort := 22
	webPort := 2080

	cmds := network.BuildRoutingCommands(remoteProxyIP, defaultIface, defaultGw, sshPort, webPort)
	if len(cmds) == 0 {
		t.Fatalf("expected non-empty routing commands")
	}

	joined := strings.Join(cmds, "\n")

	// Verify tun0 created via ip tuntap
	if !strings.Contains(joined, "ip tuntap add dev tun0 mode tun") {
		t.Errorf("expected 'ip tuntap add dev tun0 mode tun', got:\n%s", joined)
	}

	// Verify Anti-Lockout: remote proxy IP must be routed directly via default gateway AND have priority 999 rule
	if !strings.Contains(joined, "198.51.100.1") || !strings.Contains(joined, "via 192.168.1.1") {
		t.Errorf("expected direct bypass route for remote proxy IP")
	}
	if !strings.Contains(joined, "ip rule add to 198.51.100.1 table main priority 999") {
		t.Errorf("expected priority 999 bypass rule for remote proxy IP")
	}

	// Verify Anti-Lockout: SSH port 22 bypass
	if !strings.Contains(joined, "dport 22") && !strings.Contains(joined, "sport 22") {
		t.Errorf("expected SSH port anti-lockout rule")
	}

	// Verify Web UI port bypass
	if !strings.Contains(joined, "2080") {
		t.Errorf("expected Web UI port bypass rule")
	}

	// Verify table 100 default route via tun0
	if !strings.Contains(joined, "table 100") || !strings.Contains(joined, "dev tun0") {
		t.Errorf("expected table 100 default route via tun0")
	}

	// Verify Netfilter conntrack inbound preservation
	if !strings.Contains(joined, "iptables -t mangle -N V2RAYNIX_INBOUND") {
		t.Errorf("expected creation of V2RAYNIX_INBOUND chain")
	}
	if !strings.Contains(joined, "iptables -t mangle -A PREROUTING -i eth0 -m conntrack --ctstate NEW -j V2RAYNIX_INBOUND") {
		t.Errorf("expected PREROUTING jump to V2RAYNIX_INBOUND")
	}
	if !strings.Contains(joined, "iptables -t mangle -A V2RAYNIX_INBOUND -j CONNMARK --set-mark 0x52") {
		t.Errorf("expected CONNMARK set-mark 0x52")
	}
	if !strings.Contains(joined, "iptables -t mangle -A OUTPUT -m connmark --mark 0x52 -j CONNMARK --restore-mark") {
		t.Errorf("expected OUTPUT restore-mark 0x52")
	}
	if !strings.Contains(joined, "ip rule add fwmark 0x52 table main priority 1008") {
		t.Errorf("expected ip rule for fwmark 0x52 table main priority 1008")
	}
}

func TestBuildCleanupCommands(t *testing.T) {
	remoteProxyIP := "198.51.100.1"
	defaultIface := "eth0"
	defaultGw := "192.168.1.1"
	sshPort := 22
	webPort := 2080

	cmds := network.BuildCleanupCommands(remoteProxyIP, defaultIface, defaultGw, sshPort, webPort)
	if len(cmds) == 0 {
		t.Fatalf("expected non-empty cleanup commands")
	}

	joined := strings.Join(cmds, "\n")

	// Must clean up priority 999 rule, flush table 100, and delete tun0
	if !strings.Contains(joined, "ip rule del to 198.51.100.1 table main") {
		t.Errorf("expected cleanup of priority 999 rule for remote proxy IP")
	}
	if !strings.Contains(joined, "ip route flush table 100") {
		t.Errorf("expected flush table 100 in cleanup")
	}
	if !strings.Contains(joined, "ip tuntap del dev tun0 mode tun") && !strings.Contains(joined, "ip link del dev tun0") {
		t.Errorf("expected delete tun0 in cleanup")
	}

	// Must clean up conntrack rules and chain
	if !strings.Contains(joined, "iptables -t mangle -D PREROUTING -i eth0 -m conntrack --ctstate NEW -j V2RAYNIX_INBOUND") {
		t.Errorf("expected deletion of PREROUTING jump rule in cleanup")
	}
	if !strings.Contains(joined, "iptables -t mangle -D OUTPUT -m connmark --mark 0x52 -j CONNMARK --restore-mark") {
		t.Errorf("expected deletion of OUTPUT restore-mark rule in cleanup")
	}
	if !strings.Contains(joined, "iptables -t mangle -F V2RAYNIX_INBOUND") || !strings.Contains(joined, "iptables -t mangle -X V2RAYNIX_INBOUND") {
		t.Errorf("expected flush and delete of V2RAYNIX_INBOUND chain in cleanup")
	}
	if !strings.Contains(joined, "ip rule del fwmark 0x52 table main") {
		t.Errorf("expected deletion of fwmark 0x52 rule in cleanup")
	}
}

func TestValidateRoutingParams(t *testing.T) {
	// Valid parameters
	err := network.ValidateRoutingParams("198.51.100.1", "eth0", "192.168.1.1", 22, 2080)
	if err != nil {
		t.Fatalf("expected valid parameters to pass, got: %v", err)
	}

	// Invalid remote IP (containing shell metacharacters)
	err = network.ValidateRoutingParams("198.51.100.1; rm -rf /", "eth0", "192.168.1.1", 22, 2080)
	if err == nil {
		t.Errorf("expected error for injected remote IP, got nil")
	}

	// Invalid gateway
	err = network.ValidateRoutingParams("198.51.100.1", "eth0", "invalid-gw", 22, 2080)
	if err == nil {
		t.Errorf("expected error for invalid gateway, got nil")
	}

	// Invalid interface (containing shell metacharacters)
	err = network.ValidateRoutingParams("198.51.100.1", "eth0; echo pwned", "192.168.1.1", 22, 2080)
	if err == nil {
		t.Errorf("expected error for injected interface name, got nil")
	}

	// Invalid ports
	err = network.ValidateRoutingParams("198.51.100.1", "eth0", "192.168.1.1", 0, 2080)
	if err == nil {
		t.Errorf("expected error for port 0, got nil")
	}
	err = network.ValidateRoutingParams("198.51.100.1", "eth0", "192.168.1.1", 22, 70000)
	if err == nil {
		t.Errorf("expected error for port > 65535, got nil")
	}
}
