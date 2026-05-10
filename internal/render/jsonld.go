package render

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/FlavioCFOliveira/MuxMasterWebsite/internal/meta"
)

// JSONLDInputs bundles the per-recipe inputs required to build the JSON-LD
// graph for a page. Recipes assemble this struct and call BuildJSONLD; the
// result is assigned to meta.Page.JSONLD before template execution.
//
// Why a dedicated struct rather than overloading meta.Page: meta.Page is the
// view-model the template reads. JSON-LD construction needs richer inputs
// (mtime, build time, source markdown for HowTo step extraction) that the
// template never references and that must not leak into chrome metadata.
type JSONLDInputs struct {
	Page         meta.Page
	Family       string    // "landing", "doc-article", "collection", "api", "error"
	BuildTime    time.Time // process start time, used as datePublished
	LastModified time.Time // mtime of the underlying content file (when applicable)
	HowToSource  []byte    // optional; if present the generator scans for `## Step N — name` headings
}

// BuildJSONLD returns one or more JSON-LD objects (each pre-stringified) for
// the supplied page. The chrome injects each entry into its own
// `<script type="application/ld+json">` block. Output is deterministic:
// objects are emitted in a fixed order per family, json.Marshal is invoked
// with sorted map keys (encoding/json sorts struct fields by declaration
// order; we use named structs to keep the output stable across runs).
//
// Error pages and unrecognised families return nil — the head template skips
// emission cleanly when the slice is empty.
func BuildJSONLD(in JSONLDInputs) []string {
	switch in.Family {
	case "landing":
		return buildLandingJSONLD(in)
	case "doc-article":
		return buildArticleJSONLD(in)
	case "collection":
		return buildCollectionJSONLD(in)
	case "api":
		return buildAPIJSONLD(in)
	}
	return nil
}

// schema is the JSON-LD context value emitted on every object.
const schema = "https://schema.org"

// jsonOrgID, jsonSiteID, jsonSoftwareID, jsonAuthorID are the canonical @id
// values for the four project-level entities reified by
// specification/structured-data.md § Entity graph. Each entity is emitted
// in full only on / (via buildEntityGraph) and referenced by @id from every
// other page. Renaming any of these is governed by the @id migration policy
// in the same spec.
func jsonOrgID(base string) string      { return base + "/#org" }
func jsonSiteID(base string) string     { return base + "/#website" }
func jsonSoftwareID(base string) string { return base + "/#software" }
func jsonAuthorID(base string) string   { return base + "/#author" }

// buildEntityGraph emits the four reified entity nodes (WebSite,
// SoftwareSourceCode, Organization, Person) in full. Per
// specification/structured-data.md § Entity graph and § Non-negotiables,
// these nodes appear in full ONLY on the landing page; every other page
// references them by @id. Renaming or restructuring is governed by the
// @id migration policy in the same spec.
//
// Field completeness for each node is the subject of separate tasks (see
// rmp tasks #23 through #28); this helper establishes the shape and the
// single emission site, with the minimum fields required to make the
// references resolvable today.
func buildEntityGraph(in JSONLDInputs) []string {
	base := in.Page.BaseURL
	site := struct {
		Context     string `json:"@context"`
		Type        string `json:"@type"`
		ID          string `json:"@id"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		Description string `json:"description"`
	}{
		Context: schema, Type: "WebSite", ID: jsonSiteID(base),
		Name: "MuxMaster", URL: base + "/", Description: in.Page.Description,
	}
	software := struct {
		Context             string `json:"@context"`
		Type                string `json:"@type"`
		ID                  string `json:"@id"`
		Name                string `json:"name"`
		CodeRepository      string `json:"codeRepository"`
		ProgrammingLanguage string `json:"programmingLanguage"`
		License             string `json:"license"`
		Version             string `json:"version"`
		Description         string `json:"description"`
	}{
		Context: schema, Type: "SoftwareSourceCode", ID: jsonSoftwareID(base),
		Name:                "MuxMaster",
		CodeRepository:      "https://github.com/FlavioCFOliveira/MuxMaster",
		ProgrammingLanguage: "Go",
		License:             "https://opensource.org/licenses/MIT",
		Version:             in.Page.Version,
		Description:         in.Page.Description,
	}
	org := struct {
		Context string `json:"@context"`
		Type    string `json:"@type"`
		ID      string `json:"@id"`
		Name    string `json:"name"`
		URL     string `json:"url"`
	}{
		Context: schema, Type: "Organization", ID: jsonOrgID(base),
		Name: "FlavioCFOliveira", URL: "https://github.com/FlavioCFOliveira",
	}
	person := struct {
		Context string `json:"@context"`
		Type    string `json:"@type"`
		ID      string `json:"@id"`
		Name    string `json:"name"`
		URL     string `json:"url"`
	}{
		Context: schema, Type: "Person", ID: jsonAuthorID(base),
		Name: "Flávio Oliveira", URL: "https://github.com/FlavioCFOliveira",
	}
	return []string{mustJSON(site), mustJSON(software), mustJSON(org), mustJSON(person)}
}

// buildLandingJSONLD is the landing-page entry point. The landing page is
// the single emission site for the four reified entity nodes; the helper
// is delegated.
func buildLandingJSONLD(in JSONLDInputs) []string {
	return buildEntityGraph(in)
}

// article graph: TechArticle + BreadcrumbList. /docs/getting-started and a
// few examples additionally emit a HowTo when their source contains
// `## Step N — name` headings; that is layered on by the caller through the
// HowToSource field.
func buildArticleJSONLD(in JSONLDInputs) []string {
	base := in.Page.BaseURL
	canonical := in.Page.Canonical
	dateModified := in.LastModified
	if dateModified.IsZero() {
		dateModified = in.BuildTime
	}
	article := struct {
		Context       string `json:"@context"`
		Type          string `json:"@type"`
		ID            string `json:"@id"`
		Headline      string `json:"headline"`
		Description   string `json:"description"`
		URL           string `json:"url"`
		DatePublished string `json:"datePublished"`
		DateModified  string `json:"dateModified"`
		IsPartOf      idRef  `json:"isPartOf"`
		Author        idRef  `json:"author"`
	}{
		Context: schema, Type: "TechArticle", ID: canonical + "#article",
		Headline: in.Page.Title, Description: in.Page.Description, URL: canonical,
		DatePublished: in.BuildTime.UTC().Format(time.RFC3339),
		DateModified:  dateModified.UTC().Format(time.RFC3339),
		IsPartOf:      idRef{ID: jsonSiteID(base)},
		Author:        idRef{ID: jsonOrgID(base)},
	}
	out := []string{mustJSON(article), breadcrumbJSON(in.Page)}
	if howto := buildHowToJSONLD(in); howto != "" {
		out = append(out, howto)
	}
	return out
}

// collection graph: CollectionPage + BreadcrumbList. Used for the section
// indexes (/docs/, /examples/).
func buildCollectionJSONLD(in JSONLDInputs) []string {
	base := in.Page.BaseURL
	canonical := in.Page.Canonical
	collection := struct {
		Context     string `json:"@context"`
		Type        string `json:"@type"`
		ID          string `json:"@id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
		IsPartOf    idRef  `json:"isPartOf"`
	}{
		Context: schema, Type: "CollectionPage", ID: canonical + "#collection",
		Name: in.Page.Title, Description: in.Page.Description, URL: canonical,
		IsPartOf: idRef{ID: jsonSiteID(base)},
	}
	return []string{mustJSON(collection), breadcrumbJSON(in.Page)}
}

// api graph: TechArticle + BreadcrumbList + a SoftwareSourceCode entry that
// points at the upstream module. The reference SoftwareSourceCode is local
// to /api so it does not collide with the landing page's @id.
func buildAPIJSONLD(in JSONLDInputs) []string {
	out := buildArticleJSONLD(in)
	canonical := in.Page.Canonical
	software := struct {
		Context             string `json:"@context"`
		Type                string `json:"@type"`
		ID                  string `json:"@id"`
		Name                string `json:"name"`
		CodeRepository      string `json:"codeRepository"`
		ProgrammingLanguage string `json:"programmingLanguage"`
		License             string `json:"license"`
		Version             string `json:"version"`
		URL                 string `json:"url"`
	}{
		Context: schema, Type: "SoftwareSourceCode", ID: canonical + "#software",
		Name:                "MuxMaster",
		CodeRepository:      "https://github.com/FlavioCFOliveira/MuxMaster",
		ProgrammingLanguage: "Go",
		License:             "https://opensource.org/licenses/MIT",
		Version:             in.Page.Version,
		URL:                 "https://github.com/FlavioCFOliveira/MuxMaster",
	}
	return append(out, mustJSON(software))
}

// idRef is the JSON-LD "{@id: ...}" shorthand used to reference another
// node in the same graph by its identifier.
type idRef struct {
	ID string `json:"@id"`
}

// breadcrumbJSON returns the BreadcrumbList JSON-LD object for the page's
// breadcrumb trail. Position is 1-indexed per schema.org.
func breadcrumbJSON(p meta.Page) string {
	type item struct {
		Type     string `json:"@type"`
		Position int    `json:"position"`
		Name     string `json:"name"`
		Item     string `json:"item,omitempty"`
	}
	type doc struct {
		Context  string `json:"@context"`
		Type     string `json:"@type"`
		Elements []item `json:"itemListElement"`
	}
	if len(p.Breadcrumbs) == 0 {
		return ""
	}
	els := make([]item, 0, len(p.Breadcrumbs))
	for i, b := range p.Breadcrumbs {
		it := item{Type: "ListItem", Position: i + 1, Name: b.Label}
		if b.Href != "" {
			it.Item = p.BaseURL + b.Href
		} else {
			// Current page: link to its canonical URL.
			it.Item = p.Canonical
		}
		els = append(els, it)
	}
	return mustJSON(doc{Context: schema, Type: "BreadcrumbList", Elements: els})
}

// stepHeadingRE matches "## Step N — name" or "## Step N - name" (en-dash or
// hyphen). The pattern is intentionally strict so we do not invent steps
// from arbitrary ## headings.
var stepHeadingRE = regexp.MustCompile(`(?m)^##\s+Step\s+\d+\s+[—\-]\s+(.+?)\s*$`)

// buildHowToJSONLD scans the supplied Markdown source for step-shaped
// headings and emits a HowTo JSON-LD object. Returns "" when no step
// headings are found — the spec is explicit that we never invent steps.
func buildHowToJSONLD(in JSONLDInputs) string {
	if len(in.HowToSource) == 0 {
		return ""
	}
	matches := stepHeadingRE.FindAllSubmatchIndex(in.HowToSource, -1)
	if len(matches) == 0 {
		return ""
	}
	type step struct {
		Type string `json:"@type"`
		Name string `json:"name"`
		Text string `json:"text"`
	}
	type doc struct {
		Context string `json:"@context"`
		Type    string `json:"@type"`
		Name    string `json:"name"`
		Steps   []step `json:"step"`
	}
	steps := make([]step, 0, len(matches))
	for i, m := range matches {
		// Heading text is capture group 1 (m[2]:m[3]).
		name := strings.TrimSpace(string(in.HowToSource[m[2]:m[3]]))
		// First paragraph after the heading: bytes from end-of-heading-line
		// to either the next blank line or the next step heading.
		bodyStart := m[1]
		bodyEnd := len(in.HowToSource)
		if i+1 < len(matches) {
			bodyEnd = matches[i+1][0]
		}
		text := firstParagraph(in.HowToSource[bodyStart:bodyEnd])
		if text == "" {
			text = name
		}
		steps = append(steps, step{Type: "HowToStep", Name: name, Text: text})
	}
	return mustJSON(doc{Context: schema, Type: "HowTo", Name: in.Page.Title, Steps: steps})
}

// firstParagraph returns the first non-empty paragraph in src. A paragraph
// boundary is a blank line (or a fenced code block — we skip them so the
// HowTo text is prose, not code).
func firstParagraph(src []byte) string {
	lines := bytes.Split(src, []byte("\n"))
	var para []string
	inCode := false
	for _, ln := range lines {
		s := strings.TrimSpace(string(ln))
		if strings.HasPrefix(s, "```") {
			if len(para) > 0 {
				break
			}
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		if s == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		para = append(para, s)
	}
	return strings.Join(para, " ")
}

// mustJSON marshals v and returns the compact representation. Marshalling
// the named structs above never fails (no maps, no channels, no funcs); we
// surface the panic at startup if it ever does so the binary does not ship
// silently broken JSON-LD.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic("render: jsonld marshal: " + err.Error())
	}
	return string(b)
}
