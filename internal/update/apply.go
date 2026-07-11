package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	updateHelperPath = "/usr/local/sbin/boomerang-update"
	updateDirName    = ".update"
	upgradeHint      = "run: curl -fsSL https://raw.githubusercontent.com/supermaribo/boomerang/main/deploy/upgrade.sh | sudo bash"
)

func UpgradeHint() string { return upgradeHint }

func CanApply() bool {
	ok, _ := ApplyCapability()
	return ok
}

func ApplyCapability() (bool, string) {
	st, err := os.Stat(updateHelperPath)
	if err != nil || !st.Mode().IsRegular() {
		return false, "update helper is not installed — " + upgradeHint
	}
	if _, err := os.Stat("/etc/sudoers.d/boomerang-update"); err != nil {
		return false, "passwordless sudo for updates is not configured — " + upgradeHint
	}
	cmd := exec.Command("sudo", "-n", updateHelperPath, "--check")
	if err := cmd.Run(); err != nil {
		return false, "the boomerang service cannot run passwordless sudo — " + upgradeHint
	}
	return true, ""
}

func DownloadAsset(dataDir, assetURL, assetName string) (string, error) {
	if strings.TrimSpace(assetURL) == "" {
		return "", fmt.Errorf("missing download URL")
	}
	dir := filepath.Join(dataDir, updateDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, assetName)
	tmp := dest + ".part"
	_ = os.Remove(tmp)

	req, err := http.NewRequest(http.MethodGet, assetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "boomerang-appliance")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("download HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return "", err
	}
	n, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	if n < 1024*1024 {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("downloaded file looks too small (%d bytes)", n)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := verifyReleaseChecksum(dest, assetURL, assetName); err != nil {
		_ = os.Remove(dest)
		return "", err
	}
	return dest, nil
}

func verifyReleaseChecksum(path, assetURL, assetName string) error {
	sumsURL := checksumURL(assetURL)
	if sumsURL == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodGet, sumsURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "boomerang-appliance")
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil
	}
	expected, ok := ExpectedSHA256FromSums(string(raw), assetName)
	if !ok {
		return nil
	}
	return VerifyFileSHA256(path, expected)
}

func checksumURL(assetURL string) string {
	if i := strings.LastIndex(assetURL, "/"); i >= 0 {
		return assetURL[:i+1] + "SHA256SUMS"
	}
	return ""
}

func ApplyDownloaded(path string) error {
	if !CanApply() {
		return fmt.Errorf("in-place updates are not configured on this host (%s)", upgradeHint)
	}
	cmd := exec.Command("sudo", "-n", updateHelperPath, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("install update: %s", msg)
	}
	return nil
}
