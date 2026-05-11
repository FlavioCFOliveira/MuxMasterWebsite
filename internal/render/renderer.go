package render

import (
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

// Renderer parses the HTML templates once at startup and exposes the
// prerender map populated by Prerender. It is safe for concurrent use.
//
// The site is static-tending (specification/rendering-and-caching.md): every
// public route is materialised to bytes during startup, and per-request
// rendering does not exist. The Renderer therefore holds a single
// prerendered map; the lazy-cache machinery from earlier rounds has been
// removed.
type Renderer struct {
	tpl     *template.Template
	cssPath string

	mu          sync.RWMutex
	prerendered map[string]Prerendered
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

// Write performs a full ETag-aware write of an HTML page body. It honours
// If-None-Match (304) and sets Content-Type, ETag, and the supplied
// Cache-Control header. Used by handlers that compose responses outside the
// prerender map (for example operational fallbacks).
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
		"safeHTML": func(s string) template.HTML { return template.HTML(s) }, //nolint:gosec // input is fully trusted: rendered from /content/ at startup.
		"json":     func(s string) template.JS { return template.JS(s) },     //nolint:gosec // pre-encoded JSON, escape handled at construction time.
		// jsonldblock pre-renders a single JSON-LD <script> tag plus its
		// optional audit-trail HTML comment as one template.HTML chunk.
		// Why a dedicated func: html/template strips HTML comments by
		// default; emitting the comment through `{{if .Comment}}<!-- ... -->`
		// loses it before reaching the response. Concatenating the comment
		// and the script tag here, then returning them as template.HTML
		// (a trusted-string type), keeps the comment in the rendered output
		// — the audit trail mandated by spec/structured-data.md § Field
		// completeness must be visible to reviewers and validators.
		"jsonldblock": jsonldBlockHTML,
	}
}

// jsonldBlockHTML renders a meta.JSONLDBlock as the literal HTML chunk
// `<!-- <Comment> -->\n<script type="application/ld+json">{{JSON}}</script>`.
// When Comment is empty only the script tag is emitted. The JSON body is
// pre-encoded by the renderer (deterministic, marshalled from named
// structs at startup); both inputs are fully trusted.
func jsonldBlockHTML(b meta.JSONLDBlock) template.HTML {
	if b.Comment == "" {
		return template.HTML(`<script type="application/ld+json">` + b.JSON + `</script>`) //nolint:gosec
	}
	return template.HTML("<!-- " + b.Comment + " -->\n  " + //nolint:gosec
		`<script type="application/ld+json">` + b.JSON + `</script>`)
}
