package configmgr

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/v2raynix/v2raynix/internal/store"
)

var (
	ErrUnsupportedProtocol = errors.New("unsupported protocol scheme")
	ErrMalformedLink       = errors.New("malformed configuration link")
)

type VMessJSON struct {
	V    interface{} `json:"v"`
	Ps   string      `json:"ps"`
	Add  string      `json:"add"`
	Port interface{} `json:"port"`
	ID   string      `json:"id"`
	Aid  interface{} `json:"aid"`
	Scy  string      `json:"scy"`
	Net  string      `json:"net"`
	Type string      `json:"type"`
	Host string      `json:"host"`
	Path string      `json:"path"`
	TLS  string      `json:"tls"`
	Sni  string      `json:"sni"`
}

// ParseShareLink converts a share link or raw JSON to a ConfigItem
func ParseShareLink(raw string) (*store.ConfigItem, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, ErrMalformedLink
	}

	// Check if raw JSON
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return parseRawJSON(trimmed)
	}

	if strings.HasPrefix(trimmed, "vless://") {
		return parseVLESS(trimmed)
	} else if strings.HasPrefix(trimmed, "vmess://") {
		return parseVMess(trimmed)
	} else if strings.HasPrefix(trimmed, "trojan://") {
		return parseTrojan(trimmed)
	} else if strings.HasPrefix(trimmed, "ss://") {
		return parseShadowsocks(trimmed)
	}

	return nil, fmt.Errorf("%w: %s", ErrUnsupportedProtocol, trimmed)
}

func parseVLESS(link string) (*store.ConfigItem, error) {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		return nil, ErrMalformedLink
	}

	portStr := u.Port()
	if portStr == "" {
		return nil, ErrMalformedLink
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return nil, ErrMalformedLink
	}

	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("VLESS-%s:%d", u.Hostname(), port)
	} else {
		name, _ = url.QueryUnescape(name)
	}

	return &store.ConfigItem{
		ID:        generateID(),
		Name:      name,
		Protocol:  "vless",
		Server:    u.Hostname(),
		Port:      port,
		RawURL:    link,
		LatencyMs: -1,
		IsActive:  false,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func parseVMess(link string) (*store.ConfigItem, error) {
	b64Data := strings.TrimPrefix(link, "vmess://")
	if idx := strings.IndexAny(b64Data, "#?"); idx != -1 {
		b64Data = b64Data[:idx]
	}
	decoded, err := decodeBase64(b64Data)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base64 in vmess", ErrMalformedLink)
	}

	var vmess VMessJSON
	if err := json.Unmarshal(decoded, &vmess); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON in vmess: %v", ErrMalformedLink, err)
	}

	port := 0
	switch p := vmess.Port.(type) {
	case float64:
		port = int(p)
	case int:
		port = p
	case int64:
		port = int(p)
	case string:
		port, _ = strconv.Atoi(strings.TrimSpace(p))
	}

	if strings.TrimSpace(vmess.Add) == "" || port <= 0 || port > 65535 || strings.TrimSpace(vmess.ID) == "" {
		return nil, ErrMalformedLink
	}

	name := vmess.Ps
	if name == "" {
		name = fmt.Sprintf("VMess-%s:%d", vmess.Add, port)
	}

	return &store.ConfigItem{
		ID:        generateID(),
		Name:      name,
		Protocol:  "vmess",
		Server:    vmess.Add,
		Port:      port,
		RawURL:    link,
		LatencyMs: -1,
		IsActive:  false,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func parseTrojan(link string) (*store.ConfigItem, error) {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		return nil, ErrMalformedLink
	}

	portStr := u.Port()
	if portStr == "" {
		return nil, ErrMalformedLink
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return nil, ErrMalformedLink
	}

	name := u.Fragment
	if name == "" {
		name = fmt.Sprintf("Trojan-%s:%d", u.Hostname(), port)
	} else {
		name, _ = url.QueryUnescape(name)
	}

	return &store.ConfigItem{
		ID:        generateID(),
		Name:      name,
		Protocol:  "trojan",
		Server:    u.Hostname(),
		Port:      port,
		RawURL:    link,
		LatencyMs: -1,
		IsActive:  false,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func parseShadowsocks(link string) (*store.ConfigItem, error) {
	withoutScheme := strings.TrimPrefix(link, "ss://")
	parts := strings.SplitN(withoutScheme, "#", 2)
	name := ""
	if len(parts) == 2 {
		name, _ = url.QueryUnescape(parts[1])
	}

	mainPart := parts[0]
	// Can be ss://base64(method:password@host:port) or ss://base64(method:password)@host:port
	var server string
	var port int

	if strings.Contains(mainPart, "@") {
		sub := strings.SplitN(mainPart, "@", 2)
		hostPort := sub[1]
		hp := strings.SplitN(hostPort, ":", 2)
		if len(hp) != 2 {
			return nil, ErrMalformedLink
		}
		server = hp[0]
		portStr := hp[1]
		if idx := strings.IndexAny(portStr, "?#"); idx != -1 {
			portStr = portStr[:idx]
		}
		port, _ = strconv.Atoi(portStr)
	} else {
		decoded, err := decodeBase64(mainPart)
		if err != nil {
			return nil, ErrMalformedLink
		}
		decStr := string(decoded)
		if strings.Contains(decStr, "@") {
			sub := strings.SplitN(decStr, "@", 2)
			hp := strings.SplitN(sub[1], ":", 2)
			if len(hp) != 2 {
				return nil, ErrMalformedLink
			}
			server = hp[0]
			portStr := hp[1]
			if idx := strings.IndexAny(portStr, "?#"); idx != -1 {
				portStr = portStr[:idx]
			}
			port, _ = strconv.Atoi(portStr)
		}
	}

	if server == "" || port <= 0 {
		return nil, ErrMalformedLink
	}

	if name == "" {
		name = fmt.Sprintf("SS-%s:%d", server, port)
	}

	return &store.ConfigItem{
		ID:        generateID(),
		Name:      name,
		Protocol:  "shadowsocks",
		Server:    server,
		Port:      port,
		RawURL:    link,
		LatencyMs: -1,
		IsActive:  false,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func parseRawJSON(content string) (*store.ConfigItem, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, err
	}

	name := "Custom-Xray-JSON"
	return &store.ConfigItem{
		ID:        generateID(),
		Name:      name,
		Protocol:  "custom_json",
		Server:    "127.0.0.1",
		Port:      0,
		RawURL:    content,
		LatencyMs: -1,
		IsActive:  false,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func decodeBase64(s string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, s)

	// Standard with padding
	if b, err := base64.StdEncoding.DecodeString(clean); err == nil {
		return b, nil
	}
	// Raw standard without padding
	if b, err := base64.RawStdEncoding.DecodeString(clean); err == nil {
		return b, nil
	}
	// URL-safe with padding
	if b, err := base64.URLEncoding.DecodeString(clean); err == nil {
		return b, nil
	}
	// Raw URL-safe without padding
	return base64.RawURLEncoding.DecodeString(clean)
}

var idCounter uint64

func generateID() string {
	cnt := atomic.AddUint64(&idCounter, 1)
	return fmt.Sprintf("cfg-%d-%d", time.Now().UnixNano(), cnt)
}
