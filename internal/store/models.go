package store

// ConfigItem represents a saved proxy configuration
type ConfigItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Protocol  string `json:"protocol"` // vless, vmess, trojan, shadowsocks, custom_json
	Server    string `json:"server"`
	Port      int    `json:"port"`
	RawURL    string `json:"rawUrl"`
	LatencyMs int    `json:"latencyMs"`
	IsActive  bool   `json:"isActive"`
	CreatedAt string `json:"createdAt"`
}

// RoutingRule represents a traffic classification rule
type RoutingRule struct {
	ID         string `json:"id"`
	Target     string `json:"target"`     // domain or IP/CIDR
	TargetType string `json:"targetType"` // domain, ip, preset
	Action     string `json:"action"`     // direct, proxy, block
	IsEnabled  bool   `json:"isEnabled"`
	Priority   int    `json:"priority"`
}

// UserAccount represents admin authentication details
type UserAccount struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
}

// SystemSettings represents persistent application settings
type SystemSettings struct {
	WebPort         int  `json:"webPort"`
	SafeModeSeconds int  `json:"safeModeSeconds"`
	AutoStartTunnel bool `json:"autoStartTunnel"`
}
