package server

import (
	"net/http"
	"strings"

	muxm "github.com/FlavioCFOliveira/MuxMaster"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/render"
)

// route describes one entry in the day-one route table from
// specification/information-architecture.md. Every documentation route also
// gets its .md companion registered.
type route struct {
	path  string
	title string
	// headTitle, when set, is the complete <title> text; title remains the
	// navigation and breadcrumb label (specification/seo.md SEO-QUERY-1).
	headTitle   string
	description string
	upstreamURL string
	hasMarkdown bool
	contentPath string // Loader path under /content/, e.g. "docs/routing.md".
	cache       string
	section     string // Used by recipe-side breadcrumb selection.
	ogType      string
	// order is the curated rank within section for index pages. It exists
	// because /examples/ wants a learning sequence, not the alphabetical
	// order produced by path-lex sorting. Zero means "no opinion" — the
	// recipe falls back to path-lex.
	order int
	// about makes the page's TechArticle reference the MuxMaster
	// SoftwareSourceCode entity (specification/structured-data.md).
	about bool
	// version is the release version of a /releases/<v> page, emitted as
	// TechArticle.version.
	version string
}

// routeInfos returns every HTML route for the prerender recipes (sitemap,
// llms.txt, llms-full.txt, docs/examples indexes). The landing page is
// included; operational endpoints, text artefacts, and error templates are
// not. Markdown companions are not enumerated here — they derive from
// HasMarkdown.
func routeInfos() []render.RouteInfo {
	out := []render.RouteInfo{
		{Path: "/", Title: "MuxMaster", Description: LandingDescription, Section: "landing", HasMarkdown: false},
	}
	for _, r := range docRoutes() {
		out = append(out, render.RouteInfo{
			Path:        r.path,
			Title:       r.title,
			Description: r.description,
			Section:     sectionForPath(r.path),
			HasMarkdown: r.hasMarkdown,
			Order:       r.order,
		})
	}
	return out
}

// routeContentPaths returns the route → /content/ file mapping used by the
// llms-full recipe to inline every Markdown body. The map covers only
// routes whose body is a single curated file under /content/; the
// landing page and the section indexes are excluded because they are
// generated, not curated.
func routeContentPaths() map[string]string {
	out := make(map[string]string, len(docRoutes()))
	for _, r := range docRoutes() {
		if r.contentPath == "" {
			continue
		}
		out[r.path] = r.contentPath
	}
	return out
}

// sitemapContentPaths returns the route → /content/ file mapping the
// sitemap reads front-matter dates from (specification/seo.md SEO-MAP-1).
// It extends routeContentPaths with the files that carry the dates of the
// landing page and the two section indexes; an index file that does not
// exist makes the sitemap fall back to the latest child date.
func sitemapContentPaths() map[string]string {
	out := routeContentPaths()
	out["/"] = "site/landing.md"
	out["/docs/"] = "site/docs-index.md"
	out["/examples/"] = "site/examples-index.md"
	return out
}

// docPageSpecs converts the route table into the spec slice consumed by
// render.DocPageRecipe. Index routes (/docs/, /examples/) are excluded
// because they are built by their own recipes.
func docPageSpecs() []render.DocPageSpec {
	specs := make([]render.DocPageSpec, 0, len(docRoutes()))
	for _, r := range docRoutes() {
		if r.path == "/docs/" || r.path == "/examples/" {
			continue
		}
		if r.contentPath == "" {
			continue
		}
		specs = append(specs, render.DocPageSpec{
			Path:          r.path,
			Title:         r.title,
			HeadTitle:     r.headTitle,
			Description:   r.description,
			ContentPath:   r.contentPath,
			Section:       r.section,
			UpstreamURL:   r.upstreamURL,
			Cache:         r.cache,
			OGType:        r.ogType,
			AboutSoftware: r.about,
			Version:       r.version,
		})
	}
	return specs
}

// sectionForPath maps a path to the bucket name used by recipes (routes.go is
// the authoritative source). The pre-rendered index pages /docs/ and
// /examples/ belong to their own section so they appear at the top of their
// llms.txt group.
func sectionForPath(p string) string {
	switch {
	case p == "/":
		return "landing"
	case p == "/api" || p == "/api.md":
		return "api"
	case strings.HasPrefix(p, "/docs"):
		return "docs"
	case strings.HasPrefix(p, "/examples"):
		return "examples"
	case strings.HasPrefix(p, "/benchmarks"):
		return "benchmarks"
	case strings.HasPrefix(p, "/built-with-muxmaster"):
		return "built-with-muxmaster"
	case strings.HasPrefix(p, "/changelog"):
		return "changelog"
	case strings.HasPrefix(p, "/releases"):
		return "releases"
	case strings.HasPrefix(p, "/security"):
		return "security"
	case strings.HasPrefix(p, "/compatibility"):
		return "compatibility"
	case strings.HasPrefix(p, "/contributing"):
		return "contributing"
	default:
		return ""
	}
}

// docRoutes returns the route table. Order matches the spec sitemap.
//
// Descriptions are the <meta name="description">, Open Graph, Twitter,
// and JSON-LD descriptions of each page and its one-line /llms.txt entry.
// They are unique, 110 to 160 characters long (specification/seo.md), and
// carry no performance numbers (specification/overview.md INT-PERF-7).
// Upstream links cite the files at the documented release tag
// (specification/url-and-versioning.md URL-EXT-1).
func docRoutes() []route {
	const ghDocs = "https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/docs/"
	const ghRoot = "https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/"
	const ghExamples = "https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.3.0/examples/"
	return []route{
		// /docs/ index plus the thirteen sub-sections.
		{path: "/docs/", title: "Documentation", description: render.DocsIndexDescription, upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.3.0/docs", hasMarkdown: false, contentPath: "", cache: cacheControlLanding, section: "docs", ogType: "article"},
		{path: "/docs/getting-started", title: "Getting started", description: "Build a small REST API with MuxMaster step by step: install the module, register routes, read path parameters, add middleware and groups, and return JSON.", upstreamURL: ghDocs + "getting-started.md", hasMarkdown: true, contentPath: "docs/getting-started.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/routing", title: "Routing", description: "How MuxMaster matches requests: static, named, regex, optional, and catch-all patterns, match priority, method helpers including QUERY, and trailing slashes.", upstreamURL: ghDocs + "routing.md", hasMarkdown: true, contentPath: "docs/routing.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		// No one-to-one upstream file: the page is composed from several
		// upstream sources, listed in its own "Sources" section
		// (specification/content-sources.md CS-QUERY-1).
		{path: "/docs/http-query-method", title: "HTTP QUERY method (RFC 10008)", headTitle: "HTTP QUERY method (RFC 10008) in Go — MuxMaster", description: "MuxMaster supports the HTTP QUERY method (RFC 10008), a safe, idempotent method that carries a request body. Learn to register, validate, and call QUERY routes.", upstreamURL: "", hasMarkdown: true, contentPath: "docs/http-query-method.md", cache: cacheControlDocs, section: "docs", ogType: "article", about: true},
		{path: "/docs/groups", title: "Groups", description: "Share a path prefix and middleware across MuxMaster routes: Group, nested groups, Route, With, Mount for sub-routers, and ServeFiles on a group.", upstreamURL: ghDocs + "groups.md", hasMarkdown: true, contentPath: "docs/groups.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/middleware", title: "Middleware", description: "How MuxMaster middleware works: Pre, Use, UseFast, and With, writing custom middleware, and the 21 constructors of the middleware package, Logger to CORS.", upstreamURL: ghDocs + "middleware.md", hasMarkdown: true, contentPath: "docs/middleware.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/error-handling", title: "Error handling", description: "Return errors from MuxMaster handlers with HandlerFuncE and HTTPError, customise the ErrorHandler, set 404 and 405 handlers, and recover from panics.", upstreamURL: ghDocs + "error-handling.md", hasMarkdown: true, contentPath: "docs/error-handling.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/configuration", title: "Configuration", description: "Every field you can set on a MuxMaster *Mux: redirects, 405 handling, path matching, the opt-in pools, custom handlers, and the defaults that New applies.", upstreamURL: ghDocs + "configuration.md", hasMarkdown: true, contentPath: "docs/configuration.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/response-helpers", title: "Response helpers", description: "Write a complete HTTP response in one call with MuxMaster's JSON, XML, and Text helpers, use them with HandlerFuncE, or fall back to encoding/json.", upstreamURL: ghDocs + "response-helpers.md", hasMarkdown: true, contentPath: "docs/response-helpers.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/performance", title: "Performance", description: "How MuxMaster keeps allocations low, the measured results for v1.3.0, the changes since v1.1.0, and how to run the benchmarks on your own machine.", upstreamURL: ghDocs + "performance.md", hasMarkdown: true, contentPath: "docs/performance.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/max-performance", title: "Maximum performance", description: "Remove per-request allocations in MuxMaster with PoolRequestBundle, PoolFastParams, and HandleFast: the lifetime contract, the audit, and recipes.", upstreamURL: ghDocs + "max-performance.md", hasMarkdown: true, contentPath: "docs/max-performance.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/observability", title: "Observability", description: "Observe a MuxMaster service: access logs with Logger, request correlation with RequestID, Prometheus and OpenTelemetry patterns, health checks, and pprof.", upstreamURL: ghDocs + "observability.md", hasMarkdown: true, contentPath: "docs/observability.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/migration", title: "Migration", description: "Migrate to MuxMaster from gorilla/mux, chi, httprouter, or the net/http ServeMux, with side-by-side route registration and the common adjustments.", upstreamURL: ghDocs + "migration.md", hasMarkdown: true, contentPath: "docs/migration.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/cookbook", title: "Cookbook", description: "Ready-to-use MuxMaster recipes: versioned REST APIs, central error handling, authentication, JWT, pagination, QUERY endpoints, graceful shutdown, and more.", upstreamURL: ghDocs + "cookbook.md", hasMarkdown: true, contentPath: "docs/cookbook.md", cache: cacheControlDocs, section: "docs", ogType: "article"},

		{path: "/api", title: "API reference", description: "The MuxMaster v1.3.0 public API: every exported symbol of the root and middleware packages, including MethodQuery and the HTTP QUERY method helpers.", upstreamURL: ghRoot + "api.md", hasMarkdown: true, contentPath: "api.md", cache: cacheControlDocs, section: "api", ogType: "article"},

		// Examples index plus thirteen upstream examples.
		// Curated order: REST API and Maximum-performance featured first,
		// then the four pool-contract examples (versioning, SSE,
		// upload-file, reverse-proxy) before the auth and operational tier.
		{path: "/examples/", title: "Examples", description: render.ExamplesIndexDescription, upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.3.0/examples", hasMarkdown: false, contentPath: "", cache: cacheControlLanding, section: "examples", ogType: "article"},
		{path: "/examples/rest-api", title: "REST API example", description: "A MuxMaster bookstore REST API: method helpers with an HTTP QUERY (RFC 10008) search route, groups, typed parameters, error-returning handlers, and Mount.", upstreamURL: ghExamples + "rest-api", hasMarkdown: true, contentPath: "examples/rest-api.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 1},
		{path: "/examples/max-performance", title: "Maximum-performance example", description: "A MuxMaster program with PoolRequestBundle, PoolFastParams, HandleFast, UseFast, and pprof, plus a /bench endpoint comparing default and pooled routers.", upstreamURL: ghExamples + "max-performance", hasMarkdown: true, contentPath: "examples/max-performance.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 2},
		{path: "/examples/versioning", title: "Versioning example", description: "Path-based (/api/v1, /api/v2) and header-based API versioning on one MuxMaster router, with nested admin groups and PoolRequestBundle enabled.", upstreamURL: ghExamples + "versioning", hasMarkdown: true, contentPath: "examples/versioning.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 3},
		{path: "/examples/server-sent-events", title: "Server-sent events example", description: "Pool-safe SSE streaming on MuxMaster: topic-hub fan-out, drop-on-slow-subscriber backpressure, periodic server tick, and an in-browser demo page.", upstreamURL: ghExamples + "server-sent-events", hasMarkdown: true, contentPath: "examples/server-sent-events.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 4},
		{path: "/examples/upload-file", title: "Upload-file example", description: "Multipart upload with PoolRequestBundle: single, multi, and async handlers, the drain-before-spawn pattern, 32 MiB caps, a path-traversal guard, and SHA-256.", upstreamURL: ghExamples + "upload-file", hasMarkdown: true, contentPath: "examples/upload-file.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 5},
		{path: "/examples/reverse-proxy", title: "Reverse-proxy example", description: "An HTTP gateway on MuxMaster with httputil.ReverseProxy: catch-all routing, lock-free round-robin balancing, header gating, and PoolRequestBundle kept off.", upstreamURL: ghExamples + "reverse-proxy", hasMarkdown: true, contentPath: "examples/reverse-proxy.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 6},
		{path: "/examples/authn", title: "Authentication example", description: "HTTP Basic Authentication via the BasicAuth middleware, paired with ThrottlePerIP to defend against credential-stuffing attacks.", upstreamURL: ghExamples + "authn", hasMarkdown: true, contentPath: "examples/authn.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 7},
		{path: "/examples/jwt", title: "JWT example", description: "Bearer-token authentication via the JWTAuth middleware. Configures RequireExpiry: true (RFC 8725 §4.4) and shows the canonical OIDC integration shape.", upstreamURL: ghExamples + "jwt", hasMarkdown: true, contentPath: "examples/jwt.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 8},
		{path: "/examples/oauth2", title: "OAuth2 example", description: "OAuth 2.0 token introspection (RFC 7662) via the OAuth2Introspect middleware. Tokens are validated against an authorisation server, not locally.", upstreamURL: ghExamples + "oauth2", hasMarkdown: true, contentPath: "examples/oauth2.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 9},
		{path: "/examples/cache", title: "Cache example", description: "An in-memory TTL cache that avoids re-computing identical responses within a configurable horizon. For expensive, idempotent handlers.", upstreamURL: ghExamples + "cache", hasMarkdown: true, contentPath: "examples/cache.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 10},
		{path: "/examples/graceful-shutdown", title: "Graceful shutdown example", description: "Production graceful shutdown: signal handling, srv.Shutdown with a bounded drain deadline, and the recommended Server timeout set for real deployments.", upstreamURL: ghExamples + "graceful-shutdown", hasMarkdown: true, contentPath: "examples/graceful-shutdown.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 11},
		{path: "/examples/server-side-render", title: "Server-side render example", description: "A multi-page guestbook rendered by Go's html/template. The same SSR pattern that powers this documentation website you are reading now.", upstreamURL: ghExamples + "server-side-render", hasMarkdown: true, contentPath: "examples/server-side-render.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 12},
		{path: "/examples/static-site", title: "Static site example", description: "Conditional GET (304 via ETag and Last-Modified) and range requests (206) on top of MuxMaster's ServeFiles primitive. Production static-asset semantics.", upstreamURL: ghExamples + "static-site", hasMarkdown: true, contentPath: "examples/static-site.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 13},

		// The page's primary source is this repository's campaign archive,
		// pinned to its commit (URL-EXT-1, SD-BENCH-1).
		{path: "/benchmarks", title: "Benchmarks", description: "Results of this site's campaign measuring MuxMaster v1.3.0 against v1.1.0 and against httprouter, bunrouter, chi, and gorilla/mux, with significance tests.", upstreamURL: render.CampaignArchiveURL(), hasMarkdown: true, contentPath: "benchmarks.md", cache: cacheControlDocs, section: "benchmarks", ogType: "article", about: true},
		{path: "/built-with-muxmaster", title: "Built with MuxMaster", description: "How this website runs on MuxMaster: pooling opt-ins, Pre middleware, a GETFast static route, and pre-computed gzip responses, with measured per-request costs.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMasterWebsite", hasMarkdown: true, contentPath: "site/built-with-muxmaster.md", cache: cacheControlDocs, section: "built-with-muxmaster", ogType: "article"},
		{path: "/changelog", title: "Changelog", description: "The full upstream changelog: every released version of MuxMaster with one section per release, ordered newest-first. Mirrors CHANGELOG.md.", upstreamURL: ghRoot + "CHANGELOG.md", hasMarkdown: true, contentPath: "changelog.md", cache: cacheControlChangelog, section: "changelog", ogType: "article", about: true},
		{path: "/releases/v1.3.0", title: "Release notes — v1.3.0", description: "MuxMaster v1.3.0 release notes: Go 1.27.1 is the new minimum, the default OAuth2Introspect client no longer exhausts ports, and Group.ServeFiles is guarded.", upstreamURL: ghRoot + "release-notes/v1.3.0-20260926.md", hasMarkdown: true, contentPath: "release-notes/v1.3.0.md", cache: cacheControlRelease, section: "releases", ogType: "article", about: true, version: "1.3.0"},
		{path: "/releases/v1.2.0", title: "Release notes — v1.2.0", description: "MuxMaster v1.2.0 release notes: HTTP QUERY method (RFC 10008) support, Mount and routing fixes, and behaviour changes to redirects and four middleware.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.2.0/release-notes/v1.2.0-20260926.md", hasMarkdown: true, contentPath: "release-notes/v1.2.0.md", cache: cacheControlRelease, section: "releases", ogType: "article", about: true, version: "1.2.0"},
		{path: "/releases/v1.1.0", title: "Release notes — v1.1.0", description: "MuxMaster v1.1.0 release notes: the opt-in PoolRequestBundle and PoolFastParams pools, five new examples, and the maximum-performance guide.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/releases/tag/v1.1.0", hasMarkdown: true, contentPath: "release-notes/v1.1.0.md", cache: cacheControlRelease, section: "releases", ogType: "article", about: true, version: "1.1.0"},
		{path: "/releases/v1.0.0", title: "Release notes — v1.0.0", description: "Release notes for MuxMaster v1.0.0, the first general-availability release. Public API frozen, security guarantees stated, performance baseline established.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.0/release-notes/v1.0.0-20260508.md", hasMarkdown: true, contentPath: "release-notes/v1.0.0.md", cache: cacheControlRelease, section: "releases", ogType: "article", about: true, version: "1.0.0"},
		{path: "/security", title: "Security", description: "MuxMaster's security policy: supported versions, how to report an issue, current security behaviour, accepted limitations, and defaults that need opt-in.", upstreamURL: ghRoot + "SECURITY.md", hasMarkdown: true, contentPath: "security.md", cache: cacheControlDocs, section: "security", ogType: "article"},
		{path: "/compatibility", title: "Compatibility", description: "Supported Go versions, the four-tier SemVer guarantee policy, and the net/http interoperability contract that MuxMaster preserves end-to-end.", upstreamURL: ghRoot + "COMPATIBILITY.md", hasMarkdown: true, contentPath: "compatibility.md", cache: cacheControlDocs, section: "compatibility", ogType: "article"},
		{path: "/contributing", title: "Contributing", description: "How to contribute to MuxMaster: set up the repository, follow the development workflow and code guidelines, write commit messages, and open pull requests.", upstreamURL: ghRoot + "CONTRIBUTING.md", hasMarkdown: true, contentPath: "contributing.md", cache: cacheControlDocs, section: "contributing", ogType: "article"},
	}
}

// getHead registers the same handler for both GET and HEAD on the path.
// getHead registers h for GET and HEAD on path, tagged with path as its
// route_id. HEAD is required by HTTP semantics and by `curl -I` health
// probes; net/http drops the body of a HEAD response.
func getHead(m *muxm.Mux, path string, h http.HandlerFunc) {
	m.Match([]string{http.MethodGet, http.MethodHead}, path, withRoute(path, h))
}

// registerRoutes attaches every site route to the supplied *mux.Mux.
//
// Every public route (HTML and .md companion) is wired to a handler that
// reads from the prerender map populated by Server.Prerender. Per-request
// rendering does not exist (specification/rendering-and-caching.md
// "static-tending").
func (s *Server) registerRoutes(m *muxm.Mux) {
	// Operational endpoints first.
	getHead(m, "/healthz", s.healthzHandler())

	// Top-level text artefacts.
	getHead(m, "/robots.txt", s.renderer.ServePrerendered("/robots.txt", cacheControlText, http.StatusOK))
	getHead(m, "/sitemap.xml", s.renderer.ServePrerendered("/sitemap.xml", cacheControlSitemap, http.StatusOK))
	getHead(m, "/llms.txt", s.renderer.ServePrerendered("/llms.txt", cacheControlText, http.StatusOK))
	getHead(m, "/llms-full.txt", s.renderer.ServePrerendered("/llms-full.txt", cacheControlText, http.StatusOK))
	getHead(m, "/.well-known/security.txt", s.renderer.ServePrerendered("/.well-known/security.txt", cacheControlText, http.StatusOK))

	// Reserved legacy paths (see specification/url-and-versioning.md
	// "Reserved paths"). /favicon.ico must respond — every browser, RSS
	// reader, and aggregator probes it by reflex.
	getHead(m, "/favicon.ico", faviconRedirectHandler())

	// Landing.
	getHead(m, "/", s.renderer.ServePrerendered("/", cacheControlLanding, http.StatusOK))
	getHead(m, "/index.md", s.renderer.ServePrerendered("/index.md", cacheControlLanding, http.StatusOK))

	// Section indexes (HTML + Markdown companion).
	getHead(m, "/docs/", s.renderer.ServePrerendered("/docs/", cacheControlLanding, http.StatusOK))
	getHead(m, "/docs/index.md", s.renderer.ServePrerendered("/docs/index.md", cacheControlLanding, http.StatusOK))
	getHead(m, "/examples/", s.renderer.ServePrerendered("/examples/", cacheControlLanding, http.StatusOK))
	getHead(m, "/examples/index.md", s.renderer.ServePrerendered("/examples/index.md", cacheControlLanding, http.StatusOK))

	// Doc-page family: every Markdown-backed route plus its .md companion.
	for _, r := range docRoutes() {
		if r.path == "/docs/" || r.path == "/examples/" {
			// Section indexes wired above; the .md companions for the
			// indexes are explicitly out of scope today (not required by
			// specification/content-sources.md "Markdown companions").
			continue
		}
		getHead(m, r.path, s.renderer.ServePrerendered(r.path, r.cache, http.StatusOK))
		if r.hasMarkdown && r.contentPath != "" {
			getHead(m, r.path+".md", s.renderer.ServePrerendered(r.path+".md", r.cache, http.StatusOK))
		}
	}

	// Static assets, served from memory by a FastHandler: the router hands
	// the *filepath parameter over as an argument and, with
	// Mux.PoolFastParams, recycles it after the call.
	notFound := s.notFoundHandler()
	const staticPattern = "/static/*filepath"
	staticHandler := withRouteFast(staticPattern, s.static.handler(notFound))
	m.GETFast(staticPattern, staticHandler)
	m.HEADFast(staticPattern, staticHandler)

	// Branded 404 served from the prerender cache, with status 404 and
	// Cache-Control: no-store per spec.
	m.NotFound = notFound
}
