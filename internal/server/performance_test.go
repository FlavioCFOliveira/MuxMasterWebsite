package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// get drives one request through the complete server handler.
func get(t *testing.T, h http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func gunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	return out
}

// TestPrerenderedGzipRepresentation checks the stored gzip representation
// end to end, through the Pre chain: it decompresses to the identity body,
// carries its own ETag, and revalidates with that ETag.
func TestPrerenderedGzipRepresentation(t *testing.T) {
	srv := newTestServer(t)
	h := srv.httpServer.Handler
	for _, path := range []string{"/", "/docs/routing", "/docs/routing.md", "/llms.txt", "/sitemap.xml"} {
		plain := get(t, h, path, nil)
		gz := get(t, h, path, map[string]string{"Accept-Encoding": "gzip"})
		if plain.Code != http.StatusOK || gz.Code != http.StatusOK {
			t.Fatalf("%s: status identity %d, gzip %d", path, plain.Code, gz.Code)
		}
		pre, _ := srv.renderer.Prerendered(path)
		if len(pre.GzipBody) >= len(pre.Body) {
			// Compression does not pay off (tiny fixture bodies): the
			// route is identity only, for every client.
			if ce := gz.Header().Get("Content-Encoding"); ce != "" {
				t.Errorf("%s: gzip is not smaller, yet Content-Encoding is %q", path, ce)
			}
			continue
		}
		if gz.Header().Get("Content-Encoding") != "gzip" {
			t.Errorf("%s: gzip-accepting request got Content-Encoding %q", path, gz.Header().Get("Content-Encoding"))
			continue
		}
		if !bytes.Equal(gunzip(t, gz.Body.Bytes()), plain.Body.Bytes()) {
			t.Errorf("%s: gzip body does not decompress to the identity body", path)
		}
		if plain.Header().Get("ETag") == gz.Header().Get("ETag") {
			t.Errorf("%s: identity and gzip share ETag %s", path, gz.Header().Get("ETag"))
		}
		nm := get(t, h, path, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": gz.Header().Get("ETag")})
		if nm.Code != http.StatusNotModified {
			t.Errorf("%s: revalidation with the gzip ETag returned %d, want 304", path, nm.Code)
		}
	}
}

// TestStaticAssetsFromMemory covers the in-memory /static/*filepath route:
// headers per asset type, the gzip representation of the CSS bundle, 304
// revalidation, HEAD, and the branded 404 for anything that is not a loaded
// regular file.
func TestStaticAssetsFromMemory(t *testing.T) {
	srv := newTestServer(t)
	h := srv.httpServer.Handler
	css := srv.renderer.CSSPath()

	plain := get(t, h, css, nil)
	if plain.Code != http.StatusOK {
		t.Fatalf("%s: status %d", css, plain.Code)
	}
	if got := plain.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Errorf("CSS Content-Type = %q", got)
	}
	if got := plain.Header().Get("Cache-Control"); got != cacheControlHashedAsset {
		t.Errorf("CSS Cache-Control = %q, want %q", got, cacheControlHashedAsset)
	}
	gz := get(t, h, css, map[string]string{"Accept-Encoding": "gzip"})
	if gz.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("CSS with Accept-Encoding: gzip got Content-Encoding %q", gz.Header().Get("Content-Encoding"))
	}
	if !bytes.Equal(gunzip(t, gz.Body.Bytes()), plain.Body.Bytes()) {
		t.Error("CSS gzip body does not decompress to the identity body")
	}
	if v := gz.Header().Values("Vary"); len(v) != 1 || v[0] != "Accept-Encoding" {
		t.Errorf("CSS Vary = %q, want exactly [Accept-Encoding]", v)
	}
	nm := get(t, h, css, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": gz.Header().Get("ETag")})
	if nm.Code != http.StatusNotModified {
		t.Errorf("CSS revalidation: status %d, want 304", nm.Code)
	}

	head := httptest.NewRequest(http.MethodHead, css, nil)
	headRec := httptest.NewRecorder()
	h.ServeHTTP(headRec, head)
	if headRec.Code != http.StatusOK || headRec.Header().Get("Content-Length") != plain.Header().Get("Content-Length") {
		t.Errorf("HEAD %s: status %d, Content-Length %q", css, headRec.Code, headRec.Header().Get("Content-Length"))
	}

	for _, p := range []string{"/static/", "/static/css/", "/static/img", "/static/../go.mod", "/static/css/missing.css"} {
		if rec := get(t, h, p, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", p, rec.Code)
		}
	}
}

// TestStaticHandlerKeepsRequestPath guards against the old http.FileServer
// wrapper, which rewrote r.URL.Path and made the access log record the path
// without its /static prefix.
func TestStaticHandlerKeepsRequestPath(t *testing.T) {
	srv := newTestServer(t)
	css := srv.renderer.CSSPath()
	req := httptest.NewRequest(http.MethodGet, css, nil)
	srv.httpServer.Handler.ServeHTTP(httptest.NewRecorder(), req)
	if req.URL.Path != css {
		t.Errorf("request path rewritten to %q, want %q", req.URL.Path, css)
	}
}

// TestAccessLogRouteID checks the route_id field of the access log: the
// registered pattern for a matched route, including the FastHandler static
// route, and the empty string when no route matched.
func TestAccessLogRouteID(t *testing.T) {
	base := newTestServer(t)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	srv, err := New(base.cfg, logger, base.loader, base.version, "../../templates", "../../static")
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	if err := srv.Prerender(); err != nil {
		t.Fatalf("Prerender: %v", err)
	}
	css := srv.renderer.CSSPath()
	cases := []struct{ path, routeID string }{
		{"/docs/routing", "/docs/routing"},
		{"/docs/routing.md", "/docs/routing.md"},
		{"/healthz", "/healthz"},
		{css, "/static/*filepath"},
		{"/no-such-page", ""},
	}
	for _, c := range cases {
		buf.Reset()
		get(t, srv.httpServer.Handler, c.path, nil)
		var line struct {
			Msg     string  `json:"msg"`
			Path    string  `json:"path"`
			RouteID *string `json:"route_id"`
		}
		for _, l := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
			if err := json.Unmarshal(l, &line); err == nil && line.Msg == "request" {
				break
			}
		}
		if line.RouteID == nil {
			t.Errorf("%s: no access-log line with a route_id field in %q", c.path, buf.String())
			continue
		}
		if *line.RouteID != c.routeID || line.Path != c.path {
			t.Errorf("%s: logged path %q route_id %q, want path %q route_id %q", c.path, line.Path, *line.RouteID, c.path, c.routeID)
		}
	}
}

// TestLoadStaticAssetsByType checks the per-type decisions of
// loadStaticAssets on a temporary directory, so it does not depend on the
// generated images of `make assets` (CI runs only `make css`): a PNG gets no
// gzip body and no Vary, a compressible file gets both, and only the hashed
// CSS bundle is cached as immutable.
func TestLoadStaticAssetsByType(t *testing.T) {
	dir := t.TempDir()
	css := bytes.Repeat([]byte("body{margin:0}\n"), 200)
	png := []byte("\x89PNG\r\n\x1a\n not really a PNG, but served as image/png")
	for name, body := range map[string][]byte{
		"css/app.0123456789ab.css": css,
		"img/logo.png":             png,
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	assets, err := loadStaticAssets(dir, "/static/css/app.0123456789ab.css")
	if err != nil {
		t.Fatalf("loadStaticAssets: %v", err)
	}
	cases := []struct {
		key, contentType, cacheControl string
		gzip                           bool
	}{
		{"/css/app.0123456789ab.css", "text/css; charset=utf-8", cacheControlHashedAsset, true},
		{"/img/logo.png", "image/png", cacheControlStatic, false},
	}
	for _, c := range cases {
		resp, ok := assets.files[c.key]
		if !ok {
			t.Fatalf("%s not loaded", c.key)
		}
		req := httptest.NewRequest(http.MethodGet, "/static"+c.key, nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		resp.Serve(rec, req)
		if got := rec.Header().Get("Content-Type"); got != c.contentType {
			t.Errorf("%s: Content-Type %q, want %q", c.key, got, c.contentType)
		}
		if got := rec.Header().Get("Cache-Control"); got != c.cacheControl {
			t.Errorf("%s: Cache-Control %q, want %q", c.key, got, c.cacheControl)
		}
		gz := rec.Header().Get("Content-Encoding") == "gzip"
		vary := rec.Header().Get("Vary") == "Accept-Encoding"
		if gz != c.gzip || vary != c.gzip {
			t.Errorf("%s: gzip %v, Vary %v; want both %v", c.key, gz, vary, c.gzip)
		}
	}
	if _, ok := assets.files["/img"]; ok {
		t.Error("a directory was loaded as a file")
	}
}
