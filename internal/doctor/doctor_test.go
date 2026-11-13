package doctor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHostnameStripsAllButHost(t *testing.T) {
	got := Hostname("https://user:secret@Stream.Example.test:8443/live/radio?token=top-secret#frag")
	if got != "stream.example.test" {
		t.Fatalf("Hostname() = %q, want stream.example.test", got)
	}
}

func TestReportStringNeverIncludesStreamSecrets(t *testing.T) {
	report := NewReport(Options{Snapshot: Snapshot{
		StreamURL: "https://user:password@radio.example.test/private/stream?token=secret-value",
		LastIssue: "opening https://radio.example.test/private/stream?token=secret-value failed",
	}})
	text := report.String()
	for _, secret := range []string{"password", "private/stream", "token=", "secret-value"} {
		if strings.Contains(text, secret) {
			t.Errorf("report leaked %q:\n%s", secret, text)
		}
	}
	if !strings.Contains(text, "radio.example.test") {
		t.Errorf("report omitted stream host:\n%s", text)
	}
}

func TestReportStringRedactsEveryUntrustedDetail(t *testing.T) {
	report := Report{
		Application: "lofi-player test",
		Playback:    Check{Status: StatusOK, Detail: "playing https://radio.example.test/private/stream?access=abc"},
		LastIssue:   Check{Status: StatusWarning, Detail: "failed user:password@radio.example.test/private"},
		MPV:         Check{Status: StatusOK, Detail: "version https://evil.example.test/path?token=abc"},
	}
	text := report.String()
	report.DNS = Check{Status: StatusWarning, Detail: "failed mms://radio.example.test/private"}
	text = report.String()
	for _, secret := range []string{"private", "access=", "password", "token=", "evil.example", "radio.example"} {
		if strings.Contains(text, secret) {
			t.Errorf("report leaked %q:\n%s", secret, text)
		}
	}
}

func TestCollectReadOnlyChecks(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	cacheDir := filepath.Join(dir, "cache")
	if err := os.Mkdir(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "query.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("theme: tokyo-night\nstations: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	report := Collect(context.Background(), Options{
		ApplicationVersion: "test",
		ConfigPath:         configPath,
		CacheDir:           cacheDir,
		RadioAPIURL:        server.URL,
		LookPath: func(name string) (string, error) {
			if name == "mpv" {
				return "/usr/local/bin/mpv", nil
			}
			return "", errors.New("not found")
		},
		RunVersion: func(_ context.Context, path string) (string, error) {
			return "mpv v0.42.0\nmore", nil
		},
		LookupHost: func(context.Context, string) ([]string, error) { return []string{"127.0.0.1"}, nil },
	})
	if report.MPV.Status != StatusOK || report.MPV.Detail != "mpv v0.42.0" {
		t.Errorf("mpv = %#v", report.MPV)
	}
	if !strings.Contains(report.Player.Detail, "mpv v0.42.0") {
		t.Errorf("player = %#v", report.Player)
	}
	if report.YTDLP.Status != StatusWarning || report.Config.Status != StatusOK || report.DNS.Status != StatusOK || report.Discovery.Status != StatusOK || !strings.Contains(report.Discovery.Detail, "1 cached search") {
		t.Errorf("unexpected report: %#v", report)
	}
}

func TestCollectHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report := Collect(ctx, Options{
		LookPath: func(string) (string, error) { return "/bin/echo", nil },
		RunVersion: func(ctx context.Context, _ string) (string, error) {
			return "", ctx.Err()
		},
		LookupHost: func(ctx context.Context, _ string) ([]string, error) { return nil, ctx.Err() },
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.Canceled
		})},
	})
	if report.MPV.Status != StatusWarning || report.DNS.Status != StatusWarning || report.Discovery.Status != StatusWarning {
		t.Errorf("canceled report = %#v", report)
	}
}

func TestConfigCheckDoesNotCreateMissingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "config.yaml")
	check := configCheck(Options{ConfigPath: path})
	if check.Status != StatusWarning || check.Detail != "not created yet" {
		t.Fatalf("config check = %#v", check)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("doctor created config directory: stat error = %v", err)
	}
}

func TestMPVVersionCheckUsesNoConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is POSIX-only")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "mpv")
	argsPath := filepath.Join(dir, "args")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+argsPath+"\necho 'mpv vtest'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, err := commandVersion(context.Background(), path); err != nil || got != "mpv vtest\n" {
		t.Fatalf("commandVersion() = %q, %v", got, err)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(args) != "--no-config\n--version\n" {
		t.Errorf("mpv args = %q, want --no-config then --version", args)
	}
}

func TestCommandVersionReturnsSoonAfterWrapperLeavesPipeOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is POSIX-only")
	}
	path := filepath.Join(t.TempDir(), "mpv")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 10 &\nwait\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := commandVersion(ctx, path)
	if err == nil {
		t.Fatal("commandVersion unexpectedly succeeded")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("commandVersion returned after %s, want under 1s", elapsed)
	}
}

func TestBoundedCheckReturnsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	result := make(chan Check, 1)
	go func() {
		result <- boundedCheck(ctx, "timed out", func() Check {
			close(started)
			<-release
			return Check{Status: StatusOK}
		})
	}()
	<-started
	cancel()
	check := <-result
	if check.Status != StatusWarning || check.Detail != "timed out" {
		t.Fatalf("canceled check = %#v", check)
	}
	close(release)
}

func TestExecutableCheckReturnsWhenLookPathBlocks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	result := make(chan Check, 1)
	go func() {
		result <- executableCheck(ctx, "mpv", Options{LookPath: func(string) (string, error) {
			close(started)
			<-release
			return "", errors.New("not found")
		}})
	}()
	<-started
	cancel()
	check := <-result
	if check.Status != StatusWarning || check.Detail != "dependency check timed out" {
		t.Fatalf("canceled dependency check = %#v", check)
	}
	close(release)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
