package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/v2raynix/v2raynix/internal/store"
)

// HandleCLICommand handles non-interactive headless CLI subcommands.
func HandleCLICommand(args []string, bridge TUIBridge) error {
	return handleCLICommandWithOutput(args, bridge, os.Stdout)
}

func handleCLICommandWithOutput(args []string, bridge TUIBridge, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("no command specified. Run 'v2raynix help' for usage")
	}

	subcmd := strings.ToLower(args[0])
	switch subcmd {
	case "help", "--help", "-h":
		return runHelpCommand(out)
	case "list":
		return runListCommand(bridge, out)
	case "status":
		return runStatusCommand(args[1:], bridge, out)
	case "connect":
		return runConnectCommand(args[1:], bridge, out)
	case "disconnect":
		return runDisconnectCommand(bridge, out)
	case "health", "check":
		return runHealthCommand(bridge, out)
	case "confirm":
		return runConfirmCommand(bridge, out)
	case "rollback":
		return runRollbackCommand(bridge, out)
	default:
		return fmt.Errorf("unknown command '%s'. Run 'v2raynix help' for usage", args[0])
	}
}

func runHelpCommand(out io.Writer) error {
	helpText := `V2Raynix - Modern Transparent Proxy & Tunnel Manager

Usage:
  v2raynix <command> [options]

Commands:
  connect <node> [--force]   Connect to configuration (by exact ID or name substring)
  disconnect                 Disconnect active tunnel
  status [--json]            Show current service and tunnel status
  health, check              Run connectivity and latency test
  list                       List all saved configurations
  confirm                    Confirm safe mode immediately
  rollback                   Rollback safe mode immediately
  help, --help, -h           Show this help message

Options:
  --force, -f                Bypass or immediately confirm safe mode guard
  --json                     Output status as machine-readable JSON
`
	_, err := fmt.Fprint(out, helpText)
	return err
}

func runListCommand(bridge TUIBridge, out io.Writer) error {
	configs, activeID, err := bridge.ListConfigs()
	if err != nil {
		return fmt.Errorf("failed to list configurations: %w", err)
	}

	if len(configs) == 0 {
		fmt.Fprintln(out, "No configurations found.")
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tPROTOCOL\tTARGET\tACTIVE")
	fmt.Fprintln(w, "--\t----\t--------\t------\t------")
	for _, cfg := range configs {
		activeMark := "-"
		if cfg.ID == activeID {
			activeMark = "[ACTIVE]"
		}
		target := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
		if cfg.Server == "" && cfg.Port == 0 {
			target = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", cfg.ID, cfg.Name, cfg.Protocol, target, activeMark)
	}
	return w.Flush()
}

func runStatusCommand(args []string, bridge TUIBridge, out io.Writer) error {
	jsonMode := false
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			break
		}
	}

	status, err := bridge.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if jsonMode {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format status JSON: %w", err)
		}
		fmt.Fprintln(out, string(data))
		return nil
	}

	fmt.Fprintln(out, "=== V2Raynix System Status ===")
	fmt.Fprintf(out, "  Service State : %s\n", status.ServiceState)
	fmt.Fprintf(out, "  Tunnel State  : %s\n", status.TunnelState)
	if status.ConfigName != "" {
		fmt.Fprintf(out, "  Active Node   : %s (%s)\n", status.ConfigName, status.Protocol)
	}
	if status.TunnelState == "connected" {
		fmt.Fprintf(out, "  Transfer Rate : ↑ %s | ↓ %s\n", FormatSpeed(status.UploadSpeed), FormatSpeed(status.DownloadSpeed))
		fmt.Fprintf(out, "  Total Traffic : ↑ %s | ↓ %s\n", FormatBytes(status.UploadTotal), FormatBytes(status.DownloadTotal))
		fmt.Fprintf(out, "  Tunnel Uptime : %ds\n", status.Uptime)
	}
	if status.SafeModeRemaining > 0 {
		fmt.Fprintf(out, "  Safe Mode     : ACTIVE (%ds remaining to auto-rollback)\n", status.SafeModeRemaining)
	} else {
		fmt.Fprintln(out, "  Safe Mode     : Inactive")
	}
	if status.HealthState != "" {
		if status.HealthLatency > 0 {
			fmt.Fprintf(out, "  Health Probe  : %s (%dms)\n", status.HealthState, status.HealthLatency)
		} else {
			fmt.Fprintf(out, "  Health Probe  : %s\n", status.HealthState)
		}
	}
	if status.WebPort > 0 {
		webIP := status.WebIP
		if webIP == "" {
			webIP = "127.0.0.1"
		}
		fmt.Fprintf(out, "  Web Dashboard : http://%s:%d\n", webIP, status.WebPort)
	}
	return nil
}

func runConnectCommand(args []string, bridge TUIBridge, out io.Writer) error {
	var target string
	force := false

	for _, arg := range args {
		if arg == "--force" || arg == "-f" {
			force = true
		} else if target == "" && !strings.HasPrefix(arg, "-") {
			target = arg
		}
	}

	if target == "" {
		return errors.New("usage: v2raynix connect <node-id-or-name> [--force]")
	}

	configs, _, err := bridge.ListConfigs()
	if err != nil {
		return fmt.Errorf("failed to fetch configurations: %w", err)
	}

	var matched *store.ConfigItem
	// 1. Search exact ID
	for i := range configs {
		if configs[i].ID == target {
			matched = &configs[i]
			break
		}
	}

	// 2. Search exact Name
	if matched == nil {
		for i := range configs {
			if strings.EqualFold(configs[i].Name, target) {
				matched = &configs[i]
				break
			}
		}
	}

	// 3. Search Name substring (case-insensitive)
	if matched == nil {
		targetLower := strings.ToLower(target)
		for i := range configs {
			if strings.Contains(strings.ToLower(configs[i].Name), targetLower) {
				matched = &configs[i]
				break
			}
		}
	}

	if matched == nil {
		return fmt.Errorf("configuration '%s' not found", target)
	}

	if err := bridge.ConnectTunnel(matched.ID); err != nil {
		return fmt.Errorf("failed to connect tunnel: %w", err)
	}

	fmt.Fprintf(out, "Connected to configuration: %s (%s)\n", matched.Name, matched.ID)

	if force {
		if err := bridge.ConfirmSafeMode(); err != nil {
			return fmt.Errorf("connected, but failed to confirm safe mode: %w", err)
		}
		fmt.Fprintln(out, "Safe mode confirmed.")
	}

	return nil
}

func runDisconnectCommand(bridge TUIBridge, out io.Writer) error {
	if err := bridge.DisconnectTunnel(); err != nil {
		return fmt.Errorf("failed to disconnect tunnel: %w", err)
	}
	fmt.Fprintln(out, "Tunnel disconnected successfully.")
	return nil
}

func runHealthCommand(bridge TUIBridge, out io.Writer) error {
	healthy, latency, err := bridge.CheckHealth()
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	if !healthy {
		return fmt.Errorf("health check failed: unhealthy (latency: %dms)", latency)
	}
	fmt.Fprintf(out, "Health check passed: healthy (latency: %dms)\n", latency)
	return nil
}

func runConfirmCommand(bridge TUIBridge, out io.Writer) error {
	if err := bridge.ConfirmSafeMode(); err != nil {
		return fmt.Errorf("failed to confirm safe mode: %w", err)
	}
	fmt.Fprintln(out, "Safe mode confirmed successfully.")
	return nil
}

func runRollbackCommand(bridge TUIBridge, out io.Writer) error {
	if err := bridge.RollbackSafeMode(); err != nil {
		return fmt.Errorf("failed to rollback safe mode: %w", err)
	}
	fmt.Fprintln(out, "Safe mode rolled back successfully.")
	return nil
}
