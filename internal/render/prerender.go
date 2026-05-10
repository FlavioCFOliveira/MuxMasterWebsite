package render

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

// Prerendered is the materialised output of a Category A recipe. It is written
// once at startup and read-only thereafter.
//
// Invariant: a path that exists in the prerender map MUST NOT be served by the
// lazy render cache. The two caches partition the route set and never overlap.
// Category A recipes execute html/template at startup only; Category B routes
// execute it on cache miss. This is the static-tending principle from
// specification/overview.md and rendering-and-caching.md.
type Prerendered struct {
	Body         []byte
	ContentType  string
	ETag         string
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
		out[rec.Path] = Prerendered{
			Body:         body,
			ContentType:  rec.ContentType,
			ETag:         ETag(body),
			LastModified: deps.BuildTime,
		}
	}

	// Single assignment after the full pass succeeds. After this point, the
	// map is read-only and no synchronisation is required for reads. This
	// matches the static-tending invariant: Category A bytes are fixed for
	// the process lifetime.
	r.mu.Lock()
	r.prerendered = out
	r.mu.Unlock()
	return nil
}

// Prerendered looks up a prerendered entry. Returns false if no entry exists.
// Callers MUST treat the returned value as immutable.
//
// Reads take the renderer's RLock to keep the race detector quiet across the
// startup-time Prerender write. After the single startup write, the map is
// never mutated, so contention is bounded to the read path only.
func (r *Renderer) Prerendered(path string) (Prerendered, bool) {
	r.mu.RLock()
	p, ok := r.prerendered[path]
	r.mu.RUnlock()
	return p, ok
}

// PrerenderedPaths returns the sorted list of paths currently in the prerender
// cache. Useful for startup logging and tests.
func (r *Renderer) PrerenderedPaths() []string {
	r.mu.RLock()
	out := make([]string, 0, len(r.prerendered))
	for k := range r.prerendered {
		out = append(out, k)
	}
	r.mu.RUnlock()
	return out
}

// ServePrerendered returns an http.HandlerFunc that writes the recorded body
// for path with full ETag / Last-Modified / 304 negotiation. The response
// status code is the supplied status; cacheControl is the Cache-Control header
// value. The path lookup happens on each request, so the handler may be
// constructed before Prerender runs (registration order). If the path is
// missing from the prerender cache at request time, the handler answers 500.
func (r *Renderer) ServePrerendered(path string, cacheControl string, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		pre, ok := r.Prerendered(path)
		if !ok {
			http.Error(w, "500 Internal Server Error\n", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("ETag", pre.ETag)
		h.Set("Last-Modified", pre.LastModified.UTC().Format(http.TimeFormat))
		h.Set("Cache-Control", cacheControl)
		// Vary: Accept-Encoding MUST be set on the conditional 304 path
		// too, so any intermediate cache keys the empty-body response by
		// the same dimension as the corresponding 200. Otherwise a cache
		// could serve a 304 against a request with a different
		// Accept-Encoding (spec/rendering-and-caching.md § Cache headers).
		h.Set("Vary", "Accept-Encoding")

		if MatchesIfNoneMatch(req, pre.ETag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if ifModSince := req.Header.Get("If-Modified-Since"); ifModSince != "" {
			if t, err := http.ParseTime(ifModSince); err == nil && !pre.LastModified.Truncate(time.Second).After(t) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		h.Set("Content-Type", pre.ContentType)
		w.WriteHeader(status)
		// HEAD: net/http drops the body automatically.
		_, _ = w.Write(pre.Body)
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
