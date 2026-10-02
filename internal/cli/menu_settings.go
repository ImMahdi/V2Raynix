package cli

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/v2raynix/v2raynix/internal/store"
)

// HandleSettingsMenu manages the System Settings submenu (Menu 5).
func HandleSettingsMenu(bridge TUIBridge, reader *bufio.Reader) error {
	for {
		status, err := bridge.GetStatus()
		if err != nil {
			status = LiveStatusInfo{
				ServiceState: "error",
				TunnelState:  "unknown",
			}
		}

		fmt.Println(RenderStatusHeader(status))

		settings, err := bridge.GetSettings()
		if err != nil || settings == nil {
			settings = &store.SystemSettings{
				WebPort:                    2080,
				SafeModeSeconds:            120,
				HealthCheckIntervalMinutes: 5,
				HealthCheckURL:             "https://www.google.com/generate_204",
			}
		}

		listenIP := status.WebIP
		if listenIP == "" {
			listenIP = "0.0.0.0"
		}

		watchdogStatus := "Disabled"
		if settings.HealthCheckIntervalMinutes > 0 {
			watchdogStatus = fmt.Sprintf("Enabled (every %dm)", settings.HealthCheckIntervalMinutes)
		}

		targetURL := settings.HealthCheckURL
		if targetURL == "" {
			targetURL = "https://www.google.com/generate_204"
		}

		settingsLines := []string{
			fmt.Sprintf(" %-22s : %d", "Web Port", settings.WebPort),
			fmt.Sprintf(" %-22s : %s", "Listen IP", listenIP),
			fmt.Sprintf(" %-22s : %s", "Watchdog Status", watchdogStatus),
			fmt.Sprintf(" %-22s : %s", "Watchdog Target URL", targetURL),
		}
		fmt.Print(RenderBox("Current System Settings", settingsLines, 74))

		lines := []string{
			fmt.Sprintf(" %s[1]%s Change Web Port", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[2]%s Change Admin Username / Password", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[3]%s Configure Watchdog (Enable/Disable, Interval, Target Test URL)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[0]%s Back to Main Menu", AnsiRed, AnsiReset),
		}
		fmt.Print(RenderBox("Settings Operations", lines, 74))

		choice, err := ReadMenuChoice("\nSelect an option [0-3]: ", []string{"0", "1", "2", "3"}, reader)
		if err != nil {
			return err
		}

		switch choice {
		case "0", "b":
			return nil
		case "1":
			handleChangeWebPort(bridge, reader, settings)
		case "2":
			handleChangeCredentials(bridge, reader)
		case "3":
			handleConfigureWatchdog(bridge, reader, settings)
		}
	}
}

func handleChangeWebPort(bridge TUIBridge, reader *bufio.Reader, currentSettings *store.SystemSettings) {
	fmt.Printf("\nEnter new Web Port [1-65535] (current: %d): ", currentSettings.WebPort)
	input, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || trimmed == "0" || strings.ToLower(trimmed) == "b" {
		return
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil || port < 1 || port > 65535 {
		fmt.Printf("%sInvalid port number '%s'. Must be between 1 and 65535.%s\n", AnsiRed, trimmed, AnsiReset)
		return
	}
	currentSettings.WebPort = port
	if err := bridge.SaveSettings(currentSettings); err != nil {
		fmt.Printf("%sFailed to save settings: %v%s\n", AnsiRed, err, AnsiReset)
		return
	}
	fmt.Printf("%sWeb Port successfully updated to %d.%s\n", AnsiGreen, port, AnsiReset)
}

func handleChangeCredentials(bridge TUIBridge, reader *bufio.Reader) {
	fmt.Print("\nEnter new admin username (press Enter to keep 'admin'): ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	username = strings.TrimSpace(username)
	if username == "" {
		username = "admin"
	}

	fmt.Print("Enter new admin password: ")
	password, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	password = strings.TrimSpace(password)
	if password == "" {
		fmt.Printf("%sPassword cannot be empty. Aborting credential change.%s\n", AnsiRed, AnsiReset)
		return
	}

	if err := bridge.ApplyCredentials(username, password); err != nil {
		fmt.Printf("%sFailed to update credentials: %v%s\n", AnsiRed, err, AnsiReset)
		return
	}
	fmt.Printf("%sAdmin credentials successfully updated for user '%s'.%s\n", AnsiGreen, username, AnsiReset)
}

func handleConfigureWatchdog(bridge TUIBridge, reader *bufio.Reader, currentSettings *store.SystemSettings) {
	fmt.Printf("\nEnter Watchdog interval in minutes (0 to disable, current: %d): ", currentSettings.HealthCheckIntervalMinutes)
	input, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	trimmed := strings.TrimSpace(input)
	if trimmed != "" {
		interval, err := strconv.Atoi(trimmed)
		if err != nil || interval < 0 {
			fmt.Printf("%sInvalid interval '%s'. Must be >= 0.%s\n", AnsiRed, trimmed, AnsiReset)
			return
		}
		currentSettings.HealthCheckIntervalMinutes = interval
	}

	fmt.Printf("Enter Watchdog target test URL (current: %s): ", currentSettings.HealthCheckURL)
	urlInput, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	trimmedURL := strings.TrimSpace(urlInput)
	if trimmedURL != "" {
		currentSettings.HealthCheckURL = trimmedURL
	}

	if err := bridge.SaveSettings(currentSettings); err != nil {
		fmt.Printf("%sFailed to save watchdog settings: %v%s\n", AnsiRed, err, AnsiReset)
		return
	}
	fmt.Printf("%sWatchdog settings successfully updated.%s\n", AnsiGreen, AnsiReset)
}
