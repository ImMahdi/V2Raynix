package configmgr_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/v2raynix/v2raynix/internal/configmgr"
	"github.com/v2raynix/v2raynix/internal/store"
)

func TestParseShareLink_VLESS(t *testing.T) {
	// Standard Reality VLESS link
	link := "vless://96c4d7b2-520e-4b69-8ce2-4e0d4c82b952@198.51.100.1:443?type=tcp&security=reality&pbk=xyz123&fp=chrome&sni=example.com&sid=abcdef#Frankfurt-Reality"

	cfg, err := configmgr.ParseShareLink(link)
	if err != nil {
		t.Fatalf("failed to parse VLESS link: %v", err)
	}

	if cfg.Protocol != "vless" {
		t.Errorf("expected protocol vless, got %s", cfg.Protocol)
	}
	if cfg.Server != "198.51.100.1" {
		t.Errorf("expected server 198.51.100.1, got %s", cfg.Server)
	}
	if cfg.Port != 443 {
		t.Errorf("expected port 443, got %d", cfg.Port)
	}
	if cfg.Name != "Frankfurt-Reality" {
		t.Errorf("expected name Frankfurt-Reality, got %s", cfg.Name)
	}
}

func TestParseShareLink_VMess(t *testing.T) {
	// Standard base64 VMess link
	// Raw JSON: {"v":"2","ps":"Amsterdam-VMess","add":"198.51.100.2","port":"8080","id":"a6c4d7b2-520e-4b69-8ce2-4e0d4c82b952","aid":"0","scy":"auto","net":"ws","type":"none","host":"example.com","path":"/ws","tls":""}
	link := "vmess://eyJ2IjoiMiIsInBzIjoiQW1zdGVyZGFtLVZNZXNzIiwiYWRkIjoiMTk4LjUxLjEwMC4yIiwicG9ydCI6IjgwODAiLCJpZCI6ImE2YzRkN2IyLTUyMGUtNGI2OS04Y2UyLTRlMGQ0YzgyYjk1MiIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0Ijoid3MiLCJ0eXBlIjoibm9uZSIsImhvc3QiOiJleGFtcGxlLmNvbSIsInBhdGgiOiIvd3MiLCJ0bHMiOiIifQ=="

	cfg, err := configmgr.ParseShareLink(link)
	if err != nil {
		t.Fatalf("failed to parse VMess link: %v", err)
	}

	if cfg.Protocol != "vmess" {
		t.Errorf("expected protocol vmess, got %s", cfg.Protocol)
	}
	if cfg.Server != "198.51.100.2" {
		t.Errorf("expected server 198.51.100.2, got %s", cfg.Server)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.Name != "Amsterdam-VMess" {
		t.Errorf("expected name Amsterdam-VMess, got %s", cfg.Name)
	}
}

func TestParseShareLink_Trojan(t *testing.T) {
	link := "trojan://secretpassword123@198.51.100.3:443?security=tls&sni=trojan.example.com#Helsinki-Trojan"

	cfg, err := configmgr.ParseShareLink(link)
	if err != nil {
		t.Fatalf("failed to parse Trojan link: %v", err)
	}

	if cfg.Protocol != "trojan" {
		t.Errorf("expected protocol trojan, got %s", cfg.Protocol)
	}
	if cfg.Server != "198.51.100.3" {
		t.Errorf("expected server 198.51.100.3, got %s", cfg.Server)
	}
	if cfg.Port != 443 {
		t.Errorf("expected port 443, got %d", cfg.Port)
	}
	if cfg.Name != "Helsinki-Trojan" {
		t.Errorf("expected name Helsinki-Trojan, got %s", cfg.Name)
	}
}

func TestParseShareLink_Shadowsocks(t *testing.T) {
	// Standard SIP002 shadowsocks URI: ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ=@198.51.100.4:8388#Tokyo-SS
	link := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ=@198.51.100.4:8388#Tokyo-SS"

	cfg, err := configmgr.ParseShareLink(link)
	if err != nil {
		t.Fatalf("failed to parse Shadowsocks link: %v", err)
	}

	if cfg.Protocol != "shadowsocks" {
		t.Errorf("expected protocol shadowsocks, got %s", cfg.Protocol)
	}
	if cfg.Server != "198.51.100.4" {
		t.Errorf("expected server 198.51.100.4, got %s", cfg.Server)
	}
	if cfg.Port != 8388 {
		t.Errorf("expected port 8388, got %d", cfg.Port)
	}
	if cfg.Name != "Tokyo-SS" {
		t.Errorf("expected name Tokyo-SS, got %s", cfg.Name)
	}
}

func TestParseShareLink_Invalid(t *testing.T) {
	_, err := configmgr.ParseShareLink("http://not-a-v2ray-link.com")
	if err == nil {
		t.Errorf("expected error for unsupported scheme")
	}

	_, err = configmgr.ParseShareLink("vless://invalid-no-port-or-host")
	if err == nil {
		t.Errorf("expected error for malformed vless link")
	}
}

func TestGenerateXrayConfig(t *testing.T) {
	cfg := &store.ConfigItem{
		ID:       "cfg-test",
		Name:     "Test Reality",
		Protocol: "vless",
		Server:   "198.51.100.1",
		Port:     443,
		RawURL:   "vless://96c4d7b2-520e-4b69-8ce2-4e0d4c82b952@198.51.100.1:443?type=tcp&security=reality&pbk=xyz123&fp=chrome&sni=example.com&sid=abcdef#Frankfurt",
	}

	rules := []*store.RoutingRule{
		{
			ID:         "rule-1",
			Target:     "geosite:ir",
			TargetType: "domain",
			Action:     "direct",
			IsEnabled:  true,
		},
		{
			ID:         "rule-2",
			Target:     "10.0.0.0/8",
			TargetType: "ip",
			Action:     "direct",
			IsEnabled:  true,
		},
	}

	configBytes, err := configmgr.GenerateXrayConfig(cfg, rules, 10808, 10809)
	if err != nil {
		t.Fatalf("failed to generate Xray config: %v", err)
	}

	// Verify valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(configBytes, &parsed); err != nil {
		t.Fatalf("generated config is not valid JSON: %v", err)
	}

	// Verify inbounds include socks port 10808
	inbounds, ok := parsed["inbounds"].([]interface{})
	if !ok || len(inbounds) == 0 {
		t.Fatalf("missing inbounds in generated config")
	}

	// Verify outbounds
	outbounds, ok := parsed["outbounds"].([]interface{})
	if !ok || len(outbounds) == 0 {
		t.Fatalf("missing outbounds in generated config")
	}

	// Verify routing rules are present
	routing, ok := parsed["routing"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing routing in config")
	}
	routingRules, ok := routing["rules"].([]interface{})
	if !ok || len(routingRules) == 0 {
		t.Fatalf("missing routing rules")
	}

	configStr := string(configBytes)
	if !strings.Contains(configStr, "10808") {
		t.Errorf("expected local socks port 10808 in config")
	}
}

func TestParser_VMessFragment(t *testing.T) {
	// VMess share link with fragment at the end
	link := "vmess://eyJ2IjoiMiIsInBzIjoidGVzdCIsImFkZCI6IjEuMi4zLjQiLCJwb3J0IjoiNDQzIiwiaWQiOiJhNmM0ZDdiMi01MjBlLTRiNjktOGNlMi00ZTBkNGM4MmI5NTIiLCJhaWQiOiIwIiwic2N5IjoiYXV0byIsIm5ldCI6IndzIiwidHlwZSI6Im5vbmUiLCJob3N0IjoiZXhhbXBsZS5jb20iLCJwYXRoIjoiL3dzIiwidGxzIjoiIn0=#RemarkFragment"
	cfg, err := configmgr.ParseShareLink(link)
	if err != nil {
		t.Fatalf("expected VMess with fragment to parse successfully, got error: %v", err)
	}
	if cfg.Server != "1.2.3.4" || cfg.Port != 443 {
		t.Errorf("unexpected parsed server or port: %s:%d", cfg.Server, cfg.Port)
	}
}

func TestParser_Base64Whitespace(t *testing.T) {
	// Base64 with embedded newlines and spaces
	rawB64 := "eyJ2IjoiMiIsInBzIjoidGVzdCIsImFkZCI6IjEuMi4zLjQiLCJwb3J0IjoiNDQzIiwiaWQiOiJhNmM0ZDdi\r\nMi01MjBlLTRiNjktOGNlMi00ZTBkNGM4MmI5NTIiLCJhaWQiOiIwIiwic2N5IjoiYXV0byIsIm5ldCI6Indz\nIiwidHlwZSI6Im5vbmUiLCJob3N0IjoiZXhhbXBsZS5jb20iLCJwYXRoIjoiL3dzIiwidGxzIjoiIn0= "
	link := "vmess://" + rawB64
	cfg, err := configmgr.ParseShareLink(link)
	if err != nil {
		t.Fatalf("expected VMess with whitespace base64 to parse successfully, got: %v", err)
	}
	if cfg.Server != "1.2.3.4" {
		t.Errorf("expected server 1.2.3.4, got %s", cfg.Server)
	}
}

func TestParser_ShadowsocksQuery(t *testing.T) {
	// Shadowsocks link with plugin query parameters
	link := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ=@198.51.100.4:8388?plugin=obfs-local%3Bobfs%3Dhttp#Tokyo-SS"
	cfg, err := configmgr.ParseShareLink(link)
	if err != nil {
		t.Fatalf("expected Shadowsocks with query parameters to parse successfully, got: %v", err)
	}
	if cfg.Port != 8388 {
		t.Errorf("expected port 8388, got %d", cfg.Port)
	}
	if cfg.Server != "198.51.100.4" {
		t.Errorf("expected server 198.51.100.4, got %s", cfg.Server)
	}
}

func TestParser_VMessTypesAndBoundaries(t *testing.T) {
	// 1. Port as raw int in JSON (not string)
	b64_1 := "eyJ2IjoiMiIsInBzIjoidGVzdDEiLCJhZGQiOiIxLjEuMS4xIiwicG9ydCI6ODQ0MywiaWQiOiI5NmM0ZDdiMi01MjBlLTRiNjktOGNlMi00ZTBkNGM4MmI5NTIiLCJhaWQiOjY0LCJuZXQiOiJ3cyJ9"
	cfg1, err := configmgr.ParseShareLink("vmess://" + b64_1)
	if err != nil {
		t.Fatalf("expected VMess with integer port to parse successfully, got: %v", err)
	}
	if cfg1.Port != 8443 {
		t.Errorf("expected port 8443, got %d", cfg1.Port)
	}

	// 2. Port out of boundary (> 65535)
	b64_2 := "eyJ2IjoiMiIsInBzIjoidGVzdDIiLCJhZGQiOiIxLjEuMS4xIiwicG9ydCI6NzAwMDAsImlkIjoiOTZjNGQ3YjItNTIwZS00YjY5LThjZTItNGUwZDRjODJiOTUyIn0="
	_, err = configmgr.ParseShareLink("vmess://" + b64_2)
	if err == nil {
		t.Errorf("expected error for port > 65535, but got nil")
	}

	// 3. Port <= 0
	b64_3 := "eyJ2IjoiMiIsInBzIjoidGVzdDMiLCJhZGQiOiIxLjEuMS4xIiwicG9ydCI6MCwiaWQiOiI5NmM0ZDdiMi01MjBlLTRiNjktOGNlMi00ZTBkNGM4MmI5NTIifQ=="
	_, err = configmgr.ParseShareLink("vmess://" + b64_3)
	if err == nil {
		t.Errorf("expected error for port <= 0, but got nil")
	}

	// 4. Missing / empty ID
	b64_4 := "eyJ2IjoiMiIsInBzIjoidGVzdDQiLCJhZGQiOiIxLjEuMS4xIiwicG9ydCI6NDQzLCJpZCI6IiJ9"
	_, err = configmgr.ParseShareLink("vmess://" + b64_4)
	if err == nil {
		t.Errorf("expected error for empty VMess ID, but got nil")
	}
}


