package cli

import (
	"bufio"
	"strings"
	"testing"
)

func TestRenderBox(t *testing.T) {
	lines := []string{
		"Core Engine: Xray-core",
		"Status: Running",
	}
	width := 40
	out := RenderBox("Server Info", lines, width)

	// Verify box drawing borders: ┌, ─, ┐, │, └, ┘
	for _, border := range []string{"┌", "─", "┐", "│", "└", "┘"} {
		if !strings.Contains(out, border) {
			t.Errorf("expected box border %q in output, got:\n%s", border, out)
		}
	}

	// Verify content
	if !strings.Contains(out, "Server Info") {
		t.Errorf("expected title 'Server Info' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Core Engine: Xray-core") {
		t.Errorf("expected 'Core Engine: Xray-core' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Status: Running") {
		t.Errorf("expected 'Status: Running' in output, got:\n%s", out)
	}

	// Verify visual width alignment of each line
	outLines := strings.Split(strings.Trim(out, "\r\n"), "\n")
	for i, l := range outLines {
		cleanLine := strings.TrimRight(l, "\r")
		vw := VisualWidth(cleanLine)
		if vw != width {
			t.Errorf("line %d has visual width %d, expected %d: %q", i, vw, width, cleanLine)
		}
	}
}

func TestRenderTable(t *testing.T) {
	headers := []string{"#", "Name", "Protocol"}
	rows := [][]string{
		{"1", "Server A", "vless"},
		{"2", "VeryLongServerIdentifierExceedingColumn", "vmess"},
	}
	widths := []int{4, 15, 10}
	out := RenderTable(headers, rows, widths)

	// Verify headers and content
	if !strings.Contains(out, "Server A") {
		t.Errorf("expected 'Server A' in table, got:\n%s", out)
	}
	if !strings.Contains(out, "vless") {
		t.Errorf("expected 'vless' in table, got:\n%s", out)
	}

	// Verify box drawing table borders
	for _, char := range []string{"┌", "┬", "┐", "├", "┼", "┤", "└", "┴", "┘", "│", "─"} {
		if !strings.Contains(out, char) {
			t.Errorf("expected table border character %q in output, got:\n%s", char, out)
		}
	}

	// Verify column clipping with ellipsis
	if !strings.Contains(out, "...") {
		t.Errorf("expected ellipsis '...' in truncated column, got:\n%s", out)
	}

	// Verify line width consistency: sum(widths) + len(widths) + 1 borders
	expectedWidth := 4 + 15 + 10 + 4
	outLines := strings.Split(strings.Trim(out, "\r\n"), "\n")
	for i, l := range outLines {
		cleanLine := strings.TrimRight(l, "\r")
		vw := VisualWidth(cleanLine)
		if vw != expectedWidth {
			t.Errorf("table line %d has visual width %d, expected %d: %q", i, vw, expectedWidth, cleanLine)
		}
	}
}

func TestRenderStatusHeader(t *testing.T) {
	info := LiveStatusInfo{
		ServiceState:      "active",
		TunnelState:       "connected",
		ConfigName:        "EU - High Speed",
		Protocol:          "vless",
		UploadSpeed:       1048576, // 1.0 MB/s
		DownloadSpeed:     2621440, // 2.5 MB/s
		UploadTotal:       10485760,
		DownloadTotal:     52428800,
		Uptime:            3665, // 1h 1m
		SafeModeRemaining: 45,
		HealthState:       "healthy",
		HealthLatency:     42,
		WebPort:           2080,
		WebIP:             "127.0.0.1",
	}

	out := RenderStatusHeader(info)

	// Verify service bullet
	if !strings.Contains(out, "●") || !strings.Contains(out, "Active") {
		t.Errorf("expected service bullet with Active status, got:\n%s", out)
	}

	// Verify tunnel state
	if !strings.Contains(out, "Connected") || !strings.Contains(out, "EU - High Speed") || !strings.Contains(out, "vless") {
		t.Errorf("expected connected tunnel info, got:\n%s", out)
	}

	// Verify traffic
	if !strings.Contains(out, "1.0 MB/s") || !strings.Contains(out, "2.5 MB/s") {
		t.Errorf("expected upload/download speeds in status header, got:\n%s", out)
	}

	// Verify uptime
	if !strings.Contains(out, "1h 1m") {
		t.Errorf("expected uptime '1h 1m' in status header, got:\n%s", out)
	}

	// Verify safe mode
	if !strings.Contains(out, "Safe Mode") || !strings.Contains(out, "45s") {
		t.Errorf("expected active Safe Mode info with remaining seconds, got:\n%s", out)
	}
}

func TestFormatHelpers(t *testing.T) {
	// FormatSpeed
	if s := FormatSpeed(1048576); s != "1.0 MB/s" {
		t.Errorf("expected FormatSpeed(1048576) == '1.0 MB/s', got %q", s)
	}
	if s := FormatSpeed(500); s != "500 B/s" {
		t.Errorf("expected FormatSpeed(500) == '500 B/s', got %q", s)
	}
	if s := FormatSpeed(10240); s != "10.0 KB/s" {
		t.Errorf("expected FormatSpeed(10240) == '10.0 KB/s', got %q", s)
	}

	// FormatBytes
	if b := FormatBytes(1048576); b != "1.0 MB" {
		t.Errorf("expected FormatBytes(1048576) == '1.0 MB', got %q", b)
	}
	if b := FormatBytes(500); b != "500 B" {
		t.Errorf("expected FormatBytes(500) == '500 B', got %q", b)
	}

	// FormatDuration
	if d := FormatDuration(3665); d != "1h 1m" {
		t.Errorf("expected FormatDuration(3665) == '1h 1m', got %q", d)
	}
	if d := FormatDuration(45); d != "45s" {
		t.Errorf("expected FormatDuration(45) == '45s', got %q", d)
	}
	if d := FormatDuration(125); d != "2m 5s" {
		t.Errorf("expected FormatDuration(125) == '2m 5s', got %q", d)
	}
	if d := FormatDuration(90000); d != "1d 1h" {
		t.Errorf("expected FormatDuration(90000) == '1d 1h', got %q", d)
	}
}

func TestReadMenuChoice(t *testing.T) {
	// Valid choice
	r1 := bufio.NewReader(strings.NewReader("1\n"))
	c1, err := ReadMenuChoice("Option: ", []string{"1", "2"}, r1)
	if err != nil || c1 != "1" {
		t.Errorf("expected choice '1', got %q (err: %v)", c1, err)
	}

	// Back choice '0'
	r2 := bufio.NewReader(strings.NewReader("0\n"))
	c2, err := ReadMenuChoice("Option: ", []string{"1", "2"}, r2)
	if err != nil || c2 != "0" {
		t.Errorf("expected choice '0', got %q (err: %v)", c2, err)
	}

	// Back choice 'b'
	r3 := bufio.NewReader(strings.NewReader("b\n"))
	c3, err := ReadMenuChoice("Option: ", []string{"1", "2"}, r3)
	if err != nil || c3 != "b" {
		t.Errorf("expected choice 'b', got %q (err: %v)", c3, err)
	}

	// Quit choice 'q'
	r4 := bufio.NewReader(strings.NewReader("q\n"))
	c4, err := ReadMenuChoice("Option: ", []string{"1", "2"}, r4)
	if err != nil || c4 != "q" {
		t.Errorf("expected choice 'q', got %q (err: %v)", c4, err)
	}

	// Invalid choice followed by valid choice
	r5 := bufio.NewReader(strings.NewReader("invalid\n2\n"))
	c5, err := ReadMenuChoice("Option: ", []string{"1", "2"}, r5)
	if err != nil || c5 != "2" {
		t.Errorf("expected retry and selection of '2', got %q (err: %v)", c5, err)
	}
}
