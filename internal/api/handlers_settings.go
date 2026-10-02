package api

import (
	"net/http"
	"strings"
)

func (r *Router) handleGetSettings(w http.ResponseWriter, req *http.Request) {
	settings, err := r.deps.Store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (r *Router) handleSaveSettings(w http.ResponseWriter, req *http.Request) {
	current, err := r.deps.Store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := decodeJSON(w, req, defaultMaxBodyBytes, current, "invalid settings payload"); err != nil {
		return
	}

	current.HealthCheckURL = strings.TrimSpace(current.HealthCheckURL)

	if err := r.deps.Store.SaveSettings(current); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, current)
}
