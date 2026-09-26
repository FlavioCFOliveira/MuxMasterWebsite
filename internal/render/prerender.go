package render

import (
	"bytes"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Prerendered is the materialised output of a recipe. It is written once at
// startup and read-only thereafter.
//
// Body is the identity representation; GzipBody is the same bytes compressed
// once at materialisation time (specification/rendering-and-caching.md
// "Materialisation rule"). Each representation has its own strong ETag.
type Prerendered struct {
	Body         []byte
	ContentType  string
	ETag         string
	GzipBody     []byte
	GzipETag     string
	LastModified time.Time
}

// Recipe is a pure function from build-time inputs to a single response body.
// Recipes MUST NOT read from the upstream tree, perform I/O, or capture mutable
// state. Their inputs are limited to the supplied Deps.
type Recipe struct {
	Path        string
	ContentType string
	Build       func(deps Deps) ([]byte, error)
}

// RouteInfo describes one HTML route on the site for the purposes of recipe
// inputs (sitemap, llms.txt, llms-full.txt, docs and examples indexes).
// Operational endpoints, text artefacts (/llms.txt, /sitemap.xml, /robots.txt),
// and error templates MUST NOT appear in this slice.
//
// Order is the curated index-page ordering rank within Section: lower comes
// first. It exists because path-lex order is wrong for /examples/ (alphabet
// vs. learning sequence). When Order is zero (the default) recipes fall
// back to path-lex order, which is the right behaviour for the docs index
// where the docsSidebar handles ordering elsewhere.
type RouteInfo struct {
	Path        string
	Title       string
	Description string
	Section     string // "landing", "docs", "api", "examples", "benchmarks", "changelog", "releases", "security", "compatibility", "contributing"
	HasMarkdown bool
	Order       int
}

// Deps bundles the build-time inputs every recipe may consume. Keeping the
// surface narrow makes it trivial to assert that recipes are pure.
type Deps struct {
	Routes    []RouteInfo
	Version   string
	GoVersion string // Minimum supported Go version, mirrored from ../MuxMaster/go.mod (e.g. "1.26"). Sourced at build time and surfaced in JSON-LD SoftwareSourceCode.runtimePlatform.
	BaseURL   string
	BuildTime time.Time
	// Renderer is exposed so HTML recipes can execute the parsed templates
	// without re-parsing them. It MUST NOT be used to read the lazy cache.
	Renderer *Renderer
}

// Prerender executes every recipe and stores the result on the Renderer.
// It MUST be called exactly once, at startup, after route registration and
// after upstream metadata (version label, CSS hash) is resolved.
//
// Returns an error from the first recipe that fails so the binary can exit
// non-zero — broken Category A is a startup error, not a runtime degradation.
func (r *Renderer) Prerender(recipes []Recipe, deps Deps) error {
	out := make(map[string]Prerendered, len(recipes))
	for _, rec := range recipes {
		if rec.Path == "" || rec.Build == nil {
			return fmt.Errorf("render: invalid recipe (empty path or nil Build)")
		}
		body, err := rec.Build(deps)
		if err != nil {
			return fmt.Errorf("render: prerender %s: %w", rec.Path, err)
		}
		if len(body) == 0 {
			return fmt.Errorf("render: prerender %s produced empty body", rec.Path)
		}
		gz, err := gzipBytes(body)
		if err != nil {
			return fmt.Errorf("render: prerender %s: %w", rec.Path, err)
		}
		out[rec.Path] = Prerendered{
			Body:         body,
			ContentType:  rec.ContentType,
			ETag:         ETag(body),
			GzipBody:     gz,
			GzipETag:     ETag(gz),
			LastModified: deps.BuildTime,
		}
	}

	// Bind every handler returned by ServePrerendered to its pre-computed
	// response. A handler whose path has no recipe is a wiring error and
	// fails startup instead of answering 500 at request time.
	r.mu.Lock()
	bindings := r.bindings
	r.mu.Unlock()
	for _, b := range bindings {
		pre, ok := out[b.path]
		if !ok {
			return fmt.Errorf("render: no recipe for served path %s", b.path)
		}
		resp, err := NewResponse(ResponseOptions{
			Body:         pre.Body,
			GzipBody:     pre.GzipBody,
			ContentType:  pre.ContentType,
			CacheControl: b.cacheControl,
			Status:       b.status,
			LastModified: pre.LastModified,
			Gzip:         true,
			Vary:         true,
		})
		if err != nil {
			return fmt.Errorf("render: prerender %s: %w", b.path, err)
		}
		b.resp.Store(resp)
	}

	// Single assignment after the full pass succeeds. After this point, the
	// map is read-only and no synchronisation is required for reads. This
	// matches the static-tending invariant: Category A bytes are fixed for
	// the process lifetime.
	r.prerendered.Store(&out)
	return nil
}

// Prerendered looks up a prerendered entry. Returns false if no entry exists.
// Callers MUST treat the returned value as immutable.
//
// The map is published once, through an atomic pointer, after the whole
// startup pass succeeds; readers take no lock.
func (r *Renderer) Prerendered(path string) (Prerendered, bool) {
	m := r.prerendered.Load()
	if m == nil {
		return Prerendered{}, false
	}
	p, ok := (*m)[path]
	return p, ok
}

// PrerenderedPaths returns the paths currently in the prerender map, in no
// particular order. Useful for startup logging and tests.
func (r *Renderer) PrerenderedPaths() []string {
	m := r.prerendered.Load()
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(*m))
	for k := range *m {
		out = append(out, k)
	}
	return out
}

// binding ties one ServePrerendered handler to the response Prerender builds
// for it.
type binding struct {
	path         string
	cacheControl string
	status       int
	resp         atomic.Pointer[Response]
}

// ServePrerendered returns a handler that serves the pre-computed response
// for path with the given status and Cache-Control value. The handler may be
// registered before Prerender runs (route registration comes first, per
// the materialisation rule); Prerender then builds its response, with both
// representations and every header value, and fails startup if path has no
// recipe. A request that arrives before Prerender has run answers 500.
//
// Per request the handler loads one atomic pointer and calls
// Response.Serve: no map lookup, no lock, no allocation.
func (r *Renderer) ServePrerendered(path string, cacheControl string, status int) http.HandlerFunc {
	b := &binding{path: path, cacheControl: cacheControl, status: status}
	r.mu.Lock()
	r.bindings = append(r.bindings, b)
	r.mu.Unlock()
	return func(w http.ResponseWriter, req *http.Request) {
		resp := b.resp.Load()
		if resp == nil {
			http.Error(w, "500 Internal Server Error\n", http.StatusInternalServerError)
			return
		}
		resp.Serve(w, req)
	}
}

// ExecuteTemplate is a thin helper recipes can use to run a parsed template
// against a Data value, returning the rendered bytes. It bypasses the lazy
// render cache entirely — Category A bytes are stored only in the prerender
// map (see the invariant on Prerendered above).
func (r *Renderer) ExecuteTemplate(name string, data Data) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.tpl.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("render: execute %s: %w", name, err)
	}
	return buf.Bytes(), nil
}
