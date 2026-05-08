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
	path        string
	title       string
	description string
	upstreamURL string
	hasMarkdown bool
	cache       string
}

// routeInfos returns every HTML route for use by the prerender recipes
// (sitemap.xml, llms.txt, llms-full.txt, docs/examples indexes). The landing
// page is included; operational endpoints, text artefacts, and error
// templates are not. Markdown companions are not enumerated here — they
// derive from HasMarkdown.
func routeInfos() []render.RouteInfo {
	const landingDesc = "MuxMaster is a high-performance, zero-dependency HTTP router for Go. Radix-tree O(k) lookups, zero allocations on static routes, and 100% net/http compatibility."
	out := []render.RouteInfo{
		{Path: "/", Title: "MuxMaster", Description: landingDesc, Section: "landing", HasMarkdown: false},
	}
	for _, r := range docRoutes() {
		out = append(out, render.RouteInfo{
			Path:        r.path,
			Title:       r.title,
			Description: r.description,
			Section:     sectionForPath(r.path),
			HasMarkdown: r.hasMarkdown,
		})
	}
	return out
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
func docRoutes() []route {
	const ghDocs = "https://github.com/FlavioCFOliveira/MuxMaster/blob/main/docs/"
	const ghRoot = "https://github.com/FlavioCFOliveira/MuxMaster/blob/main/"
	return []route{
		// /docs/ index plus the eleven sub-sections.
		{path: "/docs/", title: "Documentation", description: "Index of MuxMaster documentation: getting started, routing, groups, middleware, error handling, configuration, response helpers, performance, observability, migration, and a cookbook.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/docs", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/getting-started", title: "Getting started", description: "Install MuxMaster, register a route, run the server, and verify the response. The shortest path to a running router.", upstreamURL: ghDocs + "getting-started.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/routing", title: "Routing", description: "Define static and parametric routes, pattern syntax, and match precedence on the MuxMaster radix tree.", upstreamURL: ghDocs + "routing.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/groups", title: "Groups", description: "Compose route prefixes, scoped middleware, and nested groups with mux.Group.", upstreamURL: ghDocs + "groups.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/middleware", title: "Middleware", description: "Built-in middleware: Recoverer, RequestID, Logger, Compress, RealIP, Timeout, Throttle, BasicAuth, JWTAuth, OAuth2Introspect, APIKey, CORS, SetHeader.", upstreamURL: ghDocs + "middleware.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/error-handling", title: "Error handling", description: "HandlerFuncE and the typed error pipeline. How errors are surfaced, logged, and rendered.", upstreamURL: ghDocs + "error-handling.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/configuration", title: "Configuration", description: "RedirectTrailingSlash, RedirectFixedPath, HandleMethodNotAllowed, CaseInsensitive, NotFound, MethodNotAllowed, and the security-relevant defaults.", upstreamURL: ghDocs + "configuration.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/response-helpers", title: "Response helpers", description: "JSON, Text, NoContent, and the typed parameter accessors that read URL params without reflection.", upstreamURL: ghDocs + "response-helpers.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/performance", title: "Performance", description: "Allocation profile, FastHandler, the radix-tree lookup cost model, and the benchmarks that back the numbers.", upstreamURL: ghDocs + "performance.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/observability", title: "Observability", description: "Structured logging with slog, RequestID correlation, custom Prometheus metrics middleware, OpenTelemetry tracing, health checks, and pprof.", upstreamURL: ghDocs + "observability.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/migration", title: "Migration", description: "Mapping idioms from net/http, gorilla/mux, chi, gin, and httprouter onto MuxMaster.", upstreamURL: ghDocs + "migration.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/docs/cookbook", title: "Cookbook", description: "Recipes for the patterns that come up most often: graceful shutdown, file uploads, server-sent events, static files, and more.", upstreamURL: ghDocs + "cookbook.md", hasMarkdown: true, cache: cacheControlDocs},

		{path: "/api", title: "API reference", description: "Complete API reference for MuxMaster: every exported type, function, method, and field, with signatures and behavioural notes.", upstreamURL: ghRoot + "api.md", hasMarkdown: true, cache: cacheControlDocs},

		// Examples index plus the eight upstream examples.
		{path: "/examples/", title: "Examples", description: "Eight runnable examples covering REST APIs, authentication, JWT, OAuth2, caching, graceful shutdown, server-side rendering, and static sites.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/examples/rest-api", title: "REST API example", description: "A small REST API built with MuxMaster: route grouping, JSON responses, and parameter parsing.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/rest-api", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/examples/authn", title: "Authentication example", description: "Username and password authentication with the BasicAuth middleware.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/authn", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/examples/jwt", title: "JWT example", description: "Bearer-token authentication with the JWTAuth middleware and required-expiry hardening.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/jwt", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/examples/oauth2", title: "OAuth2 example", description: "OAuth2 token introspection with the OAuth2Introspect middleware.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/oauth2", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/examples/cache", title: "Cache example", description: "HTTP caching headers and a small in-process response cache.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/cache", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/examples/graceful-shutdown", title: "Graceful shutdown example", description: "Signal-driven srv.Shutdown with a bounded drain deadline and the recommended timeout set.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/graceful-shutdown", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/examples/server-side-render", title: "Server-side render example", description: "html/template-based server-side rendering with MuxMaster.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/server-side-render", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/examples/static-site", title: "Static site example", description: "Serving a static site with MuxMaster's ServeFiles primitive.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/static-site", hasMarkdown: true, cache: cacheControlDocs},

		{path: "/benchmarks", title: "Benchmarks", description: "Benchmark numbers quoted verbatim from the upstream MuxMaster benchmark suite, with source-file citations.", upstreamURL: ghRoot + "bench_test.go", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/changelog", title: "Changelog", description: "Full upstream changelog: every released version of MuxMaster, with one section per release.", upstreamURL: ghRoot + "CHANGELOG.md", hasMarkdown: true, cache: cacheControlChangelog},
		{path: "/releases/v1.0.0", title: "Release notes — v1.0.0", description: "Release notes for MuxMaster v1.0.0 (the first general-availability release).", upstreamURL: ghRoot + "release-notes/v1.0.0-20260508.md", hasMarkdown: true, cache: cacheControlRelease},
		{path: "/security", title: "Security", description: "MuxMaster's security policy, threat model, resolved findings, and the operator-facing defaults that require explicit opt-in.", upstreamURL: ghRoot + "SECURITY.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/compatibility", title: "Compatibility", description: "Supported Go versions, semantic-versioning guarantees, and net/http interoperability.", upstreamURL: ghRoot + "COMPATIBILITY.md", hasMarkdown: true, cache: cacheControlDocs},
		{path: "/contributing", title: "Contributing", description: "How to contribute to MuxMaster: branch policy, tests, security review, and the gatekeeper-agent workflow.", upstreamURL: ghRoot + "CONTRIBUTING.md", hasMarkdown: true, cache: cacheControlDocs},
	}
}

// getHead registers the same handler for both GET and HEAD on the path.
// HEAD is required by HTTP semantics and by `curl -I` health probes; the
// underlying http.ResponseWriter behaviour drops the body on HEAD.
func getHead(m *muxm.Mux, path string, h http.HandlerFunc) {
	m.Match([]string{http.MethodGet, http.MethodHead}, path, h)
}

// registerRoutes attaches every site route to the supplied *mux.Mux.
//
// Category A routes (landing, /docs/, /examples/, /llms.txt, /llms-full.txt,
// /sitemap.xml, /robots.txt, /404, /500) are wired to handlers that read from
// the prerender cache populated by Server.Prerender. They MUST NOT execute
// html/template at request time. Category B routes still use the placeholder
// coming-soon handlers in this round; the next round (live upstream Markdown)
// replaces them.
func (s *Server) registerRoutes(m *muxm.Mux) {
	// Operational endpoints first.
	getHead(m, "/healthz", s.healthzHandler())

	// Top-level text artefacts (Category A).
	getHead(m, "/robots.txt", s.renderer.ServePrerendered("/robots.txt", cacheControlText, http.StatusOK))
	getHead(m, "/sitemap.xml", s.renderer.ServePrerendered("/sitemap.xml", cacheControlSitemap, http.StatusOK))
	getHead(m, "/llms.txt", s.renderer.ServePrerendered("/llms.txt", cacheControlText, http.StatusOK))
	getHead(m, "/llms-full.txt", s.renderer.ServePrerendered("/llms-full.txt", cacheControlText, http.StatusOK))

	// Landing (Category A).
	getHead(m, "/", s.renderer.ServePrerendered("/", cacheControlLanding, http.StatusOK))

	// Section indexes (Category A).
	getHead(m, "/docs/", s.renderer.ServePrerendered("/docs/", cacheControlLanding, http.StatusOK))
	getHead(m, "/examples/", s.renderer.ServePrerendered("/examples/", cacheControlLanding, http.StatusOK))

	// Category B routes — still placeholders in this round.
	for _, r := range docRoutes() {
		// /docs/ and /examples/ are Category A and registered above.
		if r.path == "/docs/" || r.path == "/examples/" {
			if r.hasMarkdown {
				getHead(m, r.path+".md", s.markdownStubHandler(r.title, r.upstreamURL))
			}
			continue
		}
		htmlH := s.comingSoonHandler(r.title, r.description, r.upstreamURL, r.cache)
		getHead(m, r.path, htmlH)
		if r.hasMarkdown {
			getHead(m, r.path+".md", s.markdownStubHandler(r.title, r.upstreamURL))
		}
	}

	// Static assets. We re-implement the trivial "file server with prefix"
	// pattern that mux.ServeFiles offers, because we need to vary
	// Cache-Control by file path (long-immutable for the hashed CSS,
	// conservative default elsewhere). The path-safety properties of
	// http.FileServer (path.Clean against traversal) still apply.
	staticHandler := s.staticCacheHandler()
	m.HandleFunc(http.MethodGet, "/static/*filepath", staticHandler)
	m.HandleFunc(http.MethodHead, "/static/*filepath", staticHandler)

	// Branded 404 served from the prerender cache. The serve helper applies
	// status 404 and the Cache-Control: no-store header per spec.
	m.NotFound = s.notFoundFromPrerender()
}

// staticCacheHandler serves files from staticDir, applying per-path
// Cache-Control headers. The hashed CSS bundle gets a one-year immutable
// cache; everything else gets a conservative one-day default.
func (s *Server) staticCacheHandler() http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(s.staticDir))
	hashedCSS := s.renderer.CSSPath()
	return func(w http.ResponseWriter, r *http.Request) {
		filepathParam := muxm.PathParam(r, "filepath")
		// Mirror http.FileServer's expectation that the URL path is the
		// resource path. ServeFiles does the same internally.
		r.URL.Path = filepathParam
		if r.URL.Path != "" && r.URL.Path[0] != '/' {
			r.URL.Path = "/" + r.URL.Path
		}
		if "/static"+r.URL.Path == hashedCSS {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		fileServer.ServeHTTP(w, r)
	}
}
