// Package server constructs and configures the HTTP server, the MuxMaster
// router, the middleware stack, and the route table.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	muxm "github.com/FlavioCFOliveira/MuxMaster"
	mwm "github.com/FlavioCFOliveira/MuxMaster/middleware"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/config"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/content"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/render"
)

// ogImagePath is the relative path of the Open Graph image. Resolved to an
// absolute URL by basePage in render/recipes.go using SITE_BASE_URL.
const ogImagePath = "/static/img/og-image.png"

// Server bundles the rendered router with its dependencies for graceful
// shutdown.
type Server struct {
	cfg        *config.Config
	logger     *slog.Logger
	renderer   *render.Renderer
	loader     *content.Loader
	version    string
	staticDir  string
	buildTime  time.Time
	httpServer *http.Server
}

// New constructs the server. templatesDir, staticDir, the loader, and the
// resolved version label are required by the renderer and the page chrome.
func New(cfg *config.Config, logger *slog.Logger, loader *content.Loader, version, templatesDir, staticDir string) (*Server, error) {
	r, err := render.New(templatesDir, staticDir)
	if err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}

	s := &Server{
		cfg:       cfg,
		logger:    logger,
		renderer:  r,
		loader:    loader,
		version:   version,
		staticDir: staticDir,
		buildTime: time.Now().UTC(),
	}

	m := muxm.New()
	m.RedirectTrailingSlash = true
	// Case-insensitive matching is OFF: case is part of the canonical URL
	// contract (specification/url-and-versioning.md).
	m.CaseInsensitive = false

	// Pre-routing middleware. Order matters: Recoverer outermost, then
	// RequestID so panics are logged with the request id, then the access
	// logger, then security headers, then compression.
	m.Pre(mwm.RecovererWithLogger(logger))
	m.Pre(mwm.RequestID())
	m.Pre(slogAccessLog(logger))
	m.Pre(securityHeaders)
	m.Pre(mwm.Compress(5))

	s.registerRoutes(m)

	s.httpServer = &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           m,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	return s, nil
}

// Prerender materialises every public route to bytes and stores them on the
// renderer. It MUST be called once between New() and Start().
//
// A failure here is fatal at startup per
// specification/rendering-and-caching.md "static-tending": a broken
// pre-render is not a runtime degradation. The same URL must return the
// same bytes for the lifetime of the process.
func (s *Server) Prerender() error {
	deps := render.Deps{
		Routes:    routeInfos(),
		Version:   s.version,
		BaseURL:   s.cfg.SiteBaseURL,
		BuildTime: s.buildTime,
		Renderer:  s.renderer,
	}
	productionRobots := s.isProduction()

	recipes := []render.Recipe{
		render.LandingRecipe(LandingDescription, ogImagePath, productionRobots),
		render.DocsIndexRecipe(s.loader, ogImagePath, productionRobots),
		render.ExamplesIndexRecipe(s.loader, ogImagePath, productionRobots),
		render.LLMsRecipe(),
		render.LLMsFullRecipe(s.loader, routeContentPaths()),
		render.SitemapRecipe(),
		render.RobotsRecipe(),
		render.NotFoundRecipe(ogImagePath, productionRobots),
		render.ServerErrorRecipe(ogImagePath, productionRobots),
	}

	// Doc-page recipes for every route in the route table that has a
	// curated Markdown source. Both the HTML route and the .md companion
	// derive from the same source (specification/content-sources.md).
	for _, spec := range docPageSpecs() {
		recipes = append(recipes, render.DocPageRecipe(spec, s.loader, ogImagePath, productionRobots))
		recipes = append(recipes, render.MarkdownCompanionRecipe(spec.Path, spec.ContentPath, s.loader))
	}

	if err := s.renderer.Prerender(recipes, deps); err != nil {
		return err
	}
	for _, p := range s.renderer.PrerenderedPaths() {
		pre, _ := s.renderer.Prerendered(p)
		s.logger.Info("prerendered", "path", p, "bytes", len(pre.Body))
	}
	return nil
}

// Start begins serving and blocks until the underlying http.Server returns.
// On http.ErrServerClosed it returns nil.
func (s *Server) Start() error {
	s.logger.Info("server starting",
		"addr", s.httpServer.Addr,
		"version", s.version,
		"css_path", s.renderer.CSSPath(),
	)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed { //nolint:errorlint // sentinel comparison.
		return err
	}
	return nil
}

// Shutdown drains in-flight requests within ctx's deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
