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
const LandingDescription = "MuxMaster is a high-performance, zero-dependency HTTP router for Go. Radix-tree O(k) lookups, zero allocations on static routes, and 100% net/http compatibility."

func (s *Server) isProduction() bool {
	return string(s.cfg.Env) == "production"
}
