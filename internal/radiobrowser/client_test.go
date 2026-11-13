package radiobrowser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestResolveMirrorNamesPrefersSRV(t *testing.T) {
	got, err := resolveMirrorNames(
		context.Background(),
		func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "", []*net.SRV{{Target: "de1.api.radio-browser.info."}, {Target: "at1.api.radio-browser.info."}}, nil
		},
		func(context.Context, string) ([]string, error) {
			t.Fatal("LookupHost should not run when SRV succeeds")
			return nil, nil
		},
		func(context.Context, string) ([]string, error) {
			t.Fatal("LookupAddr should not run when SRV succeeds")
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("resolveMirrorNames: %v", err)
	}
	if want := []string{"de1.api.radio-browser.info", "at1.api.radio-browser.info"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mirrors = %#v, want %#v", got, want)
	}
}

func TestResolveMirrorNamesFallsBackToAllHostWithoutPTR(t *testing.T) {
	got, err := resolveMirrorNames(
		context.Background(),
		func(context.Context, string, string, string) (string, []*net.SRV, error) {
			return "", nil, errors.New("no SRV")
		},
		func(context.Context, string) ([]string, error) { return []string{"240.0.0.55"}, nil },
		func(context.Context, string) ([]string, error) { return nil, errors.New("PTR blocked") },
	)
	if err != nil {
		t.Fatalf("resolveMirrorNames: %v", err)
	}
	if want := []string{globalMirrorHost}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mirrors = %#v, want %#v", got, want)
	}
}

func TestWithGlobalFallback(t *testing.T) {
	got := withGlobalFallback([]string{"de1.api.radio-browser.info"})
	want := []string{"de1.api.radio-browser.info", globalMirrorHost}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("withGlobalFallback = %#v, want %#v", got, want)
	}

	got = withGlobalFallback([]string{"https://all.api.radio-browser.info"})
	if want := []string{"https://all.api.radio-browser.info"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("withGlobalFallback did not preserve a sole global endpoint: %#v", got)
	}

	got = withGlobalFallback([]string{globalMirrorHost, "de1.api.radio-browser.info", globalMirrorHost})
	if want := []string{"de1.api.radio-browser.info", globalMirrorHost}; !reflect.DeepEqual(got, want) {
		t.Fatalf("withGlobalFallback did not move/dedupe global endpoint: %#v", got)
	}
}

func TestParseQuery(t *testing.T) {
	tests := []struct {
		raw  string
		want Query
	}{
		{"ambient", Query{Text: "ambient"}},
		{"tag:ambient", Query{Tag: "ambient"}},
		{"country:jp lang:english jazz", Query{CountryCode: "JP", Language: "english", Text: "jazz"}},
		{"language:french", Query{Language: "french"}},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ParseQuery(tt.raw)
			if err != nil {
				t.Fatalf("ParseQuery: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseQuery(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
	if _, err := ParseQuery("tag:"); err == nil {
		t.Fatal("ParseQuery(tag:) returned nil error")
	}
}

func TestSearchUsesAPIContractAndResolvedURL(t *testing.T) {
	var gotQuery url.Values
	var gotAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/stations/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotQuery = r.URL.Query()
		gotAgent = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`[
			{"stationuuid":"one","name":"First","url":"https://fallback","url_resolved":"https://resolved","votes":3,"countrycode":"JP","tags":"ambient","codec":"MP3","bitrate":128},
			{"stationuuid":"two","name":"Second","url":"https://second","votes":9}
		]`))
	}))
	defer server.Close()

	c := New(Options{CacheDir: t.TempDir(), UserAgent: "lofi-player/test", Mirrors: fixedMirrors(server.URL)})
	results, err := c.Search(context.Background(), "tag:ambient country:jp lang:english chill")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotAgent != "lofi-player/test" {
		t.Errorf("User-Agent = %q", gotAgent)
	}
	for key, want := range map[string]string{"tag": "ambient", "countrycode": "JP", "language": "english", "name": "chill", "limit": "25", "order": "votes", "reverse": "true", "hidebroken": "true"} {
		if got := gotQuery.Get(key); got != want {
			t.Errorf("query[%s] = %q, want %q", key, got, want)
		}
	}
	if len(results.Stations) != 2 || results.Stations[0].Name != "Second" {
		t.Fatalf("results = %#v", results.Stations)
	}
	if got := results.Stations[1].StreamURL; got != "https://resolved" {
		t.Errorf("resolved URL = %q", got)
	}
}

func TestSearchRetriesAnotherMirror(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusServiceUnavailable) }))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"Works","url":"https://stream"}]`))
	}))
	defer good.Close()

	c := New(Options{CacheDir: t.TempDir(), Mirrors: fixedMirrors(bad.URL, good.URL)})
	got, err := c.Search(context.Background(), "works")
	if err != nil || len(got.Stations) != 1 || got.Stations[0].Name != "Works" {
		t.Fatalf("Search = %#v, %v", got, err)
	}
}

func TestSearchReturnsStaleCacheWhenOffline(t *testing.T) {
	cacheDir := t.TempDir()
	working := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"Cached","url":"https://cached"}]`))
	}))
	workingURL, err := url.Parse(working.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := New(Options{
		CacheDir: cacheDir,
		Mirrors:  fixedMirrors(working.URL),
		HTTPClient: &http.Client{Transport: hostOnlyTransport{
			allowedHost: workingURL.Host,
		}},
	})
	if _, err := c.Search(context.Background(), "ambient"); err != nil {
		t.Fatalf("initial Search: %v", err)
	}
	working.Close()

	got, err := c.Search(context.Background(), "ambient")
	if err != nil {
		t.Fatalf("offline Search: %v", err)
	}
	if !got.Cached || len(got.Stations) != 1 || got.Stations[0].Name != "Cached" {
		t.Fatalf("stale result = %#v", got)
	}
}

// hostOnlyTransport prevents a fallback-oriented unit test from reaching the
// public Radio Browser endpoint when its local mirror intentionally goes down.
type hostOnlyTransport struct{ allowedHost string }

func (t hostOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != t.allowedHost {
		return nil, fmt.Errorf("network disabled for %s", req.URL.Host)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func TestSearchFailsWithoutCache(t *testing.T) {
	c := New(Options{CacheDir: t.TempDir(), Mirrors: func(context.Context) ([]string, error) { return nil, errors.New("offline") }})
	if _, err := c.Search(context.Background(), "ambient"); err == nil || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("Search error = %v", err)
	}
}

func TestRegisterClick(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = r.URL.Path == "/json/url/a-station"
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	c := New(Options{Mirrors: fixedMirrors(server.URL), CacheDir: t.TempDir()})
	if err := c.RegisterClick(context.Background(), "a-station"); err != nil {
		t.Fatalf("RegisterClick: %v", err)
	}
	if !called {
		t.Fatal("click endpoint was not called")
	}
}

func fixedMirrors(mirrors ...string) func(context.Context) ([]string, error) {
	return func(context.Context) ([]string, error) { return append([]string(nil), mirrors...), nil }
}
