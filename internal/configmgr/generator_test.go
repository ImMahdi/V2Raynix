package configmgr_test

import (
	"encoding/json"
	"testing"

	"github.com/v2raynix/v2raynix/internal/configmgr"
	"github.com/v2raynix/v2raynix/internal/store"
)

func TestGenerateXrayConfig_SockoptMarkAndSniffing(t *testing.T) {
	cfg := &store.ConfigItem{
		ID:       "cfg-test-sockopt",
		Name:     "Test Server",
		Protocol: "vless",
		Server:   "1.2.3.4",
		Port:     443,
		RawURL:   "vless://uuid-123@1.2.3.4:443?type=tcp&security=tls&sni=example.com",
	}

	data, err := configmgr.GenerateXrayConfig(cfg, nil, 10808, 10809)
	if err != nil {
		t.Fatalf("GenerateXrayConfig failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated json: %v", err)
	}

	// 1. Verify SOCKS inbound has sniffing for http, tls, quic
	inbounds, ok := parsed["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		t.Fatalf("expected inbounds in generated config")
	}
	socksInbound := inbounds[0].(map[string]interface{})
	sniffing, ok := socksInbound["sniffing"].(map[string]interface{})
	if !ok || sniffing["enabled"] != true {
		t.Fatalf("expected sniffing to be enabled on socks inbound")
	}
	destOverride, _ := sniffing["destOverride"].([]interface{})
	hasQuic := false
	for _, d := range destOverride {
		if d == "quic" {
			hasQuic = true
		}
	}
	if !hasQuic {
		t.Errorf("expected sniffing destOverride to include 'quic'")
	}

	// 2. Verify proxy outbound streamSettings has sockopt mark = 81
	outbounds, ok := parsed["outbounds"].([]interface{})
	if !ok || len(outbounds) == 0 {
		t.Fatalf("expected outbounds in generated config")
	}
	proxyOut := outbounds[0].(map[string]interface{})
	streamSettings, ok := proxyOut["streamSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected streamSettings in proxy outbound")
	}
	sockopt, ok := streamSettings["sockopt"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected sockopt in streamSettings, got nil")
	}
	markVal, ok := sockopt["mark"].(float64)
	if !ok || int(markVal) != 81 {
		t.Fatalf("expected sockopt.mark == 81 (0x51), got %v", sockopt["mark"])
	}
}
