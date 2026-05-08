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
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/render"
)

// Server bundles the rendered router with its dependencies for graceful
// shutdown.
type Server struct {
	cfg        *config.Config
	logger     *slog.Logger
	renderer   *render.Renderer
	version    string
	staticDir  string
	httpServer *http.Server
}

// New constructs the server. templatesDir, staticDir, and version are required
// by the renderer and the page chrome.
func New(cfg *config.Config, logger *slog.Logger, version, templatesDir, staticDir string) (*Server, error) {
	r, err := render.New(templatesDir, staticDir)
	if err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}

	s := &Server{
		cfg:       cfg,
		logger:    logger,
		renderer:  r,
		version:   version,
		staticDir: staticDir,
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
