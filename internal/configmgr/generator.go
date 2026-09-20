package configmgr

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/v2raynix/v2raynix/internal/store"
)

// GenerateXrayConfig builds the full JSON config for Xray-core
func GenerateXrayConfig(activeConfig *store.ConfigItem, rules []*store.RoutingRule, localSocksPort, localHttpPort int) ([]byte, error) {
	if activeConfig == nil {
		return nil, fmt.Errorf("activeConfig cannot be nil")
	}

	// If it's already a raw custom JSON, return its bytes directly
	if activeConfig.Protocol == "custom_json" {
		return []byte(activeConfig.RawURL), nil
	}

	proxyOutbound, err := buildProxyOutbound(activeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build proxy outbound: %w", err)
	}

	inbounds := []map[string]interface{}{
		{
			"tag":      "socks-in",
			"port":     localSocksPort,
			"listen":   "127.0.0.1",
			"protocol": "socks",
			"settings": map[string]interface{}{
				"auth": "noauth",
				"udp":  true,
			},
			"sniffing": map[string]interface{}{
				"enabled":      true,
				"destOverride": []string{"http", "tls"},
			},
		},
		{
			"tag":      "http-in",
			"port":     localHttpPort,
			"listen":   "127.0.0.1",
			"protocol": "http",
			"settings": map[string]interface{}{
				"allowTransparent": false,
			},
		},
	}

	outbounds := []map[string]interface{}{
		proxyOutbound,
		{
			"tag":      "direct",
			"protocol": "freedom",
			"settings": map[string]interface{}{},
		},
		{
			"tag":      "block",
			"protocol": "blackhole",
			"settings": map[string]interface{}{},
		},
	}

	// Build routing rules
	var xRoutingRules []map[string]interface{}
	for _, r := range rules {
		if !r.IsEnabled {
			continue
		}

		ruleMap := map[string]interface{}{
			"type":        "field",
			"outboundTag": r.Action, // "direct", "proxy", "block"
		}

		if r.Action == "proxy" {
			ruleMap["outboundTag"] = "proxy"
		}

		if r.TargetType == "domain" {
			ruleMap["domain"] = []string{r.Target}
		} else if r.TargetType == "ip" {
			ruleMap["ip"] = []string{r.Target}
		}

		xRoutingRules = append(xRoutingRules, ruleMap)
	}

	config := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing": map[string]interface{}{
			"domainStrategy": "IPIfNonMatch",
			"rules":          xRoutingRules,
		},
	}

	return json.MarshalIndent(config, "", "  ")
}

func buildProxyOutbound(cfg *store.ConfigItem) (map[string]interface{}, error) {
	outbound := map[string]interface{}{
		"tag":      "proxy",
		"protocol": cfg.Protocol,
	}

	switch cfg.Protocol {
	case "vless":
		u, err := url.Parse(cfg.RawURL)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		uuid := u.User.Username()

		vnext := map[string]interface{}{
			"address": cfg.Server,
			"port":    cfg.Port,
			"users": []map[string]interface{}{
				{
					"id":         uuid,
					"encryption": "none",
					"flow":       q.Get("flow"),
				},
			},
		}

		streamSettings := map[string]interface{}{
			"network":  q.Get("type"),
			"security": q.Get("security"),
		}

		if q.Get("security") == "reality" {
			streamSettings["realitySettings"] = map[string]interface{}{
				"serverName": q.Get("sni"),
				"publicKey":  q.Get("pbk"),
				"shortId":    q.Get("sid"),
				"fingerprint": q.Get("fp"),
			}
		} else if q.Get("security") == "tls" {
			streamSettings["tlsSettings"] = map[string]interface{}{
				"serverName": q.Get("sni"),
			}
		}

		outbound["settings"] = map[string]interface{}{
			"vnext": []interface{}{vnext},
		}
		outbound["streamSettings"] = streamSettings

	case "vmess":
		b64 := strings.TrimPrefix(cfg.RawURL, "vmess://")
		decoded, err := decodeBase64(b64)
		if err != nil {
			return nil, err
		}
		var vmess VMessJSON
		if err := json.Unmarshal(decoded, &vmess); err != nil {
			return nil, err
		}

		vnext := map[string]interface{}{
			"address": cfg.Server,
			"port":    cfg.Port,
			"users": []map[string]interface{}{
				{
					"id":       vmess.ID,
					"alterId":  0,
					"security": "auto",
				},
			},
		}

		outbound["settings"] = map[string]interface{}{
			"vnext": []interface{}{vnext},
		}

		network := vmess.Net
		if network == "" {
			network = "tcp"
		}
		streamSettings := map[string]interface{}{
			"network": network,
		}
		if vmess.TLS == "tls" {
			streamSettings["security"] = "tls"
		}
		outbound["streamSettings"] = streamSettings

	case "trojan":
		u, err := url.Parse(cfg.RawURL)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		password := u.User.Username()

		servers := map[string]interface{}{
			"address":  cfg.Server,
			"port":     cfg.Port,
			"password": password,
		}

		outbound["settings"] = map[string]interface{}{
			"servers": []interface{}{servers},
		}
		outbound["streamSettings"] = map[string]interface{}{
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"serverName": q.Get("sni"),
			},
		}

	case "shadowsocks":
		withoutScheme := strings.TrimPrefix(cfg.RawURL, "ss://")
		parts := strings.SplitN(withoutScheme, "#", 2)
		mainPart := parts[0]

		var method, password string
		if strings.Contains(mainPart, "@") {
			sub := strings.SplitN(mainPart, "@", 2)
			decoded, err := decodeBase64(sub[0])
			if err == nil {
				mp := strings.SplitN(string(decoded), ":", 2)
				if len(mp) == 2 {
					method = mp[0]
					password = mp[1]
				}
			}
		}

		servers := map[string]interface{}{
			"address":  cfg.Server,
			"port":     cfg.Port,
			"method":   method,
			"password": password,
		}

		outbound["settings"] = map[string]interface{}{
			"servers": []interface{}{servers},
		}

	default:
		return nil, fmt.Errorf("unsupported protocol for outbound: %s", cfg.Protocol)
	}

	return outbound, nil
}
