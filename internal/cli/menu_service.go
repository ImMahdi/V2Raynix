package cli

import (
	"bufio"
	"fmt"
	"os/exec"
	"sort"

	"github.com/v2raynix/v2raynix/internal/updater"
)

// HandleServiceMenu manages the Core Engine Updater & Service Supervisor submenu (Menu 6).
func HandleServiceMenu(bridge TUIBridge, reader *bufio.Reader) error {
	var lastUpdateStatus *updater.UpdateStatus

	for {
		status, err := bridge.GetStatus()
		if err != nil {
			status = LiveStatusInfo{
				ServiceState: "error",
				TunnelState:  "unknown",
			}
		}

		fmt.Println(RenderStatusHeader(status))

		svcStatus, _ := GetServiceStatus()
		if svcStatus == "" {
			if bridge.IsDaemonRunning() {
				svcStatus = "active (running)"
			} else if status.ServiceState != "" {
				svcStatus = status.ServiceState
			} else {
				svcStatus = "inactive"
			}
		}

		serviceLines := []string{
			fmt.Sprintf(" %-22s : %s", "Systemd Daemon Status", svcStatus),
			fmt.Sprintf(" %-22s : %s", "API Bridge Status", status.ServiceState),
			fmt.Sprintf(" %-22s : %s", "Supported Core Engines", "Xray, Sing-box, Mihomo"),
		}
		fmt.Print(RenderBox("Service & Engine Overview", serviceLines, 74))

		if lastUpdateStatus != nil && len(lastUpdateStatus.Cores) > 0 {
			fmt.Println(renderCoreUpdatesTable(lastUpdateStatus))
		}

		lines := []string{
			fmt.Sprintf(" %s[1]%s Check & Upgrade Core Engines (Xray / Sing-box / Mihomo)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[2]%s Restart V2Raynix Daemon (systemctl restart v2raynix)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[3]%s View System Service Status (systemctl status v2raynix)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[0]%s Back to Main Menu", AnsiRed, AnsiReset),
		}
		fmt.Print(RenderBox("Service & Maintenance Operations", lines, 74))

		choice, err := ReadMenuChoice("\nSelect an option [0-3]: ", []string{"0", "1", "2", "3"}, reader)
		if err != nil {
			return err
		}

		switch choice {
		case "0", "b":
			return nil
		case "1":
			lastUpdateStatus = handleCheckAndUpgradeCores(bridge)
		case "2":
			handleRestartService()
		case "3":
			handleViewServiceStatus(reader)
		}
	}
}

func handleCheckAndUpgradeCores(bridge TUIBridge) *updater.UpdateStatus {
	fmt.Println("\nChecking core engine updates...")
	updStatus, err := bridge.CheckCoreUpdates()
	if err != nil {
		fmt.Printf("%sFailed to check core updates: %v%s\n", AnsiRed, err, AnsiReset)
		return nil
	}

	if updStatus == nil || len(updStatus.Cores) == 0 {
		fmt.Printf("%sNo core engine information returned.%s\n", AnsiYellow, AnsiReset)
		return updStatus
	}

	fmt.Println("\n" + renderCoreUpdatesTable(updStatus))

	// Check if any core has an update available and upgrade it
	hasUpdates := false
	for name, info := range updStatus.Cores {
		if info.UpdateAvailable {
			hasUpdates = true
			fmt.Printf("%sFound update for %s (%s -> %s). Upgrading...%s\n", AnsiYellow, name, info.CurrentVersion, info.LatestVersion, AnsiReset)
			if err := bridge.UpdateCore(name); err != nil {
				fmt.Printf("%sFailed to update %s: %v%s\n", AnsiRed, name, err, AnsiReset)
			} else {
				fmt.Printf("%sSuccessfully updated %s.%s\n", AnsiGreen, name, AnsiReset)
			}
		}
	}

	if !hasUpdates {
		fmt.Printf("%sAll core engines are up to date.%s\n", AnsiGreen, AnsiReset)
	}

	return updStatus
}

func renderCoreUpdatesTable(status *updater.UpdateStatus) string {
	headers := []string{"Core Engine", "Installed", "Latest", "Update Available"}
	widths := []int{16, 16, 16, 18}

	var names []string
	for k := range status.Cores {
		names = append(names, k)
	}
	sort.Strings(names)

	var rows [][]string
	for _, k := range names {
		info := status.Cores[k]
		installed := info.CurrentVersion
		if installed == "" {
			installed = "-"
		}
		latest := info.LatestVersion
		if latest == "" {
			latest = "-"
		}
		updateStr := "No"
		if info.UpdateAvailable {
			updateStr = AnsiYellow + "YES" + AnsiReset
		}
		rows = append(rows, []string{info.Name, installed, latest, updateStr})
	}

	return RenderTable(headers, rows, widths)
}

func handleRestartService() {
	fmt.Println("\nRestarting V2Raynix daemon...")
	if err := RestartService(); err != nil {
		fmt.Printf("%sFailed to restart service: %v%s\n", AnsiRed, err, AnsiReset)
	} else {
		fmt.Printf("%sV2Raynix daemon restarted successfully.%s\n", AnsiGreen, AnsiReset)
	}
}

func handleViewServiceStatus(reader *bufio.Reader) {
	fmt.Println("\nExecuting 'systemctl status v2raynix'...")
	cmd := exec.Command("systemctl", "status", "v2raynix")
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Println(string(out))
	} else if err != nil {
		fmt.Printf("%sError querying service status: %v%s\n", AnsiRed, err, AnsiReset)
	}
	fmt.Print("\nPress Enter to continue...")
	_, _ = reader.ReadString('\n')
}
