package server

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	muxm "github.com/FlavioCFOliveira/MuxMaster"
	mwm "github.com/FlavioCFOliveira/MuxMaster/middleware"
)

// statusRecorder captures the response status and byte count for the access
// log, and the matched route pattern for its route_id field. Recorders are
// recycled through recorderPool: the access log owns each one for exactly
// one request, and no handler retains its ResponseWriter past return.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
	route  string
}

var recorderPool = sync.Pool{New: func() any { return new(statusRecorder) }}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap exposes the underlying ResponseWriter to http.ResponseController.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// setRoute records pattern as the route_id of the request that w belongs to.
// It walks the Unwrap chain because a Pre middleware between the access log
// and the router may wrap the writer; today none does, and the first type
// assertion succeeds.
func setRoute(w http.ResponseWriter, pattern string) {
	for {
		if rec, ok := w.(*statusRecorder); ok {
			rec.route = pattern
			return
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return
		}
		w = u.Unwrap()
	}
}

// withRoute wraps a route handler so the access log can report the pattern
// it was registered under. The pattern is known at registration time, so
// nothing is looked up per request.
//
// MuxMaster's RoutePattern(r) cannot serve here: the access log runs in the
// Pre chain, before the router stores the pattern, and FastHandler routes do
// not store it at all.
func withRoute(pattern string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setRoute(w, pattern)
		h(w, r)
	}
}

// withRouteFast is withRoute for muxmaster.FastHandler routes.
func withRouteFast(pattern string, h muxm.FastHandler) muxm.FastHandler {
	return func(w http.ResponseWriter, r *http.Request, ps muxm.Params) {
		setRoute(w, pattern)
		h(w, r, ps)
	}
}

// slogAccessLog produces one structured JSON log line per completed request,
// covering every field required by specification/rendering-and-caching.md
// "Logs". route_id is the pattern recorded by withRoute; it stays empty for
// requests no route matched (404s, normalisation redirects).
func slogAccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := recorderPool.Get().(*statusRecorder) //nolint:forcetypeassert // the pool only holds *statusRecorder
			*rec = statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			logger.LogAttrs(r.Context(), slog.LevelInfo, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int("bytes", rec.bytes),
				slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
				slog.String("remote_addr", clientAddr(r)),
				slog.String("user_agent", r.UserAgent()),
				slog.String("referer", r.Referer()),
				slog.String("request_id", mwm.GetRequestID(r.Context())),
				slog.String("route_id", rec.route),
			)

			*rec = statusRecorder{}
			recorderPool.Put(rec)
		})
	}
}

func clientAddr(r *http.Request) string {
	// r.RemoteAddr is the resolved client address. When a trusted proxy
	// is in front (config.TrustedProxyCIDRs is non-empty), the upstream
	// RealIP middleware rewrites r.RemoteAddr from X-Forwarded-For
	// before this access logger runs. When no proxy is trusted, the
	// raw peer address is used and any forged X-Forwarded-For from the
	// network is ignored. See spec/deployment.md § Reverse-proxy contract.
	return r.RemoteAddr
}

// Security header values (specification/seo.md "Security headers"), built
// once. The Content-Security-Policy has two variants because
// upgrade-insecure-requests is only sent when the edge is HTTPS; otherwise
// the browser would upgrade every sub-resource and break a plain-HTTP origin
// (e.g. development on a non-localhost hostname).
const cspBase = "default-src 'self'; img-src 'self' data:; style-src 'self'; " +
	"script-src 'self'; font-src 'self'; connect-src 'self'; " +
	"frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

var (
	hdrCSP                = []string{cspBase}
	hdrCSPHTTPS           = []string{cspBase + "; upgrade-insecure-requests"}
	hdrNoSniff            = []string{"nosniff"}
	hdrReferrerPolicy     = []string{"strict-origin-when-cross-origin"}
	hdrPermissionsPolicy  = []string{"accelerometer=(), camera=(), geolocation=(), gyroscope=(), microphone=(), payment=(), usb=()"}
	hdrFrameOptions       = []string{"DENY"}
	hdrSameOrigin         = []string{"same-origin"}
	hdrStrictTransportSec = []string{"max-age=63072000; includeSubDomains; preload"}
)

// securityHeaders sets the security headers on every response. HSTS, COOP
// and CORP are sent only when the edge is HTTPS: browsers ignore them on
// plain-HTTP origins and Chromium logs a console warning ("the URL's origin
// was untrustworthy") that Lighthouse counts as a Best-Practices failure.
// The binary speaks cleartext h2c; the reverse proxy terminates TLS.
//
// The values are shared []string assigned directly into the header map with
// canonical keys, so the middleware allocates nothing.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		https := r.TLS != nil || headerIs(r.Header["X-Forwarded-Proto"], "https")
		if https {
			h["Content-Security-Policy"] = hdrCSPHTTPS
		} else {
			h["Content-Security-Policy"] = hdrCSP
		}
		h["X-Content-Type-Options"] = hdrNoSniff
		h["Referrer-Policy"] = hdrReferrerPolicy
		h["Permissions-Policy"] = hdrPermissionsPolicy
		h["X-Frame-Options"] = hdrFrameOptions
		if https {
			h["Cross-Origin-Opener-Policy"] = hdrSameOrigin
			h["Cross-Origin-Resource-Policy"] = hdrSameOrigin
			h["Strict-Transport-Security"] = hdrStrictTransportSec
		}
		next.ServeHTTP(w, r)
	})
}

// headerIs reports whether the first value of a header equals want.
func headerIs(values []string, want string) bool {
	return len(values) > 0 && values[0] == want
}
