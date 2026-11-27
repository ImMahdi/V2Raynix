package cli

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstall_ParseFlags(t *testing.T) {
	opts, err := ParseUninstallFlags([]string{"--purge", "--yes"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Purge || !opts.Yes {
		t.Errorf("expected Purge=true and Yes=true, got %+v", opts)
	}

	opts2, err := ParseUninstallFlags([]string{"--keep-data", "--remove-cores", "--remove-firewall", "-y"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts2.KeepData || !opts2.RemoveCores || !opts2.RemoveFirewall || !opts2.Yes {
		t.Errorf("expected KeepData, RemoveCores, RemoveFirewall, Yes all true, got %+v", opts2)
	}
}

func TestUninstall_InteractiveCancel(t *testing.T) {
	teardownCalled := false
	runner := &UninstallRunner{
		TeardownNetwork: func(sshPort, webPort int) error {
			teardownCalled = true
			return nil
		},
		ExecCmd: func(name string, args ...string) error { return nil },
		RemoveAll: func(path string) error { return nil },
		RemoveFile: func(path string) error { return nil },
		Out:        &bytes.Buffer{},
	}

	opts := UninstallOptions{
		Yes:     false,
		DataDir: t.TempDir(),
	}
	reader := bufio.NewReader(strings.NewReader("n\n"))
	err := runner.Run(opts, reader)
	if err != nil {
		t.Fatalf("expected nil error on interactive cancel, got %v", err)
	}
	if teardownCalled {
		t.Errorf("expected TeardownNetwork not to be called when user cancels")
	}
}

func TestUninstall_KeepDataVsPurge(t *testing.T) {
	t.Run("keep-data preserves v2raynix.json and removes runtime files", func(t *testing.T) {
		dataDir := t.TempDir()
		cfgPath := filepath.Join(dataDir, "v2raynix.json")
		tokenPath := filepath.Join(dataDir, ".ipc_token")
		xrayPath := filepath.Join(dataDir, "xray-active.json")
		logPath := filepath.Join(dataDir, "v2raynix.log")

		for _, p := range []string{cfgPath, tokenPath, xrayPath, logPath} {
			if err := os.WriteFile(p, []byte("test"), 0600); err != nil {
				t.Fatalf("failed to write %s: %v", p, err)
			}
		}

		var cmds []string
		var removedFiles []string
		var out bytes.Buffer
		runner := &UninstallRunner{
			TeardownNetwork: func(sshPort, webPort int) error { return nil },
			ExecCmd: func(name string, args ...string) error {
				cmds = append(cmds, name+" "+strings.Join(args, " "))
				return nil
			},
			RemoveAll: os.RemoveAll,
			RemoveFile: func(path string) error {
				removedFiles = append(removedFiles, path)
				if strings.HasPrefix(path, dataDir) {
					return os.Remove(path)
				}
				return nil
			},
			Out: &out,
		}

		opts := UninstallOptions{
			KeepData: true,
			Yes:      true,
			DataDir:  dataDir,
			WebPort:  2080,
			SSHPort:  22,
		}
		if err := runner.Run(opts, bufio.NewReader(strings.NewReader(""))); err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if _, err := os.Stat(cfgPath); err != nil {
			t.Errorf("expected v2raynix.json to be preserved, got %v", err)
		}
		for _, p := range []string{tokenPath, xrayPath, logPath} {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("expected runtime file %s to be deleted", p)
			}
		}
		if !strings.Contains(out.String(), "[1/7]") || !strings.Contains(out.String(), "[7/7]") {
			t.Errorf("expected 7-step progress output, got:\n%s", out.String())
		}
	})

	t.Run("purge removes dataDir, core binaries, and firewall rules", func(t *testing.T) {
		dataDir := t.TempDir()
		cfgPath := filepath.Join(dataDir, "v2raynix.json")
		if err := os.WriteFile(cfgPath, []byte("test"), 0600); err != nil {
			t.Fatalf("failed to write %s: %v", cfgPath, err)
		}

		var cmds []string
		var removedAll []string
		var removedFiles []string
		var out bytes.Buffer
		runner := &UninstallRunner{
			TeardownNetwork: func(sshPort, webPort int) error { return nil },
			ExecCmd: func(name string, args ...string) error {
				cmds = append(cmds, name+" "+strings.Join(args, " "))
				return nil
			},
			RemoveAll: func(path string) error {
				removedAll = append(removedAll, path)
				if strings.HasPrefix(path, dataDir) {
					return os.RemoveAll(path)
				}
				return nil
			},
			RemoveFile: func(path string) error {
				removedFiles = append(removedFiles, path)
				return nil
			},
			Out: &out,
		}

		opts := UninstallOptions{
			Purge:   true,
			Yes:     true,
			DataDir: dataDir,
			WebPort: 2080,
			SSHPort: 22,
		}
		if err := runner.Run(opts, bufio.NewReader(strings.NewReader(""))); err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
			t.Errorf("expected dataDir %s to be removed on --purge", dataDir)
		}

		joinedFiles := strings.Join(removedFiles, "\n")
		if !strings.Contains(joinedFiles, "/usr/local/bin/xray") || !strings.Contains(joinedFiles, "/usr/local/bin/tun2socks") {
			t.Errorf("expected core binaries to be removed on --purge, got: %v", removedFiles)
		}

		joinedAll := strings.Join(removedAll, "\n")
		if !strings.Contains(joinedAll, "/usr/local/share/xray") {
			t.Errorf("expected /usr/local/share/xray to be removed on --purge, got: %v", removedAll)
		}

		joinedCmds := strings.Join(cmds, "\n")
		if !strings.Contains(joinedCmds, "ufw delete allow 2080/tcp") {
			t.Errorf("expected ufw firewall rule removal on --purge, got: %v", cmds)
		}
	})
}
