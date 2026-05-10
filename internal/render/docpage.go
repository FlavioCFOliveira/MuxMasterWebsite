package render

import (
	"fmt"
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
	{Path: "/examples/authn", Title: "Authentication"},
	{Path: "/examples/jwt", Title: "JWT"},
	{Path: "/examples/oauth2", Title: "OAuth2"},
	{Path: "/examples/cache", Title: "Cache"},
	{Path: "/examples/graceful-shutdown", Title: "Graceful shutdown"},
	{Path: "/examples/server-side-render", Title: "Server-side render"},
	{Path: "/examples/static-site", Title: "Static site"},
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
			htmlBody, err := MarkdownToHTML(src)
			if err != nil {
				return nil, fmt.Errorf("doc-page %s: %w", spec.Path, err)
			}

			page := basePage(deps, spec.Path, spec.Title, spec.Description, spec.OGType, ogImagePath, productionRobots)
			page.Breadcrumbs = breadcrumbsForDoc(spec)
			page.UpstreamURL = spec.UpstreamURL

			// mtime is best-effort. Embedded files report a zero time on
			// embed.FS; the body footer falls back to the build time so
			// every page always shows a real "Last updated" line. The
			// JSON-LD path uses sourceMtime (the unmodified zero-when-
			// missing value) so it can OMIT dateModified per the
			// no-fabricated-values rule (spec/structured-data.md § Date
			// sources for embedded content); the omission is reported via
			// an HTML-comment audit trail by BuildJSONLD.
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
			// HowTo is emitted only for pages whose body is genuinely
			// structured as `## Step N — name` headings. /docs/getting-
			// started follows that shape; the graceful-shutdown example
			// does not (its body is prose around a single program), so it
			// is intentionally absent from this list. Emitting an empty
			// HowTo block from a non-step body is a defect (rmp #48).
			var howToSrc []byte
			if spec.Path == "/docs/getting-started" {
				howToSrc = src
			}
			page.JSONLD = BuildJSONLD(JSONLDInputs{
				Page:         page,
				Family:       family,
				BuildTime:    deps.BuildTime,
				DateModified: sourceMtime, // zero → omit; never substituted with BuildTime.
				HowToSource:  howToSrc,
				RenderedHTML: htmlBody, // FAQPage scanner reads <section data-conversation>.
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
