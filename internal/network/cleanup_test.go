package network

import (
	"slices"
	"strings"
	"testing"
)

func TestForceTeardown(t *testing.T) {
	cmds := BuildForceTeardownCommands(22, 8080)

	if len(cmds) == 0 {
		t.Fatal("expected non-empty teardown commands")
	}

	// 1. Priority 2000 MUST be dropped FIRST to prevent lockout.
	if cmds[0] != "ip rule del not fwmark 0x51 table 100 priority 2000" {
		t.Errorf("expected first command to drop priority 2000 fwmark rule, got %q", cmds[0])
	}
	if cmds[1] != "ip rule del priority 2000" {
		t.Errorf("expected second command to drop priority 2000 rule, got %q", cmds[1])
	}

	// 2. Table 100 flushed.
	if !slices.Contains(cmds, "ip route flush table 100") {
		t.Error("expected command to flush table 100")
	}

	// 3. tun0 deleted.
	if !slices.Contains(cmds, "ip tuntap del dev tun0 mode tun") {
		t.Error("expected command to delete tun0 tuntap")
	}
	if !slices.Contains(cmds, "ip link delete tun0") {
		t.Error("expected command to delete tun0 link")
	}

	// 4. Bypass priority rules removed.
	priorities := []string{"1000", "1001", "1002", "1003", "1004", "1005", "1006", "1007", "1008", "1010", "1011", "1012", "1013", "999"}
	for _, prio := range priorities {
		expected := "ip rule del priority " + prio
		if !slices.Contains(cmds, expected) {
			t.Errorf("expected command %q", expected)
		}
	}

	// 5. iptables mangle V2RAYNIX_INBOUND unlinked from PREROUTING, OUTPUT connmark rule deleted, V2RAYNIX_INBOUND flushed and deleted.
	expectedIptables := []string{
		"iptables -t mangle -D PREROUTING -j V2RAYNIX_INBOUND",
		"iptables -t mangle -D OUTPUT -m connmark --mark 0x51 -j CONNMARK --restore-mark",
		"iptables -t mangle -F V2RAYNIX_INBOUND",
		"iptables -t mangle -X V2RAYNIX_INBOUND",
	}
	for _, exp := range expectedIptables {
		if !slices.Contains(cmds, exp) {
			t.Errorf("expected iptables command %q", exp)
		}
	}

	// Verify priority 2000 appears before table flush and tun0 deletion.
	idxFlush := slices.Index(cmds, "ip route flush table 100")
	idxTun := slices.Index(cmds, "ip link delete tun0")
	if idxFlush < 2 || idxTun < 2 {
		t.Errorf("expected table flush (%d) and tun0 delete (%d) after priority 2000 removal", idxFlush, idxTun)
	}
	_ = strings.Join(cmds, "\n")
}
