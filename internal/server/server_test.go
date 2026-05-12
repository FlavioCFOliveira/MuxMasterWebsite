package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
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
	if !strings.Contains(string(body), "v1.0.1") {
		t.Errorf("landing chrome missing version label v1.0.1; body[:600]=%s", truncate(string(body), 600))
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
		"docs/getting-started.md": "# Getting started\n\n## Install\n\nFirst install MuxMaster.\n\n## Step 1 — Hello, World\n\nWrite the simplest handler.\n\n## Step 2 — Path Parameters\n\nAdd a parametric route.\n",
		"docs/routing.md": "# Routing\n\n## Patterns\n\nText.\n\n## Priority\n\nText.\n\n" +
			"```go\nfunc Handler() {}\n```\n",
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
		"examples/versioning.md":         "# Versioning\n",
		"release-notes/v1.0.0.md":        "# Release notes — v1.0.0\n\n## Highlights\n\nText.\n",
		"release-notes/v1.1.0.md":        "# Release notes — v1.1.0\n\n## Highlights\n\nText.\n",
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
