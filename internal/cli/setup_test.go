package cli

import (
	"path/filepath"
	"testing"

	"github.com/v2raynix/v2raynix/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func TestApplyCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.New(filepath.Join(tmpDir, "v2raynix.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	err = ApplyCredentials(st, "newadmin", "securepass123")
	if err != nil {
		t.Fatalf("ApplyCredentials failed: %v", err)
	}

	user, err := st.GetAdminUser()
	if err != nil || user == nil {
		t.Fatalf("failed to get admin user: %v", err)
	}
	if user.Username != "newadmin" {
		t.Errorf("expected username 'newadmin', got '%s'", user.Username)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("securepass123")); err != nil {
		t.Errorf("password hash does not match plaintext: %v", err)
	}
}

func TestApplyWebPort(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.New(filepath.Join(tmpDir, "v2raynix.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Valid port
	if err := ApplyWebPort(st, 3050); err != nil {
		t.Fatalf("ApplyWebPort valid port failed: %v", err)
	}
	settings, err := st.GetSettings()
	if err != nil || settings.WebPort != 3050 {
		t.Errorf("expected web port 3050, got %d", settings.WebPort)
	}

	// Invalid ports
	if err := ApplyWebPort(st, 0); err == nil {
		t.Errorf("expected error for port 0, got nil")
	}
	if err := ApplyWebPort(st, 70000); err == nil {
		t.Errorf("expected error for port 70000, got nil")
	}
}
