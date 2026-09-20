package pinger

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/v2raynix/v2raynix/internal/store"
	"golang.org/x/net/proxy"
)

// TCPPing measures the TCP handshake time to a target server and port
func TCPPing(host string, port int, timeout time.Duration) (time.Duration, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	return time.Since(start), nil
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

// BatchPing tests multiple configs concurrently and returns latencies in milliseconds
func BatchPing(configs []*store.ConfigItem, concurrency int, timeout time.Duration) map[string]int {
	if concurrency <= 0 {
		concurrency = 5
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	results := make(map[string]int)
	var mu sync.Mutex

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, cfg := range configs {
		wg.Add(1)
		go func(c *store.ConfigItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			latencyMs := -1
			dur, err := TCPPing(c.Server, c.Port, timeout)
			if err == nil {
				latencyMs = int(dur.Milliseconds())
				if latencyMs == 0 {
					latencyMs = 1
				}
			}

			mu.Lock()
			results[c.ID] = latencyMs
			mu.Unlock()
		}(cfg)
	}

	wg.Wait()
	return results
}
