package server

import (
	"log/slog"
	"net/http"
	"time"

	muxm "github.com/FlavioCFOliveira/MuxMaster"
	mwm "github.com/FlavioCFOliveira/MuxMaster/middleware"
)

// statusRecorder captures the response status and byte count for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// slogAccessLog produces one structured JSON log line per completed request,
// covering every field required by specification/deployment.md.
func slogAccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
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
				slog.String("route_id", muxm.RoutePattern(r)),
			)
		})
	}
}

func clientAddr(r *http.Request) string {
	// X-Forwarded-For trust is governed by the reverse proxy contract; the
	// left-most entry is conventionally the original client. RealIP middleware
	// (with trusted CIDRs) is the production-grade resolver and is added by
	// the deployment overlay, not here.
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		for i := 0; i < len(v); i++ {
			if v[i] == ',' {
				return v[:i]
			}
		}
		return v
	}
	return r.RemoteAddr
}

// securityHeaders sets the static security headers required by
// specification/seo.md "Security headers". HSTS is intentionally omitted here
// because the binary speaks cleartext h2c; the reverse proxy is responsible
// for HSTS in production.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// Conservative starter CSP. Inline scripts and styles are forbidden.
		// `upgrade-insecure-requests` is appended only when the request is
		// already HTTPS at the edge — otherwise the browser would auto-upgrade
		// every sub-resource (CSS, images, fonts) and break the page when the
		// origin only speaks plain HTTP (e.g. dev on a non-localhost hostname).
		https := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
		csp := "default-src 'self'; img-src 'self' data:; style-src 'self'; " +
			"script-src 'self'; font-src 'self'; connect-src 'self'; " +
			"frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
		if https {
			csp += "; upgrade-insecure-requests"
		}
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy",
			"accelerometer=(), camera=(), geolocation=(), gyroscope=(), "+
				"microphone=(), payment=(), usb=()")
		h.Set("X-Frame-Options", "DENY")

		// COOP, CORP and HSTS are only meaningful on a "secure context" —
		// browsers ignore them on plain-HTTP origins and Chromium logs a
		// console warning ("the URL's origin was untrustworthy") that
		// Lighthouse counts as a Best-Practices failure. Gate them on the
		// edge being HTTPS so dev over HTTP is silent and production over
		// HTTPS still gets the protection.
		if https {
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		next.ServeHTTP(w, r)
	})
}
