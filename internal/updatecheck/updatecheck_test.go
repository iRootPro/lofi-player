package updatecheck

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckCachesAndUsesETag(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.Header.Get("User-Agent"); got != "lofi-player/0.4.0" {
			t.Errorf("User-Agent = %q", got)
		}
		if requests == 2 {
			if got := r.Header.Get("If-None-Match"); got != `"update-v1"` {
				t.Errorf("If-None-Match = %q", got)
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"update-v1"`)
		_, _ = w.Write([]byte(`{"tag_name":"v0.4.1","html_url":"https://example.test/v0.4.1"}`))
	}))
	defer server.Close()

	now := time.Date(2026, time.November, 18, 10, 0, 0, 0, time.UTC)
	c := &Client{CurrentVersion: "0.4.0", BaseURL: server.URL, CachePath: filepath.Join(t.TempDir(), "update.json"), Now: func() time.Time { return now }}
	first, err := c.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !first.Available || first.LatestVersion != "v0.4.1" {
		t.Fatalf("first result = %+v", first)
	}
	if _, err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := requests; got != 1 {
		t.Fatalf("requests while cache fresh = %d, want 1", got)
	}
	now = now.Add(DefaultInterval)
	second, err := c.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !second.Available || requests != 2 {
		t.Fatalf("conditional check = %+v, requests=%d", second, requests)
	}
}

func TestCheckFailuresAreThrottled(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	c := &Client{CurrentVersion: "0.4.0", BaseURL: server.URL, CachePath: filepath.Join(t.TempDir(), "update.json")}
	if _, err := c.Check(context.Background()); err == nil {
		t.Fatal("Check succeeded for 503")
	}
	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("cached failure should be silent: %v", err)
	}
	if got := requests; got != 1 {
		t.Errorf("requests = %d, want 1", got)
	}
}

func TestCheckMalformedResponseIsThrottled(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()
	c := &Client{CurrentVersion: "0.4.0", BaseURL: server.URL, CachePath: filepath.Join(t.TempDir(), "update.json")}
	if _, err := c.Check(context.Background()); err == nil {
		t.Fatal("Check succeeded for malformed JSON")
	}
	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("cached malformed result should be silent: %v", err)
	}
	if got := requests; got != 1 {
		t.Errorf("requests = %d, want 1", got)
	}
}

func TestCheckDoesNotFollowRedirects(t *testing.T) {
	targetRequests := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetRequests++
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirect.Close()
	c := &Client{CurrentVersion: "0.4.0", BaseURL: redirect.URL, CachePath: filepath.Join(t.TempDir(), "update.json")}
	if _, err := c.Check(context.Background()); err == nil {
		t.Fatal("Check succeeded after redirect")
	}
	if targetRequests != 0 {
		t.Errorf("redirect target received %d request(s), want 0", targetRequests)
	}
}

func TestCheckSkipsDevelopmentBuild(t *testing.T) {
	_, err := New("dev").Check(context.Background())
	if !errors.Is(err, ErrNonReleaseBuild) {
		t.Fatalf("error = %v, want ErrNonReleaseBuild", err)
	}
}

func TestCheckSkipsSnapshotBuildWithoutNetwork(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, err := (&Client{CurrentVersion: "0.4.1-next", BaseURL: server.URL}).Check(context.Background())
	if !errors.Is(err, ErrNonReleaseBuild) {
		t.Fatalf("error = %v, want ErrNonReleaseBuild", err)
	}
	if requests != 0 {
		t.Errorf("snapshot build made %d request(s), want 0", requests)
	}
}

func TestCheckNowBypassesFreshCache(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"tag_name":"v0.4.0","html_url":"https://example.test/v0.4.0"}`))
	}))
	defer server.Close()
	c := &Client{CurrentVersion: "0.4.0", BaseURL: server.URL, CachePath: filepath.Join(t.TempDir(), "update.json")}
	if _, err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CheckNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := requests; got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}
}

func TestCheckAcceptsStableBuildMetadata(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"tag_name":"v0.4.1+build.7","html_url":"https://example.test/v0.4.1"}`))
	}))
	defer server.Close()
	c := &Client{CurrentVersion: "0.4.0+build.1", BaseURL: server.URL, CachePath: filepath.Join(t.TempDir(), "update.json")}
	result, err := c.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Available || requests != 1 {
		t.Fatalf("result = %+v, requests=%d", result, requests)
	}
}

func TestPathRespectsXDGCacheHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/cache-override")
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := "/tmp/cache-override/lofi-player/update.json"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestSaveCacheUsesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "update.json")
	if err := saveCache(path, Cache{CheckedAt: time.Now(), LatestVersion: "v0.4.1"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("permissions = %o, want 600", got)
	}
}
