package core

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatLogLine(t *testing.T) {
	fixedTime := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	line := FormatLogLine(fixedTime, "INFO", "TUNNEL", "Interface tun0 established")
	expected := "[2026-10-02 12:00:00] [INFO] [TUNNEL] Interface tun0 established"
	if line != expected {
		t.Fatalf("expected %q, got %q", expected, line)
	}
}

func TestWriteLogLine(t *testing.T) {
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	oldStdout := stdoutWriter
	oldStderr := stderrWriter
	stdoutWriter = &stdoutBuf
	stderrWriter = &stderrBuf
	defer func() {
		stdoutWriter = oldStdout
		stderrWriter = oldStderr
	}()

	fixedTime := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

	// Test INFO -> stdout
	WriteLogLine(fixedTime, "INFO", "TUNNEL", "Connected")
	if !strings.Contains(stdoutBuf.String(), "[2026-10-02 12:00:00] [INFO] [TUNNEL] Connected\n") {
		t.Errorf("expected stdout to contain info line, got %q", stdoutBuf.String())
	}
	if stderrBuf.Len() != 0 {
		t.Errorf("expected stderr to be empty, got %q", stderrBuf.String())
	}

	stdoutBuf.Reset()
	stderrBuf.Reset()

	// Test WARN -> stderr
	WriteLogLine(fixedTime, "WARN", "WATCHDOG", "Probe slow")
	if stdoutBuf.Len() != 0 {
		t.Errorf("expected stdout to be empty, got %q", stdoutBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "[2026-10-02 12:00:00] [WARN] [WATCHDOG] Probe slow\n") {
		t.Errorf("expected stderr to contain warn line, got %q", stderrBuf.String())
	}

	stdoutBuf.Reset()
	stderrBuf.Reset()

	// Test ERROR -> stderr
	WriteLogLine(fixedTime, "ERROR", "CORE", "Process crashed")
	if stdoutBuf.Len() != 0 {
		t.Errorf("expected stdout to be empty, got %q", stdoutBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "[2026-10-02 12:00:00] [ERROR] [CORE] Process crashed\n") {
		t.Errorf("expected stderr to contain error line, got %q", stderrBuf.String())
	}

	stdoutBuf.Reset()
	stderrBuf.Reset()

	// Test lowercase level "debug" -> stdout
	WriteLogLine(fixedTime, "debug", "ROUTING", "Rule added")
	if !strings.Contains(stdoutBuf.String(), "[2026-10-02 12:00:00] [DEBUG] [ROUTING] Rule added\n") {
		t.Errorf("expected stdout to contain debug line, got %q", stdoutBuf.String())
	}
	if stderrBuf.Len() != 0 {
		t.Errorf("expected stderr to be empty, got %q", stderrBuf.String())
	}
}

func TestLogEmitter(t *testing.T) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	emitter := NewLogEmitter(&outBuf, &errBuf)
	fixedTime := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

	emitter.Emit(fixedTime, "INFO", "API", "Route registered")
	if !strings.Contains(outBuf.String(), "[2026-10-02 12:00:00] [INFO] [API] Route registered\n") {
		t.Errorf("expected outBuf to contain info log, got %q", outBuf.String())
	}

	emitter.Emit(fixedTime, "ERROR", "UPDATER", "Checksum mismatch")
	if !strings.Contains(errBuf.String(), "[2026-10-02 12:00:00] [ERROR] [UPDATER] Checksum mismatch\n") {
		t.Errorf("expected errBuf to contain error log, got %q", errBuf.String())
	}
}

