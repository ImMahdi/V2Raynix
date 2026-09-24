# Admin CLI, Installer Engine, and Core Updater Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the terminal admin CLI (`v2raynix setup`), password visibility toggle on login, auto-dependency provisioning in `install.sh`, and backend/frontend core update management for Xray and tun2socks with safe atomic rollbacks.

**Architecture:** A standalone Go CLI module in `internal/cli` provides both interactive terminal menus and scriptable flags, auto-restarting the daemon to sync cache. A new `internal/updater` package queries GitHub releases with 6-hour caching, performs zero-downtime atomic binary replacements with `.bak` rollbacks, and exposes REST endpoints. The React frontend gets a login password toggle, an update badge in the header, an interactive update modal, and core status controls in Settings.

**Tech Stack:** Go 1.22+, `golang.org/x/crypto/bcrypt`, `net/http`, React 18, Vite, Tailwind/Vanilla CSS, Lucide React icons, Bash.

**Spec:** [`docs/superpowers/specs/2026-09-24-admin-cli-installer-and-core-updater-design.md`](file:///c:/MaadZone/Github%20Projects/V2Raynix/docs/superpowers/specs/2026-09-24-admin-cli-installer-and-core-updater-design.md)

## Global Constraints
- Target Go version: 1.22+
- Single binary deployment must be preserved (`web/dist` embedded into Go executable via `go:embed`).
- Existing user configurations (`v2raynix.json`), rules, and custom password hash must never be wiped or overwritten.
- Terminal CLI (`v2raynix setup`) must enforce root privileges (`id -u == 0`).
- Updates must NEVER mandate proxy tunnel activation. If download fails, report a clear error and allow the user to activate tunnel manually.
- Binary replacement on Linux must use `os.Rename` (`mv`) to avoid `ETXTBSY (Text file busy)` errors.
- GeoData assets (`geosite.dat`, `geoip.dat`) must always accompany `xray` in `/usr/local/share/xray` and `/usr/local/bin`.

## Review Focus
1. **Daemon Stale Memory vs Disk:** Modifying credentials or port via CLI while `v2raynix.service` is running must restart the service immediately so the daemon loads disk changes.
2. **Corrupted Binary Download:** If a core binary download is interrupted or corrupt, `--version` execution test must fail and automatically restore the `.bak` file before restarting the service.
3. **GitHub API Rate Limiting (HTTP 403):** When unauthenticated requests exceed 60 req/hour, the updater must log a warning and return cached versions rather than crashing or throwing unhandled errors in UI.
4. **Firewall Port Blocking on Clean VPS:** `install.sh` must check `ufw` and `firewalld` and explicitly allow the web port so clean installs are reachable immediately.
5. **Password Field Submission on Eye Click:** In `LoginPage.jsx`, the toggle button must have `type="button"` so clicking the eye icon does not accidentally submit the login form.

---

### Task 1: Terminal Admin Setup Subsystem (`internal/cli`)

**Files:**
- Create: `internal/cli/setup.go`
- Create: `internal/cli/setup_test.go`
- Modify: `cmd/v2raynix/main.go:50-95`

**Interfaces:**
- Produces: `RunSetup(dataDir string, args []string) error`
- Produces: `ApplyCredentials(st store.Store, username, password string) error`
- Produces: `ApplyWebPort(st store.Store, port int) error`

- [ ] **Step 1: Write the failing tests for CLI setup logic**

```go
// internal/cli/setup_test.go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"v2raynix/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func TestApplyCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	st := store.NewFileStore(tmpDir)

	err := ApplyCredentials(st, "newadmin", "securepass123")
	if err != nil {
		t.Fatalf("ApplyCredentials failed: %v", err)
	}

	user, err := st.GetAdminUser()
	if err != nil || user == nil {
		t.Fatalf("failed to get admin user: %v", err)
	}
	if user.Username != "newadmin" {
		t.Errorf("expected username 'newadmin', got '%s'", user.Username)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("securepass123")); err != nil {
		t.Errorf("password hash does not match plaintext: %v", err)
	}
}

func TestApplyWebPort(t *testing.T) {
	tmpDir := t.TempDir()
	st := store.NewFileStore(tmpDir)

	// Valid port
	if err := ApplyWebPort(st, 3050); err != nil {
		t.Fatalf("ApplyWebPort valid port failed: %v", err)
	}
	settings, err := st.GetSettings()
	if err != nil || settings.WebPort != 3050 {
		t.Errorf("expected web port 3050, got %d", settings.WebPort)
	}

	// Invalid ports
	if err := ApplyWebPort(st, 0); err == nil {
		t.Errorf("expected error for port 0, got nil")
	}
	if err := ApplyWebPort(st, 70000); err == nil {
		t.Errorf("expected error for port 70000, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/cli`
Expected: FAIL with undefined `ApplyCredentials` and `ApplyWebPort`.

- [ ] **Step 3: Implement `internal/cli/setup.go`**

```go
// internal/cli/setup.go
package cli

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"v2raynix/internal/store"
	"golang.org/x/crypto/bcrypt"
)

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

func RestartService() error {
	cmd := exec.Command("systemctl", "restart", "v2raynix")
	return cmd.Run()
}

func GetServiceStatus() (string, error) {
	out, err := exec.Command("systemctl", "is-active", "v2raynix").Output()
	status := strings.TrimSpace(string(out))
	if err != nil && status == "" {
		return "inactive", err
	}
	return status, nil
}

func RunSetup(dataDir string, args []string) error {
	st := store.NewFileStore(dataDir)

	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	userFlag := fs.String("user", "", "Set new admin username")
	passFlag := fs.String("pass", "", "Set new admin password")
	portFlag := fs.Int("port", 0, "Set new web panel port")
	serviceFlag := fs.String("service", "", "Service action: start|stop|restart|status")

	if err := fs.Parse(args); err != nil {
		return err
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

	// Interactive Menu
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
				fmt.Printf("[OK] Username and password updated. Restarting service...\n")
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
				fmt.Printf("[OK] Port updated to %d. Restarting service...\n", pVal)
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
```

- [ ] **Step 4: Connect `setup` subcommand in `cmd/v2raynix/main.go`**

```go
// In cmd/v2raynix/main.go:
if len(os.Args) > 1 && os.Args[1] == "setup" {
    dataDir := "/etc/v2raynix"
    if err := cli.RunSetup(dataDir, os.Args[2:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    os.Exit(0)
}
```

- [ ] **Step 5: Run tests and verify PASS**

Run: `go test -v ./internal/cli`
Expected: PASS with 100% test success.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/ cmd/v2raynix/main.go
git commit -m "feat(cli): add v2raynix setup interactive admin and scriptable subcommands"
```

---

### Task 2: Web Login Password Visibility Toggle

**Files:**
- Modify: `web/src/pages/LoginPage.jsx:10-50`

**Interfaces:**
- Produces: Show/Hide toggle on password input field with `Eye` / `EyeOff` icons.

- [ ] **Step 1: Edit `LoginPage.jsx` to introduce `showPassword` state and toggle**

```jsx
// In web/src/pages/LoginPage.jsx:
import { Eye, EyeOff } from 'lucide-react';

// Inside LoginPage component:
const [showPassword, setShowPassword] = useState(false);

// In the password input container:
<div className="relative">
  <input
    type={showPassword ? "text" : "password"}
    value={password}
    onChange={(e) => setPassword(e.target.value)}
    className="w-full bg-slate-900 border border-slate-700 rounded-lg px-4 py-2.5 pr-10 text-white focus:outline-none focus:border-cyan-500 transition-colors"
    placeholder="Enter password"
    required
  />
  <button
    type="button"
    onClick={() => setShowPassword(!showPassword)}
    className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200 transition-colors focus:outline-none"
    aria-label={showPassword ? "Hide password" : "Show password"}
  >
    {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
  </button>
</div>
```

- [ ] **Step 2: Run frontend build to verify syntax and assets**

Run: `npm --prefix web run build`
Expected: Build successfully created in `web/dist` with 0 errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/LoginPage.jsx
git commit -m "feat(web): add show/hide password toggle to login page"
```

---

### Task 3: Automated Dependency Engine & Firewall in `install.sh`

**Files:**
- Modify: `scripts/install.sh:30-80`

**Interfaces:**
- Produces: `ensure_xray()`, `ensure_tun2socks()`, `configure_firewall()`

- [ ] **Step 1: Update `scripts/install.sh` with architecture detection, mirrors, and firewall**

```bash
# In scripts/install.sh:

# Helper: download with mirror fallback
fetch_url() {
    local primary="$1"
    local mirror="https://ghproxy.net/${primary}"
    local dest="$2"

    echo -e "${CYAN}* Downloading ${primary}...${NC}"
    if curl -fsSL --connect-timeout 10 -m 60 "$primary" -o "$dest"; then
        return 0
    else
        echo -e "${YELLOW}Primary source failed, attempting mirror...${NC}"
        curl -fsSL --connect-timeout 15 -m 120 "$mirror" -o "$dest"
    fi
}

ensure_xray() {
    if command -v xray >/dev/null 2>&1; then
        echo -e "${GREEN}* Xray core is already installed: $(xray -version | head -n1)${NC}"
        return 0
    fi

    echo -e "${YELLOW}* Xray core not found. Installing latest official release...${NC}"
    mkdir -p /usr/local/share/xray
    local tmp_zip="/tmp/xray.zip"
    local asset_name=""

    case "$GOARCH" in
        amd64) asset_name="Xray-linux-64.zip" ;;
        arm64) asset_name="Xray-linux-arm64-v8a.zip" ;;
        *) echo -e "${RED}Unsupported arch for Xray: $GOARCH${NC}"; return 1 ;;
    esac

    local download_url="https://github.com/XTLS/Xray-core/releases/latest/download/${asset_name}"
    fetch_url "$download_url" "$tmp_zip"

    unzip -o -q "$tmp_zip" -d /tmp/xray_unpack
    cp -f /tmp/xray_unpack/xray /usr/local/bin/xray
    chmod +x /usr/local/bin/xray
    if [ -f "/tmp/xray_unpack/geosite.dat" ]; then
        cp -f /tmp/xray_unpack/geosite.dat /usr/local/share/xray/
        cp -f /tmp/xray_unpack/geosite.dat /usr/local/bin/
    fi
    if [ -f "/tmp/xray_unpack/geoip.dat" ]; then
        cp -f /tmp/xray_unpack/geoip.dat /usr/local/share/xray/
        cp -f /tmp/xray_unpack/geoip.dat /usr/local/bin/
    fi
    rm -rf "$tmp_zip" /tmp/xray_unpack
    echo -e "${GREEN}* Xray successfully installed: $(xray -version | head -n1)${NC}"
}

ensure_tun2socks() {
    if command -v tun2socks >/dev/null 2>&1; then
        echo -e "${GREEN}* tun2socks is already installed.${NC}"
        return 0
    fi

    echo -e "${YELLOW}* tun2socks not found. Installing latest release...${NC}"
    local tmp_zip="/tmp/tun2socks.zip"
    local asset_name="tun2socks-linux-${GOARCH}.zip"
    local download_url="https://github.com/xjasonlyu/tun2socks/releases/latest/download/${asset_name}"

    fetch_url "$download_url" "$tmp_zip"
    unzip -o -q "$tmp_zip" -d /tmp/tun2socks_unpack
    cp -f /tmp/tun2socks_unpack/tun2socks-linux* /usr/local/bin/tun2socks || cp -f /tmp/tun2socks_unpack/tun2socks /usr/local/bin/tun2socks
    chmod +x /usr/local/bin/tun2socks
    rm -rf "$tmp_zip" /tmp/tun2socks_unpack
    echo -e "${GREEN}* tun2socks successfully installed.${NC}"
}

configure_firewall() {
    local port="${1:-2080}"
    if command -v ufw >/dev/null 2>&1 && ufw status | grep -qw "active"; then
        echo -e "${CYAN}* Allowing port ${port} in UFW...${NC}"
        ufw allow "${port}/tcp" >/dev/null 2>&1 || true
    elif command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld; then
        echo -e "${CYAN}* Allowing port ${port} in firewalld...${NC}"
        firewall-cmd --add-port="${port}/tcp" --permanent >/dev/null 2>&1 || true
        firewall-cmd --reload >/dev/null 2>&1 || true
    fi
}
```

- [ ] **Step 2: Run bash syntax check**

Run: `bash -n scripts/install.sh`
Expected: Exits with code 0 (valid bash syntax).

- [ ] **Step 3: Commit**

```bash
git add scripts/install.sh
git commit -m "feat(installer): add automated dependency provisioning and firewall config"
```

---

### Task 4: Core Updater Subsystem (`internal/updater`)

**Files:**
- Create: `internal/updater/updater.go`
- Create: `internal/updater/updater_test.go`

**Interfaces:**
- Produces: `NewUpdater(dataDir string) *Updater`
- Produces: `GetStatus() UpdateStatus`
- Produces: `CheckUpdates(force bool) (UpdateStatus, error)`
- Produces: `UpdateCore(coreName string) error`

- [ ] **Step 1: Write failing unit tests for version comparison and parsing**

```go
// internal/updater/updater_test.go
package updater

import (
	"testing"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
	}{
		{"1.8.24", "25.1.30", true},
		{"v25.1.0", "v25.1.30", true},
		{"2.5.2", "2.5.2", false},
		{"2.6.0", "2.5.2", false},
		{"", "1.0.0", true},
		{"1.0.0", "", false},
	}

	for _, tt := range tests {
		got := isNewerVersion(tt.current, tt.latest)
		if got != tt.expected {
			t.Errorf("isNewerVersion(%q, %q) = %v; want %v", tt.current, tt.latest, got, tt.expected)
		}
	}
}

func TestCleanVersion(t *testing.T) {
	if v := cleanVersion("v25.1.30"); v != "25.1.30" {
		t.Errorf("expected 25.1.30, got %s", v)
	}
	if v := cleanVersion("Xray 1.8.24 (Xray, Penetrates Everything.)"); v != "1.8.24" {
		t.Errorf("expected 1.8.24, got %s", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/updater`
Expected: FAIL with undefined types.

- [ ] **Step 3: Implement `internal/updater/updater.go`**

```go
// internal/updater/updater.go
package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CoreInfo struct {
	Name            string `json:"name"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	BinaryPath      string `json:"binary_path"`
	ReleaseURL      string `json:"release_url"`
}

type UpdateStatus struct {
	Cores         map[string]CoreInfo `json:"cores"`
	LastCheckedAt string              `json:"last_checked_at"`
	IsUpdating    bool                `json:"is_updating"`
	LastLog       string              `json:"last_log"`
}

type Updater struct {
	dataDir    string
	status     UpdateStatus
	lastCheck  time.Time
	mu         sync.RWMutex
	httpClient *http.Client
}

func NewUpdater(dataDir string) *Updater {
	return &Updater{
		dataDir: dataDir,
		status: UpdateStatus{
			Cores: make(map[string]CoreInfo),
		},
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func cleanVersion(raw string) string {
	re := regexp.MustCompile(`(\d+\.\d+(\.\d+)?)`)
	match := re.FindString(raw)
	return strings.TrimSpace(match)
}

func isNewerVersion(current, latest string) bool {
	c := cleanVersion(current)
	l := cleanVersion(latest)
	if c == "" && l != "" {
		return true
	}
	if l == "" {
		return false
	}

	cParts := strings.Split(c, ".")
	lParts := strings.Split(l, ".")

	for i := 0; i < len(cParts) && i < len(lParts); i++ {
		cV, _ := strconv.Atoi(cParts[i])
		lV, _ := strconv.Atoi(lParts[i])
		if lV > cV {
			return true
		}
		if lV < cV {
			return false
		}
	}
	return len(lParts) > len(cParts)
}

func (u *Updater) GetInstalledVersion(core string) string {
	switch core {
	case "xray":
		out, err := exec.Command("xray", "-version").Output()
		if err == nil {
			return cleanVersion(string(out))
		}
	case "tun2socks":
		out, err := exec.Command("tun2socks", "-v").Output()
		if err == nil {
			return cleanVersion(string(out))
		}
		out, err = exec.Command("tun2socks", "-version").Output()
		if err == nil {
			return cleanVersion(string(out))
		}
	}
	return ""
}

func (u *Updater) fetchLatestGitHubTag(repo string) (string, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "V2Raynix-Updater")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		// Fallback to mirror
		mirrorURL := fmt.Sprintf("https://ghproxy.net/%s", url)
		mReq, _ := http.NewRequest("GET", mirrorURL, nil)
		mReq.Header.Set("User-Agent", "V2Raynix-Updater")
		resp, err = u.httpClient.Do(mReq)
		if err != nil {
			return "", "", fmt.Errorf("failed to fetch release info: %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return "", "", fmt.Errorf("github api rate limit exceeded")
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var data struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", err
	}
	return cleanVersion(data.TagName), data.HTMLURL, nil
}

func (u *Updater) CheckUpdates(force bool) (UpdateStatus, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !force && time.Since(u.lastCheck) < 6*time.Hour && len(u.status.Cores) > 0 {
		return u.status, nil
	}

	xCurrent := u.GetInstalledVersion("xray")
	tCurrent := u.GetInstalledVersion("tun2socks")

	xLatest, xURL, _ := u.fetchLatestGitHubTag("XTLS/Xray-core")
	tLatest, tURL, _ := u.fetchLatestGitHubTag("xjasonlyu/tun2socks")

	u.status.Cores["xray"] = CoreInfo{
		Name:            "xray",
		CurrentVersion:  xCurrent,
		LatestVersion:   xLatest,
		UpdateAvailable: isNewerVersion(xCurrent, xLatest),
		BinaryPath:      "/usr/local/bin/xray",
		ReleaseURL:      xURL,
	}

	u.status.Cores["tun2socks"] = CoreInfo{
		Name:            "tun2socks",
		CurrentVersion:  tCurrent,
		LatestVersion:   tLatest,
		UpdateAvailable: isNewerVersion(tCurrent, tLatest),
		BinaryPath:      "/usr/local/bin/tun2socks",
		ReleaseURL:      tURL,
	}

	u.lastCheck = time.Now()
	u.status.LastCheckedAt = u.lastCheck.UTC().Format(time.RFC3339)
	return u.status, nil
}

func (u *Updater) GetStatus() UpdateStatus {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.status
}
```

- [ ] **Step 4: Run tests and verify PASS**

Run: `go test -v ./internal/updater`
Expected: PASS with 100% test success.

- [ ] **Step 5: Commit**

```bash
git add internal/updater/
git commit -m "feat(updater): implement core version detection, semver comparison, and github releases checker"
```

---

### Task 5: Core Updater API Endpoints & Safe Update Action

**Files:**
- Modify: `internal/api/router.go:50-250`
- Modify: `internal/api/api_test.go:150-250`

**Interfaces:**
- Produces: `GET /api/system/updates`
- Produces: `POST /api/system/check-updates`
- Produces: `POST /api/system/update-core`

- [ ] **Step 1: Write integration tests in `api_test.go`**

```go
// In internal/api/api_test.go:
func TestAPI_SystemUpdateEndpoints(t *testing.T) {
    ts, cleanup := setupTestServer(t)
    defer cleanup()

    token := getAdminToken(t, ts)

    // GET /api/system/updates
    req, _ := http.NewRequest("GET", ts.URL+"/api/system/updates", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    resp, err := http.DefaultClient.Do(req)
    if err != nil || resp.StatusCode != http.StatusOK {
        t.Fatalf("GET /api/system/updates failed: %v, status: %d", err, resp.StatusCode)
    }

    // POST /api/system/check-updates
    req, _ = http.NewRequest("POST", ts.URL+"/api/system/check-updates", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    resp, err = http.DefaultClient.Do(req)
    if err != nil || resp.StatusCode != http.StatusOK {
        t.Fatalf("POST /api/system/check-updates failed: %v, status: %d", err, resp.StatusCode)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api -run TestAPI_SystemUpdateEndpoints`
Expected: FAIL (404 Not Found).

- [ ] **Step 3: Register updater endpoints in `internal/api/router.go`**

```go
// In internal/api/router.go:
// Add updater instance to Server struct
// In registerRoutes():
protected.HandleFunc("/system/updates", s.handleGetUpdates).Methods("GET")
protected.HandleFunc("/system/check-updates", s.handleCheckUpdates).Methods("POST")
protected.HandleFunc("/system/update-core", s.handleUpdateCore).Methods("POST")
```

- [ ] **Step 4: Run tests to verify PASS**

Run: `go test -v ./internal/api -run TestAPI_SystemUpdateEndpoints`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat(api): expose core update status and trigger endpoints"
```

---

### Task 6: Web Management Panel Core Update UI & Modal

**Files:**
- Modify: `web/src/services/api.js:40-90`
- Create: `web/src/components/UpdateCoreModal.jsx`
- Modify: `web/src/components/Header.jsx:10-70`
- Modify: `web/src/pages/SettingsPage.jsx:60-120`
- Modify: `web/src/App.jsx:30-80`

**Interfaces:**
- Produces: `api.getSystemUpdates()`, `api.checkSystemUpdates()`, `api.updateCore(core)`
- Produces: Header neon pill `⚡ Core Update Available`
- Produces: `UpdateCoreModal` with version diffs and live progress

- [ ] **Step 1: Add update methods to `web/src/services/api.js`**

```js
// In web/src/services/api.js:
export async function getSystemUpdates() {
  const res = await fetch(`${API_BASE}/system/updates`, { credentials: 'include' });
  return handleResponse(res);
}

export async function checkSystemUpdates() {
  const res = await fetch(`${API_BASE}/system/check-updates`, {
    method: 'POST',
    credentials: 'include'
  });
  return handleResponse(res);
}

export async function updateCore(core) {
  const res = await fetch(`${API_BASE}/system/update-core`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ core }),
    credentials: 'include'
  });
  return handleResponse(res);
}
```

- [ ] **Step 2: Create `UpdateCoreModal.jsx`**

```jsx
// web/src/components/UpdateCoreModal.jsx
import React, { useState } from 'react';
import { RefreshCw, CheckCircle, AlertTriangle, X, Download } from 'lucide-react';

export default function UpdateCoreModal({ isOpen, onClose, updateData, onRefresh }) {
  const [updatingCore, setUpdatingCore] = useState(null);
  const [logMessage, setLogMessage] = useState('');
  const [error, setError] = useState(null);

  if (!isOpen || !updateData) return null;

  const cores = Object.values(updateData.cores || {});

  const handleUpdate = async (coreName) => {
    setUpdatingCore(coreName);
    setError(null);
    setLogMessage(`Downloading and verifying ${coreName}...`);
    try {
      // Call update API
      await onRefresh(coreName);
      setLogMessage(`${coreName} updated successfully!`);
    } catch (err) {
      setError(err.message || 'Update failed');
    } finally {
      setUpdatingCore(null);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg p-6 shadow-2xl relative">
        <button
          onClick={onClose}
          className="absolute top-4 right-4 text-slate-400 hover:text-white"
        >
          <X size={20} />
        </button>

        <h3 className="text-xl font-bold text-white mb-2 flex items-center gap-2">
          <Download className="text-cyan-400" size={22} />
          Core Engine Updates
        </h3>
        <p className="text-sm text-slate-400 mb-6">
          Upgrade proxy and tunneling cores to the latest official stable releases.
        </p>

        <div className="space-y-4">
          {cores.map((c) => (
            <div
              key={c.name}
              className="bg-slate-800/50 border border-slate-700/50 rounded-xl p-4 flex items-center justify-between"
            >
              <div>
                <div className="font-semibold text-white capitalize">{c.name}</div>
                <div className="text-xs text-slate-400 mt-1 flex items-center gap-2">
                  <span>Current: <strong className="text-slate-200">{c.current_version || 'Not installed'}</strong></span>
                  <span>→</span>
                  <span>Latest: <strong className="text-cyan-400">{c.latest_version || 'Checking...'}</strong></span>
                </div>
              </div>

              {c.update_available ? (
                <button
                  disabled={updatingCore !== null}
                  onClick={() => handleUpdate(c.name)}
                  className="px-3.5 py-1.5 bg-cyan-500 hover:bg-cyan-400 text-slate-950 font-medium text-xs rounded-lg transition-all flex items-center gap-1.5 shadow-md shadow-cyan-500/20 disabled:opacity-50"
                >
                  {updatingCore === c.name ? (
                    <RefreshCw className="animate-spin" size={14} />
                  ) : (
                    <Download size={14} />
                  )}
                  Update
                </button>
              ) : (
                <span className="text-xs text-emerald-400 flex items-center gap-1">
                  <CheckCircle size={14} /> Up to date
                </span>
              )}
            </div>
          ))}
        </div>

        {logMessage && (
          <div className="mt-4 p-3 bg-slate-950/60 border border-slate-800 rounded-lg text-xs text-slate-300 font-mono">
            {logMessage}
          </div>
        )}

        {error && (
          <div className="mt-4 p-3 bg-rose-950/40 border border-rose-800/60 rounded-lg text-xs text-rose-300 flex items-center gap-2">
            <AlertTriangle size={14} className="shrink-0" />
            {error}
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Update `Header.jsx` to show update badge pill**

```jsx
// In web/src/components/Header.jsx:
{hasUpdate && (
  <button
    onClick={() => setIsUpdateModalOpen(true)}
    className="px-2.5 py-1 bg-cyan-500/10 border border-cyan-500/40 text-cyan-400 hover:bg-cyan-500/20 text-xs font-medium rounded-full transition-all flex items-center gap-1.5 animate-pulse"
    title="Core updates available"
  >
    <span className="w-1.5 h-1.5 rounded-full bg-cyan-400"></span>
    Core Update Available
  </button>
)}
```

- [ ] **Step 4: Run frontend build to verify syntax and assets**

Run: `npm --prefix web run build`
Expected: Build successfully created in `web/dist` with 0 errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/
git commit -m "feat(web): add core update pill badge, modal, and settings integration"
```

---

### Task 7: Full Regression, Linux Release Build, & Server Deployment Verification

**Files:**
- Compile: `v2raynix-linux`
- Deploy: Server `192.168.254.80:23313`

- [ ] **Step 1: Run full Go test suite**

Run: `go test -count=1 ./...`
Expected: PASS across all packages.

- [ ] **Step 2: Compile Linux release binary**

Run: `GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o v2raynix-linux ./cmd/v2raynix`
Expected: Binary `v2raynix-linux` compiled cleanly.

- [ ] **Step 3: Deploy to test server and verify**

Deploy `v2raynix-linux` to `/usr/local/bin/v2raynix` on `192.168.254.80`.
Verify `v2raynix setup --service status` returns active.
Verify login screen contains password visibility toggle.
Verify `GET /api/system/updates` returns core version data.

- [ ] **Step 4: Update Knowledge Graph**

Run: `graphify update .`

- [ ] **Step 5: Commit and tag release**

```bash
git add -A
git commit -m "release: admin cli setup, installer dependency engine, and core updater"
```
