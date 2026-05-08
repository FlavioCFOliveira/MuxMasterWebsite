package render

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

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
	rec := DocsIndexRecipe("desc", "/static/img/og.png", false)
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
	rec := ExamplesIndexRecipe("/static/img/og.png", false)
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

func TestLLMsFullRecipePointsAtMarkdown(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	rec := LLMsFullRecipe()
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "/docs/routing.md") {
		t.Error("/llms-full.txt must link to .md companions")
	}
	if !strings.Contains(s, "TODO(spec)") {
		t.Error("/llms-full.txt must carry the TODO marker for the next round")
	}
}

func TestSitemapRecipeConformance(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	rec := SitemapRecipe()
	body, err := rec.Build(deps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	type urlEntry struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
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
		"CCBot", "Bytespider", "DiffBot", "Diffbot",
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
		SitemapRecipe(),
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
