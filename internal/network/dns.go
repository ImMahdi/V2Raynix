package network

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultResolvConfPath   = "/etc/resolv.conf"
	defaultResolvBackupPath = "/etc/v2raynix/resolv.conf.bak"
)

var defaultDNSServers = []string{"1.1.1.1", "8.8.8.8"}

// ParseDNSServers parses a comma-delimited list of DNS server IPs, trimming
// whitespace and filtering out invalid entries. Falls back to 1.1.1.1 and
// 8.8.8.8 when no valid IP addresses remain.
func ParseDNSServers(raw string) []string {
	var servers []string
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if ip := net.ParseIP(trimmed); ip != nil {
			servers = append(servers, ip.String())
		}
	}
	if len(servers) == 0 {
		out := make([]string, len(defaultDNSServers))
		copy(out, defaultDNSServers)
		return out
	}
	return servers
}

// BuildResolvectlApplyArgs builds the ordered resolvectl argument lists to
// route all host DNS lookups over the given tunnel interface.
func BuildResolvectlApplyArgs(iface string, servers []string) [][]string {
	dnsArgs := make([]string, 0, 2+len(servers))
	dnsArgs = append(dnsArgs, "dns", iface)
	dnsArgs = append(dnsArgs, servers...)

	return [][]string{
		dnsArgs,
		{"domain", iface, "~."},
		{"default-route", iface, "yes"},
		{"flush-caches"},
	}
}

// BuildResolvectlRevertArgs builds the ordered resolvectl argument lists to
// revert DNS configuration for the given tunnel interface.
func BuildResolvectlRevertArgs(iface string) [][]string {
	return [][]string{
		{"revert", iface},
		{"flush-caches"},
	}
}

// ApplyDNSOverTunnel configures host DNS resolution to traverse tun0 using
// systemd-resolved (resolvectl) when available, or /etc/resolv.conf fallback.
func ApplyDNSOverTunnel(remoteDNS string) error {
	servers := ParseDNSServers(remoteDNS)
	if _, err := exec.LookPath("resolvectl"); err == nil {
		return runResolvectl(BuildResolvectlApplyArgs(TunDevice, servers), true)
	}
	return applyResolvConfFallback(defaultResolvConfPath, defaultResolvBackupPath, servers)
}

// RevertDNSOverTunnel restores host DNS configuration by reverting tun0 in
// resolvectl (if available) and restoring any /etc/resolv.conf backup.
func RevertDNSOverTunnel() error {
	var errs []error
	if _, err := exec.LookPath("resolvectl"); err == nil {
		if err := runResolvectl(BuildResolvectlRevertArgs(TunDevice), false); err != nil {
			errs = append(errs, err)
		}
	}
	if err := revertResolvConfFallback(defaultResolvConfPath, defaultResolvBackupPath); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func runResolvectl(argsList [][]string, stopOnErr bool) error {
	var errs []error
	for _, args := range argsList {
		cmd := exec.Command("resolvectl", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			runErr := fmt.Errorf("resolvectl %s failed: %w, output: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
			if stopOnErr {
				return runErr
			}
			errs = append(errs, runErr)
		}
	}
	return errors.Join(errs...)
}

func applyResolvConfFallback(resolvConfPath, backupPath string, servers []string) error {
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if orig, readErr := os.ReadFile(resolvConfPath); readErr == nil {
			if mkErr := os.MkdirAll(filepath.Dir(backupPath), 0o755); mkErr != nil {
				return fmt.Errorf("create dns backup dir: %w", mkErr)
			}
			if writeErr := os.WriteFile(backupPath, orig, 0o644); writeErr != nil {
				return fmt.Errorf("write dns backup %s: %w", backupPath, writeErr)
			}
		} else if !os.IsNotExist(readErr) {
			return fmt.Errorf("read %s for backup: %w", resolvConfPath, readErr)
		}
	}

	var b strings.Builder
	for _, srv := range servers {
		b.WriteString("nameserver ")
		b.WriteString(srv)
		b.WriteByte('\n')
	}

	if err := os.MkdirAll(filepath.Dir(resolvConfPath), 0o755); err != nil {
		return fmt.Errorf("create resolv.conf dir: %w", err)
	}
	if err := os.WriteFile(resolvConfPath, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", resolvConfPath, err)
	}
	return nil
}

func revertResolvConfFallback(resolvConfPath, backupPath string) error {
	bak, err := os.ReadFile(backupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dns backup %s: %w", backupPath, err)
	}

	if err := os.WriteFile(resolvConfPath, bak, 0o644); err != nil {
		return fmt.Errorf("restore %s: %w", resolvConfPath, err)
	}
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove dns backup %s: %w", backupPath, err)
	}
	return nil
}
