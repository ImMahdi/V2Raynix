package updater

import (
	"testing"
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
