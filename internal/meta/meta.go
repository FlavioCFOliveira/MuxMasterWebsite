// Package meta defines the per-page metadata model used by the page templates.
package meta

import "strings"

// Breadcrumb is one step in a breadcrumb trail.
type Breadcrumb struct {
	Label string
	Href  string // Empty when this is the current page.
}

// Page captures everything <head> and the chrome need to know about one page.
// All absolute URLs (Canonical, OGImage) are resolved by the renderer using
// SITE_BASE_URL.
type Page struct {
	Title       string // <title> content, before the " — MuxMaster" suffix.
	Description string // <meta name="description"> body, 110-160 chars.
	Path        string // Absolute path on the site, e.g. "/docs/routing".
	Canonical   string // Absolute canonical URL. Empty means "do not emit a canonical link" (used for noindex pages such as /404 and /500).
	OGType      string // "website" for /, "article" otherwise.
	OGImage     string // Absolute OG image URL.
	Robots      string // Empty unless a noindex value is required.
	Breadcrumbs []Breadcrumb
	JSONLD      []string // Pre-encoded JSON-LD blocks (one per <script>).
	Version     string   // Current MuxMaster version label.
	GoVersion   string   // Minimum supported Go version (e.g. "1.26"); sourced from ../MuxMaster/go.mod at build time.
	CSSPath     string   // Hashed CSS bundle URL.
	BaseURL     string   // SITE_BASE_URL for absolute references.
	UpstreamURL string   // Optional link to the upstream source file on GitHub.
}

// FullTitle returns the title with the canonical " — MuxMaster" suffix.
// The landing page is the only place the suffix is suppressed (Title=="").
func (p Page) FullTitle() string {
	if p.Title == "" {
		return "MuxMaster — High-performance HTTP router for Go"
	}
	return p.Title + " — MuxMaster"
}

// IsHome reports whether the page is the site root.
func (p Page) IsHome() bool {
	return p.Path == "/"
}

// SectionLabel returns a short human label for the section this page belongs
// to, used by the breadcrumb default. Empty for the landing page.
func (p Page) SectionLabel() string {
	if p.IsHome() {
		return ""
	}
	parts := strings.Split(strings.Trim(p.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	switch parts[0] {
	case "docs":
		return "Documentation"
	case "api":
		return "API"
	case "examples":
		return "Examples"
	case "benchmarks":
		return "Benchmarks"
	case "changelog":
		return "Changelog"
	case "releases":
		return "Releases"
	case "security":
		return "Security"
	case "compatibility":
		return "Compatibility"
	case "contributing":
		return "Contributing"
	}
	return ""
}
