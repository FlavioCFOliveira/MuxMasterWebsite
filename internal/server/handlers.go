package server

import (
	"net/http"
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

// Header values for /healthz, built once. Response.Serve documents why a
// shared []string with len == cap is safe to assign into every response.
var (
	healthzBody          = []byte("ok\n")
	healthzContentType   = []string{"text/plain; charset=utf-8"}
	healthzCacheControl  = []string{cacheControlNoStore}
	healthzVary          = []string{"Accept-Encoding"}
	healthzContentLength = []string{"3"}
)

// healthzHandler is the operational endpoint. Plain text, never cached.
func (s *Server) healthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		h := w.Header()
		h["Content-Type"] = healthzContentType
		h["Cache-Control"] = healthzCacheControl
		h["Vary"] = healthzVary
		h["Content-Length"] = healthzContentLength
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(healthzBody)
	}
}

// notFoundHandler returns the branded 404: the pre-rendered /404 page with
// status 404 and Cache-Control: no-store, per the spec.
func (s *Server) notFoundHandler() http.HandlerFunc {
	return s.renderer.ServePrerendered("/404", cacheControlNoStore, http.StatusNotFound)
}

// LandingDescription is the canonical landing-page description used by the
// recipes and any handler that needs it.
const LandingDescription = "MuxMaster: Go router with HTTP QUERY method (RFC 10008) support, faster on static routes than httprouter, bunrouter, chi, and gorilla/mux in every mode."

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
