package core

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultWatchdogProbeURL         = "http://cp.cloudflare.com/generate_204"
	defaultWatchdogFallbackProbeURL = "http://www.google.com/generate_204"
	defaultWatchdogProbeTimeout     = 10 * time.Second
	watchdogProbeAttempts           = 3
	watchdogMaxReconnectAttempts    = 5
)

// CheckHealth executes an HTTP probe to targetURL with a 10-second timeout
// (or the configured watchdogProbeTimeout), verifying that the returned status code is 204 or 200.
func (s *Supervisor) CheckHealth(ctx context.Context, targetURL string) (bool, time.Duration, error) {
	if targetURL == "" {
		if s.store != nil {
			if settings, err := s.store.GetSettings(); err == nil && settings != nil && settings.HealthCheckURL != "" {
				targetURL = settings.HealthCheckURL
			}
		}
		if targetURL == "" {
			targetURL = defaultWatchdogProbeURL
		}
	}

	probeTimeout := defaultWatchdogProbeTimeout
	s.mu.Lock()
	if s.watchdogProbeTimeout > 0 {
		probeTimeout = s.watchdogProbeTimeout
	}
	s.mu.Unlock()

	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return false, 0, fmt.Errorf("failed to create health check request: %w", err)
	}
	req.Close = true

	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			Proxy:             nil,
		},
		Timeout: probeTimeout,
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if latency <= 0 {
		latency = time.Millisecond
	}

	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return false, latency, fmt.Errorf("health probe returned unexpected status %d", resp.StatusCode)
	}

	return true, latency, nil
}

func (s *Supervisor) probeWithFallback(ctx context.Context, primaryURL string, fallbackURLs []string) (string, time.Duration, error) {
	healthy, latency, err := s.CheckHealth(ctx, primaryURL)
	if healthy {
		return primaryURL, latency, nil
	}

	for _, fbURL := range fallbackURLs {
		if fbURL == "" || fbURL == primaryURL {
			continue
		}
		fbHealthy, fbLatency, fbErr := s.CheckHealth(ctx, fbURL)
		if fbHealthy {
			return fbURL, fbLatency, nil
		}
		_ = fbErr
	}

	return primaryURL, 0, err
}

func (s *Supervisor) resolveWatchdogFallbackURLs(primaryURL string) []string {
	s.mu.Lock()
	configured := s.watchdogFallbackURLs
	s.mu.Unlock()

	if configured != nil {
		out := make([]string, len(configured))
		copy(out, configured)
		return out
	}

	switch primaryURL {
	case defaultWatchdogProbeURL:
		return []string{defaultWatchdogFallbackProbeURL}
	case defaultWatchdogFallbackProbeURL, "https://www.google.com/generate_204":
		return []string{defaultWatchdogProbeURL}
	default:
		return nil
	}
}

func (s *Supervisor) getWatchdogRetryBackoffs() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.watchdogRetryBackoffs) > 0 {
		out := make([]time.Duration, len(s.watchdogRetryBackoffs))
		copy(out, s.watchdogRetryBackoffs)
		return out
	}
	if s.watchdogRetryDelay > 0 {
		return []time.Duration{s.watchdogRetryDelay, 2 * s.watchdogRetryDelay}
	}
	return []time.Duration{5 * time.Second, 10 * time.Second}
}

func (s *Supervisor) getWatchdogReconnectDelays() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.watchdogReconnectDelays) > 0 {
		out := make([]time.Duration, len(s.watchdogReconnectDelays))
		copy(out, s.watchdogReconnectDelays)
		return out
	}
	return []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second}
}

// StartHealthWatchdog runs the periodic health verification daemon loop.
// If 3 consecutive probes fail (with exponential backoff), it triggers automated tunnel restart
// with up to 5 reconnection attempts.
func (s *Supervisor) StartHealthWatchdog(ctx context.Context) {
	interval := 60 * time.Minute
	targetURL := defaultWatchdogProbeURL

	if s.store != nil {
		if settings, err := s.store.GetSettings(); err == nil && settings != nil {
			if settings.HealthCheckIntervalMinutes > 0 {
				interval = time.Duration(settings.HealthCheckIntervalMinutes) * time.Minute
			}
			if settings.HealthCheckURL != "" {
				targetURL = settings.HealthCheckURL
			}
		}
	}

	s.mu.Lock()
	if s.watchdogInterval > 0 {
		interval = s.watchdogInterval
	}
	s.mu.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			state := s.state
			activeTarget := targetURL
			if s.store != nil {
				if settings, err := s.store.GetSettings(); err == nil && settings != nil && settings.HealthCheckURL != "" {
					activeTarget = settings.HealthCheckURL
				}
			}
			s.mu.Unlock()

			if state != "connected" {
				continue
			}

			fallbacks := s.resolveWatchdogFallbackURLs(activeTarget)
			retryBackoffs := s.getWatchdogRetryBackoffs()

			var lastErr error
			probeSucceeded := false

			for attempt := 0; attempt < watchdogProbeAttempts; attempt++ {
				if attempt > 0 {
					delayIdx := attempt - 1
					if delayIdx >= len(retryBackoffs) {
						delayIdx = len(retryBackoffs) - 1
					}
					delay := retryBackoffs[delayIdx]
					s.addLogComponent("warn", "WATCHDOG", fmt.Sprintf("Connection health probe attempt %d/%d failed (%v). Retrying in %v...", attempt, watchdogProbeAttempts, lastErr, delay))

					select {
					case <-ctx.Done():
						return
					case <-time.After(delay):
					}
				}

				usedURL, latency, err := s.probeWithFallback(ctx, activeTarget, fallbacks)
				if err == nil {
					if attempt == 0 {
						s.addLogComponent("info", "WATCHDOG", fmt.Sprintf("Connection health probe succeeded (target: %s, latency: %v)", usedURL, latency))
					} else {
						s.addLogComponent("info", "WATCHDOG", fmt.Sprintf("Connection health probe succeeded on attempt %d/%d (target: %s, latency: %v)", attempt+1, watchdogProbeAttempts, usedURL, latency))
					}
					probeSucceeded = true
					break
				}
				lastErr = err
			}

			if probeSucceeded {
				continue
			}

			// All probe attempts failed -> trigger automated recovery with exponential backoff
			s.addLogComponent("error", "WATCHDOG", fmt.Sprintf("Connection health verification failed after %d attempts (last err: %v). Triggering auto-recovery restart...", watchdogProbeAttempts, lastErr))

			reconnectDelays := s.getWatchdogReconnectDelays()
			reconnected := false
			for attempt := 1; attempt <= watchdogMaxReconnectAttempts; attempt++ {
				if err := s.RestartTunnel(); err == nil {
					reconnected = true
					break
				} else if attempt < watchdogMaxReconnectAttempts {
					delayIdx := attempt - 1
					if delayIdx >= len(reconnectDelays) {
						delayIdx = len(reconnectDelays) - 1
					}
					delay := reconnectDelays[delayIdx]
					s.addLogComponent("warn", "WATCHDOG", fmt.Sprintf("Auto-recovery attempt %d/%d failed (%v). Retrying in %v...", attempt, watchdogMaxReconnectAttempts, err, delay))

					select {
					case <-ctx.Done():
						return
					case <-time.After(delay):
					}
				} else {
					s.addLogComponent("error", "WATCHDOG", fmt.Sprintf("Auto-recovery failed after %d attempts (%v). Transitioning to disconnected.", watchdogMaxReconnectAttempts, err))
				}
			}

			if !reconnected {
				s.mu.Lock()
				s.state = "disconnected"
				if s.watchdogCancel != nil {
					s.watchdogCancel()
					s.watchdogCancel = nil
				}
				s.mu.Unlock()
			}
			return
		}
	}
}

func (s *Supervisor) startHealthWatchdogLocked() {
	intervalMinutes := 60
	if s.store != nil {
		if settings, err := s.store.GetSettings(); err == nil && settings != nil {
			intervalMinutes = settings.HealthCheckIntervalMinutes
		}
	}
	if intervalMinutes > 0 || s.watchdogInterval > 0 {
		if s.watchdogCancel != nil {
			s.watchdogCancel()
			s.watchdogCancel = nil
		}
		wCtx, cancel := context.WithCancel(context.Background())
		s.watchdogCancel = cancel
		go s.StartHealthWatchdog(wCtx)
	}
}

// RestartTunnel cleanly stops and re-orchestrates the tunnel using active configuration.
func (s *Supervisor) RestartTunnel() error {
	s.mu.Lock()
	cfg := s.activeConfig
	if cfg == nil && s.store != nil {
		if active, err := s.store.GetActiveConfig(); err == nil {
			cfg = active
		}
	}
	s.mu.Unlock()

	if cfg == nil {
		return fmt.Errorf("no active config to restart")
	}

	s.addLogComponent("warn", "WATCHDOG", fmt.Sprintf("Restarting tunnel with config: %s (%s)", cfg.Name, cfg.Protocol))
	s.mu.Lock()
	stopErr := s.stopTunnelLocked(false)
	if stopErr == nil {
		s.restartCount++
	}
	s.mu.Unlock()
	if stopErr != nil {
		return fmt.Errorf("failed to stop tunnel during restart: %w", stopErr)
	}

	if err := s.startTunnelInternal(cfg, false); err != nil {
		s.addLogComponent("error", "WATCHDOG", fmt.Sprintf("Auto-recovery restart failed: %v", err))
		return err
	}

	s.addLogComponent("info", "WATCHDOG", "Auto-recovery restart completed successfully")
	return nil
}

// GetRestartCount returns how many times RestartTunnel has completed.
func (s *Supervisor) GetRestartCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restartCount
}

// SetWatchdogInterval sets a custom check interval for watchdog (used in testing or fine tuning).
func (s *Supervisor) SetWatchdogInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchdogInterval = d
}

// SetWatchdogRetryDelay sets a custom retry duration between failed probes (used in testing).
func (s *Supervisor) SetWatchdogRetryDelay(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchdogRetryDelay = d
}

// SetWatchdogProbeTimeout sets a custom HTTP probe timeout (used in testing).
func (s *Supervisor) SetWatchdogProbeTimeout(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchdogProbeTimeout = d
}

// SetWatchdogRetryBackoffs sets custom retry backoff delays between failed probes.
func (s *Supervisor) SetWatchdogRetryBackoffs(delays []time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchdogRetryBackoffs = delays
}

// SetWatchdogFallbackURLs sets custom fallback probe URLs.
func (s *Supervisor) SetWatchdogFallbackURLs(urls []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchdogFallbackURLs = urls
}

// SetWatchdogReconnectDelays sets custom exponential backoff delays between failed RestartTunnel attempts.
func (s *Supervisor) SetWatchdogReconnectDelays(delays []time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchdogReconnectDelays = delays
}
