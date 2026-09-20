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

	// Verify Anti-Lockout: remote proxy IP must be routed directly via default gateway
	if !strings.Contains(joined, "198.51.100.1") || !strings.Contains(joined, "via 192.168.1.1") {
		t.Errorf("expected direct bypass route for remote proxy IP")
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

	// Must flush table 100 and delete tun0
	if !strings.Contains(joined, "ip route flush table 100") {
		t.Errorf("expected flush table 100 in cleanup")
	}
	if !strings.Contains(joined, "ip link del tun0") {
		t.Errorf("expected delete tun0 in cleanup")
	}
}
