package app

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpskit.local/vpskit/internal/model"
)

type ruleCacheRoundTripper func(*http.Request) (*http.Response, error)

func (fn ruleCacheRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestRefreshManagedRuleCacheFailureLeavesNoPartialRevision(t *testing.T) {
	previousClient := rulesCacheHTTPClient
	t.Cleanup(func() { rulesCacheHTTPClient = previousClient })
	rulesCacheHTTPClient = &http.Client{Transport: ruleCacheRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header)}, nil
	})}

	cacheRoot := t.TempDir()
	activeRoot := rulesCacheRevisionRootAt(cacheRoot, 3)
	if err := os.MkdirAll(activeRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(activeRoot, "marker"), []byte("active"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := refreshManagedRuleCacheAtRoot(model.RulesProfileACL4SSR, 4, cacheRoot); err == nil {
		t.Fatal("expected unavailable upstream to fail")
	}
	if content, err := os.ReadFile(filepath.Join(activeRoot, "marker")); err != nil || string(content) != "active" {
		t.Fatalf("active cache was changed after failed refresh: content=%q error=%v", content, err)
	}
	if _, err := os.Stat(rulesCacheRevisionRootAt(cacheRoot, 4)); !os.IsNotExist(err) {
		t.Fatalf("failed refresh left a candidate revision: %v", err)
	}
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "r0003" {
		t.Fatalf("failed refresh left unexpected cache entries: %#v", entries)
	}
}

func TestRefreshManagedRuleCacheActivatesCompleteRevision(t *testing.T) {
	previousClient := rulesCacheHTTPClient
	t.Cleanup(func() { rulesCacheHTTPClient = previousClient })
	rulesCacheHTTPClient = &http.Client{Transport: ruleCacheRoundTripper(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("payload for " + request.URL.Path)), Header: make(http.Header)}, nil
	})}

	cacheRoot := t.TempDir()
	manifest, err := refreshManagedRuleCacheAtRoot(model.RulesProfileACL4SSR, 4, cacheRoot)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Revision != 4 || len(manifest.Sources) != 19 {
		t.Fatalf("unexpected completed manifest: %#v", manifest)
	}
	if _, err := os.Stat(rulesCacheManifestPathAt(cacheRoot, 4)); err != nil {
		t.Fatalf("completed cache manifest is missing: %v", err)
	}
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "r0004" {
		t.Fatalf("completed refresh left staging entries: %#v", entries)
	}
}
