package cli

import (
	"bufio"
	"fmt"
	"strings"
)

// HandleTunnelMenu manages the Tunnel & Connectivity Operations submenu (Menu 1).
func HandleTunnelMenu(bridge TUIBridge, reader *bufio.Reader) error {
	for {
		status, err := bridge.GetStatus()
		if err != nil {
			status = LiveStatusInfo{
				ServiceState: "error",
				TunnelState:  "unknown",
			}
		}

		fmt.Println(RenderStatusHeader(status))

		lines := []string{
			fmt.Sprintf(" %s[1]%s Connect / Switch Active Config", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[2]%s Disconnect Tunnel", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[3]%s Run Immediate Health & Latency Check", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[4]%s Confirm Safe Mode (make permanent)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[5]%s Rollback Safe Mode (emergency restore)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[0]%s Back to Main Menu", AnsiRed, AnsiReset),
		}
		fmt.Print(RenderBox("Tunnel & Connectivity Operations", lines, 74))

		choice, err := ReadMenuChoice("\nSelect an option [0-5]: ", []string{"0", "1", "2", "3", "4", "5"}, reader)
		if err != nil {
			return err
		}

		switch choice {
		case "0", "b":
			return nil
		case "1":
			handleConnectConfig(bridge, reader)
		case "2":
			handleDisconnect(bridge)
		case "3":
			handleHealthCheck(bridge, reader)
		case "4":
			handleConfirmSafeMode(bridge)
		case "5":
			handleRollbackSafeMode(bridge)
		}
	}
}

func handleConnectConfig(bridge TUIBridge, reader *bufio.Reader) {
	configs, activeID, err := bridge.ListConfigs()
	if err != nil {
		fmt.Printf("\n%sError retrieving configs: %v%s\n", AnsiRed, err, AnsiReset)
		return
	}
	if len(configs) == 0 {
		fmt.Printf("\n%sNo configurations found. Import configs in menu [2] first.%s\n", AnsiYellow, AnsiReset)
		fmt.Print("Press Enter to continue...")
		_, _ = reader.ReadString('\n')
		return
	}

	headers := []string{"#", "Name", "Protocol", "Server:Port", "Latency", "Status"}
	widths := []int{4, 18, 9, 20, 8, 8}
	var rows [][]string

	for i, c := range configs {
		statusText := ""
		if c.ID == activeID || c.IsActive {
			statusText = AnsiGreen + "ACTIVE" + AnsiReset
		}
		latText := "-"
		if c.LatencyMs > 0 {
			latText = fmt.Sprintf("%dms", c.LatencyMs)
		}
		addr := fmt.Sprintf("%s:%d", c.Server, c.Port)
		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1),
			c.Name,
			c.Protocol,
			addr,
			latText,
			statusText,
		})
	}

	fmt.Println("\n" + RenderTable(headers, rows, widths))
	fmt.Printf("Select configuration [1-%d] or 0 to cancel: ", len(configs))
	input, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	trimmed := strings.TrimSpace(input)
	if trimmed == "0" || strings.ToLower(trimmed) == "b" || trimmed == "" {
		return
	}

	var idx int
	if _, err := fmt.Sscanf(trimmed, "%d", &idx); err != nil || idx < 1 || idx > len(configs) {
		fmt.Printf("%sInvalid selection.%s\n", AnsiRed, AnsiReset)
		return
	}

	selected := configs[idx-1]
	fmt.Printf("Connecting to %s (%s)...\n", selected.Name, selected.Protocol)
	if err := bridge.ConnectTunnel(selected.ID); err != nil {
		fmt.Printf("%sFailed to connect: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		fmt.Printf("%sSuccessfully connected to %s!%s\n", AnsiGreen, selected.Name, AnsiReset)
	}
}

func handleDisconnect(bridge TUIBridge) {
	fmt.Println("Disconnecting tunnel...")
	if err := bridge.DisconnectTunnel(); err != nil {
		fmt.Printf("%sFailed to disconnect tunnel: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		fmt.Printf("%sTunnel disconnected successfully.%s\n", AnsiGreen, AnsiReset)
	}
}

func handleHealthCheck(bridge TUIBridge, reader *bufio.Reader) {
	fmt.Println("\nRunning immediate health check probe...")
	healthy, latency, err := bridge.CheckHealth()
	if err != nil {
		fmt.Printf("%sHealth check probe error: %v%s\n", AnsiRed, err, AnsiReset)
	} else if healthy {
		fmt.Printf("%s● Health Check: PASSED (%dms)%s\n", AnsiGreen, latency, AnsiReset)
	} else {
		fmt.Printf("%s● Health Check: FAILED%s\n", AnsiRed, AnsiReset)
	}
	fmt.Print("Press Enter to continue...")
	_, _ = reader.ReadString('\n')
}

func handleConfirmSafeMode(bridge TUIBridge) {
	fmt.Println("\nConfirming Safe Mode changes...")
	if err := bridge.ConfirmSafeMode(); err != nil {
		fmt.Printf("%sFailed to confirm Safe Mode: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		fmt.Printf("%sSafe Mode confirmed! Routing configuration is permanent.%s\n", AnsiGreen, AnsiReset)
	}
}

func handleRollbackSafeMode(bridge TUIBridge) {
	fmt.Println("\nRolling back Safe Mode...")
	if err := bridge.RollbackSafeMode(); err != nil {
		fmt.Printf("%sFailed to rollback Safe Mode: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		fmt.Printf("%sSafe Mode rolled back to previous configuration.%s\n", AnsiYellow, AnsiReset)
	}
}
