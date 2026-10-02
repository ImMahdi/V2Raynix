package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/v2raynix/v2raynix/internal/store"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

var (
	termIsTerminal   = term.IsTerminal
	termReadPassword = term.ReadPassword
)

// ReadPasswordSilent reads a password silently without echoing to the terminal.
// If standard input is not a terminal (e.g. piped or running in automated tests),
// it falls back to reading from reader.
func ReadPasswordSilent(prompt string, reader *bufio.Reader) (string, error) {
	if prompt != "" {
		fmt.Print(prompt)
	}

	fd := int(os.Stdin.Fd())
	if termIsTerminal(fd) {
		bytePass, err := termReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytePass)), nil
	}

	if reader != nil {
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}

	return "", fmt.Errorf("no terminal and no reader available")
}

// ApplyCredentials sets and persists new admin credentials
func ApplyCredentials(st store.Store, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if len(password) < 4 {
		return fmt.Errorf("password must be at least 4 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := &store.UserAccount{
		Username:     username,
		PasswordHash: string(hash),
	}
	return st.SetAdminUser(user)
}

// ApplyWebPort validates and persists a new web management port
func ApplyWebPort(st store.Store, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %d (must be 1-65535)", port)
	}

	settings, err := st.GetSettings()
	if err != nil {
		return fmt.Errorf("failed to get current settings: %w", err)
	}

	settings.WebPort = port
	return st.SaveSettings(settings)
}

// ResolveWebPort resolves the active web listening port based on explicit CLI flags and store settings.
func ResolveWebPort(flagPort int, portExplicit bool, settings *store.SystemSettings) int {
	if !portExplicit && settings != nil && settings.WebPort > 0 {
		return settings.WebPort
	}
	return flagPort
}

// RestartService restarts the v2raynix systemd service
func RestartService() error {
	cmd := exec.Command("systemctl", "restart", "v2raynix")
	return cmd.Run()
}

// GetServiceStatus returns active status of v2raynix service
func GetServiceStatus() (string, error) {
	out, err := exec.Command("systemctl", "is-active", "v2raynix").Output()
	status := strings.TrimSpace(string(out))
	if err != nil && status == "" {
		return "inactive", err
	}
	return status, nil
}

// RunSetup handles interactive and non-interactive server setup
func RunSetup(dataDir string, args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	userFlag := fs.String("user", "", "Set new admin username")
	passFlag := fs.String("pass", "", "Set new admin password")
	portFlag := fs.Int("port", 0, "Set new web panel port")
	serviceFlag := fs.String("service", "", "Service action: start|stop|restart|status")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Quick actions that do not require store access
	if *serviceFlag == "status" {
		status, _ := GetServiceStatus()
		fmt.Printf("Service status: %s\n", status)
		return nil
	}

	dbPath := filepath.Join(dataDir, "v2raynix.json")
	st, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open store at %s: %w", dbPath, err)
	}

	// Non-interactive handling
	if *userFlag != "" || *passFlag != "" || *portFlag != 0 || *serviceFlag != "" {
		if *userFlag != "" || *passFlag != "" {
			u := *userFlag
			if u == "" {
				current, _ := st.GetAdminUser()
				if current != nil {
					u = current.Username
				} else {
					u = "admin"
				}
			}
			if *passFlag == "" {
				return fmt.Errorf("--pass is required when updating credentials")
			}
			if err := ApplyCredentials(st, u, *passFlag); err != nil {
				return err
			}
			fmt.Printf("[OK] Admin credentials updated: %s\n", u)
			_ = RestartService()
		}

		if *portFlag != 0 {
			if err := ApplyWebPort(st, *portFlag); err != nil {
				return err
			}
			fmt.Printf("[OK] Web panel port set to: %d\n", *portFlag)
			_ = RestartService()
		}

		if *serviceFlag != "" {
			switch *serviceFlag {
			case "start", "stop", "restart":
				cmd := exec.Command("systemctl", *serviceFlag, "v2raynix")
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("systemctl %s failed: %w", *serviceFlag, err)
				}
				fmt.Printf("[OK] Service %sed successfully\n", *serviceFlag)
			case "status":
				status, _ := GetServiceStatus()
				fmt.Printf("Service status: %s\n", status)
			default:
				return fmt.Errorf("unknown service action: %s", *serviceFlag)
			}
		}
		return nil
	}

	// Interactive Console Menu: Launch Full TUI
	settings, _ := st.GetSettings()
	webPort := 2080
	if settings != nil && settings.WebPort > 0 {
		webPort = settings.WebPort
	}
	bridge := NewTUIBridge(dataDir, webPort)
	reader := bufio.NewReader(os.Stdin)
	return RunMainMenu(bridge, reader)
}
