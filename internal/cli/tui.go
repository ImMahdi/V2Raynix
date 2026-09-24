package cli

import (
	"fmt"
	"strings"
)

// ANSI terminal color and control escape sequences
const (
	AnsiReset     = "\033[0m"
	AnsiBold      = "\033[1m"
	AnsiCyan      = "\033[1;36m"
	AnsiGreen     = "\033[1;32m"
	AnsiRed       = "\033[1;31m"
	AnsiYellow    = "\033[1;33m"
	AnsiMagenta   = "\033[1;35m"
	AnsiGray      = "\033[0;90m"
	AnsiWhite     = "\033[1;37m"
	AnsiClearHome = "\033[H\033[2J"
)

// ClearScreen clears the terminal screen and resets cursor position to top-left
func ClearScreen() {
	fmt.Print(AnsiClearHome)
}

// FormatStatus returns a colored status badge for the daemon
func FormatStatus(status string) string {
	if strings.ToLower(status) == "active" {
		return fmt.Sprintf("%s● active%s", AnsiGreen, AnsiReset)
	}
	return fmt.Sprintf("%s● %s%s", AnsiRed, status, AnsiReset)
}

// RenderMainMenu returns a beautifully styled ANSI boxed main menu
func RenderMainMenu(status string, webPort int) string {
	var sb strings.Builder
	formattedStatus := FormatStatus(status)
	portStr := fmt.Sprintf("%s%d%s", AnsiYellow, webPort, AnsiReset)

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s┌────────────────────────────────────────────────────────────┐%s\n", AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s                 %s🛡️   V2Raynix Server Setup%s                 %s│%s\n", AnsiCyan, AnsiReset, AnsiBold, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s├────────────────────────────────────────────────────────────┤%s\n", AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  Status: %-25s %s│%s Web Port: %-19s %s│%s\n", AnsiCyan, AnsiReset, formattedStatus, AnsiCyan, AnsiReset, portStr, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s├────────────────────────────────────────────────────────────┤%s\n", AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  %s[1]%s 🔑 Change / Reset Admin Credentials                   %s│%s\n", AnsiCyan, AnsiReset, AnsiYellow, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  %s[2]%s 🌐 Change Web Panel Port                              %s│%s\n", AnsiCyan, AnsiReset, AnsiYellow, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  %s[3]%s ⚙️  Service Management (Restart / Stop / Start)        %s│%s\n", AnsiCyan, AnsiReset, AnsiYellow, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  %s[4]%s ℹ️  View Full Status & Access URLs                    %s│%s\n", AnsiCyan, AnsiReset, AnsiYellow, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  %s[0]%s 🚪 Exit                                               %s│%s\n", AnsiCyan, AnsiReset, AnsiRed, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s└────────────────────────────────────────────────────────────┘%s\n", AnsiCyan, AnsiReset))

	return sb.String()
}

// RenderServiceMenu returns a styled service management submenu
func RenderServiceMenu(status string) string {
	var sb strings.Builder
	formattedStatus := FormatStatus(status)

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s┌────────────────────────────────────────────────────────────┐%s\n", AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s                 ⚙️   Service Management                     %s│%s\n", AnsiCyan, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s├────────────────────────────────────────────────────────────┤%s\n", AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  Current Service Status: %-37s %s│%s\n", AnsiCyan, AnsiReset, formattedStatus, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s├────────────────────────────────────────────────────────────┤%s\n", AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  %s[1]%s 🔄 Restart Service                                    %s│%s\n", AnsiCyan, AnsiReset, AnsiYellow, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  %s[2]%s ⏹️  Stop Service                                       %s│%s\n", AnsiCyan, AnsiReset, AnsiYellow, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  %s[3]%s ▶️  Start Service                                      %s│%s\n", AnsiCyan, AnsiReset, AnsiYellow, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  %s[0]%s ⬅️  Back to Main Menu                                  %s│%s\n", AnsiCyan, AnsiReset, AnsiRed, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s└────────────────────────────────────────────────────────────┘%s\n", AnsiCyan, AnsiReset))

	return sb.String()
}

// RenderStatusCard returns a formatted system diagnostic summary card
func RenderStatusCard(status string, webPort int) string {
	var sb strings.Builder
	formattedStatus := FormatStatus(status)

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s┌────────────────────────────────────────────────────────────┐%s\n", AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s                 📊  V2Raynix System Status                 %s│%s\n", AnsiCyan, AnsiReset, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s├────────────────────────────────────────────────────────────┤%s\n", AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  Daemon Service:  %-44s %s│%s\n", AnsiCyan, AnsiReset, formattedStatus, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  Web Panel Port:  %-44d %s│%s\n", AnsiCyan, AnsiReset, webPort, AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  Local Access:    %-44s %s│%s\n", AnsiCyan, AnsiReset, fmt.Sprintf("http://127.0.0.1:%d", webPort), AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s│%s  Remote Access:   %-44s %s│%s\n", AnsiCyan, AnsiReset, fmt.Sprintf("http://<your-server-ip>:%d", webPort), AnsiCyan, AnsiReset))
	sb.WriteString(fmt.Sprintf("  %s└────────────────────────────────────────────────────────────┘%s\n", AnsiCyan, AnsiReset))

	return sb.String()
}

// IsCancelInput checks if the user entered a cancellation or back string
func IsCancelInput(s string) bool {
	norm := strings.TrimSpace(strings.ToLower(s))
	return norm == "" || norm == "0" || norm == "b" || norm == "back" || norm == "cancel" || norm == "q" || norm == "exit"
}
