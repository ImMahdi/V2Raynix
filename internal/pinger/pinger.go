package pinger

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/v2raynix/v2raynix/internal/configmgr"
	"github.com/v2raynix/v2raynix/internal/store"
	"golang.org/x/net/proxy"
)

// TCPPingContext measures the TCP handshake time to a target server and port with context support
func TCPPingContext(ctx context.Context, host string, port int, timeout time.Duration) (time.Duration, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{
		Timeout: timeout,
	}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	dur := time.Since(start)
	if dur <= 0 {
		dur = time.Microsecond
	}
	return dur, nil
}

// TCPPing measures the TCP handshake time to a target server and port (backward compatible)
func TCPPing(host string, port int, timeout time.Duration) (time.Duration, error) {
	return TCPPingContext(context.Background(), host, port, timeout)
}

// RealHTTPDelay tests round-trip HTTP response through a local SOCKS5 proxy
func RealHTTPDelay(socksProxyAddr, targetURL string, timeout time.Duration) (time.Duration, error) {
	if targetURL == "" {
		targetURL = "http://cp.cloudflare.com"
	}

	dialer, err := proxy.SOCKS5("tcp", socksProxyAddr, nil, proxy.Direct)
	if err != nil {
		return 0, fmt.Errorf("failed to create socks dialer: %w", err)
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	start := time.Now()
	resp, err := client.Get(targetURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return time.Since(start), nil
}

// BatchPingContext tests multiple configs using a bounded worker pool and context cancellation
func BatchPingContext(ctx context.Context, configs []*store.ConfigItem, concurrency int, timeout time.Duration) map[string]int {
	if concurrency <= 0 {
		concurrency = 5
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	results := make(map[string]int)
	if len(configs) == 0 {
		return results
	}

	// Bound worker goroutines to min(concurrency, len(configs))
	numWorkers := concurrency
	if len(configs) < numWorkers {
		numWorkers = len(configs)
	}

	jobs := make(chan *store.ConfigItem, len(configs))
	for _, cfg := range configs {
		jobs <- cfg
	}
	close(jobs)

	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cfg := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				latencyMs := -1
				dur, err := TCPPingContext(ctx, cfg.Server, cfg.Port, timeout)
				if err == nil {
					latencyMs = int(dur.Milliseconds())
					if latencyMs <= 0 {
						latencyMs = 1
					}
				}

				if ctx.Err() != nil {
					return
				}

				mu.Lock()
				results[cfg.ID] = latencyMs
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return results
}

// BatchPing tests multiple configs concurrently and returns latencies in milliseconds (backward compatible)
func BatchPing(configs []*store.ConfigItem, concurrency int, timeout time.Duration) map[string]int {
	return BatchPingContext(context.Background(), configs, concurrency, timeout)
}

func getFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// TestConfigRealDelay runs a real end-to-end HTTP delay test through Xray proxy
func TestConfigRealDelay(ctx context.Context, cfg *store.ConfigItem, timeout time.Duration) (int, error) {
	if cfg == nil {
		return -1, fmt.Errorf("config cannot be nil")
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	xrayBin, err := exec.LookPath("xray")
	if err != nil {
		// Fallback for direct socks proxy testing if protocol is socks
		if cfg.Protocol == "socks" {
			dur, err := RealHTTPDelay(net.JoinHostPort(cfg.Server, strconv.Itoa(cfg.Port)), "http://cp.cloudflare.com", timeout)
			if err != nil {
				return -1, err
			}
			lat := int(dur.Milliseconds())
			if lat <= 0 {
				lat = 1
			}
			return lat, nil
		}
		return -1, fmt.Errorf("xray core not found in system PATH")
	}

	testSocksPort, err := getFreePort()
	if err != nil {
		return -1, fmt.Errorf("failed to allocate test socks port: %w", err)
	}
	testHttpPort, err := getFreePort()
	if err != nil {
		return -1, fmt.Errorf("failed to allocate test http port: %w", err)
	}

	rawJSON, err := configmgr.GenerateXrayConfig(cfg, nil, testSocksPort, testHttpPort)
	if err != nil {
		return -1, fmt.Errorf("failed to generate test xray config: %w", err)
	}

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("v2raynix-test-%d.json", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, rawJSON, 0600); err != nil {
		return -1, fmt.Errorf("failed to write test config file: %w", err)
	}
	defer os.Remove(tmpFile)

	testCtx, cancel := context.WithTimeout(ctx, timeout+1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(testCtx, xrayBin, "run", "-c", tmpFile)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("failed to start test xray instance: %w", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	// Wait up to 400ms for test SOCKS inbound port to be open
	ready := false
	socksAddr := fmt.Sprintf("127.0.0.1:%d", testSocksPort)
	for i := 0; i < 8; i++ {
		c, err := net.DialTimeout("tcp", socksAddr, 50*time.Millisecond)
		if err == nil {
			c.Close()
			ready = true
			break
		}
		time.Sleep(30 * time.Millisecond)
	}

	if !ready {
		return -1, fmt.Errorf("test xray instance failed to bind on port %d", testSocksPort)
	}

	dur, err := RealHTTPDelay(socksAddr, "http://cp.cloudflare.com", timeout)
	if err != nil {
		return -1, err
	}

	lat := int(dur.Milliseconds())
	if lat <= 0 {
		lat = 1
	}
	return lat, nil
}

// BatchRealTestContext tests multiple configs concurrently using real Xray proxy delay
func BatchRealTestContext(ctx context.Context, configs []*store.ConfigItem, concurrency int, timeout time.Duration) map[string]int {
	if concurrency <= 0 {
		concurrency = 5
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	results := make(map[string]int)
	if len(configs) == 0 {
		return results
	}

	numWorkers := concurrency
	if len(configs) < numWorkers {
		numWorkers = len(configs)
	}

	jobs := make(chan *store.ConfigItem, len(configs))
	for _, cfg := range configs {
		jobs <- cfg
	}
	close(jobs)

	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cfg := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				latencyMs, _ := TestConfigRealDelay(ctx, cfg, timeout)

				if ctx.Err() != nil {
					return
				}

				mu.Lock()
				results[cfg.ID] = latencyMs
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return results
}

