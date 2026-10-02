package cli

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/v2raynix/v2raynix/internal/store"
)

// HandleRoutingMenu manages the Smart Policy Routing Rules submenu (Menu 3).
func HandleRoutingMenu(bridge TUIBridge, reader *bufio.Reader) error {
	for {
		status, err := bridge.GetStatus()
		if err != nil {
			status = LiveStatusInfo{
				ServiceState: "error",
				TunnelState:  "unknown",
			}
		}

		fmt.Println(RenderStatusHeader(status))

		rules, err := bridge.ListRules()
		if err != nil {
			fmt.Printf("\n%sError retrieving routing rules: %v%s\n", AnsiRed, err, AnsiReset)
		} else if len(rules) == 0 {
			fmt.Println(RenderBox("Smart Routing Rules", []string{
				fmt.Sprintf(" %sNo routing rules configured. Use option [2] or [3] to add rules.%s", AnsiYellow, AnsiReset),
			}, 74))
		} else {
			headers := []string{"#", "Pattern/Target", "Type", "Action", "Priority"}
			widths := []int{4, 30, 8, 12, 10}
			var rows [][]string

			for i, r := range rules {
				actionStr := strings.ToUpper(r.Action)
				switch strings.ToLower(r.Action) {
				case "direct":
					actionStr = AnsiGreen + "DIRECT" + AnsiReset
				case "block":
					actionStr = AnsiRed + "BLOCK" + AnsiReset
				case "proxy":
					actionStr = AnsiYellow + "PROXY" + AnsiReset
				}

				rows = append(rows, []string{
					fmt.Sprintf("%d", i+1),
					r.Target,
					r.TargetType,
					actionStr,
					fmt.Sprintf("%d", r.Priority),
				})
			}
			fmt.Println(RenderTable(headers, rows, widths))
		}

		lines := []string{
			fmt.Sprintf(" %s[1]%s View Current Routing Rules", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[2]%s Add Custom Rule (Domain / IP / GeoIP)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[3]%s Apply Iran Bypass Presets (Direct for Iranian domains/IPs, Bypass LAN)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[4]%s Delete Rule", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[0]%s Back to Main Menu", AnsiRed, AnsiReset),
		}
		fmt.Print(RenderBox("Smart Policy Routing Rules", lines, 74))

		choice, err := ReadMenuChoice("\nSelect an option [0-4]: ", []string{"0", "1", "2", "3", "4"}, reader)
		if err != nil {
			return err
		}

		switch choice {
		case "0", "b":
			return nil
		case "1":
			handleViewRulesDetailed(rules, reader)
		case "2":
			handleAddRule(bridge, reader)
		case "3":
			handleApplyPresets(bridge)
		case "4":
			handleDeleteRule(bridge, rules, reader)
		}
	}
}

func handleViewRulesDetailed(rules []store.RoutingRule, reader *bufio.Reader) {
	if len(rules) == 0 {
		fmt.Printf("\n%sNo routing rules defined yet.%s\n", AnsiYellow, AnsiReset)
		fmt.Print("Press Enter to continue...")
		_, _ = reader.ReadString('\n')
		return
	}

	headers := []string{"#", "ID", "Pattern/Target", "Type", "Action", "Priority", "Status"}
	widths := []int{4, 16, 22, 8, 10, 8, 8}
	var rows [][]string

	for i, r := range rules {
		actionStr := strings.ToUpper(r.Action)
		switch strings.ToLower(r.Action) {
		case "direct":
			actionStr = AnsiGreen + "DIRECT" + AnsiReset
		case "block":
			actionStr = AnsiRed + "BLOCK" + AnsiReset
		case "proxy":
			actionStr = AnsiYellow + "PROXY" + AnsiReset
		}

		statusStr := AnsiGreen + "ENABLED" + AnsiReset
		if !r.IsEnabled {
			statusStr = AnsiYellow + "DISABLED" + AnsiReset
		}

		shortID := r.ID
		if len(shortID) > 14 {
			shortID = shortID[:14]
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1),
			shortID,
			r.Target,
			r.TargetType,
			actionStr,
			fmt.Sprintf("%d", r.Priority),
			statusStr,
		})
	}

	fmt.Println("\n" + RenderTable(headers, rows, widths))
	fmt.Print("Press Enter to continue...")
	_, _ = reader.ReadString('\n')
}

func handleAddRule(bridge TUIBridge, reader *bufio.Reader) {
	fmt.Print("\nSelect Target Type:\n [1] Domain / GeoSite\n [2] IP / GeoIP / CIDR\nSelect [1-2] or 0 to cancel: ")
	typeInput, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	typeTrimmed := strings.TrimSpace(typeInput)
	if typeTrimmed == "0" || strings.ToLower(typeTrimmed) == "b" || typeTrimmed == "" {
		return
	}

	targetType := "domain"
	if typeTrimmed == "2" {
		targetType = "ip"
	} else if typeTrimmed != "1" {
		fmt.Printf("%sInvalid selection.%s\n", AnsiRed, AnsiReset)
		return
	}

	fmt.Print("\nEnter pattern (e.g. google.com, geosite:category-ir, 10.0.0.0/8, geoip:ir):\n> ")
	targetInput, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	target := strings.TrimSpace(targetInput)
	if target == "0" || strings.ToLower(target) == "b" || target == "" {
		return
	}

	fmt.Print("\nSelect Action:\n [1] DIRECT (Bypass tunnel)\n [2] PROXY (Route through tunnel)\n [3] BLOCK (Drop / Reject)\nSelect [1-3] or 0 to cancel: ")
	actionInput, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	actionTrimmed := strings.TrimSpace(actionInput)
	if actionTrimmed == "0" || strings.ToLower(actionTrimmed) == "b" || actionTrimmed == "" {
		return
	}

	action := "direct"
	switch actionTrimmed {
	case "1":
		action = "direct"
	case "2":
		action = "proxy"
	case "3":
		action = "block"
	default:
		fmt.Printf("%sInvalid action selection.%s\n", AnsiRed, AnsiReset)
		return
	}

	rule := store.RoutingRule{
		ID:         fmt.Sprintf("rule-%d", time.Now().UnixNano()),
		Target:     target,
		TargetType: targetType,
		Action:     action,
		Priority:   50,
		IsEnabled:  true,
	}

	if err := bridge.AddRule(rule); err != nil {
		fmt.Printf("%sFailed to add routing rule: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		fmt.Printf("%sSuccessfully added routing rule for %q -> %s!%s\n", AnsiGreen, target, strings.ToUpper(action), AnsiReset)
	}
}

func handleApplyPresets(bridge TUIBridge) {
	fmt.Println("\nApplying Iran bypass and ad blocking presets...")
	if err := bridge.ApplyPresets(); err != nil {
		fmt.Printf("%sFailed to apply presets: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		fmt.Printf("%sPresets successfully applied (Domestic Direct + Adblock)!%s\n", AnsiGreen, AnsiReset)
	}
}

func handleDeleteRule(bridge TUIBridge, rules []store.RoutingRule, reader *bufio.Reader) {
	if len(rules) == 0 {
		fmt.Printf("\n%sNo routing rules to delete.%s\n", AnsiYellow, AnsiReset)
		return
	}

	fmt.Printf("\nSelect rule to delete [1-%d] or 0 to cancel: ", len(rules))
	input, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	trimmed := strings.TrimSpace(input)
	if trimmed == "0" || strings.ToLower(trimmed) == "b" || trimmed == "" {
		return
	}

	var idx int
	if _, err := fmt.Sscanf(trimmed, "%d", &idx); err != nil || idx < 1 || idx > len(rules) {
		fmt.Printf("%sInvalid selection.%s\n", AnsiRed, AnsiReset)
		return
	}

	selected := rules[idx-1]
	if err := bridge.DeleteRule(selected.ID); err != nil {
		fmt.Printf("%sFailed to delete rule: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		fmt.Printf("%sRule %q (%s) deleted successfully.%s\n", AnsiGreen, selected.Target, selected.Action, AnsiReset)
	}
}
