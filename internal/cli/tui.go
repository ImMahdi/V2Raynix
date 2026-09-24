package cli

import (
	"fmt"
	"regexp"
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

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// ClearScreen clears the terminal screen and resets cursor position to top-left
func ClearScreen() {
	fmt.Print(AnsiClearHome)
}

// StripANSI removes ANSI escape codes from string
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// runeWidth returns the visual column width of a single rune
func runeWidth(r rune) int {
	// Variation selectors and zero-width joiners
	if (r >= 0xFE00 && r <= 0xFE0F) || r == 0x200D {
		return 0
	}
	// Wide characters / emojis (standard East Asian / Unicode width)
	if (r >= 0x1F300 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF) ||
		r == 0x2B50 || r == 0x231A || r == 0x23F0 || r == 0x23F3 || r == 0x2139 ||
		(r >= 0x2E80 && r <= 0x9FFF) {
		return 2
	}
	return 1
}

// VisualWidth calculates the visible display column width of a string
func VisualWidth(s string) int {
	clean := StripANSI(s)
	width := 0
	for _, r := range clean {
		width += runeWidth(r)
	}
	return width
}

// PadRight pads s with spaces until it reaches targetVisualWidth
func PadRight(s string, targetVisualWidth int) string {
	w := VisualWidth(s)
	if w >= targetVisualWidth {
		return s
	}
	return s + strings.Repeat(" ", targetVisualWidth-w)
}

// CenterText centers s within targetVisualWidth
func CenterText(s string, targetVisualWidth int) string {
	w := VisualWidth(s)
	if w >= targetVisualWidth {
		return s
	}
	leftPad := (targetVisualWidth - w) / 2
	rightPad := targetVisualWidth - w - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

const (
	// BoxBorderInnerWidth defines the exact printable column width inside the box
	BoxBorderInnerWidth = 58
)

// makeBoxRow wraps content inside cyan vertical borders with exact padding
func makeBoxRow(content string) string {
	padded := PadRight(content, BoxBorderInnerWidth)
	return fmt.Sprintf("  %s│%s %s %s│%s\n", AnsiCyan, AnsiReset, padded, AnsiCyan, AnsiReset)
}

func makeBoxTop() string {
	return fmt.Sprintf("  %s┌%s┐%s\n", AnsiCyan, strings.Repeat("─", BoxBorderInnerWidth+2), AnsiReset)
}

func makeBoxDivider() string {
	return fmt.Sprintf("  %s├%s┤%s\n", AnsiCyan, strings.Repeat("─", BoxBorderInnerWidth+2), AnsiReset)
}

func makeBoxBottom() string {
	return fmt.Sprintf("  %s└%s┘%s\n", AnsiCyan, strings.Repeat("─", BoxBorderInnerWidth+2), AnsiReset)
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
	sb.WriteString(makeBoxTop())
	title := fmt.Sprintf("%s🛡️  V2Raynix Server Setup%s", AnsiBold, AnsiReset)
	sb.WriteString(makeBoxRow(CenterText(title, BoxBorderInnerWidth)))
	sb.WriteString(makeBoxDivider())

	statusCol := fmt.Sprintf("Status: %s", formattedStatus)
	portCol := fmt.Sprintf("Web Port: %s", portStr)
	dividerCol := fmt.Sprintf("%s│%s", AnsiCyan, AnsiReset)
	statusPortRow := fmt.Sprintf(" %s %s %s", PadRight(statusCol, 26), dividerCol, portCol)
	sb.WriteString(makeBoxRow(statusPortRow))

	sb.WriteString(makeBoxDivider())
	sb.WriteString(makeBoxRow(fmt.Sprintf(" %s[1]%s 🔑 Change / Reset Admin Credentials", AnsiYellow, AnsiReset)))
	sb.WriteString(makeBoxRow(fmt.Sprintf(" %s[2]%s 🌐 Change Web Panel Port", AnsiYellow, AnsiReset)))
	sb.WriteString(makeBoxRow(fmt.Sprintf(" %s[3]%s ⚙️  Service Management (Restart / Stop / Start)", AnsiYellow, AnsiReset)))
	sb.WriteString(makeBoxRow(fmt.Sprintf(" %s[4]%s ℹ️  View Full Status & Access URLs", AnsiYellow, AnsiReset)))
	sb.WriteString(makeBoxRow(fmt.Sprintf(" %s[0]%s 🚪 Exit", AnsiRed, AnsiReset)))
	sb.WriteString(makeBoxBottom())

	return sb.String()
}

// RenderServiceMenu returns a styled service management submenu
func RenderServiceMenu(status string) string {
	var sb strings.Builder
	formattedStatus := FormatStatus(status)

	sb.WriteString("\n")
	sb.WriteString(makeBoxTop())
	title := fmt.Sprintf("%s⚙️  Service Management%s", AnsiBold, AnsiReset)
	sb.WriteString(makeBoxRow(CenterText(title, BoxBorderInnerWidth)))
	sb.WriteString(makeBoxDivider())
	sb.WriteString(makeBoxRow(fmt.Sprintf(" Current Service Status: %s", formattedStatus)))
	sb.WriteString(makeBoxDivider())
	sb.WriteString(makeBoxRow(fmt.Sprintf(" %s[1]%s 🔄 Restart Service", AnsiYellow, AnsiReset)))
	sb.WriteString(makeBoxRow(fmt.Sprintf(" %s[2]%s ⏹️  Stop Service", AnsiYellow, AnsiReset)))
	sb.WriteString(makeBoxRow(fmt.Sprintf(" %s[3]%s ▶️  Start Service", AnsiYellow, AnsiReset)))
	sb.WriteString(makeBoxRow(fmt.Sprintf(" %s[0]%s ⬅️  Back to Main Menu", AnsiRed, AnsiReset)))
	sb.WriteString(makeBoxBottom())

	return sb.String()
}

// RenderStatusCard returns a formatted system diagnostic summary card
func RenderStatusCard(status string, webPort int) string {
	var sb strings.Builder
	formattedStatus := FormatStatus(status)

	sb.WriteString("\n")
	sb.WriteString(makeBoxTop())
	title := fmt.Sprintf("%s📊 V2Raynix System Status%s", AnsiBold, AnsiReset)
	sb.WriteString(makeBoxRow(CenterText(title, BoxBorderInnerWidth)))
	sb.WriteString(makeBoxDivider())
	sb.WriteString(makeBoxRow(fmt.Sprintf(" Daemon Service:  %s", formattedStatus)))
	sb.WriteString(makeBoxRow(fmt.Sprintf(" Web Panel Port:  %d", webPort)))
	sb.WriteString(makeBoxRow(fmt.Sprintf(" Local Access:    http://127.0.0.1:%d", webPort)))
	sb.WriteString(makeBoxRow(fmt.Sprintf(" Remote Access:   http://<your-server-ip>:%d", webPort)))
	sb.WriteString(makeBoxBottom())

	return sb.String()
}

// IsCancelInput checks if the user entered a cancellation or back string
func IsCancelInput(s string) bool {
	norm := strings.TrimSpace(strings.ToLower(s))
	return norm == "" || norm == "0" || norm == "b" || norm == "back" || norm == "cancel" || norm == "q" || norm == "exit"
}
