package pinger_test

import (
	"context"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/v2raynix/v2raynix/internal/pinger"
	"github.com/v2raynix/v2raynix/internal/store"
)

func TestPinger_TCPPing(t *testing.T) {
	// Start local mock listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)

	// 1. Success ping to active listener
	duration, err := pinger.TCPPing("127.0.0.1", addr.Port, 1*time.Second)
	if err != nil {
		t.Fatalf("expected ping to succeed: %v", err)
	}
	if duration <= 0 {
		t.Errorf("expected positive duration, got %v", duration)
	}

	// 2. Failure ping to closed port
	_, err = pinger.TCPPing("127.0.0.1", 59999, 100*time.Millisecond)
	if err == nil {
		t.Errorf("expected error pinging non-existent port")
	}
}

func TestPinger_BatchPing(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)

	configs := []*store.ConfigItem{
		{
			ID:     "c1",
			Server: "127.0.0.1",
			Port:   addr.Port,
		},
		{
			ID:     "c2",
			Server: "127.0.0.1",
			Port:   59998,
		},
	}

	results := pinger.BatchPing(configs, 2, 200*time.Millisecond)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results["c1"] < 0 {
		t.Errorf("expected valid latency for c1, got %d", results["c1"])
	}
	if results["c2"] != -1 {
		t.Errorf("expected -1 for unreachable c2, got %d", results["c2"])
	}
}

func TestPinger_IPv6Support(t *testing.T) {
	// Pinging an IPv6 literal (::1) on an unused port should NOT fail with "too many colons in address"
	_, err := pinger.TCPPing("::1", 59997, 50*time.Millisecond)
	if err != nil && strings.Contains(err.Error(), "too many colons in address") {
		t.Fatalf("IPv6 dialing failed with malformed address syntax: %v", err)
	}

	// If IPv6 listener works on host, test successful ping
	ln, err := net.Listen("tcp", "[::1]:0")
	if err == nil {
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		dur, err := pinger.TCPPing("::1", port, 500*time.Millisecond)
		if err != nil {
			t.Fatalf("expected IPv6 ping to succeed: %v", err)
		}
		if dur <= 0 {
			t.Errorf("expected positive duration, got %v", dur)
		}
	}
}

func TestPinger_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	start := time.Now()
	_, err := pinger.TCPPingContext(ctx, "127.0.0.1", 59996, 2*time.Second)
	if err == nil {
		t.Fatalf("expected error on cancelled context, got nil")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Errorf("expected instant abort on cancelled context, took %v", time.Since(start))
	}

	configs := []*store.ConfigItem{
		{ID: "c1", Server: "127.0.0.1", Port: 59996},
		{ID: "c2", Server: "127.0.0.1", Port: 59995},
	}
	results := pinger.BatchPingContext(ctx, configs, 2, 2*time.Second)
	if len(results) > 0 {
		t.Errorf("expected empty or aborted results on pre-cancelled context, got %v", results)
	}
}

func TestPinger_WorkerPoolBound(t *testing.T) {
	configs := make([]*store.ConfigItem, 100)
	for i := 0; i < 100; i++ {
		configs[i] = &store.ConfigItem{
			ID:     string(rune('a' + i)),
			Server: "127.0.0.1",
			Port:   59994,
		}
	}

	concurrency := 3
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	beforeGoroutines := runtime.NumGoroutine()

	done := make(chan struct{})
	go func() {
		pinger.BatchPingContext(ctx, configs, concurrency, 500*time.Millisecond)
		close(done)
	}()

	var maxGoroutines int
	for i := 0; i < 5; i++ {
		time.Sleep(20 * time.Millisecond)
		current := runtime.NumGoroutine()
		if current > maxGoroutines {
			maxGoroutines = current
		}
	}
	<-done

	spawned := maxGoroutines - beforeGoroutines
	if spawned > concurrency+5 {
		t.Fatalf("worker pool unbounded: spawned %d goroutines, expected at most %d", spawned, concurrency)
	}
}

func TestPinger_RealHTTPDelay(t *testing.T) {
	// 1. Failure when socks5 proxy is not running
	_, err := pinger.RealHTTPDelay("127.0.0.1:59993", "http://127.0.0.1:59992", 100*time.Millisecond)
	if err == nil {
		t.Errorf("expected error when socks proxy is down")
	}
}

func TestPinger_BatchRealTestContext(t *testing.T) {
	configs := []*store.ConfigItem{
		{
			ID:       "cfg-dead-1",
			Name:     "Dead Server",
			Protocol: "vless",
			Server:   "127.0.0.1",
			Port:     59991,
		},
		{
			ID:       "cfg-dead-2",
			Name:     "Fake CDN Server",
			Protocol: "vless",
			Server:   "127.0.0.1",
			Port:     59990,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	results := pinger.BatchRealTestContext(ctx, configs, 2, 200*time.Millisecond)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results["cfg-dead-1"] != -1 {
		t.Errorf("expected -1 for dead server, got %d", results["cfg-dead-1"])
	}
	if results["cfg-dead-2"] != -1 {
		t.Errorf("expected -1 for fake server, got %d", results["cfg-dead-2"])
	}
}

