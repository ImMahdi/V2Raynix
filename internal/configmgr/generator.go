package configmgr

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
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
				"destOverride": []string{"http", "tls", "quic"},
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
			"streamSettings": map[string]interface{}{
				"sockopt": map[string]interface{}{
					"mark": 81,
				},
			},
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

		enc := q.Get("encryption")
		if enc == "" {
			enc = "none"
		}

		vnext := map[string]interface{}{
			"address": cfg.Server,
			"port":    cfg.Port,
			"users": []map[string]interface{}{
				{
					"id":         uuid,
					"encryption": enc,
					"flow":       q.Get("flow"),
				},
			},
		}

		netType := q.Get("type")
		if netType == "" {
			netType = "tcp"
		}

		sec := q.Get("security")
		streamSettings := map[string]interface{}{
			"network":  netType,
			"security": sec,
		}

		if sec == "reality" {
			pbk := q.Get("pbk")
			if pbk == "" {
				return nil, fmt.Errorf("vless reality requires a non-empty public key (pbk)")
			}
			sni := q.Get("sni")
			if sni == "" {
				return nil, fmt.Errorf("vless reality requires serverName (sni)")
			}
			streamSettings["realitySettings"] = map[string]interface{}{
				"serverName":  sni,
				"publicKey":   pbk,
				"shortId":     q.Get("sid"),
				"fingerprint": q.Get("fp"),
			}
		} else if sec == "tls" {
			tlsMap := map[string]interface{}{
				"serverName": q.Get("sni"),
			}
			if fp := q.Get("fp"); fp != "" {
				tlsMap["fingerprint"] = fp
			}
			if alpn := q.Get("alpn"); alpn != "" {
				tlsMap["alpn"] = strings.Split(alpn, ",")
			}
			streamSettings["tlsSettings"] = tlsMap
		}

		flow := q.Get("flow")
		if flow == "xtls-rprx-vision" {
			if netType != "tcp" || (sec != "reality" && sec != "tls") {
				return nil, fmt.Errorf("xtls-rprx-vision is only supported with TCP transport and TLS or Reality security")
			}
		}

		if netType == "xhttp" || netType == "splithttp" {
			xhttpMap := map[string]interface{}{}
			if p := q.Get("path"); p != "" {
				xhttpMap["path"] = p
			}
			if h := q.Get("host"); h != "" {
				xhttpMap["host"] = h
			}
			if m := q.Get("mode"); m != "" {
				xhttpMap["mode"] = m
			}
			streamSettings["xhttpSettings"] = xhttpMap
		} else if netType == "ws" {
			wsMap := map[string]interface{}{}
			if p := q.Get("path"); p != "" {
				wsMap["path"] = p
			}
			if h := q.Get("host"); h != "" {
				wsMap["headers"] = map[string]string{"Host": h}
			}
			streamSettings["wsSettings"] = wsMap
		} else if netType == "grpc" {
			grpcMap := map[string]interface{}{}
			if sName := q.Get("serviceName"); sName != "" {
				grpcMap["serviceName"] = sName
			}
			streamSettings["grpcSettings"] = grpcMap
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

		alterID := 0
		switch a := vmess.Aid.(type) {
		case float64:
			alterID = int(a)
		case int:
			alterID = a
		case int64:
			alterID = int(a)
		case string:
			alterID, _ = strconv.Atoi(strings.TrimSpace(a))
		}

		vnext := map[string]interface{}{
			"address": cfg.Server,
			"port":    cfg.Port,
			"users": []map[string]interface{}{
				{
					"id":       vmess.ID,
					"alterId":  alterID,
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
			tlsMap := map[string]interface{}{}
			if vmess.Sni != "" {
				tlsMap["serverName"] = vmess.Sni
			} else if vmess.Host != "" {
				tlsMap["serverName"] = vmess.Host
			}
			streamSettings["tlsSettings"] = tlsMap
		}

		if network == "ws" {
			wsMap := map[string]interface{}{}
			if vmess.Path != "" {
				wsMap["path"] = vmess.Path
			}
			if vmess.Host != "" {
				wsMap["headers"] = map[string]string{"Host": vmess.Host}
			}
			streamSettings["wsSettings"] = wsMap
		} else if network == "grpc" {
			grpcMap := map[string]interface{}{}
			if vmess.Path != "" {
				grpcMap["serviceName"] = vmess.Path
			}
			streamSettings["grpcSettings"] = grpcMap
		} else if network == "h2" || network == "http" {
			httpMap := map[string]interface{}{}
			if vmess.Path != "" {
				httpMap["path"] = vmess.Path
			}
			if vmess.Host != "" {
				httpMap["host"] = strings.Split(vmess.Host, ",")
			}
			streamSettings["httpSettings"] = httpMap
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
			// SIP002 format: base64(method:password)@host:port
			sub := strings.SplitN(mainPart, "@", 2)
			decoded, err := decodeBase64(sub[0])
			if err != nil {
				return nil, fmt.Errorf("invalid base64 in shadowsocks URL: %w", err)
			}
			mp := strings.SplitN(string(decoded), ":", 2)
			if len(mp) == 2 {
				method = mp[0]
				password = mp[1]
			}
		} else {
			// Legacy format: base64(method:password@host:port)
			decoded, err := decodeBase64(mainPart)
			if err != nil {
				return nil, fmt.Errorf("invalid base64 in legacy shadowsocks URL: %w", err)
			}
			decStr := string(decoded)
			if strings.Contains(decStr, "@") {
				sub := strings.SplitN(decStr, "@", 2)
				mp := strings.SplitN(sub[0], ":", 2)
				if len(mp) == 2 {
					method = mp[0]
					password = mp[1]
				}
			}
		}

		if method == "" || password == "" {
			return nil, fmt.Errorf("missing or invalid method/password in shadowsocks configuration")
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

	// Ensure streamSettings has sockopt with mark 81 (0x51) for Linux policy routing anti-loop bypass
	ss, ok := outbound["streamSettings"].(map[string]interface{})
	if !ok || ss == nil {
		ss = make(map[string]interface{})
	}
	ss["sockopt"] = map[string]interface{}{
		"mark": 81,
	}
	outbound["streamSettings"] = ss

	return outbound, nil
}
