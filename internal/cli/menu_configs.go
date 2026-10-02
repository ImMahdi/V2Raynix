package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/v2raynix/v2raynix/internal/store"
)

// HandleConfigsMenu manages the Configurations Management submenu (Menu 2).
func HandleConfigsMenu(bridge TUIBridge, reader *bufio.Reader) error {
	for {
		status, err := bridge.GetStatus()
		if err != nil {
			status = LiveStatusInfo{
				ServiceState: "error",
				TunnelState:  "unknown",
			}
		}

		fmt.Println(RenderStatusHeader(status))

		configs, activeID, err := bridge.ListConfigs()
		if err != nil {
			fmt.Printf("\n%sError retrieving configurations: %v%s\n", AnsiRed, err, AnsiReset)
		} else if len(configs) == 0 {
			fmt.Println(RenderBox("Configurations", []string{
				fmt.Sprintf(" %sNo configurations saved. Use option [2] to add a config.%s", AnsiYellow, AnsiReset),
			}, 74))
		} else {
			headers := []string{"#", "ID", "Name", "Protocol", "Target", "Active"}
			widths := []int{4, 10, 18, 9, 20, 8}
			var rows [][]string

			for i, c := range configs {
				shortID := c.ID
				if len(shortID) > 8 {
					shortID = shortID[:8]
				}

				activeStr := "-"
				if c.ID == activeID || c.IsActive {
					activeStr = AnsiGreen + "ACTIVE" + AnsiReset
				}

				target := "-"
				if c.Server != "" {
					target = fmt.Sprintf("%s:%d", c.Server, c.Port)
				}

				rows = append(rows, []string{
					fmt.Sprintf("%d", i+1),
					shortID,
					c.Name,
					c.Protocol,
					target,
					activeStr,
				})
			}
			fmt.Println(RenderTable(headers, rows, widths))
		}

		lines := []string{
			fmt.Sprintf(" %s[1]%s List All Configurations", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[2]%s Add Configuration (Paste Link or JSON)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[3]%s Delete Configuration", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[4]%s Set as Active Node", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[0]%s Back to Main Menu", AnsiRed, AnsiReset),
		}
		fmt.Print(RenderBox("Configuration Management", lines, 74))

		choice, err := ReadMenuChoice("\nSelect an option [0-4]: ", []string{"0", "1", "2", "3", "4"}, reader)
		if err != nil {
			return err
		}

		switch choice {
		case "0", "b":
			return nil
		case "1":
			handleListConfigsDetailed(configs, activeID, reader)
		case "2":
			handleAddConfig(bridge, reader)
		case "3":
			handleDeleteConfig(bridge, configs, reader)
		case "4":
			handleSetActiveConfig(bridge, configs, reader)
		}
	}
}

func handleListConfigsDetailed(configs []store.ConfigItem, activeID string, reader *bufio.Reader) {
	if len(configs) == 0 {
		fmt.Printf("\n%sNo configurations saved yet.%s\n", AnsiYellow, AnsiReset)
		fmt.Print("Press Enter to continue...")
		_, _ = reader.ReadString('\n')
		return
	}

	headers := []string{"#", "Name", "Protocol", "Server:Port", "Latency", "Status"}
	widths := []int{4, 18, 9, 20, 8, 8}
	var rows [][]string

	for i, c := range configs {
		statusText := "-"
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
	fmt.Print("Press Enter to continue...")
	_, _ = reader.ReadString('\n')
}

func handleAddConfig(bridge TUIBridge, reader *bufio.Reader) {
	fmt.Print("\nEnter share link (vless://, vmess://, trojan://, ss://) or raw JSON:\n> ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	trimmed := strings.TrimSpace(input)
	if trimmed == "0" || strings.ToLower(trimmed) == "b" || trimmed == "" {
		return
	}

	created, err := bridge.AddConfigFromLink(trimmed)
	if err != nil {
		fmt.Printf("\n%sFailed to add configuration: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		name := created.Name
		if name == "" {
			name = created.ID
		}
		fmt.Printf("\n%sSuccessfully added configuration: %s (%s)%s\n", AnsiGreen, name, created.Protocol, AnsiReset)
	}
}

func handleDeleteConfig(bridge TUIBridge, configs []store.ConfigItem, reader *bufio.Reader) {
	if len(configs) == 0 {
		fmt.Printf("\n%sNo configurations to delete.%s\n", AnsiYellow, AnsiReset)
		return
	}

	fmt.Printf("\nSelect configuration to delete [1-%d] or 0 to cancel: ", len(configs))
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
	if err := bridge.DeleteConfig(selected.ID); err != nil {
		fmt.Printf("%sFailed to delete configuration: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		fmt.Printf("%sConfiguration %q deleted successfully.%s\n", AnsiGreen, selected.Name, AnsiReset)
	}
}

func handleSetActiveConfig(bridge TUIBridge, configs []store.ConfigItem, reader *bufio.Reader) {
	if len(configs) == 0 {
		fmt.Printf("\n%sNo configurations available.%s\n", AnsiYellow, AnsiReset)
		return
	}

	fmt.Printf("\nSelect configuration to set as active [1-%d] or 0 to cancel: ", len(configs))
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
	if err := bridge.SetActiveConfig(selected.ID); err != nil {
		fmt.Printf("%sFailed to set active configuration: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		fmt.Printf("%sConfiguration %q is now set as active!%s\n", AnsiGreen, selected.Name, AnsiReset)
	}
}
