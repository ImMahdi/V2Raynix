package pinger_test

import (
	"net"
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
