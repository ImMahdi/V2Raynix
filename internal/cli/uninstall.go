package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/v2raynix/v2raynix/internal/network"
)

// UninstallOptions configures the native V2Raynix uninstaller.
type UninstallOptions struct {
	Purge          bool
	KeepData       bool
	RemoveCores    bool
	RemoveFirewall bool
	Yes            bool
	DataDir        string
	WebPort        int
	SSHPort        int
}

// UninstallRunner executes the 7-step uninstall sequence with injectable dependencies.
type UninstallRunner struct {
	TeardownNetwork func(sshPort, webPort int) error
	ExecCmd         func(name string, args ...string) error
	RemoveAll       func(path string) error
	RemoveFile      func(path string) error
	Out             io.Writer
}

var runUninstallFunc = RunUninstall

// ParseUninstallFlags parses CLI flags for `v2raynix uninstall`.
func ParseUninstallFlags(args []string) (UninstallOptions, error) {
	opts := UninstallOptions{
		DataDir: "/etc/v2raynix",
		WebPort: 2080,
		SSHPort: 22,
	}
	for _, arg := range args {
		switch arg {
		case "--purge":
			opts.Purge = true
		case "--keep-data":
			opts.KeepData = true
		case "--remove-cores":
			opts.RemoveCores = true
		case "--remove-firewall":
			opts.RemoveFirewall = true
		case "--yes", "-y":
			opts.Yes = true
		default:
			return opts, fmt.Errorf("unknown uninstall flag: %s", arg)
		}
	}
	return opts, nil
}

// RunUninstall executes the uninstaller using the default system runner.
func RunUninstall(opts UninstallOptions, reader *bufio.Reader) error {
	runner := &UninstallRunner{
		TeardownNetwork: network.ForceTeardownNetwork,
		ExecCmd: func(name string, args ...string) error {
			return exec.Command(name, args...).Run()
		},
		RemoveAll:  os.RemoveAll,
		RemoveFile: os.Remove,
		Out:        os.Stdout,
	}
	return runner.Run(opts, reader)
}

// Run executes the 7-step fail-safe uninstall sequence.
func (r *UninstallRunner) Run(opts UninstallOptions, reader *bufio.Reader) error {
	out := r.Out
	if out == nil {
		out = os.Stdout
	}
	if opts.DataDir == "" {
		opts.DataDir = "/etc/v2raynix"
	}
	if opts.WebPort <= 0 {
		opts.WebPort = 2080
	}
	if opts.SSHPort <= 0 {
		opts.SSHPort = 22
	}

	if !opts.Yes {
		fmt.Fprint(out, "Are you sure you want to uninstall V2Raynix? [y/N]: ")
		line, _ := reader.ReadString('\n')
		ans := strings.TrimSpace(strings.ToLower(line))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(out, "Uninstall cancelled.")
			return nil
		}
	}

	// Step 1/7: Network teardown
	fmt.Fprint(out, "[1/7] Tearing down transparent proxy & network rules... ")
	if err := r.TeardownNetwork(opts.SSHPort, opts.WebPort); err != nil {
		fmt.Fprintf(out, "[WARN: %v]\n", err)
	} else {
		fmt.Fprintln(out, "[OK]")
	}

	// Step 2/7: Stop & disable systemd service
	fmt.Fprint(out, "[2/7] Stopping and disabling v2raynix.service... ")
	_ = r.ExecCmd("systemctl", "stop", "v2raynix.service")
	_ = r.ExecCmd("systemctl", "disable", "v2raynix.service")
	fmt.Fprintln(out, "[OK]")

	// Step 3/7: Remove systemd unit & reload daemon
	fmt.Fprint(out, "[3/7] Removing systemd unit file... ")
	_ = r.RemoveFile("/etc/systemd/system/v2raynix.service")
	_ = r.ExecCmd("systemctl", "daemon-reload")
	fmt.Fprintln(out, "[OK]")

	// Step 4/7: Data directory
	fmt.Fprint(out, "[4/7] Cleaning up data directory... ")
	if opts.Purge && !opts.KeepData {
		_ = r.RemoveAll(opts.DataDir)
	} else {
		for _, name := range []string{".ipc_token", "xray-active.json"} {
			_ = r.RemoveFile(filepath.Join(opts.DataDir, name))
		}
		if matches, err := filepath.Glob(filepath.Join(opts.DataDir, "*.log")); err == nil {
			for _, m := range matches {
				_ = r.RemoveFile(m)
			}
		}
	}
	fmt.Fprintln(out, "[OK]")

	// Step 5/7: Core binaries
	fmt.Fprint(out, "[5/7] Checking core engine binaries... ")
	if opts.Purge || opts.RemoveCores {
		_ = r.RemoveFile("/usr/local/bin/xray")
		_ = r.RemoveFile("/usr/local/bin/tun2socks")
		_ = r.RemoveAll("/usr/local/share/xray")
		fmt.Fprintln(out, "[OK]")
	} else {
		fmt.Fprintln(out, "[SKIPPED]")
	}

	// Step 6/7: Firewall rules
	fmt.Fprint(out, "[6/7] Checking firewall rules... ")
	if opts.Purge || opts.RemoveFirewall {
		portSpec := fmt.Sprintf("%d/tcp", opts.WebPort)
		_ = r.ExecCmd("ufw", "delete", "allow", portSpec)
		_ = r.ExecCmd("firewall-cmd", "--remove-port="+portSpec, "--permanent")
		fmt.Fprintln(out, "[OK]")
	} else {
		fmt.Fprintln(out, "[SKIPPED]")
	}

	// Step 7/7: Remove v2raynix binaries & temp files
	fmt.Fprint(out, "[7/7] Removing v2raynix binaries and temporary files... ")
	_ = r.RemoveFile("/usr/local/bin/v2raynix")
	_ = r.RemoveFile("/usr/bin/v2raynix")
	if matches, err := filepath.Glob("/tmp/v2raynix*"); err == nil {
		for _, m := range matches {
			_ = r.RemoveAll(m)
		}
	}
	fmt.Fprintln(out, "[OK]")

	fmt.Fprintln(out, "V2Raynix has been uninstalled.")
	return nil
}
