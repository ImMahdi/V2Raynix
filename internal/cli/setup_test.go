package cli

import (
	"bufio"
	"path/filepath"
	"strings"
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

	// Verify ResolveWebPort respects store settings when port is not explicitly passed
	resolved := ResolveWebPort(2080, false, settings)
	if resolved != 3050 {
		t.Errorf("expected resolved port 3050, got %d", resolved)
	}

	// Verify ResolveWebPort respects CLI flag when explicitly passed
	resolvedExplicit := ResolveWebPort(9090, true, settings)
	if resolvedExplicit != 9090 {
		t.Errorf("expected resolved port 9090, got %d", resolvedExplicit)
	}

	// Invalid ports
	if err := ApplyWebPort(st, 0); err == nil {
		t.Errorf("expected error for port 0, got nil")
	}
	if err := ApplyWebPort(st, 70000); err == nil {
		t.Errorf("expected error for port 70000, got nil")
	}
}

func TestReadPasswordSilent(t *testing.T) {
	origIsTerminal := termIsTerminal
	termIsTerminal = func(fd int) bool { return false }
	defer func() { termIsTerminal = origIsTerminal }()

	input := "supersecret123\n"
	reader := bufio.NewReader(strings.NewReader(input))
	pass, err := ReadPasswordSilent("Enter password: ", reader)
	if err != nil {
		t.Fatalf("ReadPasswordSilent failed: %v", err)
	}
	if pass != "supersecret123" {
		t.Errorf("expected 'supersecret123', got '%s'", pass)
	}
}

func TestReadPasswordSilent_Terminal(t *testing.T) {
	origIsTerminal := termIsTerminal
	origReadPassword := termReadPassword
	termIsTerminal = func(fd int) bool { return true }
	termReadPassword = func(fd int) ([]byte, error) {
		return []byte("terminalpass123"), nil
	}
	defer func() {
		termIsTerminal = origIsTerminal
		termReadPassword = origReadPassword
	}()

	pass, err := ReadPasswordSilent("Enter password: ", nil)
	if err != nil {
		t.Fatalf("ReadPasswordSilent terminal failed: %v", err)
	}
	if pass != "terminalpass123" {
		t.Errorf("expected 'terminalpass123', got '%s'", pass)
	}
}
