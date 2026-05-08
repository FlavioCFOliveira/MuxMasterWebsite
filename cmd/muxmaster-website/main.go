// Command muxmaster-website serves the MuxMaster documentation website,
// powered by MuxMaster itself.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	root "github.com/FlavioCFOliveira/MuxMasterWebsite"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/config"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/content"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/server"
)

// buildID is set via -ldflags="-X main.buildID=..." at compile time and is
// used as part of the build identity surfaced in startup logs.
var buildID = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	logger.LogAttrs(context.Background(), slog.LevelInfo, "configuration", cfg.LogAttrs()...)
	logger.Info("build", "id", buildID)

	// Resolve the embedded /content/ tree. The runtime is self-contained
	// (specification/deployment.md): no filesystem path outside the embed
	// is consulted.
	contentRoot, err := root.ContentFS()
	if err != nil {
		return fmt.Errorf("content embed: %w", err)
	}
	loader, err := content.NewLoader(contentRoot)
	if err != nil {
		return fmt.Errorf("content loader: %w", err)
	}
	if err := loader.Verify(); err != nil {
		return err
	}

	version, err := loader.Version()
	if err != nil {
		return err
	}
	logger.Info("content version", "version", version)

	// Resolve template and static directories relative to the working
	// directory so `go run`, `make dev`, and the Docker runtime all behave
	// consistently. The runtime image's working directory is /srv.
	templatesDir, staticDir := resolveAssetDirs()

	srv, err := server.New(cfg, logger, loader, version, templatesDir, staticDir)
	if err != nil {
		return err
	}

	// Materialise every public route to bytes before accepting requests.
	// A failure here is fatal — broken pre-render is a startup error per the
	// static-tending principle (specification/rendering-and-caching.md).
	if err := srv.Prerender(); err != nil {
		return fmt.Errorf("prerender: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server start: %w", err)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Warn("graceful shutdown exceeded budget", "err", err.Error())
	}
	return nil
}

// resolveAssetDirs picks the templates/ and static/ directories. Caller-supplied
// override via MUXMASTER_SITE_DIR allows running from any working directory;
// otherwise we look beside the binary, then in the working directory.
func resolveAssetDirs() (string, string) {
	if root := os.Getenv("MUXMASTER_SITE_DIR"); root != "" {
		return filepath.Join(root, "templates"), filepath.Join(root, "static")
	}
	if wd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(wd, "templates")); err == nil {
			return filepath.Join(wd, "templates"), filepath.Join(wd, "static")
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		return filepath.Join(dir, "templates"), filepath.Join(dir, "static")
	}
	return "templates", "static"
}
