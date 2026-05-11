// Package meta defines the per-page metadata model used by the page templates.
package meta

import "strings"

// Breadcrumb is one step in a breadcrumb trail.
type Breadcrumb struct {
	Label string
	Href  string // Empty when this is the current page.
}

// JSONLDBlock is one <script type="application/ld+json"> emission, plus an
// optional one-line HTML comment that sits immediately above the script
// tag. The comment is the audit trail required by spec/structured-data.md
// § Field completeness whenever a required-or-recommended schema.org field
// is omitted (e.g. an embedded content file with no front-matter
// datePublished).
type JSONLDBlock struct {
	Comment string // optional, single line; when non-empty the chrome emits "<!-- <Comment> -->" above the script tag
	JSON    string // pre-encoded JSON-LD, one object per block
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
	JSONLD      []JSONLDBlock // Pre-encoded JSON-LD blocks (one per <script>); each may carry an audit-trail HTML comment.
	Version     string   // Current MuxMaster version label.
	GoVersion   string   // Minimum supported Go version (e.g. "1.26"); sourced from ../MuxMaster/go.mod at build time.
	CSSPath     string   // Hashed CSS bundle URL.
	BaseURL     string   // SITE_BASE_URL for absolute references.
	UpstreamURL string   // Optional link to the upstream source file on GitHub.
	HasMarkdown bool     // True when the page exposes a Markdown companion at <Canonical>.md.
	// MarkdownAlternateURL is the explicit Markdown-companion URL used by
	// <link rel="alternate" type="text/markdown" href="...">. When empty
	// the head template falls back to `<Canonical>.md`, which is correct
	// for inner pages whose canonical does not end in `/`. Index pages
	// (whose canonical ends in `/`) MUST set this explicitly because
	// `<Canonical>.md` would produce a path with a stray slash before
	// `.md` (for example `/docs/.md`).
	MarkdownAlternateURL string
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
