package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// discardWriter is a minimal http.ResponseWriter that drops the body. The
// benchmarks measure the site's full handler chain (Pre middleware, router,
// handler), so the writer must cost as little as possible and must not
// buffer the body the way httptest.ResponseRecorder does.
type discardWriter struct {
	h http.Header
}

func (d *discardWriter) Header() http.Header         { return d.h }
func (d *discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d *discardWriter) WriteHeader(int)             {}

// reset clears the header map in place so every iteration starts from an
// empty response without allocating a new map.
func (d *discardWriter) reset() { clear(d.h) }

// newBenchServer builds the test server with the production logger
// configuration of cmd/muxmaster-website: JSON at level Info. The access log
// line is therefore encoded on every iteration, exactly as in production;
// only its destination differs (io.Discard instead of stdout).
func newBenchServer(b *testing.B) *Server {
	b.Helper()
	srv := newTestServer(b)
	logger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	prod, err := New(srv.cfg, logger, srv.loader, srv.version, "../../templates", "../../static")
	if err != nil {
		b.Fatalf("server.New: %v", err)
	}
	if err := prod.Prerender(); err != nil {
		b.Fatalf("Prerender: %v", err)
	}
	return prod
}

// benchRequest drives one pre-built request through the complete server
// handler b.N times. Run with `make bench`.
func benchRequest(b *testing.B, path string, headers map[string]string) {
	b.Helper()
	srv := newBenchServer(b)
	h := srv.httpServer.Handler
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := &discardWriter{h: make(http.Header, 16)}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		w.reset()
		// Restore the path on every iteration: a handler that rewrites
		// r.URL.Path would otherwise turn every later iteration into a
		// different request.
		req.URL.Path = path
		h.ServeHTTP(w, req)
	}
}

var acceptGzip = map[string]string{"Accept-Encoding": "gzip, deflate, br"}

func BenchmarkHealthz(b *testing.B) { benchRequest(b, "/healthz", nil) }

func BenchmarkDocPageIdentity(b *testing.B) { benchRequest(b, "/docs/routing", nil) }

func BenchmarkDocPageGzip(b *testing.B) { benchRequest(b, "/docs/routing", acceptGzip) }

// BenchmarkDocPageNotModified revalidates with the ETag the server itself
// returned for the same Accept-Encoding, so it measures the 304 path of
// whichever representation a gzip-capable client holds.
func BenchmarkDocPageNotModified(b *testing.B) {
	srv := newTestServer(b)
	probe := httptest.NewRequest(http.MethodGet, "/docs/routing", nil)
	probe.Header.Set("Accept-Encoding", acceptGzip["Accept-Encoding"])
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, probe)
	etag := rec.Header().Get("ETag")
	if etag == "" {
		b.Fatal("/docs/routing returned no ETag")
	}
	benchRequest(b, "/docs/routing", map[string]string{
		"Accept-Encoding": acceptGzip["Accept-Encoding"],
		"If-None-Match":   etag,
	})
}

func BenchmarkLandingGzip(b *testing.B) { benchRequest(b, "/", acceptGzip) }

func BenchmarkMarkdownCompanionGzip(b *testing.B) {
	benchRequest(b, "/docs/routing.md", acceptGzip)
}

func BenchmarkStaticCSSGzip(b *testing.B) {
	srv := newTestServer(b)
	benchRequest(b, srv.renderer.CSSPath(), acceptGzip)
}

func BenchmarkNotFoundGzip(b *testing.B) { benchRequest(b, "/no-such-page", acceptGzip) }
