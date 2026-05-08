package render

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/meta"
)

// Renderer parses the HTML templates once at startup and renders pages on
// demand. It is safe for concurrent use.
//
// The Renderer holds two distinct caches:
//
//   - cache (lazy): Category B routes whose output depends on upstream files.
//     Populated on first request, invalidated by upstream mtime changes.
//   - prerendered (eager): Category A routes whose output is fixed for the
//     process lifetime. Populated once during Prerender at startup; read-only
//     thereafter.
//
// Invariant: a path that exists in prerendered MUST NOT also be served via
// the lazy cache. Wire-up code is responsible for routing each path to
// exactly one cache.
type Renderer struct {
	tpl     *template.Template
	cssPath string

	mu          sync.RWMutex
	cache       map[string]cachedPage
	prerendered map[string]Prerendered
}

type cachedPage struct {
	body []byte
	etag string
}

// New constructs a Renderer by parsing every .html file under templatesDir
// and discovering the hashed CSS bundle under staticDir.
func New(templatesDir, staticDir string) (*Renderer, error) {
	tpl := template.New("").Funcs(funcMap())
	patterns := []string{
		filepath.Join(templatesDir, "*.html"),
		filepath.Join(templatesDir, "partials", "*.html"),
		filepath.Join(templatesDir, "pages", "*.html"),
	}
	for _, p := range patterns {
		matches, err := filepath.Glob(p)
		if err != nil {
			return nil, fmt.Errorf("render: glob %s: %w", p, err)
		}
		if len(matches) == 0 {
			continue
		}
		tpl, err = tpl.ParseFiles(matches...)
		if err != nil {
			return nil, fmt.Errorf("render: parse %s: %w", p, err)
		}
	}

	cssPath, err := discoverHashedCSS(staticDir)
	if err != nil {
		return nil, err
	}

	return &Renderer{
		tpl:         tpl,
		cssPath:     cssPath,
		cache:       make(map[string]cachedPage),
		prerendered: make(map[string]Prerendered),
	}, nil
}

// CSSPath returns the URL path of the hashed CSS bundle.
func (r *Renderer) CSSPath() string { return r.cssPath }

// Data is the value passed to every page template.
type Data struct {
	Meta meta.Page
	Body any
}

// Render executes the named page template into a buffer and returns the bytes
// together with their strong ETag. The render result is cached in-process keyed
// by (templateName, page.Path); the cache is invalidated only on process
// restart, which matches the spec's "no upstream filesystem watch" rule.
func (r *Renderer) Render(name string, data Data) ([]byte, string, error) {
	key := name + "\x00" + data.Meta.Path
	r.mu.RLock()
	if c, ok := r.cache[key]; ok {
		r.mu.RUnlock()
		return c.body, c.etag, nil
	}
	r.mu.RUnlock()

	var buf bytes.Buffer
	if err := r.tpl.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, "", fmt.Errorf("render: execute %s: %w", name, err)
	}
	body := buf.Bytes()
	etag := ETag(body)

	r.mu.Lock()
	r.cache[key] = cachedPage{body: body, etag: etag}
	r.mu.Unlock()

	return body, etag, nil
}

// Write performs a full ETag-aware write of a rendered page.
// It honours If-None-Match (304) and sets Content-Type, ETag, and the supplied
// Cache-Control header.
func (r *Renderer) Write(w http.ResponseWriter, req *http.Request, status int, body []byte, etag, cacheControl string) {
	if MatchesIfNoneMatch(req, etag) {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", cacheControl)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("Vary", "Accept-Encoding")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func discoverHashedCSS(staticDir string) (string, error) {
	cssDir := filepath.Join(staticDir, "css")
	entries, err := os.ReadDir(cssDir)
	if err != nil {
		return "", fmt.Errorf("render: read %s: %w", cssDir, err)
	}
	var matches []string
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, "app.") || !strings.HasSuffix(n, ".css") {
			continue
		}
		// Skip the unhashed sources used by `make css-watch`.
		if n == "app.css" || n == "app.dev.css" {
			continue
		}
		matches = append(matches, n)
	}
	if len(matches) == 0 {
		return "", errors.New("render: no hashed CSS bundle found under static/css/ (run `make css`)")
	}
	sort.Strings(matches)
	return "/static/css/" + matches[len(matches)-1], nil
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) }, //nolint:gosec // input is JSON-LD/template-controlled.
		"json":     func(s string) template.JS { return template.JS(s) },     //nolint:gosec // pre-encoded JSON, escape handled at construction time.
	}
}
