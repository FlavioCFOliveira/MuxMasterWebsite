package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/config"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/content"
)

// TestSmokeFullSite boots a Server with a fixture content tree, exercises
// every public route, and checks status, content-type, and body markers.
// The server reads only the in-memory content tree — no upstream filesystem
// access — verifying the self-contained-binary invariant from
// specification/deployment.md.
func TestSmokeFullSite(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	cases := []struct {
		path        string
		wantStatus  int
		wantCT      string
		bodyMarkers []string
	}{
		{
			path:        "/",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"<!DOCTYPE html>", "MuxMaster"},
		},
		{
			path:        "/docs/",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"Getting started", "Routing", "/docs/cookbook"},
		},
		{
			path:        "/docs/routing",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"<h1", "Routing", "Patterns"},
		},
		{
			path:        "/docs/routing.md",
			wantStatus:  http.StatusOK,
			wantCT:      "text/markdown; charset=utf-8",
			bodyMarkers: []string{"# Routing"},
		},
		{
			path:        "/api",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"<h1", "API"},
		},
		{
			path:        "/examples/",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"/examples/jwt", "/examples/rest-api"},
		},
		{
			path:        "/examples/jwt",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"<h1", "JWT", "language-go"},
		},
		{
			path:        "/benchmarks",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"Benchmarks", "<table>"},
		},
		{
			path:        "/changelog",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"Changelog", "v1.0.1"},
		},
		{
			path:        "/releases/v1.0.0",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"v1.0.0"},
		},
		{
			path:        "/security",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"Security"},
		},
		{
			path:        "/llms.txt",
			wantStatus:  http.StatusOK,
			wantCT:      "text/plain; charset=utf-8",
			bodyMarkers: []string{"# MuxMaster", "## Documentation", "## API", "## Examples", "## Reference", "## Optional"},
		},
		{
			path:        "/llms-full.txt",
			wantStatus:  http.StatusOK,
			wantCT:      "text/plain; charset=utf-8",
			bodyMarkers: []string{"# MuxMaster", "/docs/routing.md", "# Full content"},
		},
		{
			path:        "/sitemap.xml",
			wantStatus:  http.StatusOK,
			wantCT:      "application/xml; charset=utf-8",
			bodyMarkers: []string{"<urlset", "<loc>", "/docs/routing"},
		},
		{
			path:        "/robots.txt",
			wantStatus:  http.StatusOK,
			wantCT:      "text/plain; charset=utf-8",
			bodyMarkers: []string{"User-agent: GPTBot", "User-agent: ClaudeBot", "User-agent: PerplexityBot", "Sitemap:"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status=%d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if got := resp.Header.Get("Content-Type"); got != tc.wantCT {
				t.Errorf("Content-Type=%q, want %q", got, tc.wantCT)
			}
			if resp.Header.Get("ETag") == "" {
				t.Error("missing ETag")
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			for _, marker := range tc.bodyMarkers {
				if !strings.Contains(string(body), marker) {
					t.Errorf("body missing marker %q\nbody[:300]=%s", marker, truncate(string(body), 300))
				}
			}
		})
	}
}

// TestSmoke404 verifies the branded 404 served from the prerender cache.
func TestSmoke404(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/no-such-page")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status=%d, want 404", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control=%q, want no-store", cc)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Page not found") {
		t.Errorf("body missing marker; body[:300]=%s", truncate(string(body), 300))
	}
}

// TestSmoke304Llms verifies the If-None-Match cycle on /llms.txt.
func TestSmoke304Llms(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/llms.txt")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("missing etag on first request")
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/llms.txt", nil)
	req.Header.Set("If-None-Match", etag)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET (conditional): %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("status=%d, want 304", resp2.StatusCode)
	}
}

// TestSmoke304DocPage verifies the If-None-Match cycle on a doc-page route.
// /docs/routing was a Category-B placeholder before this round; the test
// pins the new behaviour: real HTML body, real ETag, real 304.
func TestSmoke304DocPage(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/docs/routing")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("missing etag on first request")
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/docs/routing", nil)
	req.Header.Set("If-None-Match", etag)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET (conditional): %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("status=%d, want 304", resp2.StatusCode)
	}
}

// TestMdCompanionByteForByte verifies the /docs/routing.md companion serves
// the exact bytes from the loader (no transformation).
func TestMdCompanionByteForByte(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	want, err := srv.loader.Load("docs/routing.md")
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}
	resp, err := http.Get(ts.URL + "/docs/routing.md")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if string(got) != string(want) {
		t.Errorf("body mismatch\nwant len=%d got len=%d", len(want), len(got))
	}
}

// TestVersionLabelFromContentChangelog verifies that the version label is
// parsed from content/changelog.md and surfaces in the rendered chrome.
func TestVersionLabelFromContentChangelog(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "v1.0.1") {
		t.Errorf("landing chrome missing version label v1.0.1; body[:600]=%s", truncate(string(body), 600))
	}
}

// newTestServer constructs a Server backed by an in-memory content tree and
// runs Prerender. It does NOT bind a real socket — callers wrap
// srv.httpServer.Handler in httptest.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	loader := buildFixtureLoader(t)

	cfg := &config.Config{
		Port:        0,
		SiteBaseURL: "http://localhost",
		LogLevel:    slog.LevelError,
		Env:         config.EnvDevelopment,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	srv, err := New(cfg, logger, loader, "v1.0.1", "../../templates", "../../static")
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	if err := srv.Prerender(); err != nil {
		t.Fatalf("Prerender: %v", err)
	}
	return srv
}

// buildFixtureLoader creates an in-memory content tree that satisfies every
// required path under specification/content-sources.md "Required files".
// Bodies are minimal but include enough Markdown shape to exercise the
// markdown engine's table, code-fence, and heading-anchor pathways.
func buildFixtureLoader(t *testing.T) *content.Loader {
	t.Helper()

	files := map[string]string{
		"changelog.md":     "# Changelog\n\n## v1.0.1\n\nLatest.\n\n## v1.0.0\n\nInitial.\n",
		"api.md":           "# API\n\n## Overview\n\nReference.\n",
		"compatibility.md": "# Compatibility\n\n## Versions\n\nText.\n",
		"security.md":      "# Security\n\n## Policy\n\nText.\n",
		"contributing.md":  "# Contributing\n\n## How to\n\nText.\n",
		"benchmarks.md": "# Benchmarks\n\n## Numbers\n\n" +
			"| Route | ns/op |\n|---|---|\n| Static | 25 |\n\n" +
			"## Source\n\nText.\n",
		"docs/getting-started.md": "# Getting started\n\n## Install\n\nText.\n",
		"docs/routing.md": "# Routing\n\n## Patterns\n\nText.\n\n## Priority\n\nText.\n\n" +
			"```go\nfunc Handler() {}\n```\n",
		"docs/groups.md":                 "# Groups\n",
		"docs/middleware.md":             "# Middleware\n",
		"docs/error-handling.md":         "# Error handling\n",
		"docs/configuration.md":          "# Configuration\n",
		"docs/response-helpers.md":       "# Response helpers\n",
		"docs/performance.md":            "# Performance\n",
		"docs/observability.md":          "# Observability\n",
		"docs/migration.md":              "# Migration\n",
		"docs/cookbook.md":               "# Cookbook\n",
		"examples/rest-api.md":           "# REST API\n",
		"examples/authn.md":              "# Authn\n",
		"examples/jwt.md":                "# JWT\n\n```go\nfunc main() {}\n```\n",
		"examples/oauth2.md":             "# OAuth2\n",
		"examples/cache.md":              "# Cache\n",
		"examples/graceful-shutdown.md":  "# Graceful shutdown\n",
		"examples/server-side-render.md": "# Server-side render\n",
		"examples/static-site.md":        "# Static site\n",
		"release-notes/v1.0.0.md":        "# Release notes — v1.0.0\n\n## Highlights\n\nText.\n",
	}
	mfs := fstest.MapFS{}
	for path, body := range files {
		mfs[path] = &fstest.MapFile{Data: []byte(body)}
	}
	loader, err := content.NewLoader(mfs)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	return loader
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
