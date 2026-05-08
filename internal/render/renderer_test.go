package render

import (
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/meta"
)

// TestRenderConcurrent exercises the render cache under concurrent access so
// `go test -race` can spot any unsynchronised writes.
func TestRenderConcurrent(t *testing.T) {
	t.Parallel()

	r, err := New("../../templates", "../../static")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	page := meta.Page{
		Title:       "",
		Description: "test",
		Path:        "/",
		Canonical:   "http://localhost/",
		OGType:      "website",
		Version:     "v0.0.0",
		CSSPath:     r.CSSPath(),
		BaseURL:     "http://localhost",
	}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, etag, err := r.Render("landing.html", Data{Meta: page})
			if err != nil {
				t.Errorf("Render: %v", err)
				return
			}
			if len(body) == 0 {
				t.Error("empty body")
			}
			if etag == "" {
				t.Error("empty etag")
			}
		}()
	}
	wg.Wait()
}

// TestETagRoundTrip verifies If-None-Match is honoured.
func TestETagRoundTrip(t *testing.T) {
	t.Parallel()
	body := []byte("hello")
	tag := ETag(body)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("If-None-Match", tag)
	if !MatchesIfNoneMatch(req, tag) {
		t.Fatal("expected match for exact ETag")
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("If-None-Match", `"different"`)
	if MatchesIfNoneMatch(req2, tag) {
		t.Fatal("unexpected match for different ETag")
	}

	req3 := httptest.NewRequest("GET", "/", nil)
	req3.Header.Set("If-None-Match", "*")
	if !MatchesIfNoneMatch(req3, tag) {
		t.Fatal("expected wildcard match")
	}
}
