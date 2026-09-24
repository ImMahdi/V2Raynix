package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/v2raynix/v2raynix/internal/store"
	"golang.org/x/crypto/bcrypt"
)

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

	// Interactive Console Menu
	reader := bufio.NewReader(os.Stdin)
	for {
		settings, _ := st.GetSettings()
		webPort := 2080
		if settings != nil && settings.WebPort > 0 {
			webPort = settings.WebPort
		}
		status, _ := GetServiceStatus()

		fmt.Println("\n======================================================")
		fmt.Println("             🛡️  V2Raynix Server Setup")
		fmt.Println("======================================================")
		fmt.Printf(" Service Status: %s | Web Port: %d\n", status, webPort)
		fmt.Println("------------------------------------------------------")
		fmt.Println(" [1] Change / Reset Admin Username & Password")
		fmt.Println(" [2] Change Web Panel Port")
		fmt.Println(" [3] Service Management (Restart / Stop / Start)")
		fmt.Println(" [4] View Status & Web Access URL")
		fmt.Println(" [0] Exit")
		fmt.Println("======================================================")
		fmt.Print("Please select an option [0-4]: ")

		choiceStr, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(choiceStr)

		switch choice {
		case "1":
			fmt.Print("Enter new username [admin]: ")
			u, _ := reader.ReadString('\n')
			u = strings.TrimSpace(u)
			if u == "" {
				u = "admin"
			}
			fmt.Print("Enter new password (min 4 chars): ")
			p, _ := reader.ReadString('\n')
			p = strings.TrimSpace(p)
			if err := ApplyCredentials(st, u, p); err != nil {
				fmt.Printf("[ERROR] %v\n", err)
			} else {
				fmt.Printf("[OK] Username and password updated. Restarting service to sync memory...\n")
				_ = RestartService()
			}
		case "2":
			fmt.Printf("Enter new port (1-65535) [current %d]: ", webPort)
			pStr, _ := reader.ReadString('\n')
			pVal, err := strconv.Atoi(strings.TrimSpace(pStr))
			if err != nil {
				fmt.Println("[ERROR] Invalid port number")
				continue
			}
			if err := ApplyWebPort(st, pVal); err != nil {
				fmt.Printf("[ERROR] %v\n", err)
			} else {
				fmt.Printf("[OK] Port updated to %d. Restarting service to apply...\n", pVal)
				_ = RestartService()
			}
		case "3":
			fmt.Println(" [1] Restart Service")
			fmt.Println(" [2] Stop Service")
			fmt.Println(" [3] Start Service")
			fmt.Print("Select [1-3]: ")
			sChoice, _ := reader.ReadString('\n')
			var act string
			switch strings.TrimSpace(sChoice) {
			case "1":
				act = "restart"
			case "2":
				act = "stop"
			case "3":
				act = "start"
			default:
				continue
			}
			_ = exec.Command("systemctl", act, "v2raynix").Run()
			fmt.Printf("[OK] Service %sed.\n", act)
		case "4":
			fmt.Printf("\nPanel URL: http://<your-server-ip>:%d\n", webPort)
			fmt.Printf("Service Status: %s\n", status)
		case "0":
			return nil
		default:
			fmt.Println("Invalid choice. Try again.")
		}
	}
}
