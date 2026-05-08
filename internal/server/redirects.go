package server

import (
	"net/http"
	"strings"
)

// normalisationRedirects implements the URL-normalisation contract from
// specification/url-and-versioning.md "Redirects":
//
//   - /index.html, /docs/index.html, /examples/index.html → directory form
//   - /<any>.html (HTML extension) → /<any>
//   - mixed-case path → all-lowercase path (preserves query string, preserves
//     a .md companion suffix)
//
// All redirects are 301 (Moved Permanently). The Location header is path-only;
// the host header is never echoed back to the client. This mirrors the same
// defence pattern as MuxMaster's RedirectTrailingSlash: a request that smuggles
// a host header to construct an off-site redirect cannot influence the
// Location we emit.
//
// The middleware runs before the prerender lookup so the canonical path is
// the one served (saving a second hop and the prerender miss).
func normalisationRedirects(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		if path == "" {
			next.ServeHTTP(w, r)
			return
		}

		newPath, redirect := normalisePath(path)
		if !redirect {
			next.ServeHTTP(w, r)
			return
		}

		// Path-only Location, query string preserved. We deliberately do not
		// echo r.Host: the redirect must not be influenced by a hostile Host
		// header.
		loc := newPath
		if r.URL.RawQuery != "" {
			loc += "?" + r.URL.RawQuery
		}
		w.Header().Set("Location", loc)
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.WriteHeader(http.StatusMovedPermanently)
	})
}

// normalisePath applies the four redirect rules from the spec. Returns the
// normalised path and a boolean reporting whether a redirect should happen.
//
// The order matters:
//  1. /index.html-family → directory form (handled by the .html stripper
//     because /index.html → /index → "" is wrong; we special-case index).
//  2. .html suffix → strip it.
//  3. mixed case → lowercase (preserving a .md suffix that survived rule 2).
//
// We only consider GET/HEAD; the caller already filtered methods.
func normalisePath(path string) (string, bool) {
	original := path
	changed := false

	// Rule 1+2: strip a .html suffix and the special /index leaf that
	// remains. /index.html → "/", /docs/index.html → "/docs/", etc.
	if strings.HasSuffix(path, ".html") {
		path = strings.TrimSuffix(path, ".html")
		// /index → /, /docs/index → /docs/.
		if path == "/index" {
			path = "/"
		} else if strings.HasSuffix(path, "/index") {
			path = strings.TrimSuffix(path, "index")
		}
		changed = true
	}

	// Rule 3: lowercase the path. The .md companion suffix is already
	// lowercase by convention and survives ToLower unchanged.
	if hasUpper(path) {
		path = strings.ToLower(path)
		changed = true
	}

	if !changed || path == original {
		return original, false
	}
	return path, true
}

// hasUpper reports whether s contains any ASCII upper-case rune. We avoid
// strings.ToLower's allocation when nothing needs to change.
func hasUpper(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			return true
		}
	}
	return false
}
