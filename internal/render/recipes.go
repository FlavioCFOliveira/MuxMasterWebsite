package render

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/content"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/meta"
)

// LandingRecipe builds the landing page from the parsed landing template.
// Path: /. Content-Type: text/html; charset=utf-8.
func LandingRecipe(landingDescription, ogImagePath string, productionRobots bool) Recipe {
	return Recipe{
		Path:        "/",
		ContentType: "text/html; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			page := basePage(deps, "/", "", landingDescription, "website", ogImagePath, productionRobots)
			page.JSONLD = BuildJSONLD(JSONLDInputs{
				Page:      page,
				Family:    "landing",
				BuildTime: deps.BuildTime,
			})
			return deps.Renderer.ExecuteTemplate("landing.html", Data{Meta: page})
		},
	}
}

// DocsIndexRecipe builds /docs/ from the route table. When
// /content/site/docs-index.md is present, its body is rendered above the
// list of sections.
func DocsIndexRecipe(loader *content.Loader, ogImagePath string, productionRobots bool) Recipe {
	const path = "/docs/"
	return Recipe{
		Path:        path,
		ContentType: "text/html; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			items := filterRoutes(deps.Routes, "docs", path)
			page := basePage(deps, path,
				"Documentation",
				"Index of MuxMaster documentation: getting started, routing, groups, middleware, error handling, configuration, response helpers, performance, observability, migration, and a cookbook.",
				"article", ogImagePath, productionRobots)
			page.Breadcrumbs = []meta.Breadcrumb{
				{Label: "Home", Href: "/"},
				{Label: "Documentation"},
			}
			intro, err := optionalIntro(loader, "site/docs-index.md")
			if err != nil {
				return nil, err
			}
			body := indexPageBody{
				Heading:     "Documentation",
				Description: "Eleven sections, in the order recommended for first-time readers. Every page has a Markdown companion at the same path with a .md suffix.",
				Items:       items,
				IntroHTML:   intro,
			}
			page.JSONLD = BuildJSONLD(JSONLDInputs{
				Page: page, Family: "collection", BuildTime: deps.BuildTime,
				HasPart: itemsAsAbsoluteURLs(items, deps.BaseURL),
			})
			return deps.Renderer.ExecuteTemplate("section-index.html", Data{Meta: page, Body: body})
		},
	}
}

// ExamplesIndexRecipe builds /examples/. When /content/site/examples-index.md
// is present, its body is rendered above the cards.
func ExamplesIndexRecipe(loader *content.Loader, ogImagePath string, productionRobots bool) Recipe {
	const path = "/examples/"
	return Recipe{
		Path:        path,
		ContentType: "text/html; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			// Examples use a curated learning order (RouteInfo.Order),
			// not alphabetical. The order is REST first, auth family
			// (Authn → JWT → OAuth2) second, then operational concerns
			// (Cache → Graceful shutdown → SSR → Static site).
			items := filterRoutesByOrder(deps.Routes, "examples", path)
			page := basePage(deps, path,
				"Examples",
				"Eight runnable MuxMaster examples covering REST APIs, authentication, JWT, OAuth2, caching, graceful shutdown, server-side rendering, and static sites.",
				"article", ogImagePath, productionRobots)
			page.Breadcrumbs = []meta.Breadcrumb{
				{Label: "Home", Href: "/"},
				{Label: "Examples"},
			}
			intro, err := optionalIntro(loader, "site/examples-index.md")
			if err != nil {
				return nil, err
			}
			body := indexPageBody{
				Heading:     "Examples",
				Description: "Eight programs from the upstream MuxMaster examples directory. Each page links to the upstream source.",
				Items:       items,
				IntroHTML:   intro,
			}
			page.JSONLD = BuildJSONLD(JSONLDInputs{
				Page: page, Family: "collection", BuildTime: deps.BuildTime,
				HasPart: itemsAsAbsoluteURLs(items, deps.BaseURL),
			})
			return deps.Renderer.ExecuteTemplate("section-index.html", Data{Meta: page, Body: body})
		},
	}
}

// optionalIntro renders the supplied content path to HTML when present,
// returning an empty string when absent. Errors from a present-but-broken
// file propagate so misconfigurations are visible at startup.
func optionalIntro(loader *content.Loader, contentPath string) (string, error) {
	if loader == nil || !loader.Exists(contentPath) {
		return "", nil
	}
	src, err := loader.Load(contentPath)
	if err != nil {
		return "", err
	}
	htmlBytes, err := MarkdownToHTML(src)
	if err != nil {
		return "", err
	}
	return string(htmlBytes), nil
}

// LLMsRecipe builds /llms.txt per https://llmstxt.org and the strict structure
// in specification/geo.md. Sections in order: Documentation, API, Examples,
// Reference, Optional.
func LLMsRecipe() Recipe {
	return Recipe{
		Path:        "/llms.txt",
		ContentType: "text/plain; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			return buildLLMs(deps), nil
		},
	}
}

// LLMsFullRecipe builds /llms-full.txt: the same index as /llms.txt with
// every Markdown body concatenated under it (per specification/geo.md and
// content-sources.md). The loader is the source of every body; nothing is
// fetched at request time.
func LLMsFullRecipe(loader *content.Loader, routeToContent map[string]string) Recipe {
	return Recipe{
		Path:        "/llms-full.txt",
		ContentType: "text/plain; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			out := buildLLMs(deps)
			var b bytes.Buffer
			b.Write(out)
			b.WriteString("\n---\n\n# Full content\n\n")
			// Inlined bodies MUST appear in the same order as the routes
			// appear in the navigation index above (per specification/geo.md
			// § /llms-full.txt). The navigation index walks groupRoutes
			// output in section sequence (Documentation → API → Examples →
			// Reference); we replicate that walk here so a crawler can
			// locate the body it cares about by route path within the
			// bundle, in a position that matches its nav-index position.
			for _, r := range navIndexOrder(deps.Routes) {
				if !r.HasMarkdown {
					continue
				}
				cp, ok := routeToContent[r.Path]
				if !ok {
					continue
				}
				src, err := loader.Load(cp)
				if err != nil {
					return nil, err
				}
				// Heading is the route path (e.g. "## /docs/routing"), per
				// specification/geo.md § /llms-full.txt: "Each inlined body
				// MUST be preceded by a heading line that names the route URL
				// (for example `## /docs/routing`)."
				fmt.Fprintf(&b, "## %s\n\n", r.Path)
				b.Write(src)
				if len(src) == 0 || src[len(src)-1] != '\n' {
					b.WriteByte('\n')
				}
				b.WriteString("\n")
			}
			return b.Bytes(), nil
		},
	}
}

// SitemapRecipe builds /sitemap.xml from the route table. Markdown companions
// are excluded per specification (sitemap covers HTML routes only).
//
// changefreq and priority are populated per route family per
// specification/seo.md "sitemap.xml". lastmod is the ISO 8601 datetime of
// the underlying content file's mtime, when available; otherwise the
// process build time. Embedded files report a zero mtime under
// embed.FS — when that happens, the build time is used as a stable
// fallback so the value is never empty.
func SitemapRecipe(loader contentMtimer, routeContent map[string]string, productionRobots bool) Recipe {
	return Recipe{
		Path:        "/sitemap.xml",
		ContentType: "application/xml; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			type urlEntry struct {
				XMLName    xml.Name `xml:"url"`
				Loc        string   `xml:"loc"`
				LastMod    string   `xml:"lastmod"`
				ChangeFreq string   `xml:"changefreq"`
				Priority   string   `xml:"priority"`
			}
			type urlSet struct {
				XMLName xml.Name   `xml:"urlset"`
				Xmlns   string     `xml:"xmlns,attr"`
				URLs    []urlEntry `xml:"url"`
			}
			set := urlSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
			// Outside production every page is noindex,nofollow until the
			// canonical domain is ratified (open-questions.md item 1).
			// Listing a noindex URL in sitemap.xml is contradictory and
			// Google treats it as a low-quality signal — so we emit an
			// empty urlset on non-production environments. The endpoint
			// still exists (200, valid XML) so health probes against it
			// pass; only the URL list is suppressed.
			if !productionRobots {
				return encodeSitemap(set)
			}
			fallback := deps.BuildTime.UTC().Format(time.RFC3339)
			for _, r := range deps.Routes {
				lastMod := fallback
				if cp, ok := routeContent[r.Path]; ok && loader != nil {
					if t, err := loader.Mtime(cp); err == nil && !t.IsZero() {
						lastMod = t.UTC().Format(time.RFC3339)
					}
				}
				set.URLs = append(set.URLs, urlEntry{
					Loc:        deps.BaseURL + r.Path,
					LastMod:    lastMod,
					ChangeFreq: changeFreqFor(r.Path),
					Priority:   priorityFor(r.Path),
				})
			}
			return encodeSitemap(set)
		},
	}
}

// encodeSitemap marshals the supplied urlset to XML with the standard
// header and indentation. Defined here so the recipe body stays readable
// when both the production and non-production branches need to encode.
func encodeSitemap(v any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("sitemap: encode: %w", err)
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// contentMtimer is the narrow interface the sitemap recipe needs from a
// content loader. Defining it here (rather than importing content.Loader)
// keeps the render package free of upward dependencies and makes the
// recipe trivial to fake in tests.
type contentMtimer interface {
	Mtime(path string) (time.Time, error)
}

// changeFreqFor returns the sitemap changefreq value per route family. The
// values are taken verbatim from specification/seo.md "sitemap.xml" table:
//   weekly:  /, /changelog
//   monthly: /docs/, /examples/, every other documentation page
//   yearly:  /releases/<v>  (immutable release notes)
func changeFreqFor(path string) string {
	switch {
	case path == "/":
		return "weekly"
	case path == "/changelog":
		return "weekly"
	case strings.HasPrefix(path, "/releases/"):
		return "yearly"
	}
	return "monthly"
}

// priorityFor returns the sitemap priority value per route family.
func priorityFor(path string) string {
	// Per specification/seo.md § sitemap.xml priority table:
	//   1.0 — /
	//   0.8 — /docs/, /api, /examples/
	//   0.6 — /docs/<section>, /examples/<name>, /benchmarks
	//   0.4 — /changelog, /releases/*, /security, /compatibility,
	//         /contributing, anything else
	switch {
	case path == "/":
		return "1.0"
	case path == "/docs/" || path == "/api" || path == "/examples/":
		return "0.8"
	case path == "/benchmarks":
		return "0.6"
	case strings.HasPrefix(path, "/docs/"):
		return "0.6"
	case strings.HasPrefix(path, "/examples/"):
		return "0.6"
	}
	return "0.4"
}

// RobotsRecipe builds /robots.txt from the AI-crawler allowlist defined in
// specification/geo.md. The list is taken verbatim from the spec.
func RobotsRecipe() Recipe {
	// Order matches specification/geo.md "AI crawler allowlist (robots.txt)".
	bots := []string{
		"GPTBot",
		"ChatGPT-User",
		"OAI-SearchBot",
		"ClaudeBot",
		"anthropic-ai",
		"PerplexityBot",
		"Perplexity-User",
		"Google-Extended",
		"Applebot-Extended",
		"CCBot",
		"Bytespider",
		"Diffbot",
		"OmgiliBot",
		"Amazonbot",
		"meta-externalagent",
		"FacebookBot",
		"cohere-ai",
		"MistralAI-User",
	}
	return Recipe{
		Path:        "/robots.txt",
		ContentType: "text/plain; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			var b strings.Builder
			for _, name := range bots {
				fmt.Fprintf(&b, "User-agent: %s\nAllow: /\nDisallow: /healthz\n\n", name)
			}
			b.WriteString("User-agent: *\nAllow: /\nDisallow: /healthz\n\n")
			fmt.Fprintf(&b, "Sitemap: %s/sitemap.xml\n", deps.BaseURL)
			return []byte(b.String()), nil
		},
	}
}

// SecurityTxtRecipe builds /.well-known/security.txt per RFC 9116 §2.5.
// The Expires field is twelve months ahead of the build time; per
// specification/url-and-versioning.md "Reserved paths" it MUST be
// refreshed before it lapses (the release contract). Contact points at
// the upstream MuxMaster security-advisory channel because vulnerability
// reports against the website overlap heavily with reports against the
// router itself.
func SecurityTxtRecipe() Recipe {
	return Recipe{
		Path:        "/.well-known/security.txt",
		ContentType: "text/plain; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			expires := time.Now().UTC().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z")
			var b strings.Builder
			fmt.Fprintf(&b, "Contact: https://github.com/FlavioCFOliveira/MuxMaster/security/advisories/new\n")
			fmt.Fprintf(&b, "Expires: %s\n", expires)
			fmt.Fprintf(&b, "Preferred-Languages: en\n")
			fmt.Fprintf(&b, "Canonical: %s/.well-known/security.txt\n", deps.BaseURL)
			return []byte(b.String()), nil
		},
	}
}

// NotFoundRecipe builds the /404 body. The handler emits this body with
// status 404 and Cache-Control: no-store.
func NotFoundRecipe(ogImagePath string, productionRobots bool) Recipe {
	return Recipe{
		Path:        "/404",
		ContentType: "text/html; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			page := basePage(deps, "/404",
				"Page not found",
				"The page you requested does not exist on the MuxMaster documentation site.",
				"article", ogImagePath, productionRobots)
			// 404 pages are always noindex regardless of environment.
			page.Robots = "noindex,nofollow"
			// SEO B7: noindex pages MUST NOT carry a canonical link.
			page.Canonical = ""
			page.Breadcrumbs = []meta.Breadcrumb{
				{Label: "Home", Href: "/"},
				{Label: "Page not found"},
			}
			return deps.Renderer.ExecuteTemplate("error-page.html", Data{
				Meta: page,
				Body: errorBody{Code: "404", Heading: "Page not found", Message: "The page you requested does not exist."},
			})
		},
	}
}

// ServerErrorRecipe builds the /500 body. The handler emits this body with
// status 500 and Cache-Control: no-store.
func ServerErrorRecipe(ogImagePath string, productionRobots bool) Recipe {
	return Recipe{
		Path:        "/500",
		ContentType: "text/html; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			page := basePage(deps, "/500",
				"Server error",
				"The MuxMaster documentation site encountered an internal error.",
				"article", ogImagePath, productionRobots)
			page.Robots = "noindex,nofollow"
			// SEO B7: noindex pages MUST NOT carry a canonical link.
			page.Canonical = ""
			page.Breadcrumbs = []meta.Breadcrumb{
				{Label: "Home", Href: "/"},
				{Label: "Server error"},
			}
			return deps.Renderer.ExecuteTemplate("error-page.html", Data{
				Meta: page,
				Body: errorBody{Code: "500", Heading: "Server error", Message: "The site encountered an internal error. The incident has been logged."},
			})
		},
	}
}

// indexPageBody is the data passed to the section-index template.
type indexPageBody struct {
	Heading     string
	Description string
	Items       []RouteInfo
	IntroHTML   string // Optional pre-rendered HTML from /content/site/<index>.md.
}

// errorBody is the data passed to the error-page template.
type errorBody struct {
	Code    string
	Heading string
	Message string
}

// basePage assembles a meta.Page with the site-wide chrome inputs filled in.
func basePage(deps Deps, path, title, description, ogType, ogImagePath string, productionRobots bool) meta.Page {
	p := meta.Page{
		Title:       title,
		Description: description,
		Path:        path,
		Canonical:   deps.BaseURL + path,
		OGType:      ogType,
		OGImage:     deps.BaseURL + ogImagePath,
		Version:     deps.Version,
		GoVersion:   deps.GoVersion,
		CSSPath:     deps.Renderer.CSSPath(),
		BaseURL:     deps.BaseURL,
	}
	if !productionRobots {
		// Until the canonical domain is ratified (open-questions.md item 1),
		// every page is noindex outside production.
		p.Robots = "noindex,nofollow"
	}
	return p
}

// filterRoutes selects routes whose path lives directly under prefix and
// returns them in a stable order (path lex order). The prefix itself is
// excluded so an index page does not list itself.
// itemsAsAbsoluteURLs converts a slice of RouteInfo to the @id values of
// the TechArticle node each route emits on its own page, used as the
// input to CollectionPage.hasPart on section indexes. The article @id
// shape is `<canonical>#article` (see jsonld.go buildArticleJSONLD), so
// hasPart references that fragment URI rather than the bare canonical
// URL — otherwise the JSON-LD validation gate flags hasPart entries as
// unresolved @id references.
func itemsAsAbsoluteURLs(items []RouteInfo, baseURL string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, baseURL+it.Path+"#article")
	}
	return out
}

func filterRoutes(routes []RouteInfo, section, indexPath string) []RouteInfo {
	out := make([]RouteInfo, 0, len(routes))
	for _, r := range routes {
		if r.Path == indexPath {
			continue
		}
		if r.Section != section {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// filterRoutesByOrder is like filterRoutes but sorts by RouteInfo.Order
// ascending, falling back to path lex order when two entries share an Order
// value (typically because both are zero — meaning "no curated rank"). The
// /examples/ index uses this so the cards appear in learning sequence
// rather than alphabetical order.
func filterRoutesByOrder(routes []RouteInfo, section, indexPath string) []RouteInfo {
	out := make([]RouteInfo, 0, len(routes))
	for _, r := range routes {
		if r.Path == indexPath {
			continue
		}
		if r.Section != section {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// buildLLMs is the shared body builder for /llms.txt and /llms-full.txt. Both
// files emit the same navigation index, with links pointing at canonical HTML
// URLs (per specification/geo.md § /llms-full.txt: the bundled file MUST NOT
// list .md companion URLs in its navigation index). The difference between
// the two is that /llms-full.txt appends the inlined Markdown bodies after a
// `---` separator (handled by LLMsFullRecipe, not here).
func buildLLMs(deps Deps) []byte {
	var b strings.Builder
	b.WriteString("# MuxMaster\n\n")
	b.WriteString("> MuxMaster is a high-performance, zero-dependency HTTP router for Go. " +
		"It provides a radix-tree implementation with O(k) lookups, zero allocations on " +
		"static routes, and 100% compatibility with `net/http`. " +
		"It supports the minimum Go version stated on /compatibility.\n\n")

	groups := groupRoutes(deps.Routes)

	writeSection(&b, "Documentation", groups["docs"], deps.BaseURL)
	writeSection(&b, "API", groups["api"], deps.BaseURL)
	writeSection(&b, "Examples", groups["examples"], deps.BaseURL)

	// "Reference" covers benchmarks, changelog, releases, security,
	// compatibility, contributing — per specification/geo.md.
	var reference []RouteInfo
	for _, key := range []string{"benchmarks", "changelog", "releases", "security", "compatibility", "contributing"} {
		reference = append(reference, groups[key]...)
	}
	writeSection(&b, "Reference", reference, deps.BaseURL)

	b.WriteString("## Optional\n\n")
	b.WriteString("- [GitHub repository](https://github.com/FlavioCFOliveira/MuxMaster): canonical source for MuxMaster.\n")

	return []byte(b.String())
}

// navIndexOrder returns the routes in the same sequence the /llms.txt and
// /llms-full.txt navigation indexes emit them: Documentation → API →
// Examples → Reference (benchmarks → changelog → releases → security →
// compatibility → contributing). Within each section, groupRoutes places
// index pages first, then sorts the remaining routes by path. The /
// landing page is excluded — the navigation index has no entry for it
// and /llms-full.txt does not inline its body.
func navIndexOrder(routes []RouteInfo) []RouteInfo {
	groups := groupRoutes(routes)
	var ordered []RouteInfo
	for _, key := range []string{"docs", "api", "examples", "benchmarks", "changelog", "releases", "security", "compatibility", "contributing"} {
		ordered = append(ordered, groups[key]...)
	}
	return ordered
}

// groupRoutes buckets the routes by their Section field. Index pages
// (/docs/, /examples/) are placed at the front of their bucket.
func groupRoutes(routes []RouteInfo) map[string][]RouteInfo {
	g := make(map[string][]RouteInfo)
	for _, r := range routes {
		g[r.Section] = append(g[r.Section], r)
	}
	for k := range g {
		sort.SliceStable(g[k], func(i, j int) bool {
			// Index URLs (trailing slash) sort first within their group.
			ai := strings.HasSuffix(g[k][i].Path, "/")
			aj := strings.HasSuffix(g[k][j].Path, "/")
			if ai != aj {
				return ai
			}
			return g[k][i].Path < g[k][j].Path
		})
	}
	return g
}

func writeSection(b *strings.Builder, heading string, items []RouteInfo, baseURL string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", heading)
	for _, r := range items {
		// Navigation-index links always target the canonical HTML URL
		// (per specification/geo.md § /llms-full.txt). Markdown
		// companions are reachable via their explicit .md URLs but are
		// never listed here.
		fmt.Fprintf(b, "- [%s](%s): %s\n", r.Title, baseURL+r.Path, r.Description)
	}
	b.WriteString("\n")
}
