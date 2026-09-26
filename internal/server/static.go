package server

import (
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	muxm "github.com/FlavioCFOliveira/MuxMaster"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/render"
)

// Cache-Control values for static assets
// (specification/rendering-and-caching.md "Static assets").
const (
	cacheControlHashedAsset = "public, max-age=31536000, immutable"
	cacheControlStatic      = "public, max-age=86400"
)

// compressibleTypes lists the media types that get a stored gzip body. Every
// other type (PNG, WebP, AVIF) is already compressed at rest.
var compressibleTypes = map[string]bool{
	"text/css":                  true,
	"image/svg+xml":             true,
	"text/javascript":           true,
	"application/json":          true,
	"text/plain":                true,
	"application/xml":           true,
	"application/manifest+json": true,
}

// staticAssets is the in-memory copy of the static directory. It is built
// once at startup; the directory is never read again.
type staticAssets struct {
	// files maps the path below /static — with its leading slash, the
	// exact form of the *filepath parameter (e.g. "/css/app.<hash>.css") —
	// to its pre-computed response. Only regular files are keys, so a
	// directory, a traversal attempt, or an unknown name is simply absent.
	files map[string]*render.Response
}

// loadStaticAssets reads every regular file under dir into memory and
// pre-computes its response: identity body, gzip body for compressible
// types (kept only when smaller), strong ETag, Content-Type from the
// extension, and Cache-Control. hashedCSS is the URL path of the hashed CSS
// bundle, the only asset cached as immutable.
func loadStaticAssets(dir, hashedCSS string) (*staticAssets, error) {
	files := make(map[string]*render.Response)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		key := "/" + filepath.ToSlash(rel)

		contentType := mime.TypeByExtension(path.Ext(key))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		mediaType, _, _ := strings.Cut(contentType, ";")

		cacheControl := cacheControlStatic
		if "/static"+key == hashedCSS {
			cacheControl = cacheControlHashedAsset
		}

		resp, err := render.NewResponse(render.ResponseOptions{
			Body:         body,
			ContentType:  contentType,
			CacheControl: cacheControl,
			Gzip:         compressibleTypes[strings.TrimSpace(mediaType)],
		})
		if err != nil {
			return err
		}
		files[key] = resp
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("server: load static assets from %s: %w", dir, err)
	}
	return &staticAssets{files: files}, nil
}

// handler serves /static/*filepath from memory. It is a muxmaster.FastHandler:
// the router passes the captured parameters as an argument instead of
// storing them in the request context, and with Mux.PoolFastParams the
// Params slice is recycled after the call. The handler reads ps only during
// the call and never retains it, r, or r.URL (the lifetime contract of
// specification/rendering-and-caching.md "Router configuration").
//
// Unlike the http.FileServer it replaces, it never rewrites r.URL.Path, so
// the access log records the path the client requested.
func (a *staticAssets) handler(notFound http.HandlerFunc) muxm.FastHandler {
	return func(w http.ResponseWriter, r *http.Request, ps muxm.Params) {
		resp, ok := a.files[ps.Get("filepath")]
		if !ok {
			notFound(w, r)
			return
		}
		resp.Serve(w, r)
	}
}
