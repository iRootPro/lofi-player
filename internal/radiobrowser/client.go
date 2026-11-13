// Package radiobrowser provides a small, resilient client for the public
// Radio Browser directory API.
package radiobrowser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultLimit     = 25
	defaultUserAgent = "lofi-player/dev (+https://github.com/iRootPro/lofi-player)"
	globalMirrorHost = "all.api.radio-browser.info"
)

var ErrEmptyQuery = errors.New("radio browser query is empty")

// ErrInvalidResponse marks a syntactically invalid Radio Browser response.
// Callers can show a useful UI message without exposing decoder internals.
var ErrInvalidResponse = errors.New("invalid Radio Browser response")

// HTTPStatusError preserves an API status for UI-level recovery decisions
// while keeping the detailed server response out of normal user-facing text.
type HTTPStatusError struct{ StatusCode int }

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("Radio Browser returned HTTP %d", e.StatusCode)
}

// Query is the subset of Radio Browser's advanced search supported by the
// discovery UI. Text is matched against station names.
type Query struct {
	Text        string
	Tag         string
	CountryCode string
	Language    string
}

// ParseQuery accepts normal text as well as tag:ambient, country:JP, and
// lang:english qualifiers. Qualifiers can be combined with normal text.
func ParseQuery(raw string) (Query, error) {
	var q Query
	var text []string
	for _, token := range strings.Fields(raw) {
		parts := strings.SplitN(token, ":", 2)
		if len(parts) != 2 {
			text = append(text, token)
			continue
		}

		key, value := strings.ToLower(parts[0]), strings.TrimSpace(parts[1])
		switch key {
		case "tag":
			if value == "" {
				return Query{}, errors.New("tag query needs a value")
			}
			q.Tag = value
		case "country":
			if value == "" {
				return Query{}, errors.New("country query needs a value")
			}
			q.CountryCode = strings.ToUpper(value)
		case "lang", "language":
			if value == "" {
				return Query{}, errors.New("language query needs a value")
			}
			q.Language = value
		default:
			text = append(text, token)
		}
	}
	q.Text = strings.Join(text, " ")
	if q.Text == "" && q.Tag == "" && q.CountryCode == "" && q.Language == "" {
		return Query{}, ErrEmptyQuery
	}
	return q, nil
}

// Station is the useful, stable subset of a Radio Browser station record.
type Station struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	StreamURL   string `json:"stream_url"`
	CountryCode string `json:"countrycode"`
	Tags        string `json:"tags"`
	Codec       string `json:"codec"`
	Bitrate     int    `json:"bitrate"`
	Votes       int    `json:"votes"`
}

// Results is one discovery response. Cached is true only when a request
// failed and a previously successful result is shown instead.
type Results struct {
	Stations []Station `json:"stations"`
	Cached   bool      `json:"cached"`
}

// Client is the minimal interface consumed by the TUI. It also makes the
// discovery model easy to test without performing HTTP requests.
type Client interface {
	Search(context.Context, string) (Results, error)
	RegisterClick(context.Context, string) error
}

// Options configures a Client implementation. Mirrors is primarily useful in
// tests; production uses Radio Browser's DNS discovery when it is nil.
type Options struct {
	HTTPClient *http.Client
	UserAgent  string
	CacheDir   string
	Mirrors    func(context.Context) ([]string, error)
}

// APIClient queries Radio Browser mirrors and maintains a small local cache.
type APIClient struct {
	httpClient *http.Client
	userAgent  string
	cacheDir   string
	mirrors    func(context.Context) ([]string, error)
}

// New constructs an API client with sensible application defaults. It makes
// no network request; discovery stays entirely lazy until the user searches.
func New(opts Options) *APIClient {
	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 8 * time.Second}
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	cacheDir := opts.CacheDir
	if cacheDir == "" {
		cacheDir = defaultCacheDir()
	}
	mirrors := opts.Mirrors
	if mirrors == nil {
		mirrors = discoverMirrors
	}
	return &APIClient{httpClient: hc, userAgent: ua, cacheDir: cacheDir, mirrors: mirrors}
}

func defaultCacheDir() string {
	if base := os.Getenv("XDG_CACHE_HOME"); base != "" {
		return filepath.Join(base, "lofi-player", "radio-browser")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".cache", "lofi-player", "radio-browser")
	}
	return filepath.Join(home, ".cache", "lofi-player", "radio-browser")
}

// Search queries a randomized Radio Browser mirror and writes successful
// responses to the XDG cache. When every mirror fails, it returns the cached
// response (marked Cached) if one exists.
func (c *APIClient) Search(ctx context.Context, raw string) (Results, error) {
	q, err := ParseQuery(raw)
	if err != nil {
		return Results{}, err
	}

	results, requestErr := c.searchRemote(ctx, q)
	if requestErr == nil {
		_ = c.writeCache(raw, results)
		return results, nil
	}

	if cached, err := c.readCache(raw); err == nil {
		cached.Cached = true
		return cached, nil
	}
	return Results{}, requestErr
}

func (c *APIClient) searchRemote(ctx context.Context, q Query) (Results, error) {
	mirrors, err := c.mirrors(ctx)
	if err != nil {
		return Results{}, fmt.Errorf("finding Radio Browser mirrors: %w", err)
	}
	if len(mirrors) == 0 {
		return Results{}, errors.New("no Radio Browser mirrors available")
	}
	rand.Shuffle(len(mirrors), func(i, j int) { mirrors[i], mirrors[j] = mirrors[j], mirrors[i] })
	mirrors = withGlobalFallback(mirrors)

	var errs []error
	for _, mirror := range mirrors {
		result, err := c.searchMirror(ctx, mirror, q)
		if err == nil {
			return result, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", mirror, err))
	}
	return Results{}, fmt.Errorf("Radio Browser search failed: %w", errors.Join(errs...))
}

func (c *APIClient) searchMirror(ctx context.Context, mirror string, q Query) (Results, error) {
	u, err := mirrorURL(mirror, "/json/stations/search")
	if err != nil {
		return Results{}, err
	}
	params := u.Query()
	params.Set("hidebroken", "true")
	params.Set("limit", fmt.Sprint(defaultLimit))
	params.Set("order", "votes")
	params.Set("reverse", "true")
	if q.Text != "" {
		params.Set("name", q.Text)
	}
	if q.Tag != "" {
		params.Set("tag", q.Tag)
	}
	if q.CountryCode != "" {
		params.Set("countrycode", q.CountryCode)
	}
	if q.Language != "" {
		params.Set("language", q.Language)
	}
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Results{}, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Results{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Results{}, &HTTPStatusError{StatusCode: resp.StatusCode}
	}

	var payload []struct {
		UUID        string `json:"stationuuid"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		ResolvedURL string `json:"url_resolved"`
		CountryCode string `json:"countrycode"`
		Tags        string `json:"tags"`
		Codec       string `json:"codec"`
		Bitrate     int    `json:"bitrate"`
		Votes       int    `json:"votes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Results{}, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}

	stations := make([]Station, 0, len(payload))
	for _, s := range payload {
		streamURL := strings.TrimSpace(s.ResolvedURL)
		if streamURL == "" {
			streamURL = strings.TrimSpace(s.URL)
		}
		if streamURL == "" || strings.TrimSpace(s.Name) == "" {
			continue
		}
		stations = append(stations, Station{
			UUID: s.UUID, Name: s.Name, StreamURL: streamURL,
			CountryCode: s.CountryCode, Tags: s.Tags, Codec: s.Codec,
			Bitrate: s.Bitrate, Votes: s.Votes,
		})
	}
	// Mirrors should honour order=votes, but sort locally too so cache and UI
	// stay deterministic even when a mirror ignores the order parameter.
	sort.SliceStable(stations, func(i, j int) bool { return stations[i].Votes > stations[j].Votes })
	if len(stations) > defaultLimit {
		stations = stations[:defaultLimit]
	}
	return Results{Stations: stations}, nil
}

// RegisterClick records a playback start with Radio Browser. Callers should
// run it in a background command; a failed click must never delay audio.
func (c *APIClient) RegisterClick(ctx context.Context, stationUUID string) error {
	if strings.TrimSpace(stationUUID) == "" {
		return nil
	}
	mirrors, err := c.mirrors(ctx)
	if err != nil {
		return err
	}
	if len(mirrors) == 0 {
		return errors.New("no Radio Browser mirrors available")
	}
	rand.Shuffle(len(mirrors), func(i, j int) { mirrors[i], mirrors[j] = mirrors[j], mirrors[i] })
	mirrors = withGlobalFallback(mirrors)
	var errs []error
	for _, mirror := range mirrors {
		u, err := mirrorURL(mirror, "/json/url/"+url.PathEscape(stationUUID))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		req.Header.Set("User-Agent", c.userAgent)
		resp, err := c.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				return nil
			}
			err = fmt.Errorf("unexpected HTTP status %s", resp.Status)
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// withGlobalFallback makes the official all.api endpoint the final candidate
// even when DNS discovery returned only one slow or unhealthy named mirror.
// It is load-balanced by Radio Browser, not a hard-coded single server.
func withGlobalFallback(mirrors []string) []string {
	out := make([]string, 0, len(mirrors)+1)
	fallback := globalMirrorHost
	for _, mirror := range mirrors {
		u, err := mirrorURL(mirror, "/")
		if err == nil && strings.EqualFold(u.Hostname(), globalMirrorHost) {
			// Keep its original scheme spelling, but move it behind every
			// discovered named mirror so it truly is the final retry.
			fallback = mirror
			continue
		}
		out = append(out, mirror)
	}
	return append(out, fallback)
}

func mirrorURL(mirror, path string) (*url.URL, error) {
	if !strings.Contains(mirror, "://") {
		mirror = "https://" + mirror
	}
	u, err := url.Parse(mirror)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid Radio Browser mirror %q", mirror)
	}
	u.Path = path
	u.RawQuery = ""
	return u, nil
}

func discoverMirrors(ctx context.Context) ([]string, error) {
	return resolveMirrorNames(
		ctx,
		net.DefaultResolver.LookupSRV,
		net.DefaultResolver.LookupHost,
		net.DefaultResolver.LookupAddr,
	)
}

// resolveMirrorNames follows Radio Browser's published discovery flow while
// tolerating networks that suppress PTR records. SRV returns mirror hostnames
// directly when available; reverse lookups are the legacy fallback. The
// all.api hostname is a final, load-balanced discovery endpoint rather than a
// pinned mirror, so it keeps discovery usable on DNS proxies without reverse
// DNS (a common corporate/VPN configuration).
func resolveMirrorNames(
	ctx context.Context,
	lookupSRV func(context.Context, string, string, string) (string, []*net.SRV, error),
	lookupHost func(context.Context, string) ([]string, error),
	lookupAddr func(context.Context, string) ([]string, error),
) ([]string, error) {
	seen := map[string]struct{}{}
	mirrors := []string{}
	appendName := func(name string) {
		name = strings.TrimSuffix(strings.TrimSpace(name), ".")
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		mirrors = append(mirrors, name)
	}

	_, srvRecords, _ := lookupSRV(ctx, "api", "tcp", "radio-browser.info")
	for _, record := range srvRecords {
		appendName(record.Target)
	}
	if len(mirrors) > 0 {
		return mirrors, nil
	}

	ips, err := lookupHost(ctx, "all.api.radio-browser.info")
	if err == nil {
		for _, ip := range ips {
			names, err := lookupAddr(ctx, ip)
			if err != nil {
				continue
			}
			for _, name := range names {
				appendName(name)
			}
		}
	}
	if len(mirrors) > 0 {
		return mirrors, nil
	}
	return []string{globalMirrorHost}, nil
}

type cacheEntry struct {
	SavedAt time.Time `json:"saved_at"`
	Results Results   `json:"results"`
}

func (c *APIClient) cachePath(raw string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(raw), " "))
	sum := sha256.Sum256([]byte(normalized))
	return filepath.Join(c.cacheDir, hex.EncodeToString(sum[:])+".json")
}

func (c *APIClient) writeCache(raw string, results Results) error {
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(cacheEntry{SavedAt: time.Now().UTC(), Results: results})
	if err != nil {
		return err
	}
	return os.WriteFile(c.cachePath(raw), data, 0o644)
}

func (c *APIClient) readCache(raw string) (Results, error) {
	data, err := os.ReadFile(c.cachePath(raw))
	if err != nil {
		return Results{}, err
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return Results{}, err
	}
	return entry.Results, nil
}
