// Command muxmaster-website serves the MuxMaster documentation website,
// powered by MuxMaster itself.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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
	// `--healthcheck` is the in-container probe used by the Dockerfile
	// HEALTHCHECK directive. The distroless runtime image has neither
	// curl nor wget; the binary itself does the HTTP self-GET to
	// /healthz and exits 0 on 200, 1 otherwise. Implemented as the very
	// first thing in main so the probe path does not touch config /
	// content / server.
	if len(os.Args) > 1 && (os.Args[1] == "--healthcheck" || os.Args[1] == "-healthcheck") {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// healthcheck probes /healthz on the in-container listener (PORT env or
// 8080 default) and returns 0 when the response is 200 OK, 1 otherwise.
// The function is intentionally minimal — no imports beyond net/http
// and os — so the binary remains small and the probe is fast.
func healthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: status=%d\n", resp.StatusCode)
		return 1
	}
	return 0
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
