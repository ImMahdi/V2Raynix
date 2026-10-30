package updater

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
	}{
		{"1.8.24", "25.1.30", true},
		{"v25.1.0", "v25.1.30", true},
		{"2.5.2", "2.5.2", false},
		{"2.6.0", "2.5.2", false},
		{"", "1.0.0", true},
		{"1.0.0", "", false},
		{"2.5", "2.5.2", true},
		{"2.5.2", "2.5", false},
	}

	for _, tt := range tests {
		got := isNewerVersion(tt.current, tt.latest)
		if got != tt.expected {
			t.Errorf("isNewerVersion(%q, %q) = %v; want %v", tt.current, tt.latest, got, tt.expected)
		}
	}
}

func TestCleanVersion(t *testing.T) {
	if v := cleanVersion("v25.1.30"); v != "25.1.30" {
		t.Errorf("expected 25.1.30, got %s", v)
	}
	if v := cleanVersion("Xray 1.8.24 (Xray, Penetrates Everything.)"); v != "1.8.24" {
		t.Errorf("expected 1.8.24, got %s", v)
	}
	if v := cleanVersion("tun2socks version 2.5.2 (linux/amd64)"); v != "2.5.2" {
		t.Errorf("expected 2.5.2, got %s", v)
	}
}

func TestAtomicMove_DirectRename(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src_file")
	dst := filepath.Join(tmpDir, "dst_file")

	content := []byte("binary data 12345")
	if err := os.WriteFile(src, content, 0755); err != nil {
		t.Fatalf("failed to write src file: %v", err)
	}

	if err := atomicMove(src, dst); err != nil {
		t.Fatalf("atomicMove failed: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expected src to be removed, stat err: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("dst content mismatch: got %q, want %q", got, content)
	}
}

func TestAtomicMove_CrossDeviceFallback(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src_cross")
	dst := filepath.Join(tmpDir, "dst_cross")

	content := []byte("cross-device executable payload")
	if err := os.WriteFile(src, content, 0755); err != nil {
		t.Fatalf("failed to write src file: %v", err)
	}

	// Mock renameFn to simulate EXDEV / cross-device link error on initial attempt
	origRename := renameFn
	defer func() { renameFn = origRename }()

	renameAttempts := 0
	renameFn = func(oldpath, newpath string) error {
		renameAttempts++
		return &os.LinkError{
			Op:  "rename",
			Old: oldpath,
			New: newpath,
			Err: syscall.EXDEV,
		}
	}

	if err := atomicMove(src, dst); err != nil {
		t.Fatalf("atomicMove cross-device fallback failed: %v", err)
	}

	if renameAttempts == 0 {
		t.Fatalf("expected renameFn to be called")
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expected src to be removed after fallback move, stat err: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("dst content mismatch: got %q, want %q", got, content)
	}
}

func TestAtomicMove_NonExistentSrc(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "non_existent_file")
	dst := filepath.Join(tmpDir, "dst_file")

	if err := atomicMove(src, dst); err == nil {
		t.Fatalf("expected error for nonexistent src, got nil")
	}
}

func TestUpdater_StateTracking(t *testing.T) {
	u := NewUpdater(t.TempDir())

	// 1. Initial status
	st := u.GetStatus()
	if st.IsUpdating {
		t.Errorf("expected IsUpdating=false initially")
	}
	if st.CurrentEngine != "" {
		t.Errorf("expected empty CurrentEngine, got %q", st.CurrentEngine)
	}
	if st.LastLog != "" {
		t.Errorf("expected empty LastLog, got %q", st.LastLog)
	}
	if st.LastError != "" {
		t.Errorf("expected empty LastError, got %q", st.LastError)
	}

	// 2. Synchronous update failure with invalid engine
	err := u.UpdateCore("invalid-core")
	if err == nil {
		t.Fatalf("expected error for invalid core, got nil")
	}
	st = u.GetStatus()
	if st.IsUpdating {
		t.Errorf("expected IsUpdating=false after update finished")
	}
	if st.CurrentEngine != "invalid-core" {
		t.Errorf("expected CurrentEngine='invalid-core', got %q", st.CurrentEngine)
	}
	if st.LastError != "unknown core: invalid-core" {
		t.Errorf("expected LastError='unknown core: invalid-core', got %q", st.LastError)
	}
	if !strings.Contains(st.LastLog, "Update failed: unknown core: invalid-core") {
		t.Errorf("expected LastLog to contain 'Update failed', got %q", st.LastLog)
	}

	// 3. StartAsyncUpdate failure tracking
	err = u.StartAsyncUpdate("another-invalid")
	if err != nil {
		t.Fatalf("expected StartAsyncUpdate to return nil on start, got %v", err)
	}
	var finalSt UpdateStatus
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		finalSt = u.GetStatus()
		if !finalSt.IsUpdating {
			break
		}
	}
	if finalSt.IsUpdating {
		t.Fatalf("async update timed out waiting to complete")
	}
	if finalSt.CurrentEngine != "another-invalid" {
		t.Errorf("expected CurrentEngine='another-invalid', got %q", finalSt.CurrentEngine)
	}
	if finalSt.LastError != "unknown core: another-invalid" {
		t.Errorf("expected LastError='unknown core: another-invalid', got %q", finalSt.LastError)
	}
}

func TestUpdater_ConcurrentUpdateRejection(t *testing.T) {
	u := NewUpdater(t.TempDir())
	u.mu.Lock()
	u.status.IsUpdating = true
	u.mu.Unlock()

	// Calling UpdateCore should be rejected
	err := u.UpdateCore("xray")
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("expected already in progress error from UpdateCore, got: %v", err)
	}

	// Calling StartAsyncUpdate should also be rejected
	err = u.StartAsyncUpdate("xray")
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("expected already in progress error from StartAsyncUpdate, got: %v", err)
	}
}
