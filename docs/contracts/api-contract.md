# V2Raynix Interface & API Contract (Single Source of Truth)

This contract defines the immutable interface specifications between the Backend (Go) and Frontend (React), as well as internal system modules. All implementer agents must strictly adhere to this schema.

---

## 1. Shared Data Models

### 1.1 ConfigItem
```typescript
interface ConfigItem {
  id: string;              // UUID string
  name: string;            // User-friendly label or extracted remark
  protocol: "vless" | "vmess" | "trojan" | "shadowsocks" | "custom_json";
  server: string;          // IP or domain of proxy server
  port: number;            // Port number (1-65535)
  rawUrl: string;          // Original share link or raw JSON
  latencyMs: number;       // Latency in milliseconds (-1 for timeout/untested)
  isActive: boolean;       // True if this is the currently activated outbound
  createdAt: string;       // ISO 8601 timestamp
}
```

### 1.2 RoutingRule
```typescript
interface RoutingRule {
  id: string;              // UUID string
  target: string;          // Domain (e.g. "geosite:ir"), IP or CIDR (e.g. "192.168.1.0/24")
  targetType: "domain" | "ip" | "preset";
  action: "direct" | "proxy" | "block";
  isEnabled: boolean;
  priority: number;        // Higher priority evaluated first
}
```

### 1.3 TunnelStatus
```typescript
interface TunnelStatus {
  state: "disconnected" | "connecting" | "connected" | "rolling_back";
  activeConfigId: string | null;
  activeConfigName: string | null;
  tunInterface: string;    // e.g. "tun0"
  uptimeSeconds: number;
  uploadSpeedBps: number;  // Bytes per second
  downloadSpeedBps: number;
  totalUploadBytes: number;
  totalDownloadBytes: number;
  safeMode: {
    isActive: boolean;
    remainingSeconds: number;
  };
}
```

---

## 2. REST API Endpoints Specification

### 2.1 Authentication (`/api/auth`)

* **`POST /api/auth/login`**
  * **Request:** `{ "username": "admin", "password": "string" }`
  * **Response 200:** `{ "token": "jwt_token_string", "user": { "username": "admin" } }`
  * **Response 401:** `{ "error": "invalid credentials" }`

* **`GET /api/auth/me`** (Protected: Header `Authorization: Bearer <token>`)
  * **Response 200:** `{ "authenticated": true, "username": "admin" }`
  * **Response 401:** `{ "error": "unauthorized" }`

* **`POST /api/auth/password`** (Protected)
  * **Request:** `{ "currentPassword": "string", "newPassword": "string" }`
  * **Response 200:** `{ "message": "password updated" }`

---

### 2.2 Config Management (`/api/configs`)

* **`GET /api/configs`** (Protected)
  * **Response 200:** `ConfigItem[]`

* **`POST /api/configs`** (Protected - Import config/links)
  * **Request:** `{ "content": "vless://... or vmess://... or JSON content", "name": "optional name" }`
  * **Response 201:** `ConfigItem` (or `ConfigItem[]` if multi-line links provided)
  * **Response 400:** `{ "error": "invalid configuration format" }`

* **`DELETE /api/configs/{id}`** (Protected)
  * **Response 200:** `{ "message": "deleted" }`

* **`POST /api/configs/{id}/activate`** (Protected - Switch active config)
  * **Response 200:** `{ "message": "activated", "activeConfig": ConfigItem }`

* **`POST /api/configs/ping-all`** (Protected - Trigger latency test)
  * **Response 200:** `Record<string, number>` (Map of Config ID -> LatencyMs)

---

### 2.3 Tunnel & Safe Mode Control (`/api/tunnel`)

* **`GET /api/tunnel/status`** (Protected)
  * **Response 200:** `TunnelStatus`

* **`POST /api/tunnel/connect`** (Protected - Start tun2socks with active config)
  * **Request:** `{ "configId": "optional_id" }`
  * **Response 200:** `TunnelStatus`

* **`POST /api/tunnel/disconnect`** (Protected - Graceful stop and route cleanup)
  * **Response 200:** `TunnelStatus`

* **`POST /api/tunnel/safe-mode/confirm`** (Protected - Commit confirmed)
  * **Response 200:** `{ "message": "safe mode confirmed, changes persistent" }`

* **`POST /api/tunnel/safe-mode/rollback`** (Protected - Immediate rollback)
  * **Response 200:** `{ "message": "rolled back to safe un-tunneled state" }`

---

### 2.4 Routing Rules (`/api/routing/rules`)

* **`GET /api/routing/rules`** (Protected)
  * **Response 200:** `RoutingRule[]`

* **`POST /api/routing/rules`** (Protected - Add rule)
  * **Request:** `{ "target": "string", "targetType": "domain"|"ip", "action": "direct"|"proxy"|"block" }`
  * **Response 201:** `RoutingRule`

* **`DELETE /api/routing/rules/{id}`** (Protected)
  * **Response 200:** `{ "message": "rule deleted" }`

---

### 2.5 System & Logs (`/api/system`)

* **`GET /api/system/logs`** (Protected)
  * **Query Params:** `?limit=100&level=info|warn|error`
  * **Response 200:** `Array<{ timestamp: string, level: string, message: string }>`
