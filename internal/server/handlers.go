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

// pageData merges the per-page metadata with the global chrome inputs.
type pageData struct {
	Meta meta.Page
	// Body is populated by templates that need page-specific content.
	Body any
}

// landingHandler renders the home page.
func (s *Server) landingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := s.pageMeta("/", "", landingDescription, "website")
		body, etag, err := s.renderer.Render("landing.html", render.Data{Meta: page})
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		s.renderer.Write(w, r, http.StatusOK, body, etag, cacheControlLanding)
	}
}

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

// robotsHandler returns a minimal valid robots.txt. Filled in fully by the
// SEO and GEO specialists in a later round.
func (s *Server) robotsHandler() http.HandlerFunc {
	body := []byte("User-agent: *\nAllow: /\nDisallow: /healthz\n\n" +
		"Sitemap: " + s.cfg.SiteBaseURL + "/sitemap.xml\n")
	etag := render.ETag(body)
	return func(w http.ResponseWriter, r *http.Request) {
		if render.MatchesIfNoneMatch(r, etag) {
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", cacheControlText)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", cacheControlText)
		_, _ = w.Write(body)
	}
}

// llmsHandler is the day-one stub for /llms.txt. The content rounds replace
// this with a route-table-derived listing.
func (s *Server) llmsHandler(full bool) http.HandlerFunc {
	suffix := ""
	if full {
		suffix = "-full"
	}
	body := []byte("# MuxMaster\n\n" +
		"MuxMaster is a high-performance, zero-dependency HTTP router for Go. " +
		"This file (/llms" + suffix + ".txt) is a stub during initial scaffolding " +
		"and will be replaced by an auto-generated index of canonical URLs.\n\n" +
		"## Source\n\n- https://github.com/FlavioCFOliveira/MuxMaster\n")
	etag := render.ETag(body)
	return func(w http.ResponseWriter, r *http.Request) {
		if render.MatchesIfNoneMatch(r, etag) {
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", cacheControlText)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", cacheControlText)
		_, _ = w.Write(body)
	}
}

// sitemapHandler returns a minimal valid sitemap. Until the canonical domain
// is decided (open-questions.md item 1), the sitemap is empty in non-production
// environments per specification/deployment.md "Production launch gate".
func (s *Server) sitemapHandler() http.HandlerFunc {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n" +
		`</urlset>` + "\n")
	etag := render.ETag(body)
	return func(w http.ResponseWriter, r *http.Request) {
		if render.MatchesIfNoneMatch(r, etag) {
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", cacheControlSitemap)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", cacheControlSitemap)
		_, _ = w.Write(body)
	}
}

// notFoundHandler renders the branded 404 page.
func (s *Server) notFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := s.pageMeta(r.URL.Path, "Page not found", notFoundDescription, "article")
		page.Robots = "noindex,nofollow"
		page.Breadcrumbs = []meta.Breadcrumb{
			{Label: "Home", Href: "/"},
			{Label: "Page not found"},
		}
		body, etag, err := s.renderer.Render("coming-soon.html", render.Data{
			Meta: page,
			Body: notFoundBody{},
		})
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", cacheControlNoStore)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(body)
	}
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("render error",
		"path", r.URL.Path,
		"err", err.Error(),
	)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControlNoStore)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte("500 Internal Server Error\n"))
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

// notFoundBody is a marker type the template can detect with `{{if ...}}`.
type notFoundBody struct{}

const (
	landingDescription  = "MuxMaster is a high-performance, zero-dependency HTTP router for Go. Radix-tree O(k) lookups, zero allocations on static routes, and 100% net/http compatibility."
	notFoundDescription = "The page you requested does not exist on the MuxMaster documentation site."
)

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
