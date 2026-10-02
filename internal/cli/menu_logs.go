package cli

import (
	"bufio"
	"fmt"
	"strings"
	"time"
)

// HandleLogsMenu manages the Real-Time System & Engine Logs submenu (Menu 4).
func HandleLogsMenu(bridge TUIBridge, reader *bufio.Reader) error {
	for {
		status, err := bridge.GetStatus()
		if err != nil {
			status = LiveStatusInfo{
				ServiceState: "inactive",
				TunnelState:  "disconnected",
			}
		}

		fmt.Println(RenderStatusHeader(status))

		lines := []string{
			fmt.Sprintf(" %s[1]%s View Last 50 Logs", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[2]%s Follow Live Logs (Stream)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[0]%s Back to Main Menu", AnsiRed, AnsiReset),
		}
		fmt.Print(RenderBox("Real-Time Logs & Engine Events", lines, 74))

		choice, err := ReadMenuChoice("\nSelect an option [0-2]: ", []string{"0", "1", "2"}, reader)
		if err != nil {
			return err
		}

		switch choice {
		case "0", "b":
			return nil
		case "1":
			handleViewRecentLogs(bridge, reader)
		case "2":
			handleFollowLiveLogs(bridge, reader)
		}
	}
}

func formatLevelBadge(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "INFO":
		return AnsiGreen + "INFO" + AnsiReset
	case "WARN", "WARNING":
		return AnsiYellow + "WARN" + AnsiReset
	case "ERROR", "ERR", "FATAL":
		return AnsiRed + "ERROR" + AnsiReset
	default:
		return level
	}
}

func handleViewRecentLogs(bridge TUIBridge, reader *bufio.Reader) {
	logs, err := bridge.GetLogs(50)
	if err != nil {
		fmt.Printf("\n%sFailed to retrieve logs: %v%s\n", AnsiRed, err, AnsiReset)
		fmt.Print("Press Enter to continue...")
		_, _ = reader.ReadString('\n')
		return
	}

	if len(logs) == 0 {
		fmt.Printf("\n%sNo log entries available.%s\n", AnsiYellow, AnsiReset)
		fmt.Print("Press Enter to continue...")
		_, _ = reader.ReadString('\n')
		return
	}

	headers := []string{"Timestamp", "Level", "Message"}
	widths := []int{20, 8, 42}
	var rows [][]string

	for _, entry := range logs {
		ts := entry.Timestamp
		if ts == "" {
			ts = "-"
		}
		badge := formatLevelBadge(entry.Level)
		rows = append(rows, []string{
			ts,
			badge,
			entry.Message,
		})
	}

	fmt.Println("\n" + RenderTable(headers, rows, widths))
	fmt.Print("Press Enter to continue...")
	_, _ = reader.ReadString('\n')
}

func handleFollowLiveLogs(bridge TUIBridge, reader *bufio.Reader) {
	fmt.Printf("\n%sStreaming live logs... (Press Enter or type 'q' to stop)%s\n\n", AnsiCyan, AnsiReset)

	stopCh := make(chan struct{})
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(strings.ToLower(line)) == "q" || line == "\n" || line == "\r\n" || strings.TrimSpace(line) == "" {
				close(stopCh)
				return
			}
		}
	}()

	// Fetch and display initial tail
	initialLogs, _ := bridge.GetLogs(20)
	for _, entry := range initialLogs {
		fmt.Printf("[%s] [%s] %s\n", entry.Timestamp, formatLevelBadge(entry.Level), entry.Message)
	}
	seenCount := len(initialLogs)

	// Quick check if input was already supplied
	select {
	case <-stopCh:
		fmt.Printf("\n%sStopped live log stream.%s\n", AnsiYellow, AnsiReset)
		return
	default:
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			fmt.Printf("\n%sStopped live log stream.%s\n", AnsiYellow, AnsiReset)
			return
		case <-ticker.C:
			latest, err := bridge.GetLogs(100)
			if err == nil {
				if len(latest) < seenCount {
					seenCount = 0
				}
				if len(latest) > seenCount {
					for _, entry := range latest[seenCount:] {
						fmt.Printf("[%s] [%s] %s\n", entry.Timestamp, formatLevelBadge(entry.Level), entry.Message)
					}
					seenCount = len(latest)
				}
			}
		}
	}
}
