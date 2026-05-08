package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/config"
)

// TestSmokeCategoryA boots a Server with a fixture upstream tree, exercises
// every Category A endpoint via httptest, and checks status + headers + body
// markers. The Category B routes still return the "Coming soon" placeholder.
func TestSmokeCategoryA(t *testing.T) {
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
			path:        "/examples/",
			wantStatus:  http.StatusOK,
			wantCT:      "text/html; charset=utf-8",
			bodyMarkers: []string{"/examples/jwt", "/examples/rest-api"},
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
			bodyMarkers: []string{"# MuxMaster", "/docs/routing.md", "TODO(spec)"},
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

// TestSmoke304 verifies the If-None-Match cycle on a Category A endpoint.
func TestSmoke304(t *testing.T) {
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

// TestSmokeCategoryBStillStub verifies Category B routes still return the
// "Coming soon" placeholder so this round does not regress them.
func TestSmokeCategoryBStillStub(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	for _, p := range []string{"/docs/routing", "/api", "/examples/jwt", "/benchmarks", "/changelog"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(body), "being prepared") {
			t.Errorf("Category B route %s should still be a Coming-soon stub; body[:200]=%s", p, truncate(string(body), 200))
		}
	}
}

// newTestServer constructs a Server backed by a fixture upstream tree and runs
// Prerender. It does NOT bind a real socket — callers wrap srv.httpServer.Handler
// in httptest.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	upstream := buildFixtureUpstream(t)

	cfg := &config.Config{
		Port:               0,
		SiteBaseURL:        "http://localhost",
		MuxMasterSourceDir: upstream,
		LogLevel:           slog.LevelError,
		Env:                config.EnvDevelopment,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	srv, err := New(cfg, logger, "v1.0.1", "../../templates", "../../static")
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	if err := srv.Prerender(); err != nil {
		t.Fatalf("Prerender: %v", err)
	}
	return srv
}

// buildFixtureUpstream creates the minimal upstream tree required by
// content.VerifyUpstream. Files contain just enough to satisfy the day-one
// contract; their contents are not consumed by the Category A pass.
func buildFixtureUpstream(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"README.md":                            "# MuxMaster\n\n## Benchmarks\n\nstub\n",
		"api.md":                               "# API\n",
		"CHANGELOG.md":                         "# Changelog\n\n## v1.0.1\n",
		"COMPATIBILITY.md":                     "# Compatibility\n",
		"SECURITY.md":                          "# Security\n",
		"CONTRIBUTING.md":                      "# Contributing\n",
		"docs/getting-started.md":              "# Getting started\n",
		"docs/routing.md":                      "# Routing\n",
		"docs/groups.md":                       "# Groups\n",
		"docs/middleware.md":                   "# Middleware\n",
		"docs/error-handling.md":               "# Error handling\n",
		"docs/configuration.md":                "# Configuration\n",
		"docs/response-helpers.md":             "# Response helpers\n",
		"docs/performance.md":                  "# Performance\n",
		"docs/observability.md":                "# Observability\n",
		"docs/migration.md":                    "# Migration\n",
		"docs/cookbook.md":                     "# Cookbook\n",
		"examples/rest-api/main.go":            "package main\n",
		"examples/authn/main.go":               "package main\n",
		"examples/jwt/main.go":                 "package main\n",
		"examples/oauth2/main.go":              "package main\n",
		"examples/cache/main.go":               "package main\n",
		"examples/graceful-shutdown/main.go":   "package main\n",
		"examples/server-side-render/main.go":  "package main\n",
		"examples/static-site/main.go":         "package main\n",
		"release-notes/v1.0.0-20260508.md":     "# v1.0.0\n",
		"assets/logo-muxmaster.png":            "fake-png",
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
