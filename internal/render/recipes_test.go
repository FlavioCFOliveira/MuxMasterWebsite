package render

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/content"
)

// fixtureLoader returns a loader rooted at an in-memory content tree that
// mirrors the day-one /content/ corpus shape. Bodies are minimal; tests that
// need a particular Markdown shape supply their own fixture.
func fixtureLoader(t *testing.T) *content.Loader {
	t.Helper()
	files := map[string]string{
		"changelog.md":                    "# Changelog\n\n## v1.0.1\n",
		"api.md":                          "# API\n\n## Overview\n\nText.\n",
		"compatibility.md":                "# Compatibility\n",
		"security.md":                     "# Security\n",
		"contributing.md":                 "# Contributing\n",
		"benchmarks.md":                   "# Benchmarks\n\n## Numbers\n\nTable.\n",
		"docs/getting-started.md":         "# Getting started\n\n## Install\n\nText.\n",
		"docs/routing.md":                 "# Routing\n\n## Patterns\n\nText.\n\n## Priority\n\nText.\n",
		"docs/groups.md":                  "# Groups\n",
		"docs/middleware.md":              "# Middleware\n",
		"docs/error-handling.md":          "# Error handling\n",
		"docs/configuration.md":           "# Configuration\n",
		"docs/response-helpers.md":        "# Response helpers\n",
		"docs/performance.md":             "# Performance\n",
		"docs/observability.md":           "# Observability\n",
		"docs/migration.md":               "# Migration\n",
		"docs/cookbook.md":                "# Cookbook\n",
		"examples/rest-api.md":            "# REST API\n",
		"examples/authn.md":               "# Authn\n",
		"examples/jwt.md":                 "# JWT\n\n```go\nfunc main() {}\n```\n",
		"examples/oauth2.md":              "# OAuth2\n",
		"examples/cache.md":               "# Cache\n",
		"examples/graceful-shutdown.md":   "# Graceful shutdown\n",
		"examples/server-side-render.md":  "# Server-side render\n",
		"examples/static-site.md":         "# Static site\n",
		"release-notes/v1.0.0.md":         "# v1.0.0\n",
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

// fixtureDeps returns a Deps value sufficient for exercising every recipe.
// The renderer is constructed against the live templates and CSS bundle.
func fixtureDeps(t *testing.T) Deps {
	t.Helper()
	r, err := New("../../templates", "../../static")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return Deps{
		Routes: []RouteInfo{
			{Path: "/", Title: "MuxMaster", Description: "landing", Section: "landing", HasMarkdown: false},
			{Path: "/docs/", Title: "Documentation", Description: "Docs index", Section: "docs", HasMarkdown: true},
			{Path: "/docs/getting-started", Title: "Getting started", Description: "Start here", Section: "docs", HasMarkdown: true},
			{Path: "/docs/routing", Title: "Routing", Description: "Routes", Section: "docs", HasMarkdown: true},
			{Path: "/api", Title: "API reference", Description: "API", Section: "api", HasMarkdown: true},
			{Path: "/examples/", Title: "Examples", Description: "Examples index", Section: "examples", HasMarkdown: true},
			{Path: "/examples/jwt", Title: "JWT example", Description: "JWT", Section: "examples", HasMarkdown: true},
			{Path: "/benchmarks", Title: "Benchmarks", Description: "Numbers", Section: "benchmarks", HasMarkdown: true},
			{Path: "/changelog", Title: "Changelog", Description: "History", Section: "changelog", HasMarkdown: true},
			{Path: "/releases/v1.0.0", Title: "v1.0.0", Description: "Release", Section: "releases", HasMarkdown: true},
			{Path: "/security", Title: "Security", Description: "Security", Section: "security", HasMarkdown: true},
			{Path: "/compatibility", Title: "Compatibility", Description: "Compat", Section: "compatibility", HasMarkdown: true},
			{Path: "/contributing", Title: "Contributing", Description: "Contrib", Section: "contributing", HasMarkdown: true},
		},
		Version:   "v1.0.1",
		BaseURL:   "http://localhost:8080",
		BuildTime: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
		Renderer:  r,
	}
}

func TestLandingRecipeProducesValidHTML(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	rec := LandingRecipe("desc", "/static/img/og.png", false)
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !bytes.Contains(body, []byte("<!DOCTYPE html>")) {
		t.Error("missing doctype")
	}
	if !bytes.Contains(body, []byte("<title>")) {
		t.Error("missing title")
	}
	if !bytes.Contains(body, []byte(`<html lang="en">`)) {
		t.Error("missing html lang")
	}
}

func TestDocsIndexRecipeListsDocs(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	rec := DocsIndexRecipe(fixtureLoader(t), "/static/img/og.png", false)
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !bytes.Contains(body, []byte("Getting started")) {
		t.Error("docs index missing getting-started item")
	}
	if !bytes.Contains(body, []byte("/docs/routing")) {
		t.Error("docs index missing routing item")
	}
	// The list block (filtered Items) must not include the /docs/ index
	// itself. It is allowed in the header nav. Assert by counting that
	// the description string for the index page does NOT appear.
	if bytes.Contains(body, []byte("Docs index")) {
		t.Error("docs index lists itself in the items grid; the index page must be filtered out")
	}
}

func TestExamplesIndexRecipeListsExamples(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	rec := ExamplesIndexRecipe(fixtureLoader(t), "/static/img/og.png", false)
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !bytes.Contains(body, []byte("/examples/jwt")) {
		t.Error("examples index missing jwt entry")
	}
}

func TestLLMsRecipeStructure(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	rec := LLMsRecipe()
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"# MuxMaster",
		"## Documentation",
		"## API",
		"## Examples",
		"## Reference",
		"## Optional",
		"http://localhost:8080/docs/",
		"http://localhost:8080/api",
		"github.com/FlavioCFOliveira/MuxMaster",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("/llms.txt missing %q", want)
		}
	}
	// /llms.txt must point at HTML URLs, not .md companions.
	if strings.Contains(s, "/docs/routing.md") {
		t.Error("/llms.txt should not link to .md companions; that is /llms-full.txt")
	}
}

func TestLLMsFullRecipeNavigationLinksUseHTMLURLs(t *testing.T) {
	t.Parallel()
	// Per specification/geo.md § /llms-full.txt: the bundled file MUST NOT
	// list .md companion URLs in its navigation index. Links in the index
	// point at canonical HTML URLs only. The .md companions are reachable
	// via their explicit .md URLs but never through this index.
	deps := fixtureDeps(t)
	loader := fixtureLoader(t)
	mapping := map[string]string{
		"/docs/routing": "docs/routing.md",
		"/api":          "api.md",
		"/examples/jwt": "examples/jwt.md",
	}
	rec := LLMsFullRecipe(loader, mapping)
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	s := string(body)
	// The navigation index sits before the `---` separator. Extract it and
	// ensure no .md links appear there.
	sepIdx := strings.Index(s, "\n---\n")
	if sepIdx < 0 {
		t.Fatalf("/llms-full.txt missing the `---` separator before inlined bodies")
	}
	navIndex := s[:sepIdx]
	for _, badPath := range []string{"/docs/routing.md", "/api.md", "/examples/jwt.md"} {
		if strings.Contains(navIndex, badPath) {
			t.Errorf("/llms-full.txt navigation index must not list %s (HTML URL only)", badPath)
		}
	}
	// And the canonical HTML URL must appear.
	if !strings.Contains(navIndex, "/docs/routing)") {
		t.Error("/llms-full.txt navigation index must link to the canonical HTML URL /docs/routing")
	}
	if !strings.Contains(s, "# Full content") {
		t.Error("/llms-full.txt must carry the inlined-bodies section header")
	}
	// Inlined body of routing.md must appear.
	if !strings.Contains(s, "# Routing") {
		t.Error("/llms-full.txt must inline the body of docs/routing.md")
	}
}

func TestSitemapRecipeConformance(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	rec := SitemapRecipe(fixtureLoader(t), map[string]string{
		"/docs/routing":    "docs/routing.md",
		"/api":             "api.md",
		"/examples/jwt":    "examples/jwt.md",
		"/benchmarks":      "benchmarks.md",
		"/changelog":       "changelog.md",
		"/releases/v1.0.0": "release-notes/v1.0.0.md",
		"/security":        "security.md",
		"/compatibility":   "compatibility.md",
		"/contributing":    "contributing.md",
	}, true)
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	type urlEntry struct {
		Loc        string `xml:"loc"`
		LastMod    string `xml:"lastmod"`
		ChangeFreq string `xml:"changefreq"`
		Priority   string `xml:"priority"`
	}
	type urlSet struct {
		Xmlns string     `xml:"xmlns,attr"`
		URLs  []urlEntry `xml:"url"`
	}
	var set urlSet
	if err := xml.Unmarshal(body, &set); err != nil {
		t.Fatalf("sitemap unmarshal: %v", err)
	}
	if set.Xmlns != "http://www.sitemaps.org/schemas/sitemap/0.9" {
		t.Errorf("xmlns mismatch: %q", set.Xmlns)
	}
	if len(set.URLs) != len(deps.Routes) {
		t.Errorf("urls=%d, want %d", len(set.URLs), len(deps.Routes))
	}
	for _, u := range set.URLs {
		if u.Loc == "" {
			t.Error("empty <loc>")
		}
		if u.LastMod == "" {
			t.Error("empty <lastmod>")
		}
		if u.ChangeFreq == "" {
			t.Errorf("empty <changefreq> for %s", u.Loc)
		}
		if u.Priority == "" {
			t.Errorf("empty <priority> for %s", u.Loc)
		}
		if !strings.HasPrefix(u.Loc, "http://localhost:8080/") {
			t.Errorf("loc %q must use BaseURL", u.Loc)
		}
	}
}

func TestRobotsRecipeListsExpectedBots(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	rec := RobotsRecipe()
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	s := string(body)
	for _, bot := range []string{
		"GPTBot", "ChatGPT-User", "OAI-SearchBot",
		"ClaudeBot", "anthropic-ai",
		"PerplexityBot", "Google-Extended", "Applebot-Extended",
		"CCBot", "Bytespider", "Diffbot",
		"OmgiliBot", "Amazonbot", "meta-externalagent",
	} {
		if !strings.Contains(s, "User-agent: "+bot) {
			t.Errorf("robots.txt missing User-agent: %s", bot)
		}
	}
	if !strings.Contains(s, "Sitemap: http://localhost:8080/sitemap.xml") {
		t.Error("robots.txt missing Sitemap line")
	}
	if !strings.Contains(s, "User-agent: *") {
		t.Error("robots.txt missing wildcard User-agent")
	}
}

func TestNotFoundRecipeReturns404Body(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	rec := NotFoundRecipe("/static/img/og.png", false)
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !bytes.Contains(body, []byte("Page not found")) {
		t.Error("404 body missing 'Page not found'")
	}
	if !bytes.Contains(body, []byte("Error 404")) {
		t.Error("404 body missing 'Error 404' marker")
	}
}

func TestServerErrorRecipeReturns500Body(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	rec := ServerErrorRecipe("/static/img/og.png", false)
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !bytes.Contains(body, []byte("Server error")) {
		t.Error("500 body missing 'Server error'")
	}
	if !bytes.Contains(body, []byte("Error 500")) {
		t.Error("500 body missing 'Error 500' marker")
	}
}

func TestETagStability(t *testing.T) {
	t.Parallel()
	body := []byte("hello world")
	if ETag(body) != ETag(body) {
		t.Fatal("ETag is not deterministic for identical input")
	}
	if ETag(body) == ETag([]byte("hello world!")) {
		t.Fatal("ETag collision on different input")
	}
}

func TestPrerenderRoundTrip(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	recipes := []Recipe{
		LLMsRecipe(),
		SitemapRecipe(fixtureLoader(t), nil, true),
		RobotsRecipe(),
	}
	if err := deps.Renderer.Prerender(recipes, deps); err != nil {
		t.Fatalf("Prerender: %v", err)
	}
	for _, p := range []string{"/llms.txt", "/sitemap.xml", "/robots.txt"} {
		pre, ok := deps.Renderer.Prerendered(p)
		if !ok {
			t.Errorf("missing prerender entry for %s", p)
			continue
		}
		if len(pre.Body) == 0 {
			t.Errorf("empty body for %s", p)
		}
		if pre.ETag == "" {
			t.Errorf("empty etag for %s", p)
		}
	}
}
