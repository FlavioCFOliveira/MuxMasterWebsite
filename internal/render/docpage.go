package render

import (
	"fmt"

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
	SidebarItems []sidebarItem
	HasPrevNext  bool
	Prev         navLink
	Next         navLink
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

			body := docPageBody{
				Title: spec.Title,
				HTML:  string(htmlBody),
				TOC:   ExtractHeadings(src),
			}

			if spec.Section == "docs" {
				body.ShowSidebar = true
				body.SidebarItems = docsSidebar
				body.HasPrevNext = true
				body.Prev, body.Next = prevNextFor(spec.Path)
			}

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

// prevNextFor returns the prev and next neighbours within /docs/ in the
// order defined by docsSidebar. /docs/getting-started has no prev;
// /docs/cookbook has no next.
func prevNextFor(path string) (navLink, navLink) {
	idx := -1
	for i, it := range docsSidebar {
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
		prev = navLink{Path: docsSidebar[idx-1].Path, Title: docsSidebar[idx-1].Title}
	}
	if idx < len(docsSidebar)-1 {
		next = navLink{Path: docsSidebar[idx+1].Path, Title: docsSidebar[idx+1].Title}
	}
	return prev, next
}
