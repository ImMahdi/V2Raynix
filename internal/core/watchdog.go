package core

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// CheckHealth executes an HTTP probe to targetURL with a 5-second timeout,
// verifying that the returned status code is 204 or 200.
func (s *Supervisor) CheckHealth(ctx context.Context, targetURL string) (bool, time.Duration, error) {
	if targetURL == "" {
		if s.store != nil {
			if settings, err := s.store.GetSettings(); err == nil && settings != nil && settings.HealthCheckURL != "" {
				targetURL = settings.HealthCheckURL
			}
		}
		if targetURL == "" {
			targetURL = "http://cp.cloudflare.com/generate_204"
		}
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
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
		Timeout: 5 * time.Second,
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

// StartHealthWatchdog runs the periodic health verification daemon loop.
// If two consecutive probes fail, it triggers automated tunnel restart.
func (s *Supervisor) StartHealthWatchdog(ctx context.Context) {
	interval := 60 * time.Minute
	targetURL := "http://cp.cloudflare.com/generate_204"

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
	retryDelay := s.watchdogRetryDelay
	if retryDelay <= 0 {
		retryDelay = 3 * time.Second
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

			// Probe 1
			healthy, latency, err := s.CheckHealth(ctx, activeTarget)
			if healthy {
				s.addLogComponent("info", "WATCHDOG", fmt.Sprintf("Connection health probe succeeded (target: %s, latency: %v)", activeTarget, latency))
				continue
			}

			// Probe 1 failed
			s.addLogComponent("warn", "WATCHDOG", fmt.Sprintf("Connection health probe failed (%v). Retrying in %v...", err, retryDelay))

			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
			}

			// Probe 2 (Retry)
			healthy2, latency2, err2 := s.CheckHealth(ctx, activeTarget)
			if healthy2 {
				s.addLogComponent("info", "WATCHDOG", fmt.Sprintf("Connection health probe succeeded on retry (target: %s, latency: %v)", activeTarget, latency2))
				continue
			}

			// Both probes failed -> trigger automated recovery
			s.addLogComponent("error", "WATCHDOG", fmt.Sprintf("Connection health verification failed twice (retry err: %v). Triggering auto-recovery restart...", err2))
			_ = s.RestartTunnel()
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
	if err := s.StopTunnel(); err != nil {
		return fmt.Errorf("failed to stop tunnel during restart: %w", err)
	}

	s.mu.Lock()
	s.restartCount++
	s.mu.Unlock()

	if err := s.StartTunnel(cfg); err != nil {
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
