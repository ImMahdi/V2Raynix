package cli

import (
	"bufio"
	"fmt"
	"strings"
)

// RunMainMenu launches the primary interactive Full Terminal User Interface (TUI) loop.
func RunMainMenu(bridge TUIBridge, reader *bufio.Reader) error {
	for {
		ClearScreen()

		status, err := bridge.GetStatus()
		if err != nil {
			status = LiveStatusInfo{
				ServiceState: "inactive",
				TunnelState:  "disconnected",
			}
		}

		fmt.Print(RenderStatusHeader(status))

		fmt.Println("  " + AnsiBold + AnsiCyan + "╔══════════════════════════════════════════════════════════════════════════════╗" + AnsiReset)
		fmt.Println("  " + AnsiBold + AnsiCyan + "║                          V2RAYNIX MAIN CONTROL PANEL                         ║" + AnsiReset)
		fmt.Println("  " + AnsiBold + AnsiCyan + "╚══════════════════════════════════════════════════════════════════════════════╝" + AnsiReset)
		fmt.Println()
		fmt.Println("  " + AnsiBold + "[1]" + AnsiReset + " 🚀  Tunnel & Watchdog Controls       " + AnsiGray + "(Connect, Disconnect, Health & Latency Probe)" + AnsiReset)
		fmt.Println("  " + AnsiBold + "[2]" + AnsiReset + " 📡  Manage Configurations & Nodes      " + AnsiGray + "(Add Links/JSON, Delete, Set Active Node)" + AnsiReset)
		fmt.Println("  " + AnsiBold + "[3]" + AnsiReset + " 🛡️   Smart Routing Rules & Presets     " + AnsiGray + "(Iran Direct Bypass, GeoIP, Domain Rules)" + AnsiReset)
		fmt.Println("  " + AnsiBold + "[4]" + AnsiReset + " 📜  Real-time Service & Core Logs     " + AnsiGray + "(Inspect Last 50 Logs, Live Log Streaming)" + AnsiReset)
		fmt.Println("  " + AnsiBold + "[5]" + AnsiReset + " ⚙️   System Settings & Watchdog Config " + AnsiGray + "(Web Port, Admin Credentials, Probe URL)" + AnsiReset)
		fmt.Println("  " + AnsiBold + "[6]" + AnsiReset + " 🧰  Core Engines & Service Management " + AnsiGray + "(Check/Upgrade Xray/Sing-box, Restart)" + AnsiReset)
		fmt.Println()
		fmt.Println("  " + AnsiBold + "[0]" + AnsiReset + " 🚪  Exit TUI " + AnsiGray + "(or enter 'q')" + AnsiReset)
		fmt.Println()

		fmt.Printf("  %sSelect an option [0-6]:%s ", AnsiBold, AnsiReset)
		choiceStr, err := reader.ReadString('\n')
		if err != nil && len(choiceStr) == 0 {
			return nil
		}

		choice := strings.TrimSpace(strings.ToLower(choiceStr))

		switch choice {
		case "1":
			_ = HandleTunnelMenu(bridge, reader)
		case "2":
			_ = HandleConfigsMenu(bridge, reader)
		case "3":
			_ = HandleRoutingMenu(bridge, reader)
		case "4":
			_ = HandleLogsMenu(bridge, reader)
		case "5":
			_ = HandleSettingsMenu(bridge, reader)
		case "6":
			_ = HandleServiceMenu(bridge, reader)
		case "0", "q", "exit", "quit":
			ClearScreen()
			fmt.Printf("\n  %s✓ V2Raynix TUI closed. Have a great day!%s\n\n", AnsiGreen, AnsiReset)
			return nil
		default:
			fmt.Printf("\n  %s[ERROR]%s Invalid selection '%s'. Press Enter to retry...", AnsiRed, AnsiReset, choice)
			_, _ = reader.ReadString('\n')
		}
	}
}
