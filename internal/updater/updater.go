package updater

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
		httpClient: &http.Client{Timeout: 15 * time.Second},
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

	maxLen := len(cParts)
	if len(lParts) > maxLen {
		maxLen = len(lParts)
	}

	for i := 0; i < maxLen; i++ {
		cV := 0
		if i < len(cParts) {
			cV, _ = strconv.Atoi(cParts[i])
		}
		lV := 0
		if i < len(lParts) {
			lV, _ = strconv.Atoi(lParts[i])
		}

		if lV > cV {
			return true
		}
		if lV < cV {
			return false
		}
	}
	return false
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

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		// Try mirror
		mirrorURL := "https://ghproxy.net/" + url
		resp, err = client.Get(mirrorURL)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func unzipExtractFile(zipPath, targetName, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, targetName) || strings.EqualFold(f.Name, targetName) {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, rc)
			return err
		}
	}
	return fmt.Errorf("file %s not found in zip archive", targetName)
}

var renameFn = os.Rename

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func atomicMove(src, dst string) error {
	err := renameFn(src, dst)
	if err == nil {
		return nil
	}

	// Check for cross-device link error (EXDEV)
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		// Fallback: copy to temp file on destination directory then rename
		tmpDst := dst + ".tmp." + strconv.FormatInt(time.Now().UnixNano(), 10)
		if copyErr := copyFile(src, tmpDst); copyErr != nil {
			_ = os.Remove(tmpDst)
			return fmt.Errorf("cross-device copy failed: %w", copyErr)
		}
		_ = os.Chmod(tmpDst, 0755)
		if renErr := os.Rename(tmpDst, dst); renErr != nil {
			_ = os.Remove(tmpDst)
			return fmt.Errorf("cross-device final rename failed: %w", renErr)
		}
		_ = os.Remove(src)
		return nil
	}
	return err
}

func (u *Updater) UpdateCore(coreName string) error {
	u.mu.Lock()
	if u.status.IsUpdating {
		u.mu.Unlock()
		return fmt.Errorf("an update is already in progress")
	}
	u.status.IsUpdating = true
	u.mu.Unlock()

	defer func() {
		u.mu.Lock()
		u.status.IsUpdating = false
		u.mu.Unlock()
	}()

	arch := runtime.GOARCH
	switch coreName {
	case "xray":
		return u.updateXray(arch)
	case "tun2socks":
		return u.updateTun2socks(arch)
	case "all":
		if err := u.updateXray(arch); err != nil {
			return err
		}
		return u.updateTun2socks(arch)
	default:
		return fmt.Errorf("unknown core: %s", coreName)
	}
}

func (u *Updater) updateXray(arch string) error {
	var assetName string
	switch arch {
	case "amd64":
		assetName = "Xray-linux-64.zip"
	case "arm64":
		assetName = "Xray-linux-arm64-v8a.zip"
	default:
		return fmt.Errorf("unsupported arch for xray: %s", arch)
	}

	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/latest/download/%s", assetName)
	tmpZip := filepath.Join(os.TempDir(), "xray-update.zip")
	defer os.Remove(tmpZip)

	if err := downloadFile(url, tmpZip); err != nil {
		return fmt.Errorf("downloading xray failed: %w. Please check internet connection or activate tunnel and try again", err)
	}

	tmpBin := filepath.Join(os.TempDir(), "xray.new")
	defer os.Remove(tmpBin)
	if err := unzipExtractFile(tmpZip, "xray", tmpBin); err != nil {
		return fmt.Errorf("extracting xray binary failed: %w", err)
	}
	_ = os.Chmod(tmpBin, 0755)

	// Standalone verification
	testCmd := exec.Command(tmpBin, "-version")
	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("verification of new xray binary failed: %w", err)
	}

	// Backup current
	targetBin := "/usr/local/bin/xray"
	backupBin := targetBin + ".bak"
	if _, err := os.Stat(targetBin); err == nil {
		_ = atomicMove(targetBin, backupBin)
	}

	// Atomic rename swap
	if err := atomicMove(tmpBin, targetBin); err != nil {
		// Rollback if failed
		if _, bErr := os.Stat(backupBin); bErr == nil {
			_ = atomicMove(backupBin, targetBin)
		}
		return fmt.Errorf("failed to swap xray binary: %w", err)
	}

	// Also extract geo assets if present
	_ = unzipExtractFile(tmpZip, "geosite.dat", "/usr/local/share/xray/geosite.dat")
	_ = unzipExtractFile(tmpZip, "geoip.dat", "/usr/local/share/xray/geoip.dat")

	// Verify installed binary
	if err := exec.Command(targetBin, "-version").Run(); err != nil {
		// Automated Rollback
		_ = atomicMove(backupBin, targetBin)
		return fmt.Errorf("xray crashed after swap, rolled back: %w", err)
	}

	_ = os.Remove(backupBin)
	_, _ = u.CheckUpdates(true)
	return nil
}

func (u *Updater) updateTun2socks(arch string) error {
	assetName := fmt.Sprintf("tun2socks-linux-%s.zip", arch)
	url := fmt.Sprintf("https://github.com/xjasonlyu/tun2socks/releases/latest/download/%s", assetName)
	tmpZip := filepath.Join(os.TempDir(), "tun2socks-update.zip")
	defer os.Remove(tmpZip)

	if err := downloadFile(url, tmpZip); err != nil {
		return fmt.Errorf("downloading tun2socks failed: %w. Please check internet connection or activate tunnel and try again", err)
	}

	tmpBin := filepath.Join(os.TempDir(), "tun2socks.new")
	defer os.Remove(tmpBin)
	targetExtractName := fmt.Sprintf("tun2socks-linux-%s", arch)
	if err := unzipExtractFile(tmpZip, targetExtractName, tmpBin); err != nil {
		if err := unzipExtractFile(tmpZip, "tun2socks", tmpBin); err != nil {
			return fmt.Errorf("extracting tun2socks failed: %w", err)
		}
	}
	_ = os.Chmod(tmpBin, 0755)

	targetBin := "/usr/local/bin/tun2socks"
	backupBin := targetBin + ".bak"
	if _, err := os.Stat(targetBin); err == nil {
		_ = atomicMove(targetBin, backupBin)
	}

	// Atomic swap
	if err := atomicMove(tmpBin, targetBin); err != nil {
		if _, bErr := os.Stat(backupBin); bErr == nil {
			_ = atomicMove(backupBin, targetBin)
		}
		return fmt.Errorf("failed to swap tun2socks binary: %w", err)
	}

	_ = os.Remove(backupBin)
	_, _ = u.CheckUpdates(true)
	return nil
}
