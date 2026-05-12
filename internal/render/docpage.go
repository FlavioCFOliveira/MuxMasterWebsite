package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/content"
	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/meta"
)

// docPageBody is the data passed to the doc-page template. It is fully
// resolved at startup; the template treats every field as a literal string
// or pre-rendered HTML fragment.
type docPageBody struct {
	Title        string
	HTML         string // Pre-rendered HTML body; injected via safeHTML.
	TOC          []Heading
	ShowSidebar  bool
	SidebarTitle string // Section heading for the sidebar (e.g. "Documentation", "Reference", "Examples").
	SidebarItems []sidebarItem
	HasPrevNext  bool
	Prev         navLink
	Next         navLink
	LastModified time.Time // mtime of the underlying content file; falls back to BuildTime when zero.
}

type sidebarItem struct {
	Path  string
	Title string
}

type navLink struct {
	Path  string
	Title string
}

// docsSidebar is the eleven-section ordered list defined in
// specification/information-architecture.md "Sidebar". The order is the
// canonical reading order and also drives the prev/next chain on every
// /docs/<section> page.
var docsSidebar = []sidebarItem{
	{Path: "/docs/getting-started", Title: "Getting started"},
	{Path: "/docs/routing", Title: "Routing"},
	{Path: "/docs/groups", Title: "Groups"},
	{Path: "/docs/middleware", Title: "Middleware"},
	{Path: "/docs/error-handling", Title: "Error handling"},
	{Path: "/docs/configuration", Title: "Configuration"},
	{Path: "/docs/response-helpers", Title: "Response helpers"},
	{Path: "/docs/performance", Title: "Performance"},
	{Path: "/docs/max-performance", Title: "Maximum performance"},
	{Path: "/docs/observability", Title: "Observability"},
	{Path: "/docs/migration", Title: "Migration"},
	{Path: "/docs/cookbook", Title: "Cookbook"},
}

// referenceSidebar is shown on every doc-family page that is not part of
// /docs/ or /examples/. The order is curated to lead with the most-asked
// reference (API), then collections (Examples, Benchmarks), then change
// notes (Changelog, Release notes), then policy pages (Security,
// Compatibility, Contributing). Pages here are siblings rather than a
// sequence, so prev/next is intentionally not extended to this sidebar.
var referenceSidebar = []sidebarItem{
	{Path: "/api", Title: "API reference"},
	{Path: "/examples/", Title: "Examples"},
	{Path: "/benchmarks", Title: "Benchmarks"},
	{Path: "/changelog", Title: "Changelog"},
	{Path: "/releases/v1.1.0", Title: "Release notes — v1.1.0"},
	{Path: "/releases/v1.0.0", Title: "Release notes — v1.0.0"},
	{Path: "/security", Title: "Security"},
	{Path: "/compatibility", Title: "Compatibility"},
	{Path: "/contributing", Title: "Contributing"},
}

// examplesSidebar is shown on every /examples/<name> page. The order is the
// curated learning order from the specification (REST first, auth family
// second, then operational concerns), and also drives the prev/next chain
// across example pages.
var examplesSidebar = []sidebarItem{
	{Path: "/examples/rest-api", Title: "REST API"},
	{Path: "/examples/versioning", Title: "Versioning"},
	{Path: "/examples/authn", Title: "Authentication"},
	{Path: "/examples/jwt", Title: "JWT"},
	{Path: "/examples/oauth2", Title: "OAuth2"},
	{Path: "/examples/cache", Title: "Cache"},
	{Path: "/examples/graceful-shutdown", Title: "Graceful shutdown"},
	{Path: "/examples/server-side-render", Title: "Server-side render"},
	{Path: "/examples/static-site", Title: "Static site"},
	{Path: "/examples/reverse-proxy", Title: "Reverse proxy"},
}

// DocPageSpec describes one Markdown-backed long-form page. The same
// recipe family powers /docs/<section>, /api, /examples/<name>,
// /benchmarks, /changelog, /releases/<v>, /security, /compatibility, and
// /contributing.
type DocPageSpec struct {
	Path        string // Public route, e.g. "/docs/routing".
	Title       string // <h1> text and <title> prefix.
	Description string // <meta name="description"> content.
	ContentPath string // Loader path under /content/, e.g. "docs/routing.md".
	Section     string // Used to populate breadcrumbs and pick the sidebar.
	UpstreamURL string // Optional GitHub link, currently unused by this template.
	Cache       string // Cache-Control value (per spec families).
	OGType      string // "article" for documentation pages.
}

// DocPageRecipe builds the HTML representation of a Markdown-backed page.
//
// Why one generic recipe instead of one recipe per route: the wiring is
// identical for every route in this family — load the Markdown, render it,
// extract the in-page TOC, populate the doc-page template. Splitting by
// route would duplicate that wiring twenty-six times for no benefit. The
// per-route differences (path, title, breadcrumb shape, sidebar visibility)
// are captured by DocPageSpec.
func DocPageRecipe(spec DocPageSpec, loader *content.Loader, ogImagePath string, productionRobots bool) Recipe {
	return Recipe{
		Path:        spec.Path,
		ContentType: "text/html; charset=utf-8",
		Build: func(deps Deps) ([]byte, error) {
			src, err := loader.Load(spec.ContentPath)
			if err != nil {
				return nil, err
			}
			// Strip the leading YAML frontmatter before handing the body to
			// the CommonMark renderer (spec/rendering-and-caching.md
			// "Rendering model"). Frontmatter is metadata for the JSON-LD
			// date resolver; rendered as Markdown it would otherwise be
			// interpreted as a thematic break + setext H2 and leak into the
			// HTML body and the TOC. parseFrontmatter and HowToSource below
			// keep operating on the unstripped src so their byte offsets
			// match the file as authored.
			htmlBody, err := MarkdownToHTML(stripFrontmatter(src))
			if err != nil {
				return nil, fmt.Errorf("doc-page %s: %w", spec.Path, err)
			}

			page := basePage(deps, spec.Path, spec.Title, spec.Description, spec.OGType, ogImagePath, productionRobots)
			page.Breadcrumbs = breadcrumbsForDoc(spec)
			page.UpstreamURL = spec.UpstreamURL
			// Every DocPageSpec route ships a Markdown companion at
			// <Canonical>.md (see internal/server/routes.go and
			// content-sources.md), so the alternate-markdown <link>
			// emits unconditionally for this family.
			page.HasMarkdown = true

			// mtime is best-effort. Embedded files report a zero time on
			// embed.FS; the body footer falls back to the build time so
			// every page always shows a real "Last updated" line. The
			// JSON-LD path uses sourceMtime (the unmodified zero-when-
			// missing value) so it can OMIT dateModified per the
			// no-fabricated-values rule (spec/structured-data.md § Date
			// sources for embedded content); the omission is reported via
			// an HTML-comment audit trail by BuildJSONLD.
			// Parse the leading YAML frontmatter (when present) for
			// datePublished / dateModified. Resolution order per
			// spec/structured-data.md § Date sources for embedded content:
			// (1) front-matter, (2) git-log mtime / filesystem mtime,
			// (3) omit with audit-trail HTML comment. We honour (1) and
			// (2) here; (3) is the natural fallthrough when both are
			// zero.
			fm := parseFrontmatter(src)

			sourceMtime, _ := loader.Mtime(spec.ContentPath)
			lastMod := sourceMtime
			if lastMod.IsZero() {
				lastMod = deps.BuildTime
			}

			body := docPageBody{
				Title: spec.Title,
				HTML:  string(htmlBody),
				// Extract anchors from the rendered HTML rather than the
				// source Markdown so the TOC's href="#…" cannot drift
				// from the body's id="…". They come from the same string.
				TOC:          ExtractHeadingsFromHTML(htmlBody),
				LastModified: lastMod,
			}

			switch spec.Section {
			case "docs":
				body.ShowSidebar = true
				body.SidebarTitle = "Documentation"
				body.SidebarItems = docsSidebar
				body.HasPrevNext = true
				body.Prev, body.Next = prevNextIn(docsSidebar, spec.Path)
			case "examples":
				body.ShowSidebar = true
				body.SidebarTitle = "Examples"
				body.SidebarItems = examplesSidebar
				body.HasPrevNext = true
				body.Prev, body.Next = prevNextIn(examplesSidebar, spec.Path)
			case "api", "benchmarks", "changelog", "releases", "security", "compatibility", "contributing":
				// Reference pages are independent siblings, not a sequence —
				// the sidebar provides the lateral navigation but prev/next
				// is intentionally suppressed to avoid implying a reading
				// order across unrelated pages.
				body.ShowSidebar = true
				body.SidebarTitle = "Reference"
				body.SidebarItems = referenceSidebar
			}

			// JSON-LD: TechArticle + BreadcrumbList for every doc-family page.
			// /api additionally emits a SoftwareSourceCode object referencing
			// the upstream module. /docs/getting-started has step-shaped
			// headings; the HowTo block is emitted only when the source
			// actually contains them.
			family := "doc-article"
			switch spec.Section {
			case "api":
				family = "api"
			case "benchmarks":
				family = "benchmarks"
			}
			// HowTo emission is data-driven (per spec/geo.md § Example
			// walkthrough shape and § FAQPage and HowTo structured data):
			// the renderer feeds every doc-page source into the HowTo
			// emitter, which produces a block only when the body
			// genuinely contains a contiguous `## Step N — <name>`
			// sequence. Pages without that shape produce zero bytes and
			// no HowTo block is emitted.
			howToSrc := src
			// Resolve datePublished and dateModified per the doctrine:
			// front-matter beats build-time mtime; both zero → omit.
			datePublished := fm.DatePublished
			dateModified := fm.DateModified
			if dateModified.IsZero() {
				dateModified = sourceMtime // still zero on embed.FS; renderer omits with audit trail
			}

			page.JSONLD = BuildJSONLD(JSONLDInputs{
				Page:          page,
				Family:        family,
				BuildTime:     deps.BuildTime,
				DatePublished: datePublished,
				DateModified:  dateModified,
				HowToSource:   howToSrc,
				RenderedHTML:  htmlBody, // FAQPage scanner reads <section data-conversation>.
			})

			return deps.Renderer.ExecuteTemplate("doc-page.html", Data{Meta: page, Body: body})
		},
	}
}

// MarkdownCompanionRecipe builds the .md companion entry. It strips a
// leading `---` frontmatter block if present (per
// specification/content-sources.md "Markdown companions"), but otherwise
// serves the source bytes verbatim.
func MarkdownCompanionRecipe(routePath, contentPath string, loader *content.Loader) Recipe {
	return Recipe{
		Path:        routePath + ".md",
		ContentType: "text/markdown; charset=utf-8",
		Build: func(_ Deps) ([]byte, error) {
			src, err := loader.Load(contentPath)
			if err != nil {
				return nil, err
			}
			return stripFrontmatter(src), nil
		},
	}
}

// frontmatter is the small subset of YAML keys the renderer reads from a
// content file's leading `---` block. Only the keys named here are
// recognised; the full file is still served verbatim via the .md
// companion path, so other keys are passed through to the markdown
// pipeline (where stripFrontmatter removes the block on its way to
// goldmark). Date fields are parsed as RFC 3339 / date-only strings.
type frontmatter struct {
	DatePublished time.Time
	DateModified  time.Time
}

// parseFrontmatter reads the leading `---` YAML block of src and returns
// the recognised keys (currently datePublished and dateModified). Files
// without a frontmatter block return a zero-value frontmatter and no
// error. Malformed dates are silently ignored — the doctrine's
// resolution order (front-matter → git-log → omit with audit comment)
// handles missing values; a bad value is treated the same as missing.
func parseFrontmatter(src []byte) frontmatter {
	var fm frontmatter
	if len(src) < 4 || string(src[:4]) != "---\n" {
		return fm
	}
	rest := src[4:]
	// Find the closing `---` line (same shape as stripFrontmatter).
	var block string
	for i := 0; i < len(rest); i++ {
		if rest[i] != '\n' {
			continue
		}
		j := i + 1
		if j+3 <= len(rest) && string(rest[j:j+3]) == "---" {
			block = string(rest[:i])
			break
		}
	}
	if block == "" {
		return fm
	}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch key {
		case "datePublished":
			if t, err := parseFrontmatterDate(val); err == nil {
				fm.DatePublished = t
			}
		case "dateModified":
			if t, err := parseFrontmatterDate(val); err == nil {
				fm.DateModified = t
			}
		}
	}
	return fm
}

// parseFrontmatterDate accepts either a full RFC 3339 timestamp or a
// date-only ("YYYY-MM-DD") form. Date-only values are anchored at
// midnight UTC.
func parseFrontmatterDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

// stripFrontmatter removes a leading YAML frontmatter block delimited by
// `---` lines. Files without a frontmatter block are returned unchanged.
func stripFrontmatter(src []byte) []byte {
	if len(src) < 4 || string(src[:4]) != "---\n" {
		return src
	}
	// Find the closing `---` line.
	rest := src[4:]
	for i := 0; i < len(rest); i++ {
		// Look for "\n---\n" (or "\n---\r\n", or trailing "\n---" at EOF).
		if rest[i] != '\n' {
			continue
		}
		j := i + 1
		if j+3 <= len(rest) && string(rest[j:j+3]) == "---" {
			end := j + 3
			if end == len(rest) {
				return nil
			}
			if rest[end] == '\n' {
				return rest[end+1:]
			}
			if end+1 <= len(rest) && rest[end] == '\r' && end+1 < len(rest) && rest[end+1] == '\n' {
				return rest[end+2:]
			}
		}
	}
	return src
}

// breadcrumbsForDoc returns the breadcrumb trail for a doc-page. The shape
// matches the contract in specification/information-architecture.md
// "Breadcrumbs": Home / Section / Page (or Home / Section for an index URL,
// not relevant here because doc-page never serves the section index).
func breadcrumbsForDoc(spec DocPageSpec) []meta.Breadcrumb {
	switch spec.Section {
	case "docs":
		return []meta.Breadcrumb{
			{Label: "Home", Href: "/"},
			{Label: "Documentation", Href: "/docs/"},
			{Label: spec.Title},
		}
	case "examples":
		return []meta.Breadcrumb{
			{Label: "Home", Href: "/"},
			{Label: "Examples", Href: "/examples/"},
			{Label: spec.Title},
		}
	case "releases":
		return []meta.Breadcrumb{
			{Label: "Home", Href: "/"},
			{Label: "Releases"},
			{Label: spec.Title},
		}
	case "api", "benchmarks", "changelog", "security", "compatibility", "contributing":
		// Top-level leaf pages: Home / Page.
		return []meta.Breadcrumb{
			{Label: "Home", Href: "/"},
			{Label: spec.Title},
		}
	}
	return []meta.Breadcrumb{
		{Label: "Home", Href: "/"},
		{Label: spec.Title},
	}
}

// prevNextIn returns the prev and next neighbours of path within the
// supplied sidebar slice, in the slice's order. The first item has no prev;
// the last item has no next. Returns empty navLinks when path is not in the
// slice (defensive — should not happen for well-formed routes).
func prevNextIn(sidebar []sidebarItem, path string) (navLink, navLink) {
	idx := -1
	for i, it := range sidebar {
		if it.Path == path {
			idx = i
			break
		}
	}
	if idx == -1 {
		return navLink{}, navLink{}
	}
	var prev, next navLink
	if idx > 0 {
		prev = navLink{Path: sidebar[idx-1].Path, Title: sidebar[idx-1].Title}
	}
	if idx < len(sidebar)-1 {
		next = navLink{Path: sidebar[idx+1].Path, Title: sidebar[idx+1].Title}
	}
	return prev, next
}
