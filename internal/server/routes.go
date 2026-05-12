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
	contentPath string // Loader path under /content/, e.g. "docs/routing.md".
	cache       string
	section     string // Used by recipe-side breadcrumb selection.
	ogType      string
	// order is the curated rank within section for index pages. It exists
	// because /examples/ wants a learning sequence, not the alphabetical
	// order produced by path-lex sorting. Zero means "no opinion" — the
	// recipe falls back to path-lex.
	order int
}

// routeInfos returns every HTML route for the prerender recipes (sitemap,
// llms.txt, llms-full.txt, docs/examples indexes). The landing page is
// included; operational endpoints, text artefacts, and error templates are
// not. Markdown companions are not enumerated here — they derive from
// HasMarkdown.
func routeInfos() []render.RouteInfo {
	const landingDesc = "MuxMaster is a high-performance, zero-dependency HTTP router for Go. Radix-tree O(k) lookups, zero static-route allocations, 100% net/http compatible."
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
			Path:        r.path,
			Title:       r.title,
			Description: r.description,
			ContentPath: r.contentPath,
			Section:     r.section,
			UpstreamURL: r.upstreamURL,
			Cache:       r.cache,
			OGType:      r.ogType,
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
		{path: "/docs/", title: "Documentation", description: "Index of MuxMaster documentation: getting started, routing, groups, middleware, error handling, configuration, performance, observability, and the cookbook.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/docs", hasMarkdown: false, contentPath: "", cache: cacheControlLanding, section: "docs", ogType: "article"},
		{path: "/docs/getting-started", title: "Getting started", description: "Install MuxMaster, register a route, run the server, and verify the response. The shortest path to a running router.", upstreamURL: ghDocs + "getting-started.md", hasMarkdown: true, contentPath: "docs/getting-started.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/routing", title: "Routing", description: "Define static, parametric, and catch-all routes; pattern syntax; match precedence; and the trade-offs the MuxMaster radix tree makes.", upstreamURL: ghDocs + "routing.md", hasMarkdown: true, contentPath: "docs/routing.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/groups", title: "Groups", description: "Compose route prefixes, scoped middleware, and nested groups with mux.Group. Includes the With, Route, and Mount idioms with worked examples.", upstreamURL: ghDocs + "groups.md", hasMarkdown: true, contentPath: "docs/groups.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/middleware", title: "Middleware", description: "The 17 bundled middleware constructors covering request lifecycle (RequestID, Recoverer, Logger), transport (Compress, RealIP, CleanPath, StripSlashes, NoCache, SetHeader), throughput (Timeout, Throttle), and authentication (BasicAuth, APIKey, JWTAuth, OAuth2Introspect, CORS).", upstreamURL: ghDocs + "middleware.md", hasMarkdown: true, contentPath: "docs/middleware.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/error-handling", title: "Error handling", description: "HandlerFuncE, HTTPError, and the typed error pipeline. How errors are surfaced, logged, rendered, and turned into HTTP responses.", upstreamURL: ghDocs + "error-handling.md", hasMarkdown: true, contentPath: "docs/error-handling.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/configuration", title: "Configuration", description: "RedirectTrailingSlash, RedirectFixedPath, HandleMethodNotAllowed, CaseInsensitive, NotFound, MethodNotAllowed, and the security-relevant defaults.", upstreamURL: ghDocs + "configuration.md", hasMarkdown: true, contentPath: "docs/configuration.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/response-helpers", title: "Response helpers", description: "JSON, XML, Text, Redirect, NoContent, and the typed Params accessors that read URL parameters without reflection or per-request allocation.", upstreamURL: ghDocs + "response-helpers.md", hasMarkdown: true, contentPath: "docs/response-helpers.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/performance", title: "Performance", description: "Allocation profile, FastHandler, the radix-tree lookup cost model, and the benchmarks that back the documented latency and throughput numbers.", upstreamURL: ghDocs + "performance.md", hasMarkdown: true, contentPath: "docs/performance.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/max-performance", title: "Maximum performance", description: "Zero-allocation hot path: the PoolRequestBundle and PoolFastParams opt-ins, the strict lifetime contract for handlers, the failure modes when the contract is broken, a four-recipe playbook, and the benchmark methodology. 45 ns / 0 B / 0 allocs on a one-parameter route at 100% net/http compatibility.", upstreamURL: ghDocs + "max-performance.md", hasMarkdown: true, contentPath: "docs/max-performance.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/observability", title: "Observability", description: "Structured logging with slog, RequestID correlation, custom Prometheus metrics middleware, OpenTelemetry tracing, health checks, and pprof.", upstreamURL: ghDocs + "observability.md", hasMarkdown: true, contentPath: "docs/observability.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/migration", title: "Migration", description: "Mapping idioms from net/http, gorilla/mux, chi, gin, and httprouter onto MuxMaster. Side-by-side examples for the most common migration paths.", upstreamURL: ghDocs + "migration.md", hasMarkdown: true, contentPath: "docs/migration.md", cache: cacheControlDocs, section: "docs", ogType: "article"},
		{path: "/docs/cookbook", title: "Cookbook", description: "Recipes for the patterns that come up most often: graceful shutdown, file uploads, server-sent events, static files, and more.", upstreamURL: ghDocs + "cookbook.md", hasMarkdown: true, contentPath: "docs/cookbook.md", cache: cacheControlDocs, section: "docs", ogType: "article"},

		{path: "/api", title: "API reference", description: "Complete API reference for MuxMaster: every exported type, function, method, and field, with signatures and behavioural notes.", upstreamURL: ghRoot + "api.md", hasMarkdown: true, contentPath: "api.md", cache: cacheControlDocs, section: "api", ogType: "article"},

		// Examples index plus the eight upstream examples.
		{path: "/examples/", title: "Examples", description: "Eight runnable examples covering REST APIs, authentication, JWT, OAuth2, caching, graceful shutdown, server-side rendering, and static sites.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples", hasMarkdown: false, contentPath: "", cache: cacheControlLanding, section: "examples", ogType: "article"},
		{path: "/examples/rest-api", title: "REST API example", description: "A complete REST API built with MuxMaster: route grouping, JSON responses, parameter parsing, and a small in-memory store. CRUD over a single resource.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/rest-api", hasMarkdown: true, contentPath: "examples/rest-api.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 1},
		{path: "/examples/authn", title: "Authentication example", description: "HTTP Basic Authentication via the BasicAuth middleware, paired with ThrottlePerIP to defend against credential-stuffing attacks.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/authn", hasMarkdown: true, contentPath: "examples/authn.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 2},
		{path: "/examples/jwt", title: "JWT example", description: "Bearer-token authentication via the JWTAuth middleware. Configures RequireExpiry: true (RFC 8725 §4.4) and shows the canonical OIDC integration shape.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/jwt", hasMarkdown: true, contentPath: "examples/jwt.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 3},
		{path: "/examples/oauth2", title: "OAuth2 example", description: "OAuth 2.0 token introspection (RFC 7662) via the OAuth2Introspect middleware. Tokens are validated against an authorisation server, not locally.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/oauth2", hasMarkdown: true, contentPath: "examples/oauth2.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 4},
		{path: "/examples/cache", title: "Cache example", description: "An in-memory TTL cache that avoids re-computing identical responses within a configurable horizon. For expensive, idempotent handlers.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/cache", hasMarkdown: true, contentPath: "examples/cache.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 5},
		{path: "/examples/graceful-shutdown", title: "Graceful shutdown example", description: "Production graceful shutdown: signal handling, srv.Shutdown with a bounded drain deadline, and the recommended Server timeout set for real deployments.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/graceful-shutdown", hasMarkdown: true, contentPath: "examples/graceful-shutdown.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 6},
		{path: "/examples/server-side-render", title: "Server-side render example", description: "A multi-page guestbook rendered by Go's html/template. The same SSR pattern that powers this documentation website you are reading now.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/server-side-render", hasMarkdown: true, contentPath: "examples/server-side-render.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 7},
		{path: "/examples/static-site", title: "Static site example", description: "Conditional GET (304 via ETag and Last-Modified) and range requests (206) on top of MuxMaster's ServeFiles primitive. Production static-asset semantics.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/static-site", hasMarkdown: true, contentPath: "examples/static-site.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 8},
		{path: "/examples/versioning", title: "Versioning example", description: "Path-based (/v1, /v2) plus header-based (Accept: ...;v=N) API versioning on the same router, with nested admin groups and PoolRequestBundle = true for zero-allocation dispatch across every version branch.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/versioning", hasMarkdown: true, contentPath: "examples/versioning.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 9},
		{path: "/examples/reverse-proxy", title: "Reverse-proxy example", description: "An HTTP gateway mounting net/http/httputil.ReverseProxy on MuxMaster: catch-all path routing, round-robin load balancing with a lock-free atomic counter, per-route gating for the admin upstream, and pool-safe dispatch (the proxy returns before ServeHTTP exits).", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/reverse-proxy", hasMarkdown: true, contentPath: "examples/reverse-proxy.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 10},
		{path: "/examples/server-sent-events", title: "Server-sent events example", description: "A Server-Sent Events (SSE) endpoint with PoolRequestBundle enabled. The streaming handler keeps the request alive until client disconnect, so pooling is safe. Includes a topic-hub fan-out, a synchronous /publish endpoint with body-size cap, a periodic server-side tick, and an in-browser demo page.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/server-sent-events", hasMarkdown: true, contentPath: "examples/server-sent-events.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 11},
		{path: "/examples/upload-file", title: "Upload-file example", description: "Multipart file upload with PoolRequestBundle and the body-drain-before-spawn pattern that makes background processing pool-safe. Three handlers — single-file sync, multi-file sync, and async with goroutine — plus 32 MiB MaxBytesReader caps, path-traversal guards, and streaming SHA-256.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/tree/main/examples/upload-file", hasMarkdown: true, contentPath: "examples/upload-file.md", cache: cacheControlDocs, section: "examples", ogType: "article", order: 12},

		{path: "/benchmarks", title: "Benchmarks", description: "Benchmark numbers from the upstream MuxMaster v1.1.0 suite, quoted verbatim with source-file citations. Static lookups at 25 ns; one-parameter routes at 45 ns / 0 B / 0 allocs with the opt-in PoolRequestBundle; competitor comparison across httprouter, Fiber v3, bunrouter, chi v5, and gorilla/mux.", upstreamURL: ghRoot + "bench_test.go", hasMarkdown: true, contentPath: "benchmarks.md", cache: cacheControlDocs, section: "benchmarks", ogType: "article"},
		{path: "/changelog", title: "Changelog", description: "The full upstream changelog: every released version of MuxMaster with one section per release, ordered newest-first. Mirrors CHANGELOG.md.", upstreamURL: ghRoot + "CHANGELOG.md", hasMarkdown: true, contentPath: "changelog.md", cache: cacheControlChangelog, section: "changelog", ogType: "article"},
		{path: "/releases/v1.1.0", title: "Release notes — v1.1.0", description: "Release notes for MuxMaster v1.1.0, the minor release focused on maximum performance: 45 ns / 0 B / 0 allocs on the Pooled hot path, five new examples, and the canonical /docs/max-performance guide. Backward-compatible with v1.0.x.", upstreamURL: "https://github.com/FlavioCFOliveira/MuxMaster/releases/tag/v1.1.0", hasMarkdown: true, contentPath: "release-notes/v1.1.0.md", cache: cacheControlRelease, section: "releases", ogType: "article"},
		{path: "/releases/v1.0.0", title: "Release notes — v1.0.0", description: "Release notes for MuxMaster v1.0.0, the first general-availability release. Public API frozen, security guarantees stated, performance baseline established.", upstreamURL: ghRoot + "release-notes/v1.0.0-20260508.md", hasMarkdown: true, contentPath: "release-notes/v1.0.0.md", cache: cacheControlRelease, section: "releases", ogType: "article"},
		{path: "/security", title: "Security", description: "MuxMaster's security policy, threat model, resolved findings, and the operator-facing defaults that require explicit opt-in.", upstreamURL: ghRoot + "SECURITY.md", hasMarkdown: true, contentPath: "security.md", cache: cacheControlDocs, section: "security", ogType: "article"},
		{path: "/compatibility", title: "Compatibility", description: "Supported Go versions, the four-tier SemVer guarantee policy, and the net/http interoperability contract that MuxMaster preserves end-to-end.", upstreamURL: ghRoot + "COMPATIBILITY.md", hasMarkdown: true, contentPath: "compatibility.md", cache: cacheControlDocs, section: "compatibility", ogType: "article"},
		{path: "/contributing", title: "Contributing", description: "How to contribute to MuxMaster: branch policy, tests, security-review expectations, and the gatekeeper-agent workflow that gates every merge.", upstreamURL: ghRoot + "CONTRIBUTING.md", hasMarkdown: true, contentPath: "contributing.md", cache: cacheControlDocs, section: "contributing", ogType: "article"},
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
//
// Directory paths (resolved to a directory rather than a regular file)
// return 404 rather than the http.FileServer default of rendering the
// directory's HTML index. Surface enumeration is an information-
// disclosure vector that contradicts the site's strict CSP posture; the
// only legitimate way to reach a static asset is its full URL.
func (s *Server) staticCacheHandler() http.HandlerFunc {
	root := http.Dir(s.staticDir)
	fileServer := http.FileServer(root)
	hashedCSS := s.renderer.CSSPath()
	return func(w http.ResponseWriter, r *http.Request) {
		filepathParam := muxm.PathParam(r, "filepath")
		// Mirror http.FileServer's expectation that the URL path is the
		// resource path. ServeFiles does the same internally.
		r.URL.Path = filepathParam
		if r.URL.Path != "" && r.URL.Path[0] != '/' {
			r.URL.Path = "/" + r.URL.Path
		}
		// Reject any path that resolves to a directory. http.FileServer's
		// default is to render an HTML index of the directory; the static
		// tree of this site is not intended to be enumerable.
		if isDirectory(root, r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		if "/static"+r.URL.Path == hashedCSS {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		fileServer.ServeHTTP(w, r)
	}
}

// isDirectory reports whether name resolves to a directory inside root.
// An empty name (root URL "/static/") and a trailing-slash path both
// resolve to a directory and MUST be rejected by callers that do not
// want to expose directory enumeration. Errors opening name fall
// through as "not a directory" so the caller's regular http.FileServer
// path handles the 404 with its existing semantics.
func isDirectory(root http.FileSystem, name string) bool {
	if name == "" || name == "/" || strings.HasSuffix(name, "/") {
		return true
	}
	f, err := root.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return stat.IsDir()
}
