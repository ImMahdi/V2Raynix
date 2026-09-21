package core

import (
	"net"
	"testing"
	"time"
)

func TestWaitForPortReady(t *testing.T) {
	// Case 1: Port already open
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	if !waitForPortReady(addr, 500*time.Millisecond) {
		t.Errorf("expected waitForPortReady to return true for listening address %s", addr)
	}

	// Case 2: Port opens after a short delay (100ms)
	delayedLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	delayedAddr := delayedLn.Addr().String()
	_ = delayedLn.Close() // Close now, re-listen in goroutine

	go func() {
		time.Sleep(100 * time.Millisecond)
		reopened, err := net.Listen("tcp", delayedAddr)
		if err == nil {
			time.Sleep(200 * time.Millisecond)
			_ = reopened.Close()
		}
	}()

	if !waitForPortReady(delayedAddr, 500*time.Millisecond) {
		t.Errorf("expected waitForPortReady to succeed after delayed open on %s", delayedAddr)
	}

	// Case 3: Port unreachable / no listener
	unusedLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	unusedAddr := unusedLn.Addr().String()
	_ = unusedLn.Close()

	start := time.Now()
	if waitForPortReady(unusedAddr, 100*time.Millisecond) {
		t.Errorf("expected waitForPortReady to return false for unopened port")
	}
	elapsed := time.Since(start)
	if elapsed < 90*time.Millisecond {
		t.Errorf("expected waitForPortReady to respect timeout, elapsed: %v", elapsed)
	}
}
