package server

import (
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/config"
)

// publishedSurfaces returns every path the site serves as text: each HTML
// page, each Markdown companion, and the top-level text artefacts. JSON-LD,
// meta descriptions, and Open Graph tags are part of the HTML bodies.
func publishedSurfaces() []string {
	paths := []string{
		"/", "/index.md", "/docs/index.md", "/examples/index.md",
		"/llms.txt", "/llms-full.txt", "/robots.txt", "/sitemap.xml",
		"/.well-known/security.txt",
	}
	for _, r := range docRoutes() {
		paths = append(paths, r.path)
		if r.path == "/docs/" || r.path == "/examples/" {
			continue
		}
		if r.hasMarkdown && r.contentPath != "" {
			paths = append(paths, r.path+".md")
		}
	}
	return paths
}

// findingIDRE matches an audit or finding identifier (specification/
// overview.md INT-SEC-2).
var findingIDRE = regexp.MustCompile(`\b(TSC|CDX|MM|TM|CSA|HPS|PRF|MSR|DOS)-[0-9]`)

// securityFixRE matches the phrase INT-SEC-3 forbids, in any case.
var securityFixRE = regexp.MustCompile(`(?i)security fix`)

// supportedVersionsSentence is the only text in which INT-SEC-4 permits the
// phrase "security fixes": the supported-versions statement.
const supportedVersionsSentence = "Only the latest release receives security fixes."

// supportedVersionsAllowance is the number of times supportedVersionsSentence
// may appear on each surface. /security and /compatibility carry it once,
// in HTML and in the Markdown companion; /llms-full.txt inlines both pages.
var supportedVersionsAllowance = map[string]int{
	"/security":         1,
	"/security.md":      1,
	"/compatibility":    1,
	"/compatibility.md": 1,
	"/llms-full.txt":    2,
}

var whitespaceRE = regexp.MustCompile(`\s+`)

// TestNoSecurityDefectHistory pins INT-SEC-1 to INT-SEC-4 on every published
// surface: no finding identifier, and no "security fix" wording outside the
// supported-versions statement on /security and /compatibility.
func TestNoSecurityDefectHistory(t *testing.T) {
	t.Parallel()
	srv, _ := newRealContentServer(t, config.EnvProduction)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	for _, p := range publishedSurfaces() {
		body := whitespaceRE.ReplaceAllString(getBody(t, ts, p), " ")
		if m := findingIDRE.FindString(body); m != "" {
			t.Errorf("%s: carries finding identifier %q", p, m)
		}
		want := supportedVersionsAllowance[p]
		if got := strings.Count(body, supportedVersionsSentence); got != want {
			t.Errorf("%s: supported-versions sentence appears %d times, want %d", p, got, want)
		}
		rest := strings.ReplaceAll(body, supportedVersionsSentence, "")
		if loc := securityFixRE.FindStringIndex(rest); loc != nil {
			lo, hi := max(loc[0]-60, 0), min(loc[1]+60, len(rest))
			t.Errorf("%s: forbidden %q wording: …%s…", p, "security fix", rest[lo:hi])
		}
	}
}

var (
	hrefRE    = regexp.MustCompile(`href="([^"]*#[^"]*)"`)
	mdLinkRE  = regexp.MustCompile(`\]\(([^)\s]*#[^)\s]*)\)`)
	idAttrsRE = regexp.MustCompile(`\sid="([^"]+)"`)
)

// TestInSiteFragmentLinksResolve checks that every in-site link carrying a
// #fragment, on every published surface, points to an id on the HTML page
// it targets.
func TestInSiteFragmentLinksResolve(t *testing.T) {
	t.Parallel()
	const base = "https://muxmaster.net"
	srv, _ := newRealContentServer(t, config.EnvProduction)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	idCache := map[string]map[string]bool{}
	idsOf := func(path string) (map[string]bool, bool) {
		if ids, ok := idCache[path]; ok {
			return ids, ids != nil
		}
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		b, err := io.ReadAll(resp.Body)
		// A close error on an in-memory test response carries no signal.
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK ||
			!strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
			idCache[path] = nil
			return nil, false
		}
		ids := map[string]bool{}
		for _, m := range idAttrsRE.FindAllStringSubmatch(string(b), -1) {
			ids[html.UnescapeString(m[1])] = true
		}
		idCache[path] = ids
		return ids, true
	}

	checked := 0
	for _, src := range publishedSurfaces() {
		body := getBody(t, ts, src)
		var links []string
		re := mdLinkRE
		if !strings.HasSuffix(src, ".md") && !strings.HasSuffix(src, ".txt") && !strings.HasSuffix(src, ".xml") {
			re = hrefRE
		}
		for _, m := range re.FindAllStringSubmatch(body, -1) {
			links = append(links, html.UnescapeString(m[1]))
		}
		for _, link := range links {
			target, frag, _ := strings.Cut(link, "#")
			switch {
			case target == "":
				if strings.HasSuffix(src, ".md") || strings.HasSuffix(src, ".txt") {
					continue // a same-document fragment in Markdown has no HTML ids to check here
				}
				target = src
			case strings.HasPrefix(target, base+"/"):
				target = strings.TrimPrefix(target, base)
			case strings.HasPrefix(target, "/"):
			default:
				continue // external link
			}
			if frag == "" {
				continue
			}
			if f, err := url.PathUnescape(frag); err == nil {
				frag = f
			}
			ids, ok := idsOf(target)
			if !ok {
				t.Errorf("%s: link %q targets %s, which is not an HTML page", src, link, target)
				continue
			}
			checked++
			if !ids[frag] {
				t.Errorf("%s: link %q: no id %q on %s", src, link, frag, target)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no in-site fragment link found; the extractor is broken")
	}
}
