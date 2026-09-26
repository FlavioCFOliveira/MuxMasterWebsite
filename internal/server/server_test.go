package server

import (
	"encoding/json"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf8"

	muxmasterwebsite "github.com/FlavioCFOliveira/MuxMasterWebsite"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/config"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/content"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/render"
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
			bodyMarkers: []string{"Changelog", "1.3.0"},
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
			path:       "/llms-full.txt",
			wantStatus: http.StatusOK,
			wantCT:     "text/plain; charset=utf-8",
			// Navigation index links to canonical HTML URLs (no .md);
			// inlined-body headings use the route path form ("## /docs/
			// routing"). Both are mandated by spec/geo.md § /llms-full.txt
			// and enforced by tasks #14 and #15.
			bodyMarkers: []string{"# MuxMaster", "## /docs/routing", "# Full content"},
		},
		{
			// In the test server (Env=development → productionRobots=false),
			// the sitemap intentionally emits an empty urlset because every
			// page is noindex,nofollow until the canonical domain is
			// ratified (per task #45). Production-mode population is covered
			// by the dedicated render-package test TestSitemapRecipeConfor-
			// mance which calls SitemapRecipe(..., true).
			path:        "/sitemap.xml",
			wantStatus:  http.StatusOK,
			wantCT:      "application/xml; charset=utf-8",
			bodyMarkers: []string{"<urlset"},
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

// TestJSONLDAuditCommentEmitted verifies that the HTML-comment audit
// trail mandated by spec/structured-data.md § Field completeness reaches
// the rendered response. html/template strips HTML comments by default;
// the renderer's jsonldblock template func bypasses that via
// template.HTML so reviewers and validators can see intentional
// omissions (rmp #70).
func TestJSONLDAuditCommentEmitted(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	// /docs/routing has no datePublished front-matter today, so the
	// renderer attaches an "omitted: datePublished on TechArticle ..."
	// audit comment to the article block. The comment must reach the
	// rendered HTML above the corresponding <script> tag.
	resp, err := http.Get(ts.URL + "/docs/routing")
	if err != nil {
		t.Fatalf("GET /docs/routing: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	if !strings.Contains(s, "<!-- omitted: datePublished on TechArticle") {
		t.Errorf("rendered HTML missing JSON-LD audit comment for datePublished omission")
	}
	// And the comment must precede the script tag (sanity-check the
	// adjacency the spec mandates).
	commentIdx := strings.Index(s, "<!-- omitted: datePublished")
	scriptIdx := strings.Index(s[commentIdx:], `<script type="application/ld+json">`)
	if scriptIdx <= 0 {
		t.Errorf("audit comment is not immediately followed by its <script> tag")
	}
}

// TestStaticDirectoryListingsBlocked verifies that the /static handler
// rejects directory paths with 404 rather than serving http.FileServer's
// default HTML index. Directory enumeration would leak the asset surface
// and contradict the strict CSP posture set in middleware.go (rmp #69).
func TestStaticDirectoryListingsBlocked(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	// Every directory under /static/ must return 404, including the root
	// and any sub-directory the fixture happens to expose.
	dirs := []string{
		"/static/",
		"/static/css/",
		"/static/img/",
		"/static/favicon/",
	}
	for _, path := range dirs {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Get(ts.URL + path)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("GET %s: status=%d, want %d (directory listing must be blocked)", path, resp.StatusCode, http.StatusNotFound)
			}
		})
	}

	// The hashed CSS bundle, which is an actual file, must still 200.
	cssPath := srv.renderer.CSSPath()
	t.Run("hashed-css-still-served", func(t *testing.T) {
		resp, err := http.Get(ts.URL + cssPath)
		if err != nil {
			t.Fatalf("GET %s: %v", cssPath, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status=%d, want 200 (real asset must still serve)", cssPath, resp.StatusCode)
		}
	})
}

// TestNormalisationRedirects verifies the URL normalisation 301s required by
// specification/url-and-versioning.md "Redirects".
func TestNormalisationRedirects(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	client := &http.Client{
		// Inhibit auto-follow so we can inspect the 301 directly.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "index.html-root", path: "/index.html", want: "/"},
		{name: "index.html-docs", path: "/docs/index.html", want: "/docs/"},
		{name: "html-suffix", path: "/docs/routing.html", want: "/docs/routing"},
		{name: "mixed-case", path: "/Docs/Routing", want: "/docs/routing"},
		{name: "html+case", path: "/Docs/Routing.html", want: "/docs/routing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusMovedPermanently {
				t.Errorf("status=%d, want 301", resp.StatusCode)
			}
			if got := resp.Header.Get("Location"); got != tc.want {
				t.Errorf("Location=%q, want %q", got, tc.want)
			}
			// Path-only Location: must not include scheme or host.
			if loc := resp.Header.Get("Location"); strings.Contains(loc, "://") {
				t.Errorf("Location must be path-only, got %q", loc)
			}
		})
	}
}

// TestJSONLDPresence verifies the per-family JSON-LD object counts required
// by SEO B1 + GEO B2.
func TestJSONLDPresence(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	cases := []struct {
		path     string
		wantMin  int
		contains []string
	}{
		{path: "/", wantMin: 3, contains: []string{`"WebSite"`, `"SoftwareSourceCode"`, `"Organization"`}},
		{path: "/docs/routing", wantMin: 2, contains: []string{`"TechArticle"`, `"BreadcrumbList"`}},
		{path: "/docs/getting-started", wantMin: 3, contains: []string{`"TechArticle"`, `"HowTo"`}},
		// /api emits TechArticle + BreadcrumbList + APIReference. The
		// APIReference.about slot references SoftwareSourceCode by @id
		// (the canonical /#muxmaster node emitted in full only on /),
		// completing the entity graph without inline redefinition.
		{path: "/api", wantMin: 3, contains: []string{`"TechArticle"`, `"BreadcrumbList"`, `"APIReference"`, `/#muxmaster`}},
		{path: "/docs/", wantMin: 2, contains: []string{`"CollectionPage"`, `"BreadcrumbList"`}},
		{path: "/examples/", wantMin: 2, contains: []string{`"CollectionPage"`}},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			n := strings.Count(string(body), `application/ld+json`)
			if n < tc.wantMin {
				t.Errorf("ld+json blocks=%d, want >=%d", n, tc.wantMin)
			}
			for _, s := range tc.contains {
				if !strings.Contains(string(body), s) {
					t.Errorf("missing JSON-LD marker %q", s)
				}
			}
		})
	}
}

// TestLastUpdatedFooter confirms the doc-page footer renders a Last-updated
// line per GEO B3.
func TestLastUpdatedFooter(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/docs/routing")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Last updated") {
		t.Error("doc-page missing 'Last updated' line")
	}
	if !strings.Contains(string(body), "<time datetime=") {
		t.Error("doc-page missing <time datetime=...> element")
	}
}

// TestMobileDisclosures checks that mobile-only <details> blocks for the
// sidebar and TOC render in the doc-page output (Tailwind B7).
func TestMobileDisclosures(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/docs/routing")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{"Documentation", "On this page"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("doc-page missing mobile disclosure label %q", want)
		}
	}
	// The toggle must be reachable: aria-hidden="true" and tabindex="-1"
	// must NOT appear on the dark-toggle input.
	if strings.Contains(string(body), `id="dark-toggle" class="sr-only" aria-hidden`) {
		t.Error("dark-toggle still has aria-hidden — keyboard inaccessible")
	}
}

// TestNoCanonicalOn404 verifies SEO B7: the 404 page must NOT carry a
// canonical link.
func TestNoCanonicalOn404(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/no-such-page")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), `rel="canonical"`) {
		t.Error("404 page must not emit a canonical link")
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
	if !strings.Contains(string(body), "v1.3.0") {
		t.Errorf("landing chrome missing version label v1.3.0; body[:600]=%s", truncate(string(body), 600))
	}
}

// TestTOCAnchorsResolve enforces the property that motivates the new HTML-side
// heading extraction: every TOC link's `href="#id"` MUST land on an `id="id"`
// that exists somewhere in the rendered body. Run for every doc-page route at
// boot — this catches drift across the whole corpus, not just the fixture.
func TestTOCAnchorsResolve(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	tocHrefRE := regexp.MustCompile(`href="#([^"]+)"`)
	idAttrRE := regexp.MustCompile(`id="([^"]+)"`)

	for _, ri := range routeInfos() {
		ri := ri
		// Only HTML doc-family pages emit a TOC. The landing page and
		// section indexes don't.
		if !ri.HasMarkdown {
			continue
		}
		t.Run(ri.Path, func(t *testing.T) {
			resp, err := http.Get(ts.URL + ri.Path)
			if err != nil {
				t.Fatalf("GET %s: %v", ri.Path, err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			s := string(body)

			// Collect every fragment href and every id attribute on the
			// page. The TOC's anchors must be a subset of the body's ids.
			ids := make(map[string]struct{})
			for _, m := range idAttrRE.FindAllStringSubmatch(s, -1) {
				ids[m[1]] = struct{}{}
			}
			for _, m := range tocHrefRE.FindAllStringSubmatch(s, -1) {
				frag := m[1]
				if frag == "" || frag == "main" {
					continue
				}
				if _, ok := ids[frag]; !ok {
					t.Errorf("TOC anchor #%s has no matching id in body", frag)
				}
			}
		})
	}
}

// TestExamplesIndexCuratedOrder verifies /examples/ lists the eight examples
// in the curated learning order (REST → Authn → JWT → OAuth2 → Cache →
// Graceful shutdown → SSR → Static site), not alphabetical.
func TestExamplesIndexCuratedOrder(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/examples/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	want := []string{
		"/examples/rest-api",
		"/examples/authn",
		"/examples/jwt",
		"/examples/oauth2",
		"/examples/cache",
		"/examples/graceful-shutdown",
		"/examples/server-side-render",
		"/examples/static-site",
	}
	prev := 0
	for _, p := range want {
		idx := strings.Index(s, p)
		if idx < 0 {
			t.Fatalf("/examples/ missing %q", p)
		}
		if idx < prev {
			t.Errorf("/examples/ out of curated order: %q at %d, previous at %d", p, idx, prev)
		}
		prev = idx
	}
}

// TestReferenceSidebarOnNonDocsPage verifies that a reference page outside
// /docs/ (here: /security) renders the curated Reference sidebar with all
// seven sibling links — the fix that closes UX H#1's "no lateral navigation"
// gap on non-/docs/ pages.
func TestReferenceSidebarOnNonDocsPage(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/security")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	if !strings.Contains(s, `aria-label="Reference"`) {
		t.Error("/security missing Reference sidebar")
	}
	for _, link := range []string{
		"/api", "/examples/", "/benchmarks", "/changelog",
		"/releases/v1.0.0", "/security", "/compatibility", "/contributing",
	} {
		if !strings.Contains(s, `href="`+link+`"`) {
			t.Errorf("/security Reference sidebar missing href=%q", link)
		}
	}
}

// TestExamplesSidebarAndPrevNext verifies that an /examples/<name> page
// renders the curated Examples sidebar and a prev/next pair pointing at the
// curated neighbours (not alphabetical).
func TestExamplesSidebarAndPrevNext(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	// /examples/jwt sits between /examples/authn and /examples/oauth2 in
	// the curated order.
	resp, err := http.Get(ts.URL + "/examples/jwt")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	if !strings.Contains(s, `aria-label="Examples"`) {
		t.Error("/examples/jwt missing Examples sidebar")
	}
	for _, link := range []string{
		"/examples/rest-api", "/examples/authn", "/examples/jwt",
		"/examples/oauth2", "/examples/cache", "/examples/graceful-shutdown",
		"/examples/server-side-render", "/examples/static-site",
	} {
		if !strings.Contains(s, `href="`+link+`"`) {
			t.Errorf("/examples/jwt Examples sidebar missing href=%q", link)
		}
	}
	// Prev/next must point at curated neighbours.
	if !strings.Contains(s, `href="/examples/authn"`) {
		t.Error("/examples/jwt missing prev=/examples/authn")
	}
	if !strings.Contains(s, `href="/examples/oauth2"`) {
		t.Error("/examples/jwt missing next=/examples/oauth2")
	}
}

// TestAPIPageTOCDepth pins UX H#8: the /api page must surface a non-trivial
// in-page TOC. We assert at least two entries (the two top-level packages)
// and that the page renders an "On this page" panel — H3 inclusion is wired
// through the renderer so any future H3s in api.md will appear automatically.
//
// NOTE(curator): content/api.md currently uses H1 for second-level sections
// (Quick start, Route patterns, Middleware, Performance, Compatibility) and
// H2 for the two package roots. To take full advantage of the H2/H3 TOC the
// reviewer asked for, those H1s should be re-keyed to H2 and the per-symbol
// blocks promoted to H3 in a future curator pass.
func TestAPIPageTOCDepth(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	if !strings.Contains(s, "On this page") {
		t.Error("/api missing 'On this page' TOC label")
	}
	// The TOC must include at least one entry per H2 in the source. The
	// fixture exposes one H2 ("Overview"); the live api.md exposes the
	// two package H2s. Either way >= 1 entry must render.
	tocAnchors := strings.Count(s, `<a href="#`)
	if tocAnchors < 1 {
		t.Errorf("/api TOC has %d anchors, want >= 1", tocAnchors)
	}
}

// TestDocPageH3InTOC verifies that H3 entries actually appear in the in-page
// TOC. The fixture's docs/configuration.md replacement is enriched with one
// H3 specifically to exercise this path; the property under test is "if the
// source has H3, the TOC has H3" — independent of any one corpus.
func TestDocPageH3InTOC(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/docs/configuration")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	// The fixture body for docs/configuration includes a "### Sub option"
	// heading. Goldmark slugifies it to "sub-option".
	if !strings.Contains(s, `href="#sub-option"`) {
		t.Error("/docs/configuration TOC missing H3 anchor #sub-option")
	}
	if !strings.Contains(s, `id="sub-option"`) {
		t.Error("/docs/configuration body missing matching id=sub-option")
	}
}

// newTestServer constructs a Server backed by an in-memory content tree and
// runs Prerender. It does NOT bind a real socket — callers wrap
// srv.httpServer.Handler in httptest.
func newTestServer(t testing.TB) *Server {
	t.Helper()
	loader := buildFixtureLoader(t)

	cfg := &config.Config{
		Port:        0,
		SiteBaseURL: "http://localhost",
		LogLevel:    slog.LevelError,
		Env:         config.EnvDevelopment,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	srv, err := New(cfg, logger, loader, "v1.3.0", "../../templates", "../../static")
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
func buildFixtureLoader(t testing.TB) *content.Loader {
	t.Helper()

	files := map[string]string{
		"changelog.md":     "# Changelog\n\n## [1.3.0] - 2026-09-26\n\nLatest.\n\n## [1.2.0] - 2026-09-26\n\nQUERY.\n\n## v1.0.0\n\nInitial.\n",
		"api.md":           "# API\n\n## Overview\n\nReference.\n",
		"compatibility.md": "# Compatibility\n\n## Versions\n\nText.\n",
		"security.md":      "# Security\n\n## Policy\n\nText.\n",
		"contributing.md":  "# Contributing\n\n## How to\n\nText.\n",
		"benchmarks.md": "# Benchmarks\n\n## Numbers\n\n" +
			"| Route | ns/op |\n|---|---|\n| Static | 25 |\n\n" +
			"## Source\n\nText.\n",
		"docs/getting-started.md": "# Getting started\n\n## Install\n\nFirst install MuxMaster.\n\n## Step 1 — Hello, World\n\nWrite the simplest handler.\n\n## Step 2 — Path Parameters\n\nAdd a parametric route.\n",
		"docs/routing.md": "# Routing\n\n## Patterns\n\nText.\n\n## Priority\n\nText.\n\n" +
			"```go\nfunc Handler() {}\n```\n",
		"docs/http-query-method.md": "# HTTP QUERY method (RFC 10008)\n\nThe HTTP QUERY method is safe and idempotent.\n\n" +
			"## Step 1 — Register a QUERY route\n\nRegister the route.\n\n```go\nmux.QUERY(\"/search\", h)\n```\n\n" +
			"## Step 2 — Validate the request content\n\nValidate the body.\n\n" +
			"## Step 3 — Call the QUERY route with curl\n\nSend the request.\n\n" +
			"## Common questions\n\n<section data-conversation=\"query\">\n\n" +
			"### Does MuxMaster support the HTTP QUERY method?\n\nYes, since v1.2.0.\n\n" +
			"### How do I register a QUERY route in MuxMaster?\n\nCall mux.QUERY.\n\n" +
			"### Does MuxMaster validate the body of a QUERY request?\n\nNo.\n\n</section>\n\n## Sources\n\nText.\n",
		"docs/groups.md":                 "# Groups\n",
		"docs/middleware.md":             "# Middleware\n",
		"docs/error-handling.md":         "# Error handling\n",
		"docs/configuration.md":          "# Configuration\n\n## Options\n\nText.\n\n### Sub option\n\nDetail.\n",
		"docs/response-helpers.md":       "# Response helpers\n",
		"docs/performance.md":            "# Performance\n",
		"docs/max-performance.md":        "# Maximum performance\n",
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
		"site/built-with-muxmaster.md":   "# Built with MuxMaster\n\n## Common questions\n\nText.\n",
		"examples/versioning.md":         "# Versioning\n",
		"examples/reverse-proxy.md":      "# Reverse proxy\n",
		"examples/server-sent-events.md": "# Server-sent events\n",
		"examples/upload-file.md":        "# Upload file\n",
		"examples/max-performance.md":    "# Maximum performance\n",
		"release-notes/v1.0.0.md":        "# Release notes — v1.0.0\n\n## Highlights\n\nText.\n",
		"release-notes/v1.1.0.md":        "# Release notes — v1.1.0\n\n## Highlights\n\nText.\n",
		"release-notes/v1.2.0.md":        "# Release notes — v1.2.0\n\n## Highlights\n\nQUERY.\n",
		"release-notes/v1.3.0.md":        "# Release notes — v1.3.0\n\n## Highlights\n\nGo 1.27.1.\n",
		// site/landing.md is required by LandingMarkdownRecipe (the
		// /index.md companion source).
		"site/landing.md": "# MuxMaster\n\nA radix-tree HTTP router for Go.\n",
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

// newRealContentServer boots a Server on the embedded /content/ corpus that
// ships in the binary, so the assertions below check the published pages,
// not a fixture. env selects production (populated sitemap, indexable
// pages) or development.
func newRealContentServer(t testing.TB, env config.Env) (*Server, *content.Loader) {
	t.Helper()
	root, err := muxmasterwebsite.ContentFS()
	if err != nil {
		t.Fatalf("ContentFS: %v", err)
	}
	loader, err := content.NewLoader(root)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	if err := loader.Verify(); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	version, err := loader.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	cfg := &config.Config{
		Port:        0,
		SiteBaseURL: "https://muxmaster.net",
		LogLevel:    slog.LevelError,
		Env:         env,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	srv, err := New(cfg, logger, loader, version, "../../templates", "../../static")
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	if err := srv.Prerender(); err != nil {
		t.Fatalf("Prerender: %v", err)
	}
	return srv, loader
}

// getBody fetches path from ts and fails the test unless the status is 200.
func getBody(t *testing.T, ts *httptest.Server, path string) string {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	// A close error on an in-memory test response carries no signal.
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", path, resp.StatusCode)
	}
	return string(body)
}

// perfNumberRE matches a performance figure: a number followed by a time
// unit, a percentage, a per-operation unit, an allocation count, or a
// speed-up factor. Space-limited surfaces MUST NOT carry one
// (specification/overview.md INT-PERF-7).
var perfNumberRE = regexp.MustCompile(`\d[\d.,]*\s?(ns|µs|ms|%|B/op|allocs?\b|×)|\d[\d.]*x\b`)

var (
	metaDescRE    = regexp.MustCompile(`<meta name="description" content="([^"]*)">`)
	ogDescRE      = regexp.MustCompile(`<meta property="og:description" content="([^"]*)">`)
	twitterDescRE = regexp.MustCompile(`<meta name="twitter:description" content="([^"]*)">`)
	titleRE       = regexp.MustCompile(`<title>([^<]*)</title>`)
)

// TestPublishedVersionFacts pins URL-VER-1 and SEO-VER-1: the label read
// from the embedded changelog is v1.3.0, and the chrome, the footer, and
// the landing page state v1.3.0 and Go 1.27.1.
func TestPublishedVersionFacts(t *testing.T) {
	t.Parallel()
	srv, loader := newRealContentServer(t, config.EnvDevelopment)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	if v, err := loader.Version(); err != nil || v != "v1.3.0" {
		t.Fatalf("Version()=%q, %v; want v1.3.0", v, err)
	}
	landing := getBody(t, ts, "/")
	for _, want := range []string{
		`aria-label="MuxMaster version">v1.3.0<`,
		"Requires Go&nbsp;1.27.1 or later.",
		`href="/releases/v1.3.0"`,
		`href="/docs/http-query-method"`,
	} {
		if !strings.Contains(landing, want) {
			t.Errorf("landing page missing %q", want)
		}
	}
	for _, stale := range []string{"Go&nbsp;1.26", "Go 1.26", "v1.1.0 Pooled", "fastest Go HTTP router"} {
		if strings.Contains(landing, stale) {
			t.Errorf("landing page still contains %q", stale)
		}
	}
}

// TestMetaDescriptionsConform checks every indexable HTML route of the
// published site: the <meta name="description"> is unique, 110 to 160
// characters long (specification/seo.md), carries no performance number
// (INT-PERF-7), and equals the Open Graph and Twitter descriptions.
func TestMetaDescriptionsConform(t *testing.T) {
	t.Parallel()
	srv, _ := newRealContentServer(t, config.EnvDevelopment)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	seen := map[string]string{}
	for _, ri := range routeInfos() {
		body := getBody(t, ts, ri.Path)
		m := metaDescRE.FindStringSubmatch(body)
		if m == nil {
			t.Errorf("%s: no meta description", ri.Path)
			continue
		}
		desc := html.UnescapeString(m[1])
		if n := utf8.RuneCountInString(desc); n < 110 || n > 160 {
			t.Errorf("%s: meta description is %d characters, want 110-160: %q", ri.Path, n, desc)
		}
		if loc := perfNumberRE.FindString(desc); loc != "" {
			t.Errorf("%s: meta description carries performance number %q: %q", ri.Path, loc, desc)
		}
		if other, dup := seen[desc]; dup {
			t.Errorf("%s: meta description duplicates %s", ri.Path, other)
		}
		seen[desc] = ri.Path
		for name, re := range map[string]*regexp.Regexp{"og:description": ogDescRE, "twitter:description": twitterDescRE} {
			if mm := re.FindStringSubmatch(body); mm == nil || html.UnescapeString(mm[1]) != desc {
				t.Errorf("%s: %s does not equal the meta description", ri.Path, name)
			}
		}
	}
}

// TestLandingDescriptionRules pins SEO-LAND-1: the landing description
// names the HTTP QUERY method (RFC 10008) and positions MuxMaster in words.
func TestLandingDescriptionRules(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"HTTP QUERY method (RFC 10008)", "static routes", "httprouter"} {
		if !strings.Contains(LandingDescription, want) {
			t.Errorf("LandingDescription missing %q", want)
		}
	}
}

// TestQueryMethodPage pins IA-QUERY-1/2, SEO-QUERY-1, and SD-QUERY-1 on the
// published /docs/http-query-method page.
func TestQueryMethodPage(t *testing.T) {
	t.Parallel()
	srv, _ := newRealContentServer(t, config.EnvDevelopment)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	body := getBody(t, ts, "/docs/http-query-method")
	if m := titleRE.FindStringSubmatch(body); m == nil || html.UnescapeString(m[1]) != "HTTP QUERY method (RFC 10008) in Go — MuxMaster" {
		t.Errorf("<title>=%v, want HTTP QUERY method (RFC 10008) in Go — MuxMaster", m)
	}
	// SD-QUERY-2: the HowTo name is fixed.
	var howToName string
	for _, raw := range extractJSONLDBlocks(body) {
		var doc struct {
			Type string `json:"@type"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(raw), &doc); err == nil && doc.Type == "HowTo" {
			howToName = doc.Name
		}
	}
	if howToName != "How to serve the HTTP QUERY method (RFC 10008) with MuxMaster" {
		t.Errorf("HowTo.name=%q", howToName)
	}
	desc := html.UnescapeString(metaDescRE.FindStringSubmatch(body)[1])
	for _, want := range []string{"HTTP QUERY method", "RFC 10008", "MuxMaster", "safe", "idempotent", "body"} {
		if !strings.Contains(desc, want) {
			t.Errorf("meta description missing %q: %q", want, desc)
		}
	}
	if strings.Count(body, "<h1") != 1 {
		t.Errorf("want exactly one <h1>")
	}

	// Sidebar order and prev/next (information-architecture.md "Sidebar").
	wantSidebar := []string{
		"/docs/getting-started", "/docs/routing", "/docs/http-query-method", "/docs/groups",
		"/docs/middleware", "/docs/error-handling", "/docs/configuration", "/docs/response-helpers",
		"/docs/performance", "/docs/max-performance", "/docs/observability", "/docs/migration", "/docs/cookbook",
	}
	prev := -1
	for _, p := range wantSidebar {
		idx := strings.Index(body, `href="`+p+`"`)
		if idx < 0 {
			t.Fatalf("sidebar missing %s", p)
		}
		if idx < prev {
			t.Errorf("sidebar out of order at %s", p)
		}
		prev = idx
	}
	if !strings.Contains(body, `aria-current="page"`) {
		t.Error("active sidebar entry lacks aria-current")
	}
	routing := getBody(t, ts, "/docs/routing")
	groups := getBody(t, ts, "/docs/groups")
	anchor := "HTTP QUERY method (RFC 10008)"
	if !strings.Contains(routing, anchor) || !strings.Contains(groups, anchor) {
		t.Error("prev/next neighbours do not link to the QUERY page with its title as anchor text")
	}

	// JSON-LD (SD-QUERY-1).
	var faq, howto, article map[string]any
	for _, raw := range extractJSONLDBlocks(body) {
		var doc map[string]any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatalf("invalid JSON-LD: %v", err)
		}
		switch doc["@type"] {
		case "FAQPage":
			faq = doc
		case "HowTo":
			howto = doc
		case "TechArticle":
			article = doc
		}
	}
	if faq == nil || howto == nil || article == nil {
		t.Fatalf("missing JSON-LD: FAQPage=%v HowTo=%v TechArticle=%v", faq != nil, howto != nil, article != nil)
	}
	qs, _ := faq["mainEntity"].([]any)
	if len(qs) < 5 {
		t.Errorf("FAQPage has %d questions, want >= 5 (GEO-QUERY-4)", len(qs))
	}
	if len(qs) > 0 {
		if first, _ := qs[0].(map[string]any)["name"].(string); first != "Does MuxMaster support the HTTP QUERY method?" {
			t.Errorf("first FAQ question = %q", first)
		}
	}
	steps, _ := howto["step"].([]any)
	if len(steps) != 3 {
		t.Errorf("HowTo has %d steps, want 3", len(steps))
	}
	for i, st := range steps {
		u, _ := st.(map[string]any)["url"].(string)
		frag, ok := strings.CutPrefix(u, "https://muxmaster.net/docs/http-query-method#")
		if !ok || frag == "" || !strings.Contains(body, `id="`+frag+`"`) {
			t.Errorf("HowTo step %d url %q does not resolve to an anchor on the page", i+1, u)
		}
	}
	if about, _ := article["about"].(map[string]any); about["@id"] != "https://muxmaster.net/#muxmaster" {
		t.Errorf("TechArticle.about = %v, want the SoftwareSourceCode @id", article["about"])
	}

	md := getBody(t, ts, "/docs/http-query-method.md")
	if !strings.HasPrefix(strings.TrimSpace(md), "# HTTP QUERY method (RFC 10008)") {
		t.Errorf("markdown companion does not start with the page title: %q", truncate(md, 80))
	}
}

// TestReleasePages pins URL-REL-1 and SEO-REL-1 and the footer target.
func TestReleasePages(t *testing.T) {
	t.Parallel()
	srv, _ := newRealContentServer(t, config.EnvDevelopment)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	cases := []struct{ path, title, mustMention string }{
		{"/releases/v1.3.0", "MuxMaster v1.3.0 release notes", "Go 1.27.1"},
		{"/releases/v1.2.0", "MuxMaster v1.2.0 release notes", "HTTP QUERY method (RFC 10008)"},
	}
	// SEO-REL-1 titles and IA-BC-1/2 breadcrumbs on every release page.
	crumbRE := regexp.MustCompile(`(?s)<nav aria-label="Breadcrumb"[^>]*>(.*?)</nav>`)
	for _, v := range []string{"1.3.0", "1.2.0", "1.1.0", "1.0.0"} {
		path := "/releases/v" + v
		body := getBody(t, ts, path)
		if m := titleRE.FindStringSubmatch(body); m == nil || html.UnescapeString(m[1]) != "MuxMaster v"+v+" release notes" {
			t.Errorf("%s: <title>=%v", path, m)
		}
		nav := crumbRE.FindStringSubmatch(body)
		if nav == nil {
			t.Errorf("%s: no breadcrumb", path)
			continue
		}
		if !strings.Contains(nav[1], `<a href="/" `) || !strings.Contains(nav[1], `<a href="/changelog" `) || !strings.Contains(nav[1], ">Changelog</a>") {
			t.Errorf("%s: breadcrumb is not Home / Changelog / release: %s", path, nav[1])
		}
		if strings.Count(nav[1], "aria-current") != 1 || !regexp.MustCompile(`aria-current="page">[^<]*v`+regexp.QuoteMeta(v)+`</span>\s*</li>\s*</ol>`).MatchString(nav[1]) {
			t.Errorf("%s: aria-current must be on the last crumb only: %s", path, nav[1])
		}
	}
	for _, tc := range cases {
		body := getBody(t, ts, tc.path)
		if m := titleRE.FindStringSubmatch(body); m == nil || html.UnescapeString(m[1]) != tc.title {
			t.Errorf("%s: <title>=%v, want %q", tc.path, m, tc.title)
		}
		desc := html.UnescapeString(metaDescRE.FindStringSubmatch(body)[1])
		if !strings.Contains(desc, tc.mustMention) {
			t.Errorf("%s: description %q does not mention %q", tc.path, desc, tc.mustMention)
		}
		if !strings.Contains(body, `"version":"`+strings.TrimPrefix(tc.path, "/releases/v")+`"`) {
			t.Errorf("%s: TechArticle.version missing", tc.path)
		}
		md := getBody(t, ts, tc.path+".md")
		if !strings.Contains(md, "Release Notes") {
			t.Errorf("%s.md: unexpected companion body %q", tc.path, truncate(md, 80))
		}
		sec := getBody(t, ts, "/security")
		if !strings.Contains(sec, `href="`+tc.path+`"`) {
			t.Errorf("reference sidebar missing %s", tc.path)
		}
	}
	if footer := getBody(t, ts, "/security"); !strings.Contains(footer, `href="/releases/v1.3.0" class="inline-flex`) {
		t.Error("footer Releases link does not target /releases/v1.3.0")
	}
}

// TestLLMsTxtConformance pins GEO-QUERY-1/2, GEO-PERF-1, and the Reference
// section of /llms.txt and /llms-full.txt on the published corpus.
func TestLLMsTxtConformance(t *testing.T) {
	t.Parallel()
	srv, _ := newRealContentServer(t, config.EnvDevelopment)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	llms := getBody(t, ts, "/llms.txt")
	full := getBody(t, ts, "/llms-full.txt")
	nav, _, ok := strings.Cut(full, "\n---\n")
	if !ok {
		t.Fatal("/llms-full.txt has no separator")
	}
	for name, idx := range map[string]string{"/llms.txt": llms, "/llms-full.txt index": nav} {
		for _, want := range []string{
			"It supports the HTTP QUERY method (RFC 10008) with `MethodQuery`, `Mux.QUERY`, and `Group.QUERY`.",
			"It requires Go 1.27.1 or later.",
			"(https://muxmaster.net/docs/http-query-method): MuxMaster supports the HTTP QUERY method (RFC 10008), a safe, idempotent method",
			"(https://muxmaster.net/benchmarks)",
			"(https://muxmaster.net/releases/v1.3.0)",
			"(https://muxmaster.net/releases/v1.2.0)",
		} {
			if !strings.Contains(idx, want) {
				t.Errorf("%s missing %q", name, want)
			}
		}
		// Every line of the index (blurb and entries) is space-limited.
		for _, line := range strings.Split(idx, "\n") {
			// "100% compatibility with net/http" is mandated verbatim by
			// geo.md GEO-QUERY-1 and is not a performance figure.
			line = strings.ReplaceAll(line, "100% compatibility", "")
			if loc := perfNumberRE.FindString(line); loc != "" {
				t.Errorf("%s line carries performance number %q: %q", name, loc, line)
			}
		}
		order := []string{"/releases/v1.3.0)", "/releases/v1.2.0)", "/releases/v1.1.0)", "/releases/v1.0.0)"}
		last := -1
		for _, o := range order {
			i := strings.Index(idx, o)
			if i < last {
				t.Errorf("%s: release notes not listed newest first at %s", name, o)
			}
			last = i
		}
	}
	for _, heading := range []string{"## /docs/http-query-method\n", "## /releases/v1.3.0\n", "## /releases/v1.2.0\n", "## /benchmarks\n"} {
		if !strings.Contains(full, heading) {
			t.Errorf("/llms-full.txt does not inline %q", strings.TrimSpace(heading))
		}
	}
}

// TestSitemapListsNewRoutes checks, in production mode, that the sitemap
// lists the routes added for v1.3.0 with a lastmod and the priority of
// their family.
func TestSitemapListsNewRoutes(t *testing.T) {
	t.Parallel()
	srv, _ := newRealContentServer(t, config.EnvProduction)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	sm := getBody(t, ts, "/sitemap.xml")
	for loc, prio := range map[string]string{
		"https://muxmaster.net/docs/http-query-method": "0.6",
		"https://muxmaster.net/releases/v1.2.0":        "0.4",
		"https://muxmaster.net/releases/v1.3.0":        "0.4",
	} {
		entryRE := regexp.MustCompile(`<loc>` + regexp.QuoteMeta(loc) + `</loc>\s*<lastmod>(\d{4}-\d{2}-\d{2})</lastmod>\s*<changefreq>[a-z]+</changefreq>\s*<priority>` + regexp.QuoteMeta(prio) + `</priority>`)
		if !entryRE.MatchString(sm) {
			t.Errorf("sitemap missing a complete entry for %s (priority %s)", loc, prio)
		}
	}
	// SEO-MAP-1: lastmod is the later front-matter date of the route's file.
	for loc, date := range map[string]string{
		"https://muxmaster.net/docs/http-query-method": "2026-09-26",
		"https://muxmaster.net/releases/v1.0.0":        "2026-05-08",
		"https://muxmaster.net/":                       "2026-09-26",
		"https://muxmaster.net/docs/":                  "2026-09-26",
		"https://muxmaster.net/examples/":              "2026-09-26",
	} {
		if !strings.Contains(sm, "<loc>"+loc+"</loc>\n    <lastmod>"+date+"</lastmod>") {
			t.Errorf("sitemap lastmod of %s is not %s", loc, date)
		}
	}
	robots := getBody(t, ts, "/robots.txt")
	if !strings.Contains(robots, "Sitemap: https://muxmaster.net/sitemap.xml") {
		t.Error("robots.txt does not reference the sitemap")
	}
}

// TestLandingJSONLDFacts pins SD-LAND-1 and URL-VER-1 on the landing
// SoftwareSourceCode node.
func TestLandingJSONLDFacts(t *testing.T) {
	t.Parallel()
	srv, _ := newRealContentServer(t, config.EnvDevelopment)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	var sw map[string]any
	for _, raw := range extractJSONLDBlocks(getBody(t, ts, "/")) {
		var doc map[string]any
		if err := json.Unmarshal([]byte(raw), &doc); err == nil && doc["@type"] == "SoftwareSourceCode" {
			sw = doc
		}
	}
	if sw == nil {
		t.Fatal("no SoftwareSourceCode on /")
	}
	if sw["version"] != "1.3.0" || sw["runtimePlatform"] != "Go 1.27.1" || sw["programmingLanguage"] != "Go" {
		t.Errorf("SoftwareSourceCode version=%v runtimePlatform=%v programmingLanguage=%v", sw["version"], sw["runtimePlatform"], sw["programmingLanguage"])
	}
	desc, _ := sw["description"].(string)
	if !strings.Contains(desc, "HTTP QUERY method (RFC 10008)") {
		t.Errorf("SoftwareSourceCode.description does not state QUERY support: %q", desc)
	}
	if loc := perfNumberRE.FindString(desc); loc != "" {
		t.Errorf("SoftwareSourceCode.description carries performance number %q", loc)
	}
}

// TestBenchmarksDataset pins SD-BENCH-1: the Dataset describes only the
// campaign archive, pinned to render.CampaignArchiveCommit.
func TestBenchmarksDataset(t *testing.T) {
	t.Parallel()
	if c := render.CampaignArchiveCommit; c != "26abbe6c1cf2f4c9c16af45f4c02377a685f352d" && !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(c) {
		t.Fatalf("CampaignArchiveCommit=%q is neither the placeholder nor a full commit SHA", c)
	}
	srv, _ := newRealContentServer(t, config.EnvDevelopment)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	body := getBody(t, ts, "/benchmarks")
	var ds map[string]any
	for _, raw := range extractJSONLDBlocks(body) {
		var doc map[string]any
		if err := json.Unmarshal([]byte(raw), &doc); err == nil && doc["@type"] == "Dataset" {
			ds = doc
		}
	}
	if ds == nil {
		t.Fatal("no Dataset on /benchmarks")
	}
	if ds["temporalCoverage"] != "2026-09-26" {
		t.Errorf("temporalCoverage=%v", ds["temporalCoverage"])
	}
	dist, _ := ds["distribution"].([]any)
	if len(dist) == 0 {
		t.Fatal("empty distribution")
	}
	prefix := "https://raw.githubusercontent.com/FlavioCFOliveira/MuxMasterWebsite/" + render.CampaignArchiveCommit + "/" + render.CampaignArchiveDir + "/"
	for _, d := range dist {
		u, _ := d.(map[string]any)["contentUrl"].(string)
		rel, ok := strings.CutPrefix(u, prefix)
		if !ok {
			t.Errorf("distribution URL %q is not pinned to the campaign archive", u)
			continue
		}
		// The file must exist in the archive this test runs against.
		if _, err := os.Stat(filepath.Join("..", "..", render.CampaignArchiveDir, rel)); err != nil {
			t.Errorf("distribution file %s: %v", rel, err)
		}
	}
	// SD-BENCH-2.
	if ds["name"] != "MuxMaster v1.3.0 router benchmark campaign, 2026-09-26" {
		t.Errorf("Dataset.name=%v", ds["name"])
	}
	if ds["measurementTechnique"] != "go test -bench, -count=10, benchstat, alpha = 0.05" {
		t.Errorf("Dataset.measurementTechnique=%v", ds["measurementTechnique"])
	}
	if kw, _ := ds["keywords"].([]any); len(kw) == 0 {
		t.Error("Dataset.keywords is empty")
	}
	if ds["isAccessibleForFree"] != true {
		t.Errorf("Dataset.isAccessibleForFree=%v", ds["isAccessibleForFree"])
	}
	// TechArticle.about references the SoftwareSourceCode node.
	var aboutOK bool
	for _, raw := range extractJSONLDBlocks(body) {
		var doc struct {
			Type  string `json:"@type"`
			About struct {
				ID string `json:"@id"`
			} `json:"about"`
		}
		if err := json.Unmarshal([]byte(raw), &doc); err == nil && doc.Type == "TechArticle" {
			aboutOK = strings.HasSuffix(doc.About.ID, "/#muxmaster")
		}
	}
	if !aboutOK {
		t.Error("/benchmarks TechArticle.about does not reference #muxmaster")
	}
	desc := html.UnescapeString(metaDescRE.FindStringSubmatch(body)[1])
	for _, want := range []string{"v1.3.0", "v1.1.0", "httprouter"} {
		if !strings.Contains(desc, want) {
			t.Errorf("/benchmarks description missing %q (SEO-BENCH-1): %q", want, desc)
		}
	}
}

// TestLandingFAQHTMLAndJSONLDAgree enforces the invariant documented on
// landingFAQEntries: the FAQ rendered on / and the FAQPage JSON-LD carry
// the same questions and answers, in the same order.
func TestLandingFAQHTMLAndJSONLDAgree(t *testing.T) {
	t.Parallel()
	srv, _ := newRealContentServer(t, config.EnvDevelopment)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	body := getBody(t, ts, "/")
	sec := regexp.MustCompile(`(?s)<div data-conversation="landing-faq"[^>]*>(.*?)</div>\s*</section>`).FindStringSubmatch(body)
	if sec == nil {
		t.Fatal("landing FAQ section not found")
	}
	artRE := regexp.MustCompile(`(?s)<article>\s*<h3[^>]*>(.*?)</h3>(.*?)</article>`)
	tagRE := regexp.MustCompile(`<[^>]+>`)
	spaceBeforePunctRE := regexp.MustCompile(`\s+([?.,;:)])`)
	plain := func(s string) string {
		s = tagRE.ReplaceAllString(s, " ")
		s = strings.Join(strings.Fields(html.UnescapeString(s)), " ")
		return spaceBeforePunctRE.ReplaceAllString(s, "$1")
	}
	var htmlQA [][2]string
	for _, m := range artRE.FindAllStringSubmatch(sec[1], -1) {
		htmlQA = append(htmlQA, [2]string{plain(m[1]), plain(m[2])})
	}
	var jsonQA [][2]string
	for _, raw := range extractJSONLDBlocks(body) {
		var doc struct {
			Type       string `json:"@type"`
			MainEntity []struct {
				Name           string `json:"name"`
				AcceptedAnswer struct {
					Text string `json:"text"`
				} `json:"acceptedAnswer"`
			} `json:"mainEntity"`
		}
		if err := json.Unmarshal([]byte(raw), &doc); err != nil || doc.Type != "FAQPage" {
			continue
		}
		for _, q := range doc.MainEntity {
			jsonQA = append(jsonQA, [2]string{plain(q.Name), plain(q.AcceptedAnswer.Text)})
		}
	}
	if len(htmlQA) == 0 || len(htmlQA) != len(jsonQA) {
		t.Fatalf("HTML FAQ has %d pairs, JSON-LD has %d", len(htmlQA), len(jsonQA))
	}
	for i := range htmlQA {
		if htmlQA[i] != jsonQA[i] {
			t.Errorf("pair %d differs:\nHTML: %q\nJSON: %q", i+1, htmlQA[i], jsonQA[i])
		}
	}
}

// TestStructuredTextAndBreadcrumbIDs checks every HTML route: FAQPage
// answers are plain text with no whitespace before closing punctuation or
// after an opening parenthesis (SD-TEXT-1, SD-TEXT-2), and every
// BreadcrumbList position carries a distinct @id (IA-BC-1).
func TestStructuredTextAndBreadcrumbIDs(t *testing.T) {
	t.Parallel()
	srv, _ := newRealContentServer(t, config.EnvDevelopment)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	answers := 0
	for _, ri := range routeInfos() {
		for _, raw := range extractJSONLDBlocks(getBody(t, ts, ri.Path)) {
			var doc struct {
				Type       string `json:"@type"`
				MainEntity []struct {
					AcceptedAnswer struct {
						Text string `json:"text"`
					} `json:"acceptedAnswer"`
				} `json:"mainEntity"`
				Elements []struct {
					Item struct {
						ID string `json:"@id"`
					} `json:"item"`
				} `json:"itemListElement"`
			}
			if err := json.Unmarshal([]byte(raw), &doc); err != nil {
				continue
			}
			switch doc.Type {
			case "FAQPage":
				for _, q := range doc.MainEntity {
					a := q.AcceptedAnswer.Text
					answers++
					if htmlMarkupRE.MatchString(a) {
						t.Errorf("%s: FAQ answer carries markup: %q", ri.Path, a)
					}
					for _, bad := range []string{" ,", " .", " ?", " ;", " :", " !", " )", "( "} {
						if strings.Contains(a, bad) {
							t.Errorf("%s: FAQ answer contains %q: %q", ri.Path, bad, a)
						}
					}
				}
			case "BreadcrumbList":
				seen := map[string]bool{}
				for _, e := range doc.Elements {
					if seen[e.Item.ID] {
						t.Errorf("%s: BreadcrumbList repeats @id %s", ri.Path, e.Item.ID)
					}
					seen[e.Item.ID] = true
				}
			}
		}
	}
	if answers == 0 {
		t.Fatal("no FAQPage answer found on any page")
	}
}

// htmlMarkupRE matches an HTML tag that FAQ answer text must not carry.
// Literal placeholders such as "<method>" are text, not markup.
var htmlMarkupRE = regexp.MustCompile(`(?i)</?(a|b|i|em|strong|code|span|p|ul|ol|li|pre|br|div)\b[^>]*>`)

// TestLLMsFullOmitsFrontMatter pins GEO-FULL-1.
func TestLLMsFullOmitsFrontMatter(t *testing.T) {
	t.Parallel()
	srv, _ := newRealContentServer(t, config.EnvDevelopment)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	full := getBody(t, ts, "/llms-full.txt")
	for _, bad := range []string{"datePublished:", "dateModified:"} {
		if strings.Contains(full, bad) {
			t.Errorf("/llms-full.txt carries front matter %q", bad)
		}
	}
	if !strings.Contains(full, "## /docs/http-query-method\n\n# HTTP QUERY method (RFC 10008)") {
		t.Error("/llms-full.txt body of /docs/http-query-method does not start with its H1")
	}
}
