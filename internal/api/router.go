package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/v2raynix/v2raynix/internal/auth"
	"github.com/v2raynix/v2raynix/internal/configmgr"
	"github.com/v2raynix/v2raynix/internal/core"
	"github.com/v2raynix/v2raynix/internal/pinger"
	"github.com/v2raynix/v2raynix/internal/store"
	"github.com/v2raynix/v2raynix/internal/updater"
)

const (
	defaultMaxBodyBytes = 2 << 20  // 2 MB for auth, rules, password
	configMaxBodyBytes  = 10 << 20 // 10 MB for configuration imports
)

type Dependencies struct {
	Store      store.Store
	Supervisor *core.Supervisor
	JWTSecret  []byte
	StaticFS   fs.FS
	Updater    *updater.Updater
}

type Router struct {
	deps         *Dependencies
	mux          *http.ServeMux
	loginLimiter *loginRateLimiter
}

func NewRouter(deps *Dependencies) http.Handler {
	if deps.Updater == nil {
		deps.Updater = updater.NewUpdater("/etc/v2raynix")
	}
	r := &Router{
		deps:         deps,
		mux:          http.NewServeMux(),
		loginLimiter: newLoginRateLimiter(5, time.Minute),
	}
	r.registerRoutes()
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Global CORS and JSON default
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	r.mux.ServeHTTP(w, req)
}

func (r *Router) registerRoutes() {
	// Public Auth
	r.mux.HandleFunc("POST /api/auth/login", r.handleLogin)

	// Protected Auth
	r.mux.HandleFunc("GET /api/auth/me", r.requireAuth(r.handleMe))
	r.mux.HandleFunc("POST /api/auth/password", r.requireAuth(r.handlePassword))

	// Configs
	r.mux.HandleFunc("GET /api/configs", r.requireAuth(r.handleGetConfigs))
	r.mux.HandleFunc("POST /api/configs", r.requireAuth(r.handleCreateConfig))
	r.mux.HandleFunc("DELETE /api/configs/{id}", r.requireAuth(r.handleDeleteConfig))
	r.mux.HandleFunc("POST /api/configs/{id}/activate", r.requireAuth(r.handleActivateConfig))
	r.mux.HandleFunc("POST /api/configs/ping-all", r.requireAuth(r.handlePingAll))
	r.mux.HandleFunc("POST /api/configs/{id}/test", r.requireAuth(r.handleTestConfig))
	r.mux.HandleFunc("POST /api/configs/test-all", r.requireAuth(r.handleTestAll))

	// Tunnel & Safe Mode
	r.mux.HandleFunc("GET /api/tunnel/status", r.requireAuth(r.handleTunnelStatus))
	r.mux.HandleFunc("POST /api/tunnel/connect", r.requireAuth(r.handleTunnelConnect))
	r.mux.HandleFunc("POST /api/tunnel/disconnect", r.requireAuth(r.handleTunnelDisconnect))
	r.mux.HandleFunc("POST /api/tunnel/safe-mode/confirm", r.requireAuth(r.handleSafeModeConfirm))
	r.mux.HandleFunc("POST /api/tunnel/safe-mode/rollback", r.requireAuth(r.handleSafeModeRollback))

	// Routing Rules
	r.mux.HandleFunc("GET /api/routing/rules", r.requireAuth(r.handleGetRoutingRules))
	r.mux.HandleFunc("POST /api/routing/rules", r.requireAuth(r.handleCreateRoutingRule))
	r.mux.HandleFunc("DELETE /api/routing/rules/{id}", r.requireAuth(r.handleDeleteRoutingRule))

	// System Logs
	r.mux.HandleFunc("GET /api/system/logs", r.requireAuth(r.handleGetLogs))

	// System Updates
	r.mux.HandleFunc("GET /api/system/updates", r.requireAuth(r.handleGetUpdates))
	r.mux.HandleFunc("POST /api/system/check-updates", r.requireAuth(r.handleCheckUpdates))
	r.mux.HandleFunc("POST /api/system/update-core", r.requireAuth(r.handleUpdateCore))

	// Static SPA Serving (if provided)
	if r.deps.StaticFS != nil {
		fileServer := http.FileServer(http.FS(r.deps.StaticFS))
		r.mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			if strings.HasPrefix(req.URL.Path, "/api/") {
				http.NotFound(w, req)
				return
			}
			fileServer.ServeHTTP(w, req)
		})
	}
}

func (r *Router) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		authHeader := req.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := auth.ValidateJWT(tokenString, r.deps.JWTSecret)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		_ = claims
		next(w, req)
	}
}

// Handler implementations

func (r *Router) handleLogin(w http.ResponseWriter, req *http.Request) {
	clientIP := getClientIP(req)
	if r.loginLimiter.isBlocked(clientIP) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many failed login attempts, please try again later"})
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, req, defaultMaxBodyBytes, &body, "invalid request body"); err != nil {
		return
	}

	admin, err := r.deps.Store.GetAdminUser()
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Initialize default admin only if explicitly not found in store
			hash, _ := auth.HashPassword("admin")
			admin = &store.UserAccount{
				Username:     "admin",
				PasswordHash: hash,
			}
			_ = r.deps.Store.SetAdminUser(admin)
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query user store"})
			return
		}
	} else if admin == nil {
		hash, _ := auth.HashPassword("admin")
		admin = &store.UserAccount{
			Username:     "admin",
			PasswordHash: hash,
		}
		_ = r.deps.Store.SetAdminUser(admin)
	}

	if body.Username != admin.Username || !auth.CheckPassword(admin.PasswordHash, body.Password) {
		r.loginLimiter.recordFailure(clientIP)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	r.loginLimiter.reset(clientIP)

	token, err := auth.GenerateJWT(admin.Username, r.deps.JWTSecret, 24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]string{
			"username": admin.Username,
		},
	})
}

func (r *Router) handleMe(w http.ResponseWriter, req *http.Request) {
	admin, err := r.deps.Store.GetAdminUser()
	username := "admin"
	if err == nil && admin != nil {
		username = admin.Username
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"username":      username,
	})
}

func (r *Router) handlePassword(w http.ResponseWriter, req *http.Request) {
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := decodeJSON(w, req, defaultMaxBodyBytes, &body, "invalid request body"); err != nil {
		return
	}

	admin, err := r.deps.Store.GetAdminUser()
	if err != nil || admin == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user not found"})
		return
	}

	if !auth.CheckPassword(admin.PasswordHash, body.CurrentPassword) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "incorrect current password"})
		return
	}

	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash new password"})
		return
	}

	admin.PasswordHash = newHash
	if err := r.deps.Store.SetAdminUser(admin); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save password"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password updated"})
}

func (r *Router) handleGetConfigs(w http.ResponseWriter, req *http.Request) {
	configs, err := r.deps.Store.GetConfigs()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, configs)
}

func (r *Router) handleCreateConfig(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Content string `json:"content"`
		Name    string `json:"name"`
	}
	if err := decodeJSON(w, req, configMaxBodyBytes, &body, "invalid request body"); err != nil {
		return
	}

	lines := strings.Split(body.Content, "\n")
	var created []*store.ConfigItem

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		item, err := configmgr.ParseShareLink(trimmed)
		if err != nil {
			continue
		}

		if body.Name != "" && len(lines) == 1 {
			item.Name = body.Name
		}

		created = append(created, item)
	}

	if len(created) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no valid proxy links found"})
		return
	}

	if err := r.deps.Store.SaveConfigsBatch(created); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist configurations"})
		return
	}

	if len(created) == 1 {
		writeJSON(w, http.StatusCreated, created[0])
	} else {
		writeJSON(w, http.StatusCreated, created)
	}
}

func (r *Router) handleDeleteConfig(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	if err := r.deps.Store.DeleteConfig(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (r *Router) handleActivateConfig(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	if err := r.deps.Store.SetActiveConfig(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	active, _ := r.deps.Store.GetActiveConfig()

	// If tunnel currently running, switch live; otherwise update supervisor preselected config
	if r.deps.Supervisor != nil {
		if r.deps.Supervisor.GetStatus().State == "connected" {
			_ = r.deps.Supervisor.StartTunnel(active)
		} else {
			r.deps.Supervisor.SetActiveConfig(active)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "activated",
		"activeConfig": active,
	})
}

func (r *Router) handlePingAll(w http.ResponseWriter, req *http.Request) {
	configs, err := r.deps.Store.GetConfigs()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	results := pinger.BatchPingContext(req.Context(), configs, 5, 2*time.Second)
	_ = r.deps.Store.UpdateLatenciesBatch(results)

	writeJSON(w, http.StatusOK, results)
}

func (r *Router) handleTestConfig(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	cfg, err := r.deps.Store.GetConfigByID(id)
	if err != nil || cfg == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "config not found"})
		return
	}

	lat, _ := pinger.TestConfigRealDelay(req.Context(), cfg, 3*time.Second)
	_ = r.deps.Store.UpdateLatency(id, lat)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":        id,
		"latencyMs": lat,
	})
}

func (r *Router) handleTestAll(w http.ResponseWriter, req *http.Request) {
	configs, err := r.deps.Store.GetConfigs()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	results := pinger.BatchRealTestContext(req.Context(), configs, 5, 3*time.Second)
	_ = r.deps.Store.UpdateLatenciesBatch(results)

	writeJSON(w, http.StatusOK, results)
}

func (r *Router) handleTunnelStatus(w http.ResponseWriter, req *http.Request) {
	status := r.deps.Supervisor.GetStatus()
	writeJSON(w, http.StatusOK, status)
}

func (r *Router) handleTunnelConnect(w http.ResponseWriter, req *http.Request) {
	active, err := r.deps.Store.GetActiveConfig()
	if err != nil || active == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no active config selected"})
		return
	}

	if err := r.deps.Supervisor.StartTunnel(active); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, r.deps.Supervisor.GetStatus())
}

func (r *Router) handleTunnelDisconnect(w http.ResponseWriter, req *http.Request) {
	if err := r.deps.Supervisor.StopTunnel(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, r.deps.Supervisor.GetStatus())
}

func (r *Router) handleSafeModeConfirm(w http.ResponseWriter, req *http.Request) {
	ok := r.deps.Supervisor.ConfirmSafeMode()
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "safe mode is not active"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "safe mode confirmed, changes persistent"})
}

func (r *Router) handleSafeModeRollback(w http.ResponseWriter, req *http.Request) {
	ok := r.deps.Supervisor.RollbackSafeMode()
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "safe mode is not active"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "rolled back to safe un-tunneled state"})
}

func (r *Router) handleGetRoutingRules(w http.ResponseWriter, req *http.Request) {
	rules, err := r.deps.Store.GetRoutingRules()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (r *Router) handleCreateRoutingRule(w http.ResponseWriter, req *http.Request) {
	var rule store.RoutingRule
	if err := decodeJSON(w, req, defaultMaxBodyBytes, &rule, "invalid rule format"); err != nil {
		return
	}

	rule.Target = strings.TrimSpace(rule.Target)
	if rule.Target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target cannot be empty"})
		return
	}

	rule.Action = strings.ToLower(strings.TrimSpace(rule.Action))
	if rule.Action != "direct" && rule.Action != "proxy" && rule.Action != "block" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "action must be direct, proxy, or block"})
		return
	}

	rule.TargetType = strings.ToLower(strings.TrimSpace(rule.TargetType))
	if rule.TargetType != "domain" && rule.TargetType != "ip" && rule.TargetType != "preset" {
		rule.TargetType = "ip"
	}

	if rule.ID == "" {
		rule.ID = "rule-" + time.Now().Format("20060102150405")
	}
	rule.IsEnabled = true

	if err := r.deps.Store.SaveRoutingRule(&rule); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, rule)
}

func (r *Router) handleDeleteRoutingRule(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	if err := r.deps.Store.DeleteRoutingRule(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "rule deleted"})
}

func (r *Router) handleGetLogs(w http.ResponseWriter, req *http.Request) {
	logs := r.deps.Supervisor.GetLogs(100)
	writeJSON(w, http.StatusOK, logs)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func decodeJSON(w http.ResponseWriter, req *http.Request, maxBytes int64, dst interface{}, fallbackErrMsg string) error {
	req.Body = http.MaxBytesReader(w, req.Body, maxBytes)
	if err := json.NewDecoder(req.Body).Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})
			return err
		}
		if fallbackErrMsg == "" {
			fallbackErrMsg = "invalid request body"
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fallbackErrMsg})
		return err
	}
	return nil
}

func getClientIP(req *http.Request) string {
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := req.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		return host
	}
	return req.RemoteAddr
}

type loginRateLimiter struct {
	mu          sync.Mutex
	attempts    map[string][]time.Time
	maxAttempts int
	window      time.Duration
}

func newLoginRateLimiter(maxAttempts int, window time.Duration) *loginRateLimiter {
	return &loginRateLimiter{
		attempts:    make(map[string][]time.Time),
		maxAttempts: maxAttempts,
		window:      window,
	}
}

func (rl *loginRateLimiter) isBlocked(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	timestamps := rl.attempts[ip]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) == 0 {
		delete(rl.attempts, ip)
		return false
	}
	rl.attempts[ip] = valid

	return len(valid) >= rl.maxAttempts
}

func (rl *loginRateLimiter) recordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	timestamps := rl.attempts[ip]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	valid = append(valid, now)
	rl.attempts[ip] = valid
}

func (rl *loginRateLimiter) reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

func (r *Router) handleGetUpdates(w http.ResponseWriter, req *http.Request) {
	status := r.deps.Updater.GetStatus()
	if len(status.Cores) == 0 {
		status, _ = r.deps.Updater.CheckUpdates(false)
	}
	writeJSON(w, http.StatusOK, status)
}

func (r *Router) handleCheckUpdates(w http.ResponseWriter, req *http.Request) {
	status, err := r.deps.Updater.CheckUpdates(true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (r *Router) handleUpdateCore(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Core string `json:"core"`
	}
	if err := decodeJSON(w, req, defaultMaxBodyBytes, &body, "invalid request body"); err != nil {
		return
	}

	body.Core = strings.ToLower(strings.TrimSpace(body.Core))
	if body.Core != "xray" && body.Core != "tun2socks" && body.Core != "all" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid core: must be 'xray', 'tun2socks', or 'all'"})
		return
	}

	err := r.deps.Updater.UpdateCore(body.Core)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("%s core updated successfully", body.Core),
	})
}

