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
	if alpn, ok := tlsSettings["alpn"].([]interface{}); !ok || len(alpn) != 2 || alpn[0] != "h2" || alpn[1] != "http/1.1" {
		t.Errorf("expected alpn [h2, http/1.1], got %v", tlsSettings["alpn"])
	}
}

func TestDirectOutboundMark(t *testing.T) {
	cfg := &store.ConfigItem{
		ID:       "cfg-test-direct",
		Name:     "Test Direct Outbound",
		Protocol: "vless",
		Server:   "1.2.3.4",
		Port:     443,
		RawURL:   "vless://uuid-123@1.2.3.4:443?type=tcp&security=tls",
	}

	data, err := configmgr.GenerateXrayConfig(cfg, nil, 10808, 10809)
	if err != nil {
		t.Fatalf("GenerateXrayConfig failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated json: %v", err)
	}

	outbounds, ok := parsed["outbounds"].([]interface{})
	if !ok || len(outbounds) < 2 {
		t.Fatalf("expected at least 2 outbounds in config")
	}

	var directOut map[string]interface{}
	for _, o := range outbounds {
		outMap := o.(map[string]interface{})
		if outMap["tag"] == "direct" {
			directOut = outMap
			break
		}
	}

	if directOut == nil {
		t.Fatalf("expected 'direct' outbound in generated config")
	}

	streamSettings, ok := directOut["streamSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected streamSettings in direct outbound to prevent routing loop (CFG-01)")
	}

	sockopt, ok := streamSettings["sockopt"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected sockopt in direct streamSettings")
	}

	if mark, ok := sockopt["mark"].(float64); !ok || int(mark) != 81 {
		t.Errorf("expected sockopt.mark == 81 on direct outbound, got %v", sockopt["mark"])
	}
}

func TestGenerateXrayConfig_VMessAlterID(t *testing.T) {
	// VMess config with aid = 64
	b64 := "eyJ2IjoiMiIsInBzIjoiYWlkLW5vZGUiLCJhZGQiOiIxLjIuMy40IiwicG9ydCI6IjQ0MyIsImlkIjoiOTZjNGQ3YjItNTIwZS00YjY5LThjZTItNGUwZDRjODJiOTUyIiwiYWlkIjoiNjQiLCJuZXQiOiJ0Y3AifQ=="
	cfg := &store.ConfigItem{
		ID:       "cfg-vmess-aid",
		Name:     "VMess Aid",
		Protocol: "vmess",
		Server:   "1.2.3.4",
		Port:     443,
		RawURL:   "vmess://" + b64,
	}

	data, err := configmgr.GenerateXrayConfig(cfg, nil, 10808, 10809)
	if err != nil {
		t.Fatalf("GenerateXrayConfig failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	outbounds := parsed["outbounds"].([]interface{})
	proxyOut := outbounds[0].(map[string]interface{})
	settings := proxyOut["settings"].(map[string]interface{})
	vnext := settings["vnext"].([]interface{})[0].(map[string]interface{})
	users := vnext["users"].([]interface{})[0].(map[string]interface{})

	if aid, ok := users["alterId"].(float64); !ok || int(aid) != 64 {
		t.Errorf("expected alterId 64, got %v", users["alterId"])
	}
}

func TestGenerateXrayConfig_ShadowsocksLegacy(t *testing.T) {
	// ss://base64(method:password@host:port)#LegacyNode
	// bf-cfb:test@198.51.100.4:8888 -> base64: YmYtY2ZiOnRlc3RAMTk4LjUxLjEwMC40Ojg4ODg=
	cfg := &store.ConfigItem{
		ID:       "cfg-ss-legacy",
		Name:     "Legacy SS",
		Protocol: "shadowsocks",
		Server:   "198.51.100.4",
		Port:     8888,
		RawURL:   "ss://YmYtY2ZiOnRlc3RAMTk4LjUxLjEwMC40Ojg4ODg=#LegacyNode",
	}

	data, err := configmgr.GenerateXrayConfig(cfg, nil, 10808, 10809)
	if err != nil {
		t.Fatalf("GenerateXrayConfig failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	outbounds := parsed["outbounds"].([]interface{})
	proxyOut := outbounds[0].(map[string]interface{})
	settings := proxyOut["settings"].(map[string]interface{})
	servers := settings["servers"].([]interface{})[0].(map[string]interface{})

	if servers["method"] != "bf-cfb" {
		t.Errorf("expected method 'bf-cfb', got '%v'", servers["method"])
	}
	if servers["password"] != "test" {
		t.Errorf("expected password 'test', got '%v'", servers["password"])
	}
}

func TestGenerateXrayConfig_VMessCamouflage(t *testing.T) {
	// 1. VMess WebSocket + TLS + custom Path & Host & SNI
	b64WS := "eyJ2IjoiMiIsInBzIjoid3Mtbm9kZSIsImFkZCI6IjEuMi4zLjQiLCJwb3J0IjoiNDQzIiwiaWQiOiI5NmM0ZDdiMi01MjBlLTRiNjktOGNlMi00ZTBkNGM4MmI5NTIiLCJhaWQiOiIwIiwibmV0Ijoid3MiLCJwYXRoIjoiL215LXdzLXBhdGgiLCJob3N0IjoiY2RuLmV4YW1wbGUuY29tIiwidGxzIjoidGxzIiwic25pIjoic25pLmV4YW1wbGUuY29tIn0="
	cfgWS := &store.ConfigItem{
		ID:       "cfg-vmess-ws",
		Name:     "VMess WS",
		Protocol: "vmess",
		Server:   "1.2.3.4",
		Port:     443,
		RawURL:   "vmess://" + b64WS,
	}

	dataWS, err := configmgr.GenerateXrayConfig(cfgWS, nil, 10808, 10809)
	if err != nil {
		t.Fatalf("GenerateXrayConfig failed: %v", err)
	}

	var parsedWS map[string]interface{}
	if err := json.Unmarshal(dataWS, &parsedWS); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	outboundsWS := parsedWS["outbounds"].([]interface{})
	proxyOutWS := outboundsWS[0].(map[string]interface{})
	streamWS := proxyOutWS["streamSettings"].(map[string]interface{})

	// Check security and tlsSettings
	if streamWS["security"] != "tls" {
		t.Errorf("expected security tls, got %v", streamWS["security"])
	}
	tlsSettings, ok := streamWS["tlsSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tlsSettings in streamSettings")
	}
	if tlsSettings["serverName"] != "sni.example.com" {
		t.Errorf("expected serverName sni.example.com, got %v", tlsSettings["serverName"])
	}

	// Check wsSettings
	wsSettings, ok := streamWS["wsSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected wsSettings in streamSettings")
	}
	if wsSettings["path"] != "/my-ws-path" {
		t.Errorf("expected path /my-ws-path, got %v", wsSettings["path"])
	}
	headers, ok := wsSettings["headers"].(map[string]interface{})
	if !ok || headers["Host"] != "cdn.example.com" {
		t.Errorf("expected Host header cdn.example.com, got %v", headers["Host"])
	}

	// 2. VMess gRPC + serviceName
	b64GRPC := "eyJ2IjoiMiIsInBzIjoiZ3JwYy1ub2RlIiwiYWRkIjoiMS4yLjMuNCIsInBvcnQiOiI0NDMiLCJpZCI6Ijk2YzRkN2IyLTUyMGUtNGI2OS04Y2UyLTRlMGQ0YzgyYjk1MiIsImFpZCI6IjAiLCJuZXQiOiJncnBjIiwicGF0aCI6Im15LWdycGMtc2VydmljZSIsInRscyI6IiJ9"
	cfgGRPC := &store.ConfigItem{
		ID:       "cfg-vmess-grpc",
		Name:     "VMess gRPC",
		Protocol: "vmess",
		Server:   "1.2.3.4",
		Port:     443,
		RawURL:   "vmess://" + b64GRPC,
	}

	dataGRPC, err := configmgr.GenerateXrayConfig(cfgGRPC, nil, 10808, 10809)
	if err != nil {
		t.Fatalf("GenerateXrayConfig failed: %v", err)
	}

	var parsedGRPC map[string]interface{}
	if err := json.Unmarshal(dataGRPC, &parsedGRPC); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	outboundsGRPC := parsedGRPC["outbounds"].([]interface{})
	proxyOutGRPC := outboundsGRPC[0].(map[string]interface{})
	streamGRPC := proxyOutGRPC["streamSettings"].(map[string]interface{})

	grpcSettings, ok := streamGRPC["grpcSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected grpcSettings in streamSettings")
	}
	if grpcSettings["serviceName"] != "my-grpc-service" {
		t.Errorf("expected serviceName my-grpc-service, got %v", grpcSettings["serviceName"])
	}
}



