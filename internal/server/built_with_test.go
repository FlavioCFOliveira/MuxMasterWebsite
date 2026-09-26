package server

import (
	"net/http"
	"strings"
	"testing"
)

// TestBuiltWithMuxMasterPage covers the site-owned /built-with-muxmaster page
// (specification/information-architecture.md "Built with MuxMaster page"):
// both representations, the footer link, the sitemap and llms.txt entries,
// and the cross-links from /benchmarks and /examples/max-performance.
func TestBuiltWithMuxMasterPage(t *testing.T) {
	srv := newTestServer(t)
	h := srv.httpServer.Handler

	page := get(t, h, "/built-with-muxmaster", nil)
	if page.Code != http.StatusOK {
		t.Fatalf("/built-with-muxmaster: status %d", page.Code)
	}
	md := get(t, h, "/built-with-muxmaster.md", nil)
	if md.Code != http.StatusOK || !strings.HasPrefix(md.Header().Get("Content-Type"), "text/markdown") {
		t.Fatalf("/built-with-muxmaster.md: status %d, Content-Type %q", md.Code, md.Header().Get("Content-Type"))
	}
	if got := page.Header().Get("Cache-Control"); got != cacheControlDocs {
		t.Errorf("Cache-Control = %q, want %q", got, cacheControlDocs)
	}

	link := `href="/built-with-muxmaster"`
	for _, p := range []string{"/", "/docs/routing", "/benchmarks", "/examples/max-performance"} {
		if body := get(t, h, p, nil).Body.String(); !strings.Contains(body, link) {
			t.Errorf("%s does not link to /built-with-muxmaster", p)
		}
	}
	for _, p := range []string{"/llms.txt", "/llms-full.txt"} {
		if body := get(t, h, p, nil).Body.String(); !strings.Contains(body, "/built-with-muxmaster") {
			t.Errorf("%s does not list /built-with-muxmaster", p)
		}
	}
	for _, want := range []string{`"TechArticle"`, `"BreadcrumbList"`} {
		if !strings.Contains(page.Body.String(), want) {
			t.Errorf("/built-with-muxmaster JSON-LD lacks %s", want)
		}
	}
}
