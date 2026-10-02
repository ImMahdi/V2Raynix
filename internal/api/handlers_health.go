package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

type HealthCheckResponse struct {
	Healthy   bool   `json:"healthy"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error"`
}

func (r *Router) handleHealthCheck(w http.ResponseWriter, req *http.Request) {
	if r.deps.Supervisor == nil {
		writeJSON(w, http.StatusServiceUnavailable, HealthCheckResponse{
			Healthy:   false,
			LatencyMs: 0,
			Error:     "supervisor not initialized",
		})
		return
	}

	var body struct {
		TargetURL string `json:"targetUrl"`
	}
	if req.Body != nil {
		_ = json.NewDecoder(req.Body).Decode(&body)
	}

	targetURL := strings.TrimSpace(body.TargetURL)
	if targetURL == "" && r.deps.Store != nil {
		if settings, err := r.deps.Store.GetSettings(); err == nil && settings != nil {
			targetURL = strings.TrimSpace(settings.HealthCheckURL)
		}
	}

	healthy, latency, err := r.deps.Supervisor.CheckHealth(req.Context(), targetURL)
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	writeJSON(w, http.StatusOK, HealthCheckResponse{
		Healthy:   healthy,
		LatencyMs: latency.Milliseconds(),
		Error:     errStr,
	})
}
