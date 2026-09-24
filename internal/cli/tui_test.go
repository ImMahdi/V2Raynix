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

func TestVisualWidth(t *testing.T) {
	// Plain text
	if w := VisualWidth("hello"); w != 5 {
		t.Errorf("expected 5, got %d", w)
	}

	// ANSI colored text
	colored := "\033[1;32m● active\033[0m"
	if w := VisualWidth(colored); w != 8 { // "● active" is 1 + 1 + 6 = 8
		t.Errorf("expected 8 for %q, got %d", colored, w)
	}

	// Text with emoji
	emojiText := "🔑 Key"
	if w := VisualWidth(emojiText); w != 6 { // emoji width 2 + space 1 + 'Key' 3 = 6
		t.Errorf("expected 6 for %q, got %d", emojiText, w)
	}
}

func TestBoxAlignment(t *testing.T) {
	menus := []struct {
		name string
		text string
	}{
		{"MainMenu_Active", RenderMainMenu("active", 2080)},
		{"MainMenu_Inactive", RenderMainMenu("inactive", 3000)},
		{"ServiceMenu_Active", RenderServiceMenu("active")},
		{"ServiceMenu_Inactive", RenderServiceMenu("inactive")},
		{"StatusCard_Active", RenderStatusCard("active", 2080)},
	}

	for _, tc := range menus {
		lines := strings.Split(strings.Trim(tc.text, "\r\n"), "\n")
		var expectedWidth int
		for i, line := range lines {
			w := VisualWidth(line)
			if i == 0 {
				expectedWidth = w
			} else if w != expectedWidth {
				t.Errorf("[%s] line %d has visual width %d, expected %d: %s", tc.name, i, w, expectedWidth, line)
			}
		}
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
