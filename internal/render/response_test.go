package render

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAcceptsGzip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		header string
		want   bool
	}{
		{"", false},
		{"gzip", true},
		{"GZIP", true},
		{"x-gzip", true},
		{"gzip, deflate, br", true},
		{"br;q=1.0, gzip;q=0.8", true},
		{"gzip;q=0", false},
		{"gzip; q=0.000", false},
		{"gzip;q=0.001", true},
		{"deflate, br", false},
		{"identity", false},
		{"*", true},
		{"*;q=0", false},
		{"*, gzip;q=0", false},
		{"gzip;q=0, *", false},
		{"br, *;q=0.5", true},
	}
	for _, c := range cases {
		var values []string
		if c.header != "" {
			values = []string{c.header}
		}
		if got := AcceptsGzip(values); got != c.want {
			t.Errorf("AcceptsGzip(%q) = %v, want %v", c.header, got, c.want)
		}
	}
	// Multiple header lines are one list (RFC 9110 § 5.3).
	if !AcceptsGzip([]string{"br", "gzip"}) {
		t.Error("gzip on a second Accept-Encoding line was not honoured")
	}
}

func TestMatchesIfNoneMatchWeakComparison(t *testing.T) {
	t.Parallel()
	const tag = `"abc"`
	cases := []struct {
		header string
		want   bool
	}{
		{`"abc"`, true},
		{`W/"abc"`, true},
		{`"x", "abc"`, true},
		{` "x" ,W/"abc" `, true},
		{`"x", "y"`, false},
		{`*`, true},
		{`abc`, false},
	}
	for _, c := range cases {
		if got := matchesIfNoneMatch([]string{c.header}, tag); got != c.want {
			t.Errorf("matchesIfNoneMatch(%q) = %v, want %v", c.header, got, c.want)
		}
	}
}

func testResponse(t *testing.T) (*Response, []byte) {
	t.Helper()
	body := []byte(strings.Repeat("<p>MuxMaster pre-computed response.</p>\n", 200))
	resp, err := NewResponse(ResponseOptions{
		Body:         body,
		ContentType:  "text/html; charset=utf-8",
		CacheControl: "public, max-age=600",
		LastModified: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC),
		Gzip:         true,
		Vary:         true,
	})
	if err != nil {
		t.Fatalf("NewResponse: %v", err)
	}
	return resp, body
}

func serve(resp *Response, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	resp.Serve(rec, req)
	return rec
}

func TestResponseRepresentations(t *testing.T) {
	t.Parallel()
	resp, body := testResponse(t)
	idTag, gzTag := resp.ETags()
	if idTag == "" || gzTag == "" || idTag == gzTag {
		t.Fatalf("ETags = %q, %q; want two distinct non-empty validators", idTag, gzTag)
	}

	plain := serve(resp, nil)
	if plain.Code != http.StatusOK || !bytes.Equal(plain.Body.Bytes(), body) {
		t.Fatalf("identity: status %d, body equal %v", plain.Code, bytes.Equal(plain.Body.Bytes(), body))
	}
	if ce := plain.Header().Get("Content-Encoding"); ce != "" {
		t.Errorf("identity carries Content-Encoding %q", ce)
	}
	if got := plain.Header().Get("ETag"); got != idTag {
		t.Errorf("identity ETag = %q, want %q", got, idTag)
	}

	gz := serve(resp, map[string]string{"Accept-Encoding": "gzip"})
	if gz.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("gzip-accepting request did not receive Content-Encoding: gzip")
	}
	if got := gz.Header().Get("ETag"); got != gzTag {
		t.Errorf("gzip ETag = %q, want %q", got, gzTag)
	}
	if got, want := gz.Header().Get("Content-Length"), strconv.Itoa(gz.Body.Len()); got != want {
		t.Errorf("gzip Content-Length = %s, want %s", got, want)
	}
	zr, err := gzip.NewReader(gz.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	decoded, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if !bytes.Equal(decoded, body) {
		t.Fatal("gzip body does not decompress to the identity body")
	}
	for _, rec := range []*httptest.ResponseRecorder{plain, gz} {
		if v := rec.Header().Values("Vary"); len(v) != 1 || v[0] != "Accept-Encoding" {
			t.Errorf("Vary = %q, want exactly [Accept-Encoding]", v)
		}
	}
}

func TestResponseConditional(t *testing.T) {
	t.Parallel()
	resp, _ := testResponse(t)
	idTag, gzTag := resp.ETags()

	nm := serve(resp, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": gzTag})
	if nm.Code != http.StatusNotModified || nm.Body.Len() != 0 {
		t.Fatalf("gzip revalidation: status %d, body %d bytes", nm.Code, nm.Body.Len())
	}
	if nm.Header().Get("Vary") != "Accept-Encoding" {
		t.Error("304 is missing Vary: Accept-Encoding")
	}
	// The identity validator does not validate the gzip representation.
	if rec := serve(resp, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": idTag}); rec.Code != http.StatusOK {
		t.Errorf("identity ETag revalidated the gzip representation: status %d", rec.Code)
	}

	lm := resp.lastModified[0]
	if rec := serve(resp, map[string]string{"If-Modified-Since": lm}); rec.Code != http.StatusNotModified {
		t.Errorf("If-Modified-Since equal to Last-Modified: status %d, want 304", rec.Code)
	}
	later := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	if rec := serve(resp, map[string]string{"If-Modified-Since": later}); rec.Code != http.StatusNotModified {
		t.Errorf("If-Modified-Since after Last-Modified: status %d, want 304", rec.Code)
	}
	earlier := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	if rec := serve(resp, map[string]string{"If-Modified-Since": earlier}); rec.Code != http.StatusOK {
		t.Errorf("If-Modified-Since before Last-Modified: status %d, want 200", rec.Code)
	}
	// If-None-Match takes precedence over If-Modified-Since (RFC 9110 § 13.2.2).
	if rec := serve(resp, map[string]string{"If-None-Match": `"other"`, "If-Modified-Since": lm}); rec.Code != http.StatusOK {
		t.Errorf("a failing If-None-Match was overridden by If-Modified-Since: status %d", rec.Code)
	}
}

func TestResponseSkipsGzipWhenNotSmaller(t *testing.T) {
	t.Parallel()
	resp, err := NewResponse(ResponseOptions{Body: []byte("ok"), ContentType: "text/plain", CacheControl: "no-store", Gzip: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GzipBody() != nil {
		t.Fatal("stored a gzip body larger than the identity body")
	}
	rec := serve(resp, map[string]string{"Accept-Encoding": "gzip"})
	if rec.Header().Get("Content-Encoding") != "" || rec.Header().Get("Vary") != "" {
		t.Errorf("identity-only response sent Content-Encoding %q / Vary %q",
			rec.Header().Get("Content-Encoding"), rec.Header().Get("Vary"))
	}
}

type nopWriter struct{ h http.Header }

func (n *nopWriter) Header() http.Header         { return n.h }
func (n *nopWriter) Write(b []byte) (int, error) { return len(b), nil }
func (n *nopWriter) WriteHeader(int)             {}

func TestResponseServeAllocatesNothing(t *testing.T) {
	resp, _ := testResponse(t)
	_, gzTag := resp.ETags()
	for name, hdr := range map[string]string{
		"gzip": "gzip, deflate, br",
		"304":  "",
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		if hdr == "" {
			req.Header.Set("If-None-Match", gzTag)
		}
		w := &nopWriter{h: make(http.Header, 8)}
		allocs := testing.AllocsPerRun(1000, func() {
			clear(w.h)
			resp.Serve(w, req)
		})
		if allocs != 0 {
			t.Errorf("%s: Serve allocated %.0f times per request, want 0", name, allocs)
		}
	}
}

func TestPrerenderRejectsUnboundPath(t *testing.T) {
	t.Parallel()
	deps := fixtureDeps(t)
	_ = deps.Renderer.ServePrerendered("/no-recipe", "no-store", http.StatusOK)
	if err := deps.Renderer.Prerender([]Recipe{RobotsRecipe()}, deps); err == nil {
		t.Fatal("Prerender accepted a served path that has no recipe")
	}
}
