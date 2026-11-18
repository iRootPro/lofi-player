// Package updatecheck checks the public GitHub release feed without blocking
// the player. It stores only release metadata in the XDG cache; no account,
// station, listening, or machine-identifying data is sent or retained.
package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	LatestReleaseURL = "https://api.github.com/repos/iRootPro/lofi-player/releases/latest"
	DefaultInterval  = 24 * time.Hour
	DefaultTimeout   = 3 * time.Second
)

// ErrNonReleaseBuild means the current binary is a dev, snapshot, or
// prerelease build. Only stable released binaries check the network.
var ErrNonReleaseBuild = errors.New("non-release builds do not check for updates")

// Result is safe, public release metadata suitable for a UI notice or CLI.
type Result struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseURL     string
	Available      bool
}

// Cache is the small, disposable XDG cache document. CheckedAt is updated
// after every attempt, including a failure, so an offline launch never retries
// on every startup.
type Cache struct {
	CheckedAt     time.Time `json:"checked_at"`
	ETag          string    `json:"etag,omitempty"`
	LatestVersion string    `json:"latest_version,omitempty"`
	ReleaseURL    string    `json:"release_url,omitempty"`
}

// Client performs update checks. BaseURL, HTTPClient and Now make it entirely
// deterministic under test.
type Client struct {
	CurrentVersion string
	BaseURL        string
	CachePath      string
	HTTPClient     *http.Client
	Now            func() time.Time
	Interval       time.Duration
	Timeout        time.Duration
}

// New returns a client with privacy-preserving defaults suitable for both the
// interactive TUI and --check-update.
func New(currentVersion string) *Client {
	return &Client{CurrentVersion: currentVersion}
}

// Check uses cached metadata until its interval expires. It issues an ETag
// conditional request after that interval.
func (c *Client) Check(ctx context.Context) (Result, error) {
	return c.check(ctx, false)
}

// CheckNow bypasses a fresh cache entry. It is used by the explicit CLI flag;
// the interactive TUI should use Check so ordinary launches stay quiet.
func (c *Client) CheckNow(ctx context.Context) (Result, error) {
	return c.check(ctx, true)
}

func (c *Client) check(ctx context.Context, force bool) (Result, error) {
	current, ok := parseVersion(c.CurrentVersion)
	if !ok || current.pre != "" {
		return Result{CurrentVersion: c.CurrentVersion}, ErrNonReleaseBuild
	}

	now := c.now().UTC()
	cache := loadCache(c.cachePath())
	if !force && !cache.CheckedAt.IsZero() && now.Sub(cache.CheckedAt) < c.interval() {
		return c.result(current, cache), nil
	}

	requestCtx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, c.baseURL(), nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lofi-player/"+c.CurrentVersion)
	if cache.ETag != "" {
		req.Header.Set("If-None-Match", cache.ETag)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		cache.CheckedAt = now
		_ = saveCache(c.cachePath(), cache)
		return Result{}, err
	}
	defer resp.Body.Close()

	cache.CheckedAt = now
	if resp.StatusCode == http.StatusNotModified {
		if err := saveCache(c.cachePath(), cache); err != nil {
			return Result{}, err
		}
		return c.result(current, cache), nil
	}
	if resp.StatusCode != http.StatusOK {
		_ = saveCache(c.cachePath(), cache)
		return Result{}, fmt.Errorf("latest release: unexpected HTTP status %s", resp.Status)
	}

	var release struct {
		TagName    string `json:"tag_name"`
		HTMLURL    string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	if err := decoder.Decode(&release); err != nil {
		_ = saveCache(c.cachePath(), cache)
		return Result{}, fmt.Errorf("decode latest release: %w", err)
	}
	if release.Draft || release.Prerelease {
		_ = saveCache(c.cachePath(), cache)
		return Result{}, errors.New("latest release is not stable")
	}
	if _, ok := parseVersion(release.TagName); !ok {
		_ = saveCache(c.cachePath(), cache)
		return Result{}, fmt.Errorf("latest release has invalid version %q", release.TagName)
	}
	cache.ETag = resp.Header.Get("ETag")
	cache.LatestVersion = release.TagName
	cache.ReleaseURL = release.HTMLURL
	if err := saveCache(c.cachePath(), cache); err != nil {
		return Result{}, err
	}
	return c.result(current, cache), nil
}

func (c *Client) result(current version, cache Cache) Result {
	latest, ok := parseVersion(cache.LatestVersion)
	return Result{
		CurrentVersion: c.CurrentVersion,
		LatestVersion:  cache.LatestVersion,
		ReleaseURL:     cache.ReleaseURL,
		Available:      ok && latest.compare(current) > 0,
	}
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return LatestReleaseURL
}

func (c *Client) cachePath() string {
	if c.CachePath != "" {
		return c.CachePath
	}
	p, err := Path()
	if err != nil {
		return ""
	}
	return p
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	client := *http.DefaultClient
	// A release check must not carry even its version User-Agent to a redirect
	// target outside the documented GitHub API endpoint.
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &client
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Client) interval() time.Duration {
	if c.Interval > 0 {
		return c.Interval
	}
	return DefaultInterval
}

func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultTimeout
}

// Path returns a disposable cache location shared by macOS and Linux.
func Path() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "lofi-player", "update.json"), nil
}

func loadCache(path string) Cache {
	if path == "" {
		return Cache{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Cache{}
	}
	var cache Cache
	if json.Unmarshal(data, &cache) != nil {
		return Cache{}
	}
	return cache
}

func saveCache(path string, cache Cache) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".update-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

type version struct {
	major, minor, patch int
	pre                 string
}

func parseVersion(raw string) (version, bool) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "v")
	withoutBuild := strings.SplitN(raw, "+", 2)
	if len(withoutBuild) == 2 {
		if !validVersionIdentifiers(withoutBuild[1]) {
			return version{}, false
		}
		raw = withoutBuild[0]
	}
	parts := strings.SplitN(raw, "-", 2)
	base := strings.Split(parts[0], ".")
	if len(base) != 3 {
		return version{}, false
	}
	v := version{}
	if _, err := fmt.Sscanf(parts[0], "%d.%d.%d", &v.major, &v.minor, &v.patch); err != nil || v.major < 0 || v.minor < 0 || v.patch < 0 {
		return version{}, false
	}
	if fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch) != parts[0] {
		return version{}, false
	}
	if len(parts) == 2 {
		v.pre = parts[1]
		if !validVersionIdentifiers(v.pre) {
			return version{}, false
		}
	}
	return v, true
}

func validVersionIdentifiers(value string) bool {
	if value == "" {
		return false
	}
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" {
			return false
		}
		for _, r := range identifier {
			if (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '-' {
				return false
			}
		}
	}
	return true
}

func (v version) compare(other version) int {
	for _, pair := range [][2]int{{v.major, other.major}, {v.minor, other.minor}, {v.patch, other.patch}} {
		if pair[0] > pair[1] {
			return 1
		}
		if pair[0] < pair[1] {
			return -1
		}
	}
	if v.pre == other.pre {
		return 0
	}
	if v.pre == "" {
		return 1
	}
	if other.pre == "" {
		return -1
	}
	if v.pre > other.pre {
		return 1
	}
	return -1
}
