package cli

import (
	"bufio"
	"strings"
	"testing"
)

func TestRunMainMenu_ExitOptions(t *testing.T) {
	bridge := NewTUIBridge(t.TempDir(), 29876)

	// Test exit on "0"
	in0 := bufio.NewReader(strings.NewReader("0\n"))
	if err := RunMainMenu(bridge, in0); err != nil {
		t.Fatalf("expected nil error on 0, got %v", err)
	}

	// Test exit on "q"
	inQ := bufio.NewReader(strings.NewReader("q\n"))
	if err := RunMainMenu(bridge, inQ); err != nil {
		t.Fatalf("expected nil error on q, got %v", err)
	}

	// Test exit on "exit"
	inExit := bufio.NewReader(strings.NewReader("exit\n"))
	if err := RunMainMenu(bridge, inExit); err != nil {
		t.Fatalf("expected nil error on exit, got %v", err)
	}
}

func TestRunMainMenu_SubmenuNavigation(t *testing.T) {
	bridge := NewTUIBridge(t.TempDir(), 29876)

	// Enter option 1 (Tunnel) -> 0 (Back) -> 0 (Exit)
	in1 := bufio.NewReader(strings.NewReader("1\n0\n0\n"))
	if err := RunMainMenu(bridge, in1); err != nil {
		t.Fatalf("expected nil error navigating tunnel menu, got %v", err)
	}

	// Enter option 2 (Configs) -> 0 (Back) -> 0 (Exit)
	in2 := bufio.NewReader(strings.NewReader("2\n0\n0\n"))
	if err := RunMainMenu(bridge, in2); err != nil {
		t.Fatalf("expected nil error navigating configs menu, got %v", err)
	}

	// Enter option 3 (Routing) -> 0 (Back) -> 0 (Exit)
	in3 := bufio.NewReader(strings.NewReader("3\n0\n0\n"))
	if err := RunMainMenu(bridge, in3); err != nil {
		t.Fatalf("expected nil error navigating routing menu, got %v", err)
	}

	// Enter option 4 (Logs) -> 0 (Back) -> 0 (Exit)
	in4 := bufio.NewReader(strings.NewReader("4\n0\n0\n"))
	if err := RunMainMenu(bridge, in4); err != nil {
		t.Fatalf("expected nil error navigating logs menu, got %v", err)
	}

	// Enter option 5 (Settings) -> 0 (Back) -> 0 (Exit)
	in5 := bufio.NewReader(strings.NewReader("5\n0\n0\n"))
	if err := RunMainMenu(bridge, in5); err != nil {
		t.Fatalf("expected nil error navigating settings menu, got %v", err)
	}

	// Enter option 6 (Service) -> 0 (Back) -> 0 (Exit)
	in6 := bufio.NewReader(strings.NewReader("6\n0\n0\n"))
	if err := RunMainMenu(bridge, in6); err != nil {
		t.Fatalf("expected nil error navigating service menu, got %v", err)
	}
}
