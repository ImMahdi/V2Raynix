package cli

import (
	"bufio"
	"fmt"
	"strings"
)

// Additional ANSI color and style escape sequences
const (
	AnsiBlue      = "\033[1;34m"
	AnsiDim       = "\033[2m"
	AnsiUnderline = "\033[4m"
)

// LiveStatusInfo captures the dynamic health, traffic, and runtime state of the daemon and tunnel.
type LiveStatusInfo struct {
	ServiceState      string // "active", "inactive"
	TunnelState       string // "connected", "disconnected", "connecting"
	ConfigName        string // e.g. "EU - High Speed"
	Protocol          string // e.g. "vless"
	UploadSpeed       int64  // bytes per second
	DownloadSpeed     int64  // bytes per second
	UploadTotal       int64  // total bytes sent
	DownloadTotal     int64  // total bytes received
	Uptime            int64  // tunnel active duration in seconds
	SafeModeRemaining int    // countdown seconds remaining (> 0 means active)
	HealthState       string // "healthy", "unhealthy", "not verified"
	HealthLatency     int64  // probe latency in milliseconds
	WebPort           int    // web panel port
	WebIP             string // host or binding IP
}

// FormatSpeed formats bytes per second into human-readable network transfer rates.
func FormatSpeed(bps int64) string {
	if bps < 0 {
		bps = 0
	}
	f := float64(bps)
	switch {
	case f < 1024:
		return fmt.Sprintf("%d B/s", bps)
	case f < 1024*1024:
		return fmt.Sprintf("%.1f KB/s", f/1024.0)
	case f < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB/s", f/(1024.0*1024.0))
	default:
		return fmt.Sprintf("%.1f GB/s", f/(1024.0*1024.0*1024.0))
	}
}

// FormatBytes formats byte counts into human-readable storage/transfer units.
func FormatBytes(b int64) string {
	if b < 0 {
		b = 0
	}
	f := float64(b)
	switch {
	case f < 1024:
		return fmt.Sprintf("%d B", b)
	case f < 1024*1024:
		return fmt.Sprintf("%.1f KB", f/1024.0)
	case f < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", f/(1024.0*1024.0))
	default:
		return fmt.Sprintf("%.1f GB", f/(1024.0*1024.0*1024.0))
	}
}

// FormatDuration formats seconds into a concise human-readable duration (e.g. 45s, 1h 1m, 1d 2h).
func FormatDuration(sec int64) string {
	if sec <= 0 {
		return "0s"
	}
	switch {
	case sec < 60:
		return fmt.Sprintf("%ds", sec)
	case sec < 3600:
		m := sec / 60
		s := sec % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm %ds", m, s)
	case sec < 86400:
		h := sec / 3600
		m := (sec % 3600) / 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		d := sec / 86400
		h := (sec % 86400) / 3600
		if h == 0 {
			return fmt.Sprintf("%dd", d)
		}
		return fmt.Sprintf("%dd %dh", d, h)
	}
}

// TruncateString truncates string s to maxWidth visual columns, appending '...' if truncated.
func TruncateString(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	clean := StripANSI(s)
	if VisualWidth(clean) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		runes := []rune(clean)
		if len(runes) > maxWidth {
			return string(runes[:maxWidth])
		}
		return string(runes)
	}

	target := maxWidth - 3
	runes := []rune(clean)
	var cur strings.Builder
	w := 0
	for _, r := range runes {
		rw := runeWidth(r)
		if w+rw > target {
			break
		}
		cur.WriteRune(r)
		w += rw
	}
	return cur.String() + "..."
}

// RenderBox draws an ANSI border box with title, content lines, and exact width alignment.
func RenderBox(title string, lines []string, width int) string {
	if width < 10 {
		width = 60
	}
	innerWidth := width - 2
	var sb strings.Builder

	// Top border: ┌─ Title ─────┐ or ┌──────────┐
	sb.WriteString(AnsiCyan + "┌")
	if title != "" {
		trimmedTitle := strings.TrimSpace(title)
		titleFormatted := " " + trimmedTitle + " "
		tWidth := VisualWidth(titleFormatted)
		if tWidth > innerWidth-4 {
			titleFormatted = " " + TruncateString(trimmedTitle, innerWidth-6) + " "
			tWidth = VisualWidth(titleFormatted)
		}
		sb.WriteString("─" + AnsiBold + titleFormatted + AnsiReset + AnsiCyan)
		remaining := innerWidth - 1 - tWidth
		if remaining < 0 {
			remaining = 0
		}
		sb.WriteString(strings.Repeat("─", remaining))
	} else {
		sb.WriteString(strings.Repeat("─", innerWidth))
	}
	sb.WriteString("┐" + AnsiReset + "\n")

	// Content lines: │ Content   │
	for _, line := range lines {
		vWidth := VisualWidth(line)
		content := line
		if vWidth > innerWidth-2 {
			content = TruncateString(line, innerWidth-2)
			vWidth = VisualWidth(content)
		}
		padded := " " + content + strings.Repeat(" ", innerWidth-1-vWidth)
		sb.WriteString(AnsiCyan + "│" + AnsiReset + padded + AnsiCyan + "│" + AnsiReset + "\n")
	}

	// Bottom border: └──────────┘
	sb.WriteString(AnsiCyan + "└" + strings.Repeat("─", innerWidth) + "┘" + AnsiReset + "\n")

	return sb.String()
}

// formatTableCell pads or truncates cell content to fit column width.
func formatTableCell(text string, width int) string {
	if width <= 0 {
		return ""
	}
	clean := StripANSI(text)
	vWidth := VisualWidth(clean)
	if vWidth > width {
		truncated := TruncateString(clean, width)
		tWidth := VisualWidth(truncated)
		if tWidth < width {
			return truncated + strings.Repeat(" ", width-tWidth)
		}
		return truncated
	}
	cell := text
	if !strings.HasPrefix(clean, " ") && vWidth < width {
		cell = " " + text
		vWidth = VisualWidth(StripANSI(cell))
	}
	if vWidth < width {
		cell = cell + strings.Repeat(" ", width-vWidth)
	}
	return cell
}

// RenderTable renders a structured ANSI data table with border separators and auto-clipping.
func RenderTable(headers []string, rows [][]string, widths []int) string {
	if len(widths) == 0 {
		return ""
	}
	var sb strings.Builder

	// Top border: ┌────┬──────────┐
	sb.WriteString(AnsiCyan + "┌")
	for i, w := range widths {
		sb.WriteString(strings.Repeat("─", w))
		if i < len(widths)-1 {
			sb.WriteString("┬")
		} else {
			sb.WriteString("┐" + AnsiReset + "\n")
		}
	}

	// Header row: │ #  │ Name     │
	if len(headers) > 0 {
		sb.WriteString(AnsiCyan + "│" + AnsiReset)
		for i, w := range widths {
			headerText := ""
			if i < len(headers) {
				headerText = headers[i]
			}
			cell := formatTableCell(headerText, w)
			sb.WriteString(AnsiBold + cell + AnsiReset + AnsiCyan + "│" + AnsiReset)
		}
		sb.WriteString("\n")

		// Divider: ├────┼──────────┤
		sb.WriteString(AnsiCyan + "├")
		for i, w := range widths {
			sb.WriteString(strings.Repeat("─", w))
			if i < len(widths)-1 {
				sb.WriteString("┼")
			} else {
				sb.WriteString("┤" + AnsiReset + "\n")
			}
		}
	}

	// Data rows
	for _, row := range rows {
		sb.WriteString(AnsiCyan + "│" + AnsiReset)
		for i, w := range widths {
			cellText := ""
			if i < len(row) {
				cellText = row[i]
			}
			cell := formatTableCell(cellText, w)
			sb.WriteString(cell + AnsiCyan + "│" + AnsiReset)
		}
		sb.WriteString("\n")
	}

	// Bottom border: └────┴──────────┘
	sb.WriteString(AnsiCyan + "└")
	for i, w := range widths {
		sb.WriteString(strings.Repeat("─", w))
		if i < len(widths)-1 {
			sb.WriteString("┴")
		} else {
			sb.WriteString("┘" + AnsiReset + "\n")
		}
	}

	return sb.String()
}

// RenderStatusHeader renders a live status card with service bullet, tunnel state, traffic, uptime, and safe mode.
func RenderStatusHeader(info LiveStatusInfo) string {
	serviceStr := fmt.Sprintf("%s● Service: Active%s", AnsiGreen, AnsiReset)
	if strings.ToLower(info.ServiceState) != "active" {
		serviceStr = fmt.Sprintf("%s● Service: Inactive%s", AnsiRed, AnsiReset)
	}

	tunnelStr := ""
	switch strings.ToLower(info.TunnelState) {
	case "connected":
		cfg := info.ConfigName
		if cfg == "" {
			cfg = "Active Node"
		}
		proto := info.Protocol
		if proto != "" {
			tunnelStr = fmt.Sprintf("%s● Tunnel: Connected%s (%s | %s)", AnsiGreen, AnsiReset, cfg, proto)
		} else {
			tunnelStr = fmt.Sprintf("%s● Tunnel: Connected%s (%s)", AnsiGreen, AnsiReset, cfg)
		}
	case "connecting":
		tunnelStr = fmt.Sprintf("%s● Tunnel: Connecting...%s", AnsiYellow, AnsiReset)
	default:
		tunnelStr = fmt.Sprintf("%s● Tunnel: Disconnected%s", AnsiGray, AnsiReset)
	}

	line1 := fmt.Sprintf("%s    %s", serviceStr, tunnelStr)

	trafficStr := fmt.Sprintf("Traffic: ▲ %s  ▼ %s", FormatSpeed(info.UploadSpeed), FormatSpeed(info.DownloadSpeed))
	if info.UploadTotal > 0 || info.DownloadTotal > 0 {
		trafficStr += fmt.Sprintf("  (Tot: ▲ %s  ▼ %s)", FormatBytes(info.UploadTotal), FormatBytes(info.DownloadTotal))
	}

	uptimeStr := fmt.Sprintf("Uptime: %s", FormatDuration(info.Uptime))
	safeModeStr := "Safe Mode: Inactive"
	if info.SafeModeRemaining > 0 {
		safeModeStr = fmt.Sprintf("%sSafe Mode: Active (%ds remaining)%s", AnsiYellow, info.SafeModeRemaining, AnsiReset)
	}
	line3 := fmt.Sprintf("%s    %s", uptimeStr, safeModeStr)

	healthStr := ""
	switch strings.ToLower(info.HealthState) {
	case "healthy":
		healthStr = fmt.Sprintf("%s● Health: Healthy (%dms)%s", AnsiGreen, info.HealthLatency, AnsiReset)
	case "unhealthy":
		healthStr = fmt.Sprintf("%s● Health: Unhealthy%s", AnsiRed, AnsiReset)
	default:
		healthState := info.HealthState
		if healthState == "" {
			healthState = "Not verified"
		}
		healthStr = fmt.Sprintf("%s● Health: %s%s", AnsiGray, healthState, AnsiReset)
	}

	webURL := ""
	if info.WebPort > 0 {
		ip := info.WebIP
		if ip == "" {
			ip = "127.0.0.1"
		}
		webURL = fmt.Sprintf("Web UI: http://%s:%d", ip, info.WebPort)
	}
	line4 := healthStr
	if webURL != "" {
		line4 = fmt.Sprintf("%s    %s", healthStr, webURL)
	}

	lines := []string{line1, trafficStr, line3, line4}
	return RenderBox("V2Raynix System Status", lines, 74)
}

// ReadMenuChoice displays a prompt and reads user selection from reader.
// Supports '0' / 'b' for returning to previous menu, and 'q' for quit.
func ReadMenuChoice(prompt string, validChoices []string, reader *bufio.Reader) (string, error) {
	for {
		if prompt != "" {
			fmt.Print(prompt)
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		raw := strings.TrimSpace(line)
		if raw == "" {
			continue
		}
		norm := strings.ToLower(raw)

		// Navigation shortcuts
		if norm == "0" || norm == "b" || norm == "back" {
			if norm == "0" {
				return "0", nil
			}
			return "b", nil
		}
		if norm == "q" || norm == "quit" {
			return "q", nil
		}

		// Check against explicit valid choices
		if len(validChoices) == 0 {
			return raw, nil
		}
		matched := false
		for _, v := range validChoices {
			if strings.EqualFold(raw, v) {
				matched = true
				break
			}
		}
		if matched {
			return raw, nil
		}

		fmt.Printf("  %sInvalid option '%s'. Please select a valid option:%s ", AnsiRed, raw, AnsiReset)
	}
}
