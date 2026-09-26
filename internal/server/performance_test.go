package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
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
