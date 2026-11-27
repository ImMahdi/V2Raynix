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

		dnsStatus := "Disabled"
		if settings.DNSOverTunnel {
			remoteDNS := settings.RemoteDNS
			if remoteDNS == "" {
				remoteDNS = "1.1.1.1,8.8.8.8"
			}
			dnsStatus = fmt.Sprintf("Enabled (%s)", remoteDNS)
		}

		settingsLines := []string{
			fmt.Sprintf(" %-22s : %d", "Web Port", settings.WebPort),
			fmt.Sprintf(" %-22s : %s", "Listen IP", listenIP),
			fmt.Sprintf(" %-22s : %s", "Watchdog Status", watchdogStatus),
			fmt.Sprintf(" %-22s : %s", "Watchdog Target URL", targetURL),
			fmt.Sprintf(" %-22s : %s", "DNS Over Tunnel", dnsStatus),
		}
		fmt.Print(RenderBox("Current System Settings", settingsLines, 74))

		lines := []string{
			fmt.Sprintf(" %s[1]%s Change Web Port", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[2]%s Change Admin Username / Password", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[3]%s Configure Watchdog (Enable/Disable, Interval, Target Test URL)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[4]%s Configure DNS Over Tunnel (Anti-Poisoning)", AnsiYellow, AnsiReset),
			fmt.Sprintf(" %s[0]%s Back to Main Menu", AnsiRed, AnsiReset),
		}
		fmt.Print(RenderBox("Settings Operations", lines, 74))

		choice, err := ReadMenuChoice("\nSelect an option [0-4]: ", []string{"0", "1", "2", "3", "4"}, reader)
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
		case "4":
			handleConfigureDNSOverTunnel(bridge, reader)
		}
	}
}

func handleChangeWebPort(bridge TUIBridge, reader *bufio.Reader, currentSettings *store.SystemSettings) {
	fmt.Printf("\nEnter new port (1-65535) [current %d, or '0' / 'b' to cancel]: ", currentSettings.WebPort)
	input, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	trimmed := strings.TrimSpace(input)
	if IsCancelInput(trimmed) {
		fmt.Printf("%sOperation cancelled.%s\n", AnsiYellow, AnsiReset)
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
	fmt.Print("\nEnter new admin username (or '0' / 'b' to cancel): ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	username = strings.TrimSpace(username)
	if IsCancelInput(username) {
		fmt.Printf("%sOperation cancelled.%s\n", AnsiYellow, AnsiReset)
		return
	}

	fmt.Print("Enter new admin password (or '0' / 'b' to cancel): ")
	password, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	password = strings.TrimSpace(password)
	if IsCancelInput(password) {
		fmt.Printf("%sOperation cancelled.%s\n", AnsiYellow, AnsiReset)
		return
	}

	if err := bridge.ApplyCredentials(username, password); err != nil {
		fmt.Printf("%sFailed to update credentials: %v%s\n", AnsiRed, err, AnsiReset)
		return
	}
	fmt.Printf("%sAdmin credentials successfully updated for user '%s'.%s\n", AnsiGreen, username, AnsiReset)
}

func handleConfigureWatchdog(bridge TUIBridge, reader *bufio.Reader, currentSettings *store.SystemSettings) {
	fmt.Printf("\nEnter Watchdog interval in minutes [current %d, or '0' / 'b' to cancel]: ", currentSettings.HealthCheckIntervalMinutes)
	input, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	trimmed := strings.TrimSpace(input)
	if IsCancelInput(trimmed) {
		fmt.Printf("%sOperation cancelled.%s\n", AnsiYellow, AnsiReset)
		return
	}

	interval, err := strconv.Atoi(trimmed)
	if err != nil || interval <= 0 {
		fmt.Printf("%sInvalid interval '%s'. Must be a positive integer.%s\n", AnsiRed, trimmed, AnsiReset)
		return
	}
	currentSettings.HealthCheckIntervalMinutes = interval

	fmt.Printf("Enter Watchdog target test URL [current: %s, or '0' / 'b' to cancel]: ", currentSettings.HealthCheckURL)
	urlInput, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	trimmedURL := strings.TrimSpace(urlInput)
	if IsCancelInput(trimmedURL) {
		fmt.Printf("%sOperation cancelled.%s\n", AnsiYellow, AnsiReset)
		return
	}
	currentSettings.HealthCheckURL = trimmedURL

	if err := bridge.SaveSettings(currentSettings); err != nil {
		fmt.Printf("%sFailed to save watchdog settings: %v%s\n", AnsiRed, err, AnsiReset)
		return
	}
	fmt.Printf("%sWatchdog settings successfully updated.%s\n", AnsiGreen, AnsiReset)
}

func handleConfigureDNSOverTunnel(bridge TUIBridge, reader *bufio.Reader) {
	settings, err := bridge.GetSettings()
	if err != nil || settings == nil {
		settings = &store.SystemSettings{
			WebPort:                    2080,
			SafeModeSeconds:            120,
			HealthCheckIntervalMinutes: 5,
			HealthCheckURL:             "https://www.google.com/generate_204",
			DNSOverTunnel:              true,
			RemoteDNS:                  "1.1.1.1,8.8.8.8",
		}
	}

	currentStatus := "Disabled"
	if settings.DNSOverTunnel {
		currentStatus = "Enabled"
	}

	fmt.Printf("\nEnable DNS Over Tunnel? (y/n) [current: %s, or '0' / 'b' to cancel]: ", currentStatus)
	input, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	trimmed := strings.TrimSpace(input)
	if IsCancelInput(trimmed) {
		fmt.Printf("%sOperation cancelled.%s\n", AnsiYellow, AnsiReset)
		return
	}

	var enabled bool
	switch strings.ToLower(trimmed) {
	case "y", "yes", "true", "1":
		enabled = true
	case "n", "no", "false":
		enabled = false
	default:
		fmt.Printf("%sInvalid choice '%s'. Please enter 'y' or 'n'.%s\n", AnsiRed, trimmed, AnsiReset)
		return
	}

	remoteDNS := settings.RemoteDNS
	if enabled {
		currentDNS := remoteDNS
		if currentDNS == "" {
			currentDNS = "1.1.1.1,8.8.8.8"
		}
		fmt.Printf("Enter Remote DNS servers (comma-separated) [current: %s, or '0' / 'b' to cancel]: ", currentDNS)
		dnsInput, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		trimmedDNS := strings.TrimSpace(dnsInput)
		if IsCancelInput(trimmedDNS) {
			fmt.Printf("%sOperation cancelled.%s\n", AnsiYellow, AnsiReset)
			return
		}
		remoteDNS = trimmedDNS
	}

	settings.DNSOverTunnel = enabled
	settings.RemoteDNS = remoteDNS

	if err := bridge.SaveSettings(settings); err != nil {
		fmt.Printf("%sFailed to save DNS settings: %v%s\n", AnsiRed, err, AnsiReset)
		return
	}

	if enabled {
		fmt.Printf("%sDNS Over Tunnel enabled (%s).%s\n", AnsiGreen, remoteDNS, AnsiReset)
	} else {
		fmt.Printf("%sDNS Over Tunnel disabled.%s\n", AnsiGreen, AnsiReset)
	}
}
