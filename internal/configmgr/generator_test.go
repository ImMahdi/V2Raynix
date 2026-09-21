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
		t.Fatalf("expected sockopt in streamSettings")
	}
	if mark, ok := sockopt["mark"].(float64); !ok || int(mark) != 81 {
		t.Errorf("expected sockopt.mark to be 81, got %v", sockopt["mark"])
	}
}

func TestGenerateXrayConfig_TransportAndEncryption(t *testing.T) {
	cfg := &store.ConfigItem{
		ID:       "cfg-test-xhttp",
		Name:     "XHTTP Server",
		Protocol: "vless",
		Server:   "dl.example.com",
		Port:     2096,
		RawURL:   "vless://a51bf165-5f41-4b1b-abc6-9989f8dda239@dl.example.com:2096?encryption=mlkem768x25519plus.native.0rtt.key123&security=tls&sni=dl.example.com&fp=chrome&alpn=h2%2Chttp%2F1.1&type=xhttp&host=dl.example.com&path=%2Fcustom-path&mode=auto",
	}

	data, err := configmgr.GenerateXrayConfig(cfg, nil, 10808, 10809)
	if err != nil {
		t.Fatalf("GenerateXrayConfig failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated json: %v", err)
	}

	outbounds := parsed["outbounds"].([]interface{})
	proxyOut := outbounds[0].(map[string]interface{})

	// Check encryption in vnext user
	settings := proxyOut["settings"].(map[string]interface{})
	vnext := settings["vnext"].([]interface{})[0].(map[string]interface{})
	users := vnext["users"].([]interface{})[0].(map[string]interface{})
	if users["encryption"] != "mlkem768x25519plus.native.0rtt.key123" {
		t.Errorf("expected encryption mlkem768x25519plus.native.0rtt.key123, got %v", users["encryption"])
	}

	// Check streamSettings
	stream := proxyOut["streamSettings"].(map[string]interface{})
	if stream["network"] != "xhttp" {
		t.Errorf("expected network xhttp, got %v", stream["network"])
	}

	// Check xhttpSettings
	xhttp, ok := stream["xhttpSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected xhttpSettings in streamSettings")
	}
	if xhttp["path"] != "/custom-path" {
		t.Errorf("expected path /custom-path, got %v", xhttp["path"])
	}
	if xhttp["host"] != "dl.example.com" {
		t.Errorf("expected host dl.example.com, got %v", xhttp["host"])
	}
	if xhttp["mode"] != "auto" {
		t.Errorf("expected mode auto, got %v", xhttp["mode"])
	}

	// Check tlsSettings alpn and fingerprint
	tlsSettings := stream["tlsSettings"].(map[string]interface{})
	if tlsSettings["fingerprint"] != "chrome" {
		t.Errorf("expected fingerprint chrome, got %v", tlsSettings["fingerprint"])
	}
	alpn, ok := tlsSettings["alpn"].([]interface{})
	if !ok || len(alpn) != 2 || alpn[0] != "h2" || alpn[1] != "http/1.1" {
		t.Errorf("expected alpn [h2, http/1.1], got %v", tlsSettings["alpn"])
	}
}
