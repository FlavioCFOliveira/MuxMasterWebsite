package render

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

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
			items := filterRoutes(deps.Routes, "examples", path)
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
			return buildLLMs(deps, false), nil
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
			out := buildLLMs(deps, true)
			var b bytes.Buffer
			b.Write(out)
			b.WriteString("\n---\n\n# Full content\n\n")
			// Walk the route table in deterministic order so the output is
			// stable across runs (a precondition for stable ETags).
			paths := make([]string, 0, len(deps.Routes))
			for _, r := range deps.Routes {
				if !r.HasMarkdown {
					continue
				}
				if _, ok := routeToContent[r.Path]; !ok {
					continue
				}
				paths = append(paths, r.Path)
			}
			sort.Strings(paths)
			for _, p := range paths {
				cp := routeToContent[p]
				src, err := loader.Load(cp)
				if err != nil {
					return nil, err
				}
				fmt.Fprintf(&b, "## %s\n\n", deps.BaseURL+p)
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
func SitemapRecipe() Recipe {
	return Recipe{
		Path:        "/sitemap.xml",
		ContentType: "application/xml; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			type urlEntry struct {
				XMLName xml.Name `xml:"url"`
				Loc     string   `xml:"loc"`
				LastMod string   `xml:"lastmod"`
			}
			type urlSet struct {
				XMLName xml.Name   `xml:"urlset"`
				Xmlns   string     `xml:"xmlns,attr"`
				URLs    []urlEntry `xml:"url"`
			}
			lastMod := deps.BuildTime.UTC().Format("2006-01-02")
			set := urlSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
			for _, r := range deps.Routes {
				set.URLs = append(set.URLs, urlEntry{
					Loc:     deps.BaseURL + r.Path,
					LastMod: lastMod,
				})
			}
			var buf bytes.Buffer
			buf.WriteString(xml.Header)
			enc := xml.NewEncoder(&buf)
			enc.Indent("", "  ")
			if err := enc.Encode(set); err != nil {
				return nil, fmt.Errorf("sitemap: encode: %w", err)
			}
			buf.WriteByte('\n')
			return buf.Bytes(), nil
		},
	}
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
		"Google-Extended",
		"Applebot-Extended",
		"CCBot",
		"Bytespider",
		"DiffBot",
		"Diffbot",
		"OmgiliBot",
		"Amazonbot",
		"meta-externalagent",
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

// buildLLMs is the shared body builder for /llms.txt and /llms-full.txt. The
// only difference between the two is the link target: HTML in /llms.txt, .md
// in /llms-full.txt (with the HTML equivalent shown alongside).
func buildLLMs(deps Deps, full bool) []byte {
	var b strings.Builder
	b.WriteString("# MuxMaster\n\n")
	b.WriteString("> MuxMaster is a high-performance, zero-dependency HTTP router for Go. " +
		"It provides a radix-tree implementation with O(k) lookups, zero allocations on " +
		"static routes, and 100% compatibility with `net/http`. " +
		"It supports the minimum Go version stated on /compatibility.\n\n")

	groups := groupRoutes(deps.Routes)

	writeSection(&b, "Documentation", groups["docs"], deps.BaseURL, full)
	writeSection(&b, "API", groups["api"], deps.BaseURL, full)
	writeSection(&b, "Examples", groups["examples"], deps.BaseURL, full)

	// "Reference" covers benchmarks, changelog, releases, security,
	// compatibility, contributing — per specification/geo.md.
	var reference []RouteInfo
	for _, key := range []string{"benchmarks", "changelog", "releases", "security", "compatibility", "contributing"} {
		reference = append(reference, groups[key]...)
	}
	writeSection(&b, "Reference", reference, deps.BaseURL, full)

	b.WriteString("## Optional\n\n")
	b.WriteString("- [GitHub repository](https://github.com/FlavioCFOliveira/MuxMaster): canonical source for MuxMaster.\n")

	return []byte(b.String())
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

func writeSection(b *strings.Builder, heading string, items []RouteInfo, baseURL string, full bool) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", heading)
	for _, r := range items {
		htmlURL := baseURL + r.Path
		if full && r.HasMarkdown {
			mdURL := htmlURL + ".md"
			fmt.Fprintf(b, "- [%s](%s): %s (HTML: %s)\n", r.Title, mdURL, r.Description, htmlURL)
		} else {
			fmt.Fprintf(b, "- [%s](%s): %s\n", r.Title, htmlURL, r.Description)
		}
	}
	b.WriteString("\n")
}
