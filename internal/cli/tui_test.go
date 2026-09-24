package cli

import (
	"strings"
	"testing"
)

func TestFormatStatus(t *testing.T) {
	active := FormatStatus("active")
	if !strings.Contains(active, "● active") {
		t.Errorf("expected '● active', got %q", active)
	}
	if !strings.Contains(active, "\033[1;32m") {
		t.Errorf("expected green ANSI code in active status, got %q", active)
	}

	inactive := FormatStatus("inactive")
	if !strings.Contains(inactive, "● inactive") {
		t.Errorf("expected '● inactive', got %q", inactive)
	}
	if !strings.Contains(inactive, "\033[1;31m") {
		t.Errorf("expected red ANSI code in inactive status, got %q", inactive)
	}
}

func TestRenderMainMenu(t *testing.T) {
	menu := RenderMainMenu("active", 2080)

	// Unicode box checks
	for _, char := range []string{"┌", "┐", "└", "┘", "│", "─"} {
		if !strings.Contains(menu, char) {
			t.Errorf("expected box character %q in main menu", char)
		}
	}

	// Port and status checks
	if !strings.Contains(menu, "2080") {
		t.Errorf("expected port '2080' in menu")
	}
	if !strings.Contains(menu, "● active") {
		t.Errorf("expected status '● active' in menu")
	}

	// Options checks
	if !strings.Contains(menu, "[1]") || !strings.Contains(menu, "[2]") ||
		!strings.Contains(menu, "[3]") || !strings.Contains(menu, "[4]") ||
		!strings.Contains(menu, "[0]") {
		t.Errorf("menu missing expected options [0-4]")
	}
}

func TestRenderServiceMenu(t *testing.T) {
	menu := RenderServiceMenu("active")

	if !strings.Contains(menu, "Service Management") {
		t.Errorf("expected 'Service Management' in title")
	}
	if !strings.Contains(menu, "[0]") || !strings.Contains(menu, "Back to Main Menu") {
		t.Errorf("expected back option [0] in service menu")
	}
}

func TestIsCancelInput(t *testing.T) {
	cancels := []string{"0", "b", "B", "back", "cancel", "q", "exit", ""}
	for _, c := range cancels {
		if !IsCancelInput(c) {
			t.Errorf("expected %q to be recognized as cancel input", c)
		}
	}

	nonCancels := []string{"admin", "3000", "1", "2", "mysecretpass"}
	for _, nc := range nonCancels {
		if IsCancelInput(nc) {
			t.Errorf("expected %q to NOT be cancel input", nc)
		}
	}
}
