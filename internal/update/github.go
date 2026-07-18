package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

const (
	repoOwner = "supermaribo"
	repoName  = "boomerang"
)

type Release struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	HTMLURL     string  `json:"html_url"`
	Body        string  `json:"body"`
	PublishedAt string  `json:"published_at"`
	Assets      []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type CheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl,omitempty"`
	ReleaseNotes    string `json:"releaseNotes,omitempty"`
	PublishedAt     string `json:"publishedAt,omitempty"`
	AssetName       string `json:"assetName,omitempty"`
	AssetURL        string `json:"assetUrl,omitempty"`
	AssetBytes      int64  `json:"assetBytes,omitempty"`
	CanApply        bool   `json:"canApply"`
	CanApplyReason  string `json:"canApplyReason,omitempty"`
	CheckError      string `json:"checkError,omitempty"`
}

func Check(current string, canApply bool) CheckResult {
	return CheckWithReason(current, canApply, "")
}

func CheckWithReason(current string, canApply bool, canApplyReason string) CheckResult {
	out := CheckResult{
		CurrentVersion: displayVersion(current),
		CanApply:       canApply,
		CanApplyReason: canApplyReason,
	}
	rel, err := fetchLatestRelease()
	if err != nil {
		out.CheckError = err.Error()
		return out
	}
	latest := normalizeTag(rel.TagName)
	out.LatestVersion = displayVersion(latest)
	out.ReleaseURL = rel.HTMLURL
	out.ReleaseNotes = strings.TrimSpace(rel.Body)
	out.PublishedAt = rel.PublishedAt
	asset, err := pickAsset(rel.Assets)
	if err != nil {
		out.CheckError = err.Error()
		return out
	}
	out.AssetName = asset.Name
	out.AssetURL = asset.BrowserDownloadURL
	out.AssetBytes = asset.Size
	out.UpdateAvailable = isNewer(current, latest)
	return out
}

func fetchLatestRelease() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "boomerang-appliance")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases published yet on GitHub")
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github API %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return nil, fmt.Errorf("release has no tag")
	}
	return &rel, nil
}

func pickAsset(assets []Asset) (*Asset, error) {
	want := fmt.Sprintf("boomerang-linux-%s", runtime.GOARCH)
	for i := range assets {
		if assets[i].Name == want {
			return &assets[i], nil
		}
	}
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.Contains(name, "linux") && strings.Contains(name, runtime.GOARCH) && !strings.HasSuffix(name, ".sha256") {
			return &assets[i], nil
		}
	}
	return nil, fmt.Errorf("no release asset for linux/%s (expected %q)", runtime.GOARCH, want)
}

func isNewer(current, latest string) bool {
	cur := semverTag(current)
	lat := semverTag(latest)
	if lat == "" {
		return false
	}
	if cur == "" {
		return true
	}
	return semver.Compare(cur, lat) < 0
}

// ClientUpdateAvailable reports whether a known agent version is behind latest.
// Empty or "dev" agent versions return false (unknown, not outdated).
func ClientUpdateAvailable(clientVersion, latest string) bool {
	cur := semverTag(clientVersion)
	lat := semverTag(latest)
	if cur == "" || lat == "" {
		return false
	}
	return semver.Compare(cur, lat) < 0
}

// IsNewer reports whether latest is a newer semver than current (appliance check).
func IsNewer(current, latest string) bool {
	return isNewer(current, latest)
}

func semverTag(v string) string {
	v = normalizeTag(v)
	if v == "" || v == "dev" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ""
	}
	return v
}

func normalizeTag(tag string) string {
	return strings.TrimPrefix(strings.TrimSpace(tag), "v")
}

func displayVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "dev"
	}
	return strings.TrimPrefix(v, "v")
}

var (
	releaseCacheMu  sync.Mutex
	releaseCacheAt  time.Time
	releaseCacheTag string
	releaseCacheErr error
	releaseCacheTTL = time.Hour
)

// LatestTagCached returns the latest GitHub release tag (without leading v),
// caching successful and failed lookups for about an hour.
func LatestTagCached() (string, error) {
	releaseCacheMu.Lock()
	defer releaseCacheMu.Unlock()
	if time.Since(releaseCacheAt) < releaseCacheTTL && (releaseCacheTag != "" || releaseCacheErr != nil) {
		return releaseCacheTag, releaseCacheErr
	}
	rel, err := fetchLatestRelease()
	releaseCacheAt = time.Now()
	if err != nil {
		releaseCacheTag = ""
		releaseCacheErr = err
		return "", err
	}
	releaseCacheTag = displayVersion(rel.TagName)
	releaseCacheErr = nil
	return releaseCacheTag, nil
}
