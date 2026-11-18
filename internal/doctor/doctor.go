// Package doctor collects read-only diagnostics for lofi-player support.
package doctor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/iRootPro/lofi-player/internal/config"
	"github.com/iRootPro/lofi-player/internal/i18n"
	"github.com/iRootPro/lofi-player/internal/radiobrowser"
	"gopkg.in/yaml.v3"
)

const (
	radioBrowserHost = "all.api.radio-browser.info"
	defaultAPIURL    = "https://all.api.radio-browser.info/json/stations/search?name=lofi&limit=1"
)

var (
	urlDetailPattern        = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://\S+`)
	credentialDetailPattern = regexp.MustCompile(`\b[^\s:@]+:[^\s@]+@[^\s/]+(?:/\S*)?`)
)

// Status is a textual diagnostic state. Views must always render its detail,
// rather than relying on color alone.
type Status string

const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusError   Status = "error"
	StatusPending Status = "pending"
)

// Check is one safe-to-copy diagnostic outcome.
type Check struct {
	Status Status
	Detail string
}

// Snapshot is live, read-only state supplied by the TUI. StreamURL is never
// emitted directly: Report exposes only its hostname.
type Snapshot struct {
	PlayerConnected bool
	PlaybackState   string
	StreamURL       string
	CacheSeconds    float64
	Buffering       bool
	LastIssue       string
}

// Options supplies the environment and makes the collector testable. All
// optional function fields use their standard-library defaults.
type Options struct {
	ApplicationVersion string
	Locale             i18n.Locale
	ConfigPath         string
	CacheDir           string
	Snapshot           Snapshot

	LookPath    func(string) (string, error)
	RunVersion  func(context.Context, string) (string, error)
	LookupHost  func(context.Context, string) ([]string, error)
	HTTPClient  *http.Client
	RadioAPIURL string
}

// Report is deliberately composed only of safe, human-readable data. It
// never retains full station URLs, query strings, tokens, or raw errors.
type Report struct {
	Locale      i18n.Locale
	Application string
	Player      Check
	Playback    Check
	StreamHost  Check
	Discovery   Check
	LastIssue   Check
	MPV         Check
	YTDLP       Check
	Config      Check
	DNS         Check
}

// NewReport creates an immediate live-state report. Call Collect to add the
// cancellable dependency and Radio Browser checks.
func NewReport(opts Options) Report {
	snapshot := opts.Snapshot
	player := Check{Status: StatusPending, Detail: "not connected"}
	if snapshot.PlayerConnected {
		player = Check{Status: StatusOK, Detail: "connected"}
	}

	playback := Check{Status: StatusPending, Detail: "not playing"}
	if snapshot.PlaybackState != "" {
		playback = Check{Status: StatusOK, Detail: snapshot.PlaybackState}
		if snapshot.Buffering {
			playback = Check{Status: StatusWarning, Detail: "buffering"}
		}
	}

	host := Hostname(snapshot.StreamURL)
	streamHost := Check{Status: StatusPending, Detail: "no active stream"}
	if host != "" {
		streamHost = Check{Status: StatusOK, Detail: host}
	}

	lastIssue := Check{Status: StatusOK, Detail: "none"}
	if safe := sanitizeIssue(snapshot.LastIssue); safe != "" {
		lastIssue = Check{Status: StatusWarning, Detail: safe}
	}

	return Report{
		Locale:      opts.Locale,
		Application: applicationVersion(opts.ApplicationVersion),
		Player:      player,
		Playback:    playback,
		StreamHost:  streamHost,
		Discovery:   Check{Status: StatusPending, Detail: "checks not run"},
		LastIssue:   lastIssue,
		MPV:         Check{Status: StatusPending, Detail: "checks not run"},
		YTDLP:       Check{Status: StatusPending, Detail: "checks not run"},
		Config:      Check{Status: StatusPending, Detail: "checks not run"},
		DNS:         Check{Status: StatusPending, Detail: "checks not run"},
	}
}

// Collect runs short, cancellable host and Radio Browser checks. It never
// starts playback, probes an arbitrary station URL, or writes any file.
func Collect(ctx context.Context, opts Options) Report {
	report := NewReport(opts)
	report.MPV = executableCheck(ctx, "mpv", opts)
	report.Player = playerCheck(opts.Snapshot, report.MPV)
	report.YTDLP = executableCheck(ctx, "yt-dlp", opts)
	report.Config = boundedCheck(ctx, "config check timed out", func() Check { return configCheck(opts) })
	report.DNS = dnsCheck(ctx, opts)
	report.Discovery = apiCheck(ctx, opts)
	return report
}

func playerCheck(snapshot Snapshot, mpv Check) Check {
	if snapshot.PlayerConnected {
		if mpv.Status == StatusOK {
			return Check{Status: StatusOK, Detail: "connected · " + mpv.Detail}
		}
		return Check{Status: StatusWarning, Detail: "connected; mpv check: " + mpv.Detail}
	}
	if mpv.Status == StatusOK {
		return Check{Status: StatusPending, Detail: "not running (Doctor does not start playback) · " + mpv.Detail}
	}
	return Check{Status: mpv.Status, Detail: "not running; mpv " + mpv.Detail}
}

// Hostname returns only the hostname of a stream URL. Credentials, ports,
// paths, queries, and fragments are intentionally discarded.
func Hostname(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// String formats a safe support report. Do not add raw command output, URLs,
// config contents, or error values here: reports are intended for sharing.
func (r Report) String() string {
	return strings.Join([]string{
		"lofi-player " + i18n.T(r.Locale, "doctor_report"),
		i18n.T(r.Locale, "application") + ": " + r.Application,
		formatCheckFor(r.Locale, i18n.T(r.Locale, "player"), r.Player),
		formatCheckFor(r.Locale, i18n.T(r.Locale, "playback"), r.Playback),
		formatCheckFor(r.Locale, i18n.T(r.Locale, "stream_host"), r.StreamHost),
		formatCheckFor(r.Locale, diagnosticLabel(r.Locale, "discovery"), r.Discovery),
		formatCheckFor(r.Locale, i18n.T(r.Locale, "last_issue"), r.LastIssue),
		formatCheckFor(r.Locale, diagnosticLabel(r.Locale, "mpv"), r.MPV),
		formatCheckFor(r.Locale, diagnosticLabel(r.Locale, "yt-dlp"), r.YTDLP),
		formatCheckFor(r.Locale, diagnosticLabel(r.Locale, "config"), r.Config),
		formatCheckFor(r.Locale, diagnosticLabel(r.Locale, "Radio Browser DNS"), r.DNS),
	}, "\n")
}

func formatCheck(name string, check Check) string {
	return formatCheckFor(i18n.EN, name, check)
}

func formatCheckFor(locale i18n.Locale, name string, check Check) string {
	detail := Localize(locale, sanitizeDetail(check.Detail))
	if detail == "" {
		detail = "not available"
	}
	return fmt.Sprintf("%s: %s — %s", name, StatusLabel(locale, check.Status), detail)
}

// Localize translates only fixed diagnostic phrases produced by this package.
// Values from mpv, DNS, or the network deliberately pass through unchanged.
func Localize(locale i18n.Locale, detail string) string {
	if locale != i18n.RU {
		return detail
	}
	if value, ok := russianDiagnosticText[detail]; ok {
		return value
	}
	if strings.HasPrefix(detail, "playing · buffer ") {
		return strings.Replace(detail, "playing · buffer ", "воспроизводится · буфер ", 1)
	}
	if strings.HasPrefix(detail, "connected · ") {
		return strings.Replace(detail, "connected · ", "подключён · ", 1)
	}
	replacements := strings.NewReplacer(
		"not running (Doctor does not start playback)", "не запущен (диагностика не запускает воспроизведение)",
		"connected; mpv check:", "подключён; проверка mpv:",
		"not running; mpv", "не запущен; mpv",
		"Radio Browser API check unavailable", "проверка API Radio Browser недоступна",
		"Radio Browser API unreachable", "API Radio Browser недоступен",
		"Radio Browser API returned an error", "API Radio Browser вернул ошибку",
		"Radio Browser API reachable", "API Radio Browser доступен",
		"1 cached search", "1 поиск в кеше",
		"cached searches", "поисков в кеше",
	)
	return replacements.Replace(detail)
}

func diagnosticLabel(locale i18n.Locale, label string) string {
	if locale != i18n.RU {
		return label
	}
	switch label {
	case "discovery":
		return "поиск"
	case "config":
		return "конфигурация"
	case "Radio Browser DNS":
		return "DNS Radio Browser"
	default:
		return label
	}
}

func StatusLabel(locale i18n.Locale, status Status) string {
	if locale != i18n.RU {
		return string(status)
	}
	switch status {
	case StatusOK:
		return "готово"
	case StatusWarning:
		return "предупреждение"
	case StatusError:
		return "ошибка"
	default:
		return "ожидание"
	}
}

var russianDiagnosticText = map[string]string{
	"not connected": "не подключён", "connected": "подключён", "not playing": "не воспроизводится", "loading": "загрузка", "playing": "воспроизводится", "paused": "на паузе", "buffering": "буферизация", "no active stream": "нет активного потока", "none": "нет", "checks not run": "проверки не запускались", "not available": "недоступно", "dependency check timed out": "проверка зависимости истекла по времени", "config check timed out": "проверка конфигурации истекла по времени", "cache check timed out": "проверка кеша истекла по времени", "path unavailable": "путь недоступен", "not created yet": "ещё не создан", "could not read": "не удалось прочитать", "not a regular file": "не обычный файл", "invalid YAML": "некорректный YAML", "readable YAML": "YAML читается", "Radio Browser name lookup failed": "не удалось найти имя Radio Browser", "Radio Browser host resolves": "хост Radio Browser разрешается", "no cached searches yet": "в кеше пока нет поисков", "cache unavailable": "кеш недоступен", "not found": "не найден", "not found (needed only for YouTube stations)": "не найден (нужен только для станций YouTube)", "found, but version check failed": "найден, но проверка версии не удалась", "version available": "версия доступна", "details redacted": "детали скрыты", "Could not play this station. Try another one.": "Не удалось воспроизвести эту станцию. Попробуйте другую.", "The stream ended.": "Поток завершился.", "A player command could not be completed.": "Команду плеера не удалось выполнить.",
}

func applicationVersion(version string) string {
	if version == "" {
		return "lofi-player"
	}
	return "lofi-player " + version
}

func executableCheck(ctx context.Context, name string, opts Options) Check {
	return boundedCheck(ctx, "dependency check timed out", func() Check {
		return executableCheckNow(ctx, name, opts)
	})
}

func executableCheckNow(ctx context.Context, name string, opts Options) Check {
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	path, err := lookPath(name)
	if err != nil {
		if name == "yt-dlp" {
			return Check{Status: StatusWarning, Detail: "not found (needed only for YouTube stations)"}
		}
		return Check{Status: StatusError, Detail: "not found"}
	}

	runVersion := opts.RunVersion
	if runVersion == nil {
		runVersion = commandVersion
	}
	version, err := runVersion(ctx, path)
	if err != nil {
		return Check{Status: StatusWarning, Detail: "found, but version check failed"}
	}
	return Check{Status: StatusOK, Detail: sanitizeDetail(firstLine(version))}
}

func commandVersion(ctx context.Context, path string) (string, error) {
	args := []string{"--version"}
	// mpv creates its XDG config directory while locating its normal config,
	// even for --version. --no-config keeps this diagnostic strictly read-only.
	if filepath.Base(path) == "mpv" {
		args = []string{"--no-config", "--version"}
	}
	cmd := exec.CommandContext(ctx, path, args...)
	// A wrapper may leave a child process holding stdout/stderr open after the
	// context kills its parent. WaitDelay bounds Output's pipe-drain wait, so
	// Doctor checks always return shortly after cancellation.
	cmd.WaitDelay = 200 * time.Millisecond
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func configCheck(opts Options) Check {
	path := opts.ConfigPath
	if path == "" {
		var err error
		path, err = config.Path()
		if err != nil {
			return Check{Status: StatusWarning, Detail: "path unavailable"}
		}
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Check{Status: StatusWarning, Detail: "not created yet"}
	}
	if err != nil {
		return Check{Status: StatusError, Detail: "could not read"}
	}
	if !info.Mode().IsRegular() {
		return Check{Status: StatusError, Detail: "not a regular file"}
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Check{Status: StatusWarning, Detail: "not created yet"}
	}
	if err != nil {
		return Check{Status: StatusError, Detail: "could not read"}
	}
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Check{Status: StatusError, Detail: "invalid YAML"}
	}
	return Check{Status: StatusOK, Detail: "readable YAML"}
}

func dnsCheck(ctx context.Context, opts Options) Check {
	lookupHost := opts.LookupHost
	if lookupHost == nil {
		lookupHost = net.DefaultResolver.LookupHost
	}
	if _, err := lookupHost(ctx, radioBrowserHost); err != nil {
		return Check{Status: StatusWarning, Detail: "Radio Browser name lookup failed"}
	}
	return Check{Status: StatusOK, Detail: "Radio Browser host resolves"}
}

func apiCheck(ctx context.Context, opts Options) Check {
	cache := cacheSummary(ctx, opts)
	apiURL := opts.RadioAPIURL
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return Check{Status: StatusWarning, Detail: "Radio Browser API check unavailable; " + cache}
	}
	req.Header.Set("User-Agent", "lofi-player playback doctor")
	resp, err := client.Do(req)
	if err != nil {
		return Check{Status: StatusWarning, Detail: "Radio Browser API unreachable; " + cache}
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Check{Status: StatusWarning, Detail: "Radio Browser API returned an error; " + cache}
	}
	return Check{Status: StatusOK, Detail: "Radio Browser API reachable; " + cache}
}

func cacheSummary(ctx context.Context, opts Options) string {
	return boundedCheck(ctx, "cache check timed out", func() Check {
		return cacheCheck(opts)
	}).Detail
}

func cacheCheck(opts Options) Check {
	dir := opts.CacheDir
	if dir == "" {
		dir = radiobrowser.DefaultCacheDir()
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return Check{Status: StatusWarning, Detail: "no cached searches yet"}
	}
	if err != nil {
		return Check{Status: StatusWarning, Detail: "cache unavailable"}
	}
	if len(entries) == 1 {
		return Check{Status: StatusOK, Detail: "1 cached search"}
	}
	return Check{Status: StatusOK, Detail: fmt.Sprintf("%d cached searches", len(entries))}
}

// boundedCheck lets the caller stop waiting for a slow external dependency,
// including an unavailable network filesystem in PATH, config, or cache. The
// result is discarded after cancellation and the checks never write state.
func boundedCheck(ctx context.Context, timeoutDetail string, check func() Check) Check {
	if ctx.Err() != nil {
		return Check{Status: StatusWarning, Detail: timeoutDetail}
	}
	result := make(chan Check, 1)
	go func() { result <- check() }()
	select {
	case value := <-result:
		return value
	case <-ctx.Done():
		return Check{Status: StatusWarning, Detail: timeoutDetail}
	}
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(value, "\n")
	line = strings.TrimSpace(line)
	if line == "" {
		return "version available"
	}
	return line
}

func sanitizeIssue(issue string) string {
	return sanitizeDetail(issue)
}

// sanitizeDetail is intentionally conservative because report fields can be
// populated by external commands and media metadata. A report is for sharing,
// so a suspicious value is less useful than accidentally exposing a secret.
func sanitizeDetail(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return ""
	}
	lower := strings.ToLower(detail)
	for _, marker := range []string{"token", "password", "authorization", "cookie", "secret", "credential", "key="} {
		if strings.Contains(lower, marker) {
			return "details redacted"
		}
	}
	if urlDetailPattern.MatchString(detail) || credentialDetailPattern.MatchString(detail) || strings.Contains(detail, "?") {
		return "details redacted"
	}
	return detail
}
