package server

import (
	"net/http"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/meta"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/render"
)

const (
	cacheControlLanding   = "public, max-age=300, stale-while-revalidate=60"
	cacheControlDocs      = "public, max-age=600, stale-while-revalidate=120"
	cacheControlChangelog = "public, max-age=300, stale-while-revalidate=60"
	cacheControlRelease   = "public, max-age=86400, immutable"
	cacheControlText      = "public, max-age=300"
	cacheControlSitemap   = "public, max-age=300"
	cacheControlNoStore   = "no-store"
)

// comingSoonHandler renders the placeholder template for any route the
// content rounds have not filled in yet.
func (s *Server) comingSoonHandler(title, description, upstreamURL, cacheControl string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := s.pageMeta(r.URL.Path, title, description, "article")
		page.UpstreamURL = upstreamURL
		page.Breadcrumbs = breadcrumbsFor(page)

		body, etag, err := s.renderer.Render("coming-soon.html", render.Data{Meta: page})
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		s.renderer.Write(w, r, http.StatusOK, body, etag, cacheControl)
	}
}

// markdownStubHandler serves the .md companion for routes the content rounds
// have not filled in yet. A short placeholder Markdown body is enough to
// satisfy the Content-Type and route-shape checks.
func (s *Server) markdownStubHandler(title, upstreamURL string) http.HandlerFunc {
	body := []byte("# " + title + "\n\nThis page is being prepared.\n\n" +
		"Read the canonical upstream document: " + upstreamURL + "\n")
	etag := render.ETag(body)
	return func(w http.ResponseWriter, r *http.Request) {
		if render.MatchesIfNoneMatch(r, etag) {
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", cacheControlDocs)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", cacheControlDocs)
		w.Header().Set("Vary", "Accept-Encoding")
		_, _ = w.Write(body)
	}
}

// healthzHandler is the operational endpoint. Plain text, never cached.
func (s *Server) healthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", cacheControlNoStore)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}
}

// notFoundFromPrerender returns the branded 404 handler. The body is read
// from the prerender cache at request time (registration order is route
// registration first, prerender second); the handler overrides the status
// code to 404 and Cache-Control to no-store per spec.
func (s *Server) notFoundFromPrerender() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pre, ok := s.renderer.Prerendered("/404")
		if !ok {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", cacheControlNoStore)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("404 Not Found\n"))
			return
		}
		w.Header().Set("ETag", pre.ETag)
		w.Header().Set("Cache-Control", cacheControlNoStore)
		if render.MatchesIfNoneMatch(r, pre.ETag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", pre.ContentType)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(pre.Body)
	}
}

// serverError emits the prerendered /500 page. Status 500, no-store.
func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("render error",
		"path", r.URL.Path,
		"err", err.Error(),
	)
	pre, ok := s.renderer.Prerendered("/500")
	if !ok {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", cacheControlNoStore)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("500 Internal Server Error\n"))
		return
	}
	w.Header().Set("Content-Type", pre.ContentType)
	w.Header().Set("Cache-Control", cacheControlNoStore)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(pre.Body)
}

func (s *Server) pageMeta(path, title, description, ogType string) meta.Page {
	page := meta.Page{
		Title:       title,
		Description: description,
		Path:        path,
		Canonical:   s.cfg.SiteBaseURL + path,
		OGType:      ogType,
		OGImage:     s.cfg.SiteBaseURL + "/static/img/og-image.png",
		Version:     s.version,
		CSSPath:     s.renderer.CSSPath(),
		BaseURL:     s.cfg.SiteBaseURL,
	}
	if !s.isProduction() {
		// Until the canonical domain is decided, every page must be noindex
		// (specification/deployment.md "Production launch gate").
		page.Robots = "noindex,nofollow"
	}
	return page
}

// LandingDescription is the canonical landing-page description used by the
// recipes and any handler that needs it.
const LandingDescription = "MuxMaster is a high-performance, zero-dependency HTTP router for Go. Radix-tree O(k) lookups, zero allocations on static routes, and 100% net/http compatibility."

func breadcrumbsFor(p meta.Page) []meta.Breadcrumb {
	if p.IsHome() {
		return nil
	}
	crumbs := []meta.Breadcrumb{{Label: "Home", Href: "/"}}
	if section := p.SectionLabel(); section != "" {
		// Index URLs (e.g. /docs/) show only Home / Section.
		// Leaf URLs add a third crumb with the page title.
		segments := splitPath(p.Path)
		if len(segments) == 1 {
			crumbs = append(crumbs, meta.Breadcrumb{Label: section})
			return crumbs
		}
		crumbs = append(crumbs, meta.Breadcrumb{Label: section, Href: "/" + segments[0] + indexSlash(p.Path, segments)})
		crumbs = append(crumbs, meta.Breadcrumb{Label: p.Title})
	}
	return crumbs
}

func splitPath(p string) []string {
	out := make([]string, 0, 4)
	start := -1
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			if start >= 0 {
				out = append(out, p[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, p[start:])
	}
	return out
}

// indexSlash returns "/" if the section parent is an index URL (/docs/, /examples/),
// "" otherwise (/api, /benchmarks).
func indexSlash(_ string, segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	switch segments[0] {
	case "docs", "examples", "releases":
		return "/"
	}
	return ""
}

func (s *Server) isProduction() bool {
	return string(s.cfg.Env) == "production"
}
