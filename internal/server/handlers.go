package server

import (
	"net/http"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/render"
)

// Cache-Control values per route family — taken verbatim from
// specification/rendering-and-caching.md "HTTP cache headers per route
// family". Kept centralised so a spec change shows up in one diff.
const (
	cacheControlLanding   = "public, max-age=300, stale-while-revalidate=60"
	cacheControlDocs      = "public, max-age=600, stale-while-revalidate=120"
	cacheControlChangelog = "public, max-age=300, stale-while-revalidate=60"
	cacheControlRelease   = "public, max-age=86400, immutable"
	cacheControlText      = "public, max-age=300"
	cacheControlSitemap   = "public, max-age=300"
	cacheControlNoStore   = "no-store"
)

// healthzHandler is the operational endpoint. Plain text, never cached.
func (s *Server) healthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", cacheControlNoStore)
		w.Header().Set("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}
}

// notFoundFromPrerender returns the branded 404 handler. The body is read
// from the prerender map at request time (registration order is route
// registration first, prerender second); the handler overrides the status
// code to 404 and Cache-Control to no-store per spec.
func (s *Server) notFoundFromPrerender() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pre, ok := s.renderer.Prerendered("/404")
		if !ok {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", cacheControlNoStore)
			w.Header().Set("Vary", "Accept-Encoding")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("404 Not Found\n"))
			return
		}
		w.Header().Set("ETag", pre.ETag)
		w.Header().Set("Cache-Control", cacheControlNoStore)
		w.Header().Set("Vary", "Accept-Encoding")
		if render.MatchesIfNoneMatch(r, pre.ETag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", pre.ContentType)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(pre.Body)
	}
}

// LandingDescription is the canonical landing-page description used by the
// recipes and any handler that needs it.
const LandingDescription = "Zero-dependency Go HTTP router. Radix-tree O(k), 25 ns static, 45 ns / 0 alloc one-parameter Pooled — 20 % faster than httprouter, 100 % net/http compatible."

func (s *Server) isProduction() bool {
	return string(s.cfg.Env) == "production"
}

// faviconRedirectHandler serves /favicon.ico as a 301 redirect to the 32x32
// hashed favicon. Per specification/url-and-versioning.md "Reserved paths":
// browsers, RSS readers, bookmark engines, and various aggregators probe
// /favicon.ico by reflex even when the page declares modern <link rel=icon>
// alternatives. Returning a 7 KB HTML 404 body for every such probe wastes
// bandwidth and pollutes logs; a 301 to the smallest existing favicon variant
// is small, cacheable for a day, and resolves the request in a single hop.
// The redirect Location is path-only — the host header is never echoed back
// to the client, mirroring the defensive pattern of normalisationRedirects.
func faviconRedirectHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/static/favicon/favicon-32.png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusMovedPermanently)
	}
}
