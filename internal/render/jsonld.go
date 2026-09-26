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
	Page          meta.Page
	Family        string         // "landing", "doc-article", "collection", "api", "error"
	BuildTime     time.Time      // process start time, retained for diagnostics; MUST NOT be used as a date substitute (spec/structured-data.md § Date sources for embedded content).
	DatePublished time.Time      // truthful first-publication date sourced from front-matter; zero means omit per the doctrine.
	DateModified  time.Time      // truthful last-modified date sourced from front-matter or git-log build manifest; zero means omit per the doctrine.
	HowToSource   []byte         // optional; if present the generator scans for `## Step N — name` headings
	HasPart       []string       // for "collection" family: canonical absolute URLs of the items the page lists; emitted as CollectionPage.hasPart.
	ItemListItems []ItemListItem // optional; when non-empty the collection family additionally emits an ItemList block with one ListItem per entry. The 1-indexed position is the slice order.
	RenderedHTML  []byte         // optional; when present the FAQPage scanner walks <section data-conversation> regions to extract Q→A pairs into a single flat FAQPage block (spec/geo.md § Question-Oriented Content).
	// AboutSoftware adds TechArticle.about referencing the SoftwareSourceCode
	// entity (spec/structured-data.md master table: /docs/http-query-method,
	// /changelog, /releases/<v>).
	AboutSoftware bool
	// ArticleVersion is TechArticle.version; set on /releases/<v> only.
	ArticleVersion string
}

// ItemListItem is one entry in a collection-page ItemList JSON-LD block.
// schema.org ListItem requires `name` (the human-readable label) and
// `url` (the canonical absolute URL of the item); `description` is
// optional and emitted with omitempty so empty descriptions do not
// produce empty JSON fields.
type ItemListItem struct {
	Name        string
	URL         string
	Description string
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
func BuildJSONLD(in JSONLDInputs) []meta.JSONLDBlock {
	var jsons []string
	switch in.Family {
	case "landing":
		jsons = buildLandingJSONLD(in)
	case "doc-article":
		jsons = buildArticleJSONLD(in)
	case "collection":
		jsons = buildCollectionJSONLD(in)
	case "api":
		jsons = buildAPIJSONLD(in)
	case "benchmarks":
		jsons = buildBenchmarksJSONLD(in)
	default:
		return nil
	}
	out := make([]meta.JSONLDBlock, 0, len(jsons))
	for _, j := range jsons {
		out = append(out, meta.JSONLDBlock{JSON: j})
	}
	// Per spec/structured-data.md § Field completeness, intentionally
	// omitted required-or-recommended fields are recorded as a one-line
	// HTML comment immediately above the relevant <script> tag. The
	// article-shape families omit datePublished / dateModified when they
	// cannot be sourced truthfully from front-matter or the build mani-
	// fest (see § Date sources for embedded content). Attach the audit
	// comment to the first block (the TechArticle) when applicable.
	if (in.Family == "doc-article" || in.Family == "api" || in.Family == "benchmarks") && len(out) > 0 {
		var notes []string
		if in.DatePublished.IsZero() {
			notes = append(notes, "omitted: datePublished on TechArticle — front-matter date not yet authored")
		}
		if in.DateModified.IsZero() {
			notes = append(notes, "omitted: dateModified on TechArticle — neither front-matter nor git history available")
		}
		if len(notes) > 0 {
			out[0].Comment = strings.Join(notes, "; ")
		}
	}
	// The benchmarks Dataset omits license: no licence is declared for the
	// campaign archive in this repository (spec/structured-data.md
	// § Dataset). The audit comment sits above the Dataset block.
	if in.Family == "benchmarks" && len(out) > 0 {
		out[len(out)-1].Comment = "omitted: license on Dataset — no licence is declared for the campaign archive in this repository"
	}
	return out
}

// schema is the JSON-LD context value emitted on every object.
const schema = "https://schema.org"

// UpstreamMinimumGoVersion mirrors the `go` directive in
// ../MuxMaster/go.mod and is surfaced in JSON-LD as
// SoftwareSourceCode.runtimePlatform. The release-manager agent MUST
// update this constant whenever the upstream go.mod bumps the minimum.
// Hard-coded rather than parsed at runtime because go.mod is not
// embedded in this binary; the constant is the build-time mirror.
const UpstreamMinimumGoVersion = "1.27.1"

// softwareRuntimePlatform formats the minimum supported Go version as a
// schema.org runtimePlatform value (e.g. "Go 1.27.1"). Returns "" when the
// caller did not thread a Go version through Deps/Page; the caller emits
// the field with omitempty so an absent value produces no fabrication.
func softwareRuntimePlatform(goVersion string) string {
	if goVersion == "" {
		return ""
	}
	return "Go " + goVersion
}

// SoftwareDescription is SoftwareSourceCode.description on /. Per
// specification/structured-data.md SD-LAND-1 it states the HTTP QUERY
// method support and a performance-positioning statement in words, with
// the categories, handler modes, and measured routers named (INT-PERF-3)
// and no performance numbers (INT-PERF-7). The positioning follows the
// campaign archive reports/benchmarks-2026-09-26/README.md.
const SoftwareDescription = "MuxMaster is a zero-dependency HTTP router for Go with O(k) radix-tree lookups and full net/http compatibility. " +
	"It supports the HTTP QUERY method (RFC 10008). " +
	"In this website's benchmark campaign against httprouter, bunrouter, chi, and gorilla/mux, MuxMaster was the fastest on static, not-found, and parallel static routes in its default mode, " +
	"on 1-parameter routes with HandleFast, and on 3-parameter and parallel 1-parameter routes with PoolRequestBundle; " +
	"httprouter was the fastest on catch-all routes, and MuxMaster's default mode was slower than httprouter on every parameterised route."

// softwareVersion converts the chrome version label ("v1.3.0") to the bare
// semantic version ("1.3.0") used by SoftwareSourceCode.version and
// APIReference.assemblyVersion (spec/structured-data.md field tables).
func softwareVersion(label string) string {
	return strings.TrimPrefix(label, "v")
}

// jsonOrgID, jsonSiteID, jsonSoftwareID, jsonAuthorID are the canonical @id
// values for the four project-level entities reified by
// specification/structured-data.md § Entity graph. Each entity is emitted
// in full only on / (via buildEntityGraph) and referenced by @id from every
// other page. Renaming any of these is governed by the @id migration policy
// in the same spec.
func jsonOrgID(base string) string      { return base + "/#org" }
func jsonSiteID(base string) string     { return base + "/#website" }
func jsonSoftwareID(base string) string { return base + "/#muxmaster" }
func jsonAuthorID(base string) string   { return base + "/#author" }

// jsonLegacySoftwareID is the previous canonical @id for the MuxMaster
// module, retained ONLY so the bridging mechanism in
// specification/structured-data.md § @id migration policy can list it in
// the SoftwareSourceCode node's sameAs array during the 90-day transition
// window. After the window ends, this constant and its usage MUST be
// deleted (rmp follow-up task to be created at the end of the window).
func jsonLegacySoftwareID(base string) string { return base + "/#software" }

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
		InLanguage  string `json:"inLanguage"`
		Publisher   idRef  `json:"publisher"`
	}{
		Context: schema, Type: "WebSite", ID: jsonSiteID(base),
		Name:        "MuxMaster",
		URL:         base + "/",
		Description: in.Page.Description,
		InLanguage:  "en",
		Publisher:   idRef{ID: jsonOrgID(base)},
	}
	type targetProductT struct {
		Type                string `json:"@type"`
		ApplicationCategory string `json:"applicationCategory"`
	}
	software := struct {
		Context             string         `json:"@context"`
		Type                string         `json:"@type"`
		ID                  string         `json:"@id"`
		Name                string         `json:"name"`
		CodeRepository      string         `json:"codeRepository"`
		ProgrammingLanguage string         `json:"programmingLanguage"`
		License             string         `json:"license"`
		Version             string         `json:"version"`
		Description         string         `json:"description"`
		RuntimePlatform     string         `json:"runtimePlatform,omitempty"`
		TargetProduct       targetProductT `json:"targetProduct"`
		SameAs              []string       `json:"sameAs,omitempty"`
	}{
		Context: schema, Type: "SoftwareSourceCode", ID: jsonSoftwareID(base),
		Name:                "MuxMaster",
		CodeRepository:      "https://github.com/FlavioCFOliveira/MuxMaster",
		ProgrammingLanguage: "Go",
		License:             "https://opensource.org/licenses/MIT",
		Version:             softwareVersion(in.Page.Version),
		Description:         SoftwareDescription,
		RuntimePlatform:     softwareRuntimePlatform(in.Page.GoVersion),
		TargetProduct: targetProductT{
			Type:                "SoftwareApplication",
			ApplicationCategory: "DeveloperApplication",
		},
		// sameAs carries the legacy @id during the migration window
		// (per task #23 + the @id migration policy in spec/structured-
		// data.md), plus the authoritative third-party identity URLs
		// for the MuxMaster module.
		SameAs: []string{
			"https://github.com/FlavioCFOliveira/MuxMaster",
			"https://pkg.go.dev/github.com/FlavioCFOliveira/MuxMaster",
			jsonLegacySoftwareID(base),
		},
	}
	// Organization.logo is emitted as a structured ImageObject (not a bare
	// URL string) so that consumers — Google's Organization rich-result
	// pipeline, AI ingestion graphs, schema.org validators — receive the
	// width and height up-front without fetching the binary. Google's
	// Organization documentation prefers the ImageObject form whenever the
	// aspect ratio is determinate, which it is here (the 384x384 PNG is
	// generated deterministically from the canonical logo source by
	// tools/imagegen). The bare-URL form remains valid schema.org, but the
	// structured form is strictly more useful and equally cheap.
	type imageObject struct {
		Type       string `json:"@type"`
		URL        string `json:"url"`
		ContentURL string `json:"contentUrl"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
	}
	org := struct {
		Context string      `json:"@context"`
		Type    string      `json:"@type"`
		ID      string      `json:"@id"`
		Name    string      `json:"name"`
		URL     string      `json:"url"`
		Logo    imageObject `json:"logo"`
		SameAs  []string    `json:"sameAs,omitempty"`
	}{
		Context: schema, Type: "Organization", ID: jsonOrgID(base),
		Name: "FlavioCFOliveira",
		URL:  "https://github.com/FlavioCFOliveira",
		Logo: imageObject{
			Type:       "ImageObject",
			URL:        base + "/static/img/logo-384.png",
			ContentURL: base + "/static/img/logo-384.png",
			Width:      384,
			Height:     384,
		},
		// Authoritative third-party identity URL for the publisher.
		// Additional entries (project page, public technical blog) are
		// added here only as they become verifiable; never invented.
		SameAs: []string{"https://github.com/FlavioCFOliveira"},
	}
	person := struct {
		Context string   `json:"@context"`
		Type    string   `json:"@type"`
		ID      string   `json:"@id"`
		Name    string   `json:"name"`
		URL     string   `json:"url"`
		SameAs  []string `json:"sameAs,omitempty"`
	}{
		Context: schema, Type: "Person", ID: jsonAuthorID(base),
		Name: "Flávio Oliveira",
		URL:  "https://github.com/FlavioCFOliveira",
		// Authoritative third-party profiles only (per
		// specification/structured-data.md § Required field-by-type
		// expectations / Person and the audit's no-social-only rule).
		// GitHub is the maintainer's primary identity profile; additional
		// authoritative entries (project page, public technical blog) are
		// added here as they become available — never invented.
		SameAs: []string{"https://github.com/FlavioCFOliveira"},
	}
	return []string{mustJSON(site), mustJSON(software), mustJSON(org), mustJSON(person)}
}

// buildLandingJSONLD is the landing-page entry point. The landing page is
// the single emission site for the four reified entity nodes (delegated
// to buildEntityGraph) and additionally carries a FAQPage block whose
// Q→A pairs answer the questions an AI ingestion pipeline is most
// likely to receive about MuxMaster from a cold prompt. The FAQ HTML in
// templates/pages/landing.html and the FAQ JSON-LD here MUST be kept in
// sync; TestLandingFAQHTMLAndJSONLDAgree in internal/server enforces the
// invariant by comparing the question and answer lists.
func buildLandingJSONLD(in JSONLDInputs) []string {
	out := buildEntityGraph(in)
	if faq := buildLandingFAQPageJSONLD(in); faq != "" {
		out = append(out, faq)
	}
	return out
}

// landingFAQEntries is the canonical list of homepage FAQ pairs. The HTML
// in templates/pages/landing.html duplicates the same question text and
// answer text inside <section data-conversation="landing-faq">.
//
// Each answer is written with the same inline formatting (<code>, <a href>)
// that the rendered page shows; buildLandingFAQPageJSONLD reduces it to
// plain text with faqPlainText before emission, because FAQPage answer
// text MUST carry no HTML markup (specification/structured-data.md
// SD-TEXT-1).
var landingFAQEntries = []struct {
	Q, A string
}{
	{
		Q: "What is MuxMaster?",
		A: `MuxMaster is a zero-dependency HTTP router for Go that uses a radix tree for O(k) route lookups and keeps full compatibility with the <code>net/http</code> <code>Handler</code> interface. Static routes allocate nothing, parameterised routes make one allocation by default or none with the opt-in <code>PoolRequestBundle</code>, and the <code>middleware</code> package provides 21 middleware constructors.`,
	},
	{
		Q: "What Go version does MuxMaster require?",
		A: `MuxMaster {version} requires Go {go} or later, as declared by the <code>go</code> directive in its <code>go.mod</code>. With the default <code>GOTOOLCHAIN=auto</code>, an older Go toolchain switches to Go {go} or newer automatically. See <a href="{base}/compatibility">Compatibility</a> for the version policy.`,
	},
	{
		Q: "Does MuxMaster support the HTTP QUERY method?",
		A: `Yes: MuxMaster has supported the HTTP QUERY method defined in RFC 10008 since v1.2.0, through <code>MethodQuery</code> and the <code>QUERY</code>, <code>QUERYE</code>, and <code>QUERYFast</code> registration methods. The <a href="{base}/docs/http-query-method">HTTP QUERY method (RFC 10008)</a> guide shows how to register a QUERY route, validate its request body, and call it with <code>curl</code>.`,
	},
	{
		Q: "Is MuxMaster compatible with net/http?",
		A: `Yes: a <code>*muxmaster.Mux</code> implements <code>http.Handler</code>, handlers use the <code>http.HandlerFunc</code> signature, and middleware uses <code>func(http.Handler) http.Handler</code>. Any code that accepts an <code>http.Handler</code>, such as <code>http.Server</code> or <code>httptest.NewServer</code>, accepts the router, so adoption can be incremental.`,
	},
	{
		Q: "How fast is MuxMaster compared with other routers?",
		A: `In this website's 2026-09-26 benchmark campaign, MuxMaster v1.3.0 was the fastest of five measured Go routers (MuxMaster, httprouter, bunrouter, chi, and gorilla/mux) in six of the eight route categories of the upstream competitor suite, and httprouter was the fastest on catch-all routes. MuxMaster led on static, not-found, and parallel static routes in its default mode, on 1-parameter routes with <code>HandleFast</code>, and on 3-parameter and parallel 1-parameter routes with <code>PoolRequestBundle</code>; on 2-parameter routes, MuxMaster with <code>PoolRequestBundle</code> and httprouter showed no significant difference. MuxMaster's default mode was slower than httprouter on every parameterised route. The full data, host, and method are on the <a href="{base}/benchmarks">Benchmarks</a> page.`,
	},
	{
		Q: "What is the catch with PoolRequestBundle?",
		A: `<code>PoolRequestBundle</code> recycles the per-request bundle through <code>sync.Pool</code>, so handlers must not retain <code>*http.Request</code> after they return, for example in a goroutine that outlives the handler. Reverse proxies built on <code>net/http.Transport</code> and handlers that hijack the connection must keep the pool off. The <a href="{base}/docs/max-performance">Maximum performance guide</a> lists the rules and an audit checklist.`,
	},
	{
		Q: "What is MuxMaster's license?",
		A: `MuxMaster is released under the MIT License. The full text is in the upstream repository at <a href="https://github.com/FlavioCFOliveira/MuxMaster/blob/main/LICENSE">LICENSE</a>.`,
	},
}

// buildLandingFAQPageJSONLD emits the homepage FAQPage block. Returns the
// JSON string ready to embed; never empty (landingFAQEntries is a
// compile-time constant with seven entries, well above the FAQPage minimum
// of three).
func buildLandingFAQPageJSONLD(in JSONLDInputs) string {
	base := in.Page.BaseURL
	type answerT struct {
		Type string `json:"@type"`
		Text string `json:"text"`
	}
	type questionT struct {
		Type           string  `json:"@type"`
		Name           string  `json:"name"`
		AcceptedAnswer answerT `json:"acceptedAnswer"`
	}
	// {base}, {version}, and {go} are expanded from the page so that the
	// answers state the same version facts as the rendered chrome.
	expand := strings.NewReplacer("{base}", base, "{version}", in.Page.Version, "{go}", in.Page.GoVersion)
	mainEntity := make([]questionT, 0, len(landingFAQEntries))
	for _, e := range landingFAQEntries {
		mainEntity = append(mainEntity, questionT{
			Type:           "Question",
			Name:           e.Q,
			AcceptedAnswer: answerT{Type: "Answer", Text: faqPlainText([]byte(expand.Replace(e.A)))},
		})
	}
	faq := struct {
		Context    string      `json:"@context"`
		Type       string      `json:"@type"`
		ID         string      `json:"@id"`
		IsPartOf   idRef       `json:"isPartOf"`
		MainEntity []questionT `json:"mainEntity"`
	}{
		Context:    schema,
		Type:       "FAQPage",
		ID:         base + "/#faq",
		IsPartOf:   idRef{ID: jsonSiteID(base)},
		MainEntity: mainEntity,
	}
	return mustJSON(faq)
}

// article graph: TechArticle + BreadcrumbList. /docs/getting-started and a
// few examples additionally emit a HowTo when their source contains
// `## Step N — name` headings; that is layered on by the caller through the
// HowToSource field.
func buildArticleJSONLD(in JSONLDInputs) []string {
	base := in.Page.BaseURL
	canonical := in.Page.Canonical
	article := struct {
		Context          string `json:"@context"`
		Type             string `json:"@type"`
		ID               string `json:"@id"`
		Headline         string `json:"headline"`
		Description      string `json:"description"`
		URL              string `json:"url"`
		InLanguage       string `json:"inLanguage"`
		DatePublished    string `json:"datePublished,omitempty"`
		DateModified     string `json:"dateModified,omitempty"`
		MainEntityOfPage string `json:"mainEntityOfPage"`
		IsPartOf         idRef  `json:"isPartOf"`
		Author           idRef  `json:"author"`
		Publisher        idRef  `json:"publisher"`
		About            *idRef `json:"about,omitempty"`
		Version          string `json:"version,omitempty"`
	}{
		Context: schema, Type: "TechArticle", ID: canonical + "#article",
		Headline: in.Page.Title, Description: in.Page.Description, URL: canonical,
		InLanguage: "en",
		// datePublished and dateModified are populated only when truthful
		// values are available (front-matter or git-log build manifest).
		// Per spec/structured-data.md § Field completeness, fabricated or
		// build-time substitutes are forbidden: missing values are omitted
		// (omitempty) and the HTML-comment audit trail is attached by
		// BuildJSONLD.
		DatePublished:    formatRFC3339OrEmpty(in.DatePublished),
		DateModified:     formatRFC3339OrEmpty(in.DateModified),
		MainEntityOfPage: canonical,
		IsPartOf:         idRef{ID: jsonSiteID(base)},
		// Author is the Person entity (per spec/structured-data.md
		// § TechArticle table); previously mis-wired to Organization@id.
		Author:    idRef{ID: jsonAuthorID(base)},
		Publisher: idRef{ID: jsonOrgID(base)},
		Version:   in.ArticleVersion,
	}
	if in.AboutSoftware {
		article.About = &idRef{ID: jsonSoftwareID(base)}
	}
	out := []string{mustJSON(article), breadcrumbJSON(in.Page)}
	if howto := buildHowToJSONLD(in); howto != "" {
		out = append(out, howto)
	}
	if faq := buildFAQPageJSONLD(in); faq != "" {
		out = append(out, faq)
	}
	if dts := buildDefinedTermSetJSONLD(in); dts != "" {
		out = append(out, dts)
	}
	if codes := buildCodeSnippetsJSONLD(in); len(codes) > 0 {
		out = append(out, codes...)
	}
	return out
}

func formatRFC3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// collection graph: CollectionPage + BreadcrumbList. Used for the section
// indexes (/docs/, /examples/).
func buildCollectionJSONLD(in JSONLDInputs) []string {
	base := in.Page.BaseURL
	canonical := in.Page.Canonical
	hasPart := make([]idRef, 0, len(in.HasPart))
	for _, u := range in.HasPart {
		hasPart = append(hasPart, idRef{ID: u})
	}
	collection := struct {
		Context     string  `json:"@context"`
		Type        string  `json:"@type"`
		ID          string  `json:"@id"`
		Name        string  `json:"name"`
		Description string  `json:"description"`
		URL         string  `json:"url"`
		InLanguage  string  `json:"inLanguage"`
		IsPartOf    idRef   `json:"isPartOf"`
		Publisher   idRef   `json:"publisher"`
		HasPart     []idRef `json:"hasPart,omitempty"`
	}{
		Context: schema, Type: "CollectionPage", ID: canonical + "#collection",
		Name: in.Page.Title, Description: in.Page.Description, URL: canonical,
		InLanguage: "en",
		IsPartOf:   idRef{ID: jsonSiteID(base)},
		Publisher:  idRef{ID: jsonOrgID(base)},
		HasPart:    hasPart,
	}
	out := []string{mustJSON(collection), breadcrumbJSON(in.Page)}
	if itemList := buildItemListJSONLD(in); itemList != "" {
		out = append(out, itemList)
	}
	return out
}

// buildItemListJSONLD emits a schema.org ItemList block for a collection
// page. Returns "" when in.ItemListItems is empty. The 1-indexed
// position field is the canonical ListItem ordering signal that Google's
// rich-result pipeline and most AI ingestion engines key on; without it
// the items are treated as an unordered set, which loses the curated
// learning sequence of /examples/ (REST → auth → operational).
func buildItemListJSONLD(in JSONLDInputs) string {
	if len(in.ItemListItems) == 0 {
		return ""
	}
	canonical := in.Page.Canonical
	type listItem struct {
		Type        string `json:"@type"`
		Position    int    `json:"position"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		Description string `json:"description,omitempty"`
	}
	items := make([]listItem, 0, len(in.ItemListItems))
	for i, it := range in.ItemListItems {
		items = append(items, listItem{
			Type:        "ListItem",
			Position:    i + 1,
			Name:        it.Name,
			URL:         it.URL,
			Description: it.Description,
		})
	}
	list := struct {
		Context         string     `json:"@context"`
		Type            string     `json:"@type"`
		ID              string     `json:"@id"`
		Name            string     `json:"name"`
		Description     string     `json:"description"`
		URL             string     `json:"url"`
		NumberOfItems   int        `json:"numberOfItems"`
		ItemListOrder   string     `json:"itemListOrder"`
		ItemListElement []listItem `json:"itemListElement"`
	}{
		Context:         schema,
		Type:            "ItemList",
		ID:              canonical + "#list",
		Name:            in.Page.Title,
		Description:     in.Page.Description,
		URL:             canonical,
		NumberOfItems:   len(items),
		ItemListOrder:   "https://schema.org/ItemListOrderAscending",
		ItemListElement: items,
	}
	return mustJSON(list)
}

// api graph: TechArticle + BreadcrumbList + APIReference. /api references
// the SoftwareSourceCode entity by @id (the canonical /#muxmaster node
// emitted in full only on /).
//
// Per spec/structured-data.md § Non-negotiables, this function MUST NOT
// emit an inline SoftwareSourceCode redefinition — the helper that used
// to live here was deleted because it minted a duplicate /api#software
// @id, fragmenting the citation graph for AI engines.
func buildAPIJSONLD(in JSONLDInputs) []string {
	out := buildArticleJSONLD(in)
	base := in.Page.BaseURL
	canonical := in.Page.Canonical
	if dts := buildAPIDefinedTermSetJSONLD(in); dts != "" {
		out = append(out, dts)
	}
	apiRef := struct {
		Context               string `json:"@context"`
		Type                  string `json:"@type"`
		ID                    string `json:"@id"`
		Name                  string `json:"name"`
		Description           string `json:"description"`
		URL                   string `json:"url"`
		TargetPlatform        string `json:"targetPlatform"`
		ProgrammingModel      string `json:"programmingModel"`
		ExecutableLibraryName string `json:"executableLibraryName"`
		AssemblyVersion       string `json:"assemblyVersion,omitempty"`
		About                 idRef  `json:"about"`
	}{
		Context:               schema,
		Type:                  "APIReference",
		ID:                    canonical + "#apiref",
		Name:                  in.Page.Title,
		Description:           in.Page.Description,
		URL:                   canonical,
		TargetPlatform:        "Go",
		ProgrammingModel:      "HTTP request multiplexer (radix-tree, net/http-compatible)",
		ExecutableLibraryName: "github.com/FlavioCFOliveira/MuxMaster",
		AssemblyVersion:       softwareVersion(in.Page.Version),
		About:                 idRef{ID: jsonSoftwareID(base)},
	}
	return append(out, mustJSON(apiRef))
}

// apiDefinedTerms is the curated list of MuxMaster public-API symbols
// surfaced as schema.org DefinedTerm entries on /api. Each entry pairs a
// canonical fully-qualified symbol name with a one-sentence description
// that an AI ingestion pipeline can quote verbatim.
//
// The list is curated rather than auto-extracted from content/api.md
// because the source is rendered `go doc` plain text — multi-paragraph
// descriptions with nested code blocks, indented elaboration, and
// embedded examples — which does not map cleanly onto the
// DefinedTerm.description field (which is a single short string). A
// faithful auto-extraction would either truncate descriptions
// arbitrarily or emit DefinedTerm objects whose bodies contain code
// fences and example wrappers, neither of which is useful to a
// downstream consumer. Curated entries are deliberately short,
// fact-shaped, and answer the question "what is this symbol for" in one
// sentence. When upstream MuxMaster adds or renames a symbol the
// release-manager agent MUST update this list and run the JSON-LD
// validation gate.
var apiDefinedTerms = []struct {
	Name        string
	Description string
}{
	// Top-level package (github.com/FlavioCFOliveira/MuxMaster)
	{"muxmaster.New", "Constructor returning a fresh *Mux ready for route registration."},
	{"muxmaster.Mux", "Radix-tree HTTP request multiplexer; implements http.Handler so any net/http infrastructure that accepts a handler accepts a *Mux."},
	{"muxmaster.Group", "Sub-router sharing a path prefix and middleware stack with its parent; created via Mux.Group and nestable via Group.Group."},
	{"muxmaster.Params", "Slice of (Name, Value) pairs holding the parameters captured by a matched route. Returned by ParamsFromContext."},
	{"muxmaster.PathParam", "Reads a single named path parameter from a request as a string; equivalent to ParamsFromContext(r.Context()).Get(name)."},
	{"muxmaster.ParamsFromContext", "Extracts the full Params slice from a request context, exposing the typed helpers Int, Bool, UUID, and Float."},
	{"muxmaster.RoutePattern", "Returns the registered route pattern that matched the request, or \"\" if none is stored in the context."},
	{"muxmaster.HandlerFuncE", "Handler signature that returns an error; threaded through middleware and surfaced via Mux.ErrorHandler."},
	{"muxmaster.HTTPError", "Typed error carrying an HTTP status code and a public-facing message; returned by HandlerFuncE handlers."},
	{"muxmaster.FastHandler", "High-performance request handler that receives parameters as a direct argument, bypassing the context allocation overhead of http.Handler routes."},
	{"muxmaster.FastMiddleware", "Middleware wrapping a FastHandler; composes like stdlib middleware but for FastHandler routes only."},
	{"muxmaster.JSON", "Marshals a value to JSON and writes it with the given status code; sets Content-Type: application/json."},
	{"muxmaster.Text", "Writes a string as plain text with the given status code; sets Content-Type: text/plain; charset=utf-8."},
	{"muxmaster.XML", "Marshals a value to XML and writes it with the given status code; sets Content-Type: application/xml."},
	{"muxmaster.NoContent", "Writes a 204 No Content response with no body."},
	{"muxmaster.Redirect", "Sends an HTTP redirect to a target URL with the given status code; path-only to prevent off-site smuggling."},
	{"muxmaster.ServeFiles", "Serves a directory tree with full conditional-GET (304 via ETag and Last-Modified) and range-request (206) semantics."},
	// Middleware sub-package (github.com/FlavioCFOliveira/MuxMaster/middleware)
	{"middleware.RequestID", "Assigns a unique ID to every request and exposes it via context for log correlation; uses the inbound X-Request-Id header when trusted."},
	{"middleware.Recoverer", "Catches panics from downstream handlers, logs the stack trace, and emits a 500 response so a single bad handler does not crash the process."},
	{"middleware.Logger", "Structured access-log middleware emitting one log line per completed request with method, path, status, bytes, and duration."},
	{"middleware.Compress", "Negotiates Content-Encoding with the client and compresses the response body via gzip (or brotli when the build supports it) above a configurable size threshold."},
	{"middleware.RealIP", "Rewrites r.RemoteAddr from X-Forwarded-For when the immediate peer is in the supplied CIDR allowlist; ignores the header otherwise."},
	{"middleware.Timeout", "Wraps every handler in a per-request deadline using context.WithTimeout; emits 503 when the deadline fires before the handler returns."},
	{"middleware.Throttle", "Limits the request rate per key (IP, header, custom selector) using a token-bucket implementation; rejects excess requests with 429."},
	{"middleware.BasicAuth", "RFC 7617 HTTP Basic Authentication with a constant-time credential comparison and a configurable realm."},
	{"middleware.JWTAuth", "Bearer-token authentication for HS256 / RS256 JWTs; requires the exp claim (RFC 8725 §4.4) by default."},
	{"middleware.OAuth2Introspect", "RFC 7662 OAuth 2.0 token introspection against an authorisation server; caches positive responses with a configurable TTL."},
	{"middleware.APIKey", "Validates a static API key passed via the X-API-Key header against a constant-time-compared allowlist."},
	{"middleware.CORS", "Implements the W3C CORS preflight protocol with configurable origins, methods, headers, exposed headers, max-age, and credentials handling."},
}

// buildAPIDefinedTermSetJSONLD emits the curated DefinedTermSet for the
// /api page. The set lists every entry in apiDefinedTerms as a
// DefinedTerm; the parent DefinedTermSet is referenced by canonical#api.
// Returns "" only when the apiDefinedTerms slice is empty (it is
// compile-time non-empty, so this never returns "" in practice; the
// guard exists to match the pattern used by buildFAQPageJSONLD and
// buildDefinedTermSetJSONLD).
func buildAPIDefinedTermSetJSONLD(in JSONLDInputs) string {
	if len(apiDefinedTerms) == 0 {
		return ""
	}
	type term struct {
		Type        string `json:"@type"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	terms := make([]term, 0, len(apiDefinedTerms))
	for _, t := range apiDefinedTerms {
		terms = append(terms, term{Type: "DefinedTerm", Name: t.Name, Description: t.Description})
	}
	canonical := in.Page.Canonical
	set := struct {
		Context        string `json:"@context"`
		Type           string `json:"@type"`
		ID             string `json:"@id"`
		Name           string `json:"name"`
		Description    string `json:"description"`
		HasDefinedTerm []term `json:"hasDefinedTerm"`
	}{
		Context:        schema,
		Type:           "DefinedTermSet",
		ID:             canonical + "#defined-terms",
		Name:           "MuxMaster public API surface",
		Description:    "Curated list of MuxMaster public-API symbols (functions, types, methods, middleware constructors), each with a one-sentence description an AI ingestion pipeline can quote verbatim.",
		HasDefinedTerm: terms,
	}
	return mustJSON(set)
}

// CampaignArchiveCommit is the full commit SHA of this repository that
// contains the benchmark campaign archive. Every link into the archive
// (Dataset.distribution on /benchmarks, the /benchmarks "Source" link) is
// pinned to it (specification/url-and-versioning.md URL-EXT-1,
// specification/structured-data.md SD-BENCH-1).
//
// "26abbe6c1cf2f4c9c16af45f4c02377a685f352d" is a deliberate placeholder, identical to the one in
// content/benchmarks.md: the SHA is known only after the archive is
// committed. Replace this constant and the occurrences in
// content/benchmarks.md with the 40-hex commit SHA before release.
const CampaignArchiveCommit = "26abbe6c1cf2f4c9c16af45f4c02377a685f352d"

// CampaignArchiveDir is the repository-relative directory of the current
// benchmark campaign archive (content-sources.md CS-BENCH-1).
const CampaignArchiveDir = "reports/benchmarks-2026-09-26"

// campaignDate is the date the campaign's measurements were taken, from
// the host facts in CampaignArchiveDir/README.md; it is the Dataset's
// temporalCoverage.
const campaignDate = "2026-09-26"

// campaignFiles lists the raw go test and benchstat output files of the
// campaign archive published as Dataset.distribution. Historical
// v1.1.0-era upstream data is deliberately absent (SD-BENCH-1).
var campaignFiles = []string{
	"raw/root-v110.txt",
	"raw/root-v130.txt",
	"raw/root-v130aa.txt",
	"raw/pa-v110-clean.txt",
	"raw/pa-v130-clean.txt",
	"raw/middleware-v130.txt",
	"raw/wastehunt-v130.txt",
	"raw/competitor-v130.txt",
	"raw/competitor-v130-by-router.txt",
	"benchstat/root-v110-vs-v130.txt",
	"benchstat/root-v130-aa-noise-floor.txt",
	"benchstat/perfaudit-v110-vs-v130.txt",
	"benchstat/middleware-v130.txt",
	"benchstat/wastehunt-v130.txt",
	"benchstat/competitor-v130.txt",
	"benchstat/competitor-v130-vs-MuxMaster.txt",
	"benchstat/competitor-v130-vs-MuxMasterPooled.txt",
	"benchstat/competitor-v130-vs-MuxMasterFast.txt",
	"benchstat/competitor-v130-vs-HTTProuter.txt",
}

// CampaignArchiveURL is the GitHub tree URL of the campaign archive,
// pinned to CampaignArchiveCommit.
func CampaignArchiveURL() string {
	return "https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/" + CampaignArchiveCommit + "/" + CampaignArchiveDir
}

// campaignFileURL is the raw download URL of one archive file, pinned to
// CampaignArchiveCommit.
func campaignFileURL(rel string) string {
	return "https://raw.githubusercontent.com/FlavioCFOliveira/MuxMasterWebsite/" + CampaignArchiveCommit + "/" + CampaignArchiveDir + "/" + rel
}

// benchmarks graph: TechArticle + BreadcrumbList + Dataset. The Dataset
// describes the current campaign results only (SD-BENCH-1): its
// distribution lists the raw files of the campaign archive.
func buildBenchmarksJSONLD(in JSONLDInputs) []string {
	out := buildArticleJSONLD(in)
	base := in.Page.BaseURL
	canonical := in.Page.Canonical
	type distribution struct {
		Type           string `json:"@type"`
		Name           string `json:"name"`
		EncodingFormat string `json:"encodingFormat"`
		ContentURL     string `json:"contentUrl"`
	}
	type variable struct {
		Type        string `json:"@type"`
		Name        string `json:"name"`
		Description string `json:"description"`
		UnitText    string `json:"unitText"`
	}
	dist := make([]distribution, 0, len(campaignFiles))
	for _, f := range campaignFiles {
		dist = append(dist, distribution{
			Type:           "DataDownload",
			Name:           CampaignArchiveDir + "/" + f,
			EncodingFormat: "text/plain",
			ContentURL:     campaignFileURL(f),
		})
	}
	// license is omitted: no licence is declared for the campaign archive
	// in this repository. BuildJSONLD attaches the audit-trail comment.
	dataset := struct {
		Context          string     `json:"@context"`
		Type             string     `json:"@type"`
		ID               string     `json:"@id"`
		Name             string     `json:"name"`
		Description      string     `json:"description"`
		URL              string     `json:"url"`
		InLanguage       string     `json:"inLanguage"`
		Creator          idRef      `json:"creator"`
		TemporalCoverage string     `json:"temporalCoverage"`
		VariableMeasured []variable `json:"variableMeasured"`
		// SD-BENCH-2 fields.
		MeasurementTechnique string         `json:"measurementTechnique"`
		Keywords             []string       `json:"keywords"`
		IsAccessibleForFree  bool           `json:"isAccessibleForFree"`
		Distribution         []distribution `json:"distribution"`
	}{
		Context:          schema,
		Type:             "Dataset",
		ID:               canonical + "#dataset",
		Name:             "MuxMaster v1.3.0 router benchmark campaign, " + campaignDate,
		Description:      in.Page.Description,
		URL:              canonical,
		InLanguage:       "en",
		Creator:          idRef{ID: jsonOrgID(base)},
		TemporalCoverage: campaignDate,
		VariableMeasured: []variable{
			{Type: "PropertyValue", Name: "ns/op", Description: "Nanoseconds per operation; lower is better.", UnitText: "ns"},
			{Type: "PropertyValue", Name: "B/op", Description: "Bytes allocated per operation; lower is better.", UnitText: "B"},
			{Type: "PropertyValue", Name: "allocs/op", Description: "Heap allocations per operation; lower is better.", UnitText: "allocations"},
		},
		MeasurementTechnique: "go test -bench, -count=10, benchstat, alpha = 0.05",
		Keywords:             []string{"MuxMaster", "Go", "HTTP router", "benchmark", "httprouter", "bunrouter", "chi", "gorilla/mux"},
		IsAccessibleForFree:  true,
		Distribution:         dist,
	}
	return append(out, mustJSON(dataset))
}

// idRef is the JSON-LD "{@id: ...}" shorthand used to reference another
// node in the same graph by its identifier.
type idRef struct {
	ID string `json:"@id"`
}

// breadcrumbJSON returns the BreadcrumbList JSON-LD object for the page's
// breadcrumb trail. Position is 1-indexed per schema.org. Each element's
// item is an object {@id, name}, per spec/structured-data.md
// § BreadcrumbList — the visible name lives at item.name; the bare URL
// string form (deprecated by Google's rich-result documentation) is not
// emitted.
func breadcrumbJSON(p meta.Page) string {
	type itemRef struct {
		ID   string `json:"@id"`
		Name string `json:"name"`
	}
	type element struct {
		Type     string  `json:"@type"`
		Position int     `json:"position"`
		Item     itemRef `json:"item"`
	}
	type doc struct {
		Context  string    `json:"@context"`
		Type     string    `json:"@type"`
		Elements []element `json:"itemListElement"`
	}
	if len(p.Breadcrumbs) == 0 {
		return ""
	}
	els := make([]element, 0, len(p.Breadcrumbs))
	for i, b := range p.Breadcrumbs {
		var url string
		if b.Href != "" {
			url = p.BaseURL + b.Href
		} else {
			// Current page: anchor on the canonical URL.
			url = p.Canonical
		}
		els = append(els, element{
			Type:     "ListItem",
			Position: i + 1,
			Item:     itemRef{ID: url, Name: b.Label},
		})
	}
	return mustJSON(doc{Context: schema, Type: "BreadcrumbList", Elements: els})
}

// conversationSectionRE matches a <section data-conversation="..."> ... </section>
// region in rendered HTML. Multi-line, non-greedy: a page may contain
// several chains and the scanner walks all of them.
var conversationSectionRE = regexp.MustCompile(`(?is)<section\s+[^>]*\bdata-conversation\s*=\s*"[^"]*"[^>]*>(.*?)</section>`)

// definitionListRE matches a <dl>...</dl> region in the rendered HTML;
// the scanner walks each block to extract <dt> + following <dd> pairs
// into a DefinedTermSet (spec/structured-data.md § Auxiliary schemas).
var definitionListRE = regexp.MustCompile(`(?is)<dl\b[^>]*>(.*?)</dl>`)

// dtOpenRE matches the opening of a <dt> tag inside a <dl> body. The
// scanner uses positional walking (RE2 has no backreferences) to slice
// each <dt>...</dt><dd>...</dd> pair.
var dtOpenRE = regexp.MustCompile(`(?is)<dt\b[^>]*>`)

// buildDefinedTermSetJSONLD scans the rendered HTML for <dl>...</dl>
// regions and emits a single DefinedTermSet block listing every
// <dt>/<dd> pair as a DefinedTerm. Returns "" when no definition list
// is present or the list contains no usable pairs.
func buildDefinedTermSetJSONLD(in JSONLDInputs) string {
	if len(in.RenderedHTML) == 0 {
		return ""
	}
	type term struct {
		Type        string `json:"@type"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	type set struct {
		Context        string `json:"@context"`
		Type           string `json:"@type"`
		ID             string `json:"@id"`
		Name           string `json:"name"`
		HasDefinedTerm []term `json:"hasDefinedTerm"`
	}
	var terms []term
	for _, dl := range definitionListRE.FindAllSubmatch(in.RenderedHTML, -1) {
		body := dl[1]
		opens := dtOpenRE.FindAllSubmatchIndex(body, -1)
		for i, om := range opens {
			afterOpen := om[1]
			closeIdx := bytes.Index(body[afterOpen:], []byte("</dt>"))
			if closeIdx < 0 {
				continue
			}
			name := faqPlainText(body[afterOpen : afterOpen+closeIdx])
			defStart := afterOpen + closeIdx + len("</dt>")
			defEnd := len(body)
			if i+1 < len(opens) {
				defEnd = opens[i+1][0]
			}
			description := faqPlainText(body[defStart:defEnd])
			if name == "" || description == "" {
				continue
			}
			terms = append(terms, term{Type: "DefinedTerm", Name: name, Description: description})
		}
	}
	if len(terms) == 0 {
		return ""
	}
	return mustJSON(set{
		Context:        schema,
		Type:           "DefinedTermSet",
		ID:             in.Page.Canonical + "#defined-terms",
		Name:           in.Page.Title + " — defined terms",
		HasDefinedTerm: terms,
	})
}

// goCodeBlockRE matches every Goldmark-rendered Go code block. The
// pattern is anchored on language-go so non-Go fences (bash, json,
// http) do not produce noisy Code emissions.
var goCodeBlockRE = regexp.MustCompile(`(?is)<pre><code class="language-go">(.*?)</code></pre>`)

// anchoredHeadingRE matches a heading with an id attribute. The scanner
// uses these anchors as the name of the Code snippet they precede.
var anchoredHeadingRE = regexp.MustCompile(`(?is)<(h[2-6])\b[^>]*\bid="([^"]+)"[^>]*>(.*?)</h[2-6]>`)

// buildCodeSnippetsJSONLD scans the rendered HTML for Go code blocks
// that follow an anchored heading and emits one Code block per snippet.
// Snippets without a preceding anchored heading are skipped — the
// schema requires a stable, citable anchor (spec/structured-data.md
// § Auxiliary schemas).
func buildCodeSnippetsJSONLD(in JSONLDInputs) []string {
	if len(in.RenderedHTML) == 0 {
		return nil
	}
	type code struct {
		Context             string `json:"@context"`
		Type                string `json:"@type"`
		ID                  string `json:"@id"`
		Name                string `json:"name"`
		ProgrammingLanguage string `json:"programmingLanguage"`
		CodeSampleType      string `json:"codeSampleType"`
		Text                string `json:"text"`
	}
	headings := anchoredHeadingRE.FindAllSubmatchIndex(in.RenderedHTML, -1)
	if len(headings) == 0 {
		return nil
	}
	codeBlocks := goCodeBlockRE.FindAllSubmatchIndex(in.RenderedHTML, -1)
	canonical := in.Page.Canonical
	var out []string
	for _, cb := range codeBlocks {
		// Find the most recent heading anchor that precedes this code
		// block. If no anchored heading precedes it, skip — citing an
		// unnamed snippet violates the auxiliary-schema contract.
		var anchorID, headingText string
		for _, h := range headings {
			if h[0] >= cb[0] {
				break
			}
			anchorID = string(in.RenderedHTML[h[4]:h[5]])
			headingText = faqPlainText(in.RenderedHTML[h[6]:h[7]])
		}
		if anchorID == "" {
			continue
		}
		text := decodeHTMLEntitiesPreserveText(string(in.RenderedHTML[cb[2]:cb[3]]))
		if text == "" {
			continue
		}
		out = append(out, mustJSON(code{
			Context:             schema,
			Type:                "Code",
			ID:                  canonical + "#code-" + anchorID,
			Name:                headingText,
			ProgrammingLanguage: "Go",
			CodeSampleType:      "code snippet",
			Text:                text,
		}))
	}
	return out
}

// decodeHTMLEntitiesPreserveText converts HTML entities back to their
// literal characters so the Code.text field is the verbatim Go source
// (per spec/structured-data.md § Code: "as it appears inside <pre><code>,
// post-Markdown-render, no line numbers"). Tags inside the snippet are
// left intact because Go source contains no HTML-shaped tokens after
// Goldmark's escaping pass beyond the entities listed in htmlEntities.
func decodeHTMLEntitiesPreserveText(s string) string {
	for ent, repl := range htmlEntities {
		s = strings.ReplaceAll(s, ent, repl)
	}
	return s
}

// faqHeadingOpenRE matches the opening tag of an in-section question
// heading (h2, h3, h4). RE2 has no backreferences, so the scanner finds
// these positions and walks each heading manually to extract the heading
// text (until the matching close tag) and the answer body (until the next
// heading or end of section).
var faqHeadingOpenRE = regexp.MustCompile(`(?is)<(h[234])\b[^>]*>`)

// htmlTagsRE strips HTML tags; used to convert an answer body to plain
// text suitable for schema.org Question.acceptedAnswer.text.
var htmlTagsRE = regexp.MustCompile(`<[^>]+>`)

// htmlEntities maps the small set of named entities Goldmark emits in
// rendered text. Numeric entities are decoded inline.
var htmlEntities = map[string]string{
	"&amp;":  "&",
	"&lt;":   "<",
	"&gt;":   ">",
	"&quot;": `"`,
	"&apos;": "'",
	"&#39;":  "'",
	"&nbsp;": " ",
}

// buildFAQPageJSONLD scans the rendered HTML for <section data-conversation>
// regions, collects every interrogative heading + following content as a
// Question/Answer pair, and emits a single flat FAQPage block when at
// least three pairs are present. Lead and follow-up questions across
// multiple chains on the same page are flattened into one mainEntity
// array (per spec/structured-data.md § Master schema table cross-cutting
// rule + spec/geo.md § Question-Oriented Content § JSON-LD coupling).
//
// Returns "" when the HTML carries no <section data-conversation> region
// or fewer than three Q→A pairs in total.
func buildFAQPageJSONLD(in JSONLDInputs) string {
	if len(in.RenderedHTML) == 0 {
		return ""
	}
	type qaAnswer struct {
		Type string `json:"@type"`
		Text string `json:"text"`
	}
	type qa struct {
		Type           string   `json:"@type"`
		Name           string   `json:"name"`
		AcceptedAnswer qaAnswer `json:"acceptedAnswer"`
	}
	type doc struct {
		Context    string `json:"@context"`
		Type       string `json:"@type"`
		MainEntity []qa   `json:"mainEntity"`
	}
	var pairs []qa
	for _, sectionMatch := range conversationSectionRE.FindAllSubmatch(in.RenderedHTML, -1) {
		body := sectionMatch[1]
		// Find every opening heading; each becomes a question candidate.
		// The body of an answer runs from the heading's closing tag to
		// the next heading or end of the section.
		opens := faqHeadingOpenRE.FindAllSubmatchIndex(body, -1)
		for i, om := range opens {
			tag := string(body[om[2]:om[3]])
			closeTag := []byte("</" + tag + ">")
			afterOpen := om[1]
			closeIdx := bytes.Index(body[afterOpen:], closeTag)
			if closeIdx < 0 {
				continue
			}
			question := faqPlainText(body[afterOpen : afterOpen+closeIdx])
			if !strings.HasSuffix(question, "?") {
				continue
			}
			answerStart := afterOpen + closeIdx + len(closeTag)
			answerEnd := len(body)
			if i+1 < len(opens) {
				answerEnd = opens[i+1][0]
			}
			answer := faqPlainText(body[answerStart:answerEnd])
			if answer == "" {
				continue
			}
			pairs = append(pairs, qa{
				Type:           "Question",
				Name:           question,
				AcceptedAnswer: qaAnswer{Type: "Answer", Text: answer},
			})
		}
	}
	if len(pairs) < 3 {
		return ""
	}
	return mustJSON(doc{Context: schema, Type: "FAQPage", MainEntity: pairs})
}

// faqPlainText converts an HTML fragment to a normalised single-line
// plain-text string suitable for FAQPage schema fields
// (specification/structured-data.md SD-TEXT-1 and SD-TEXT-2). Inline tags
// (code, a, em, strong, span, b, i) are removed without a separator, so
// "<code>Mux</code>," stays "Mux,"; every other tag is a block boundary
// and becomes a space. Entities are decoded, whitespace is collapsed, no
// whitespace is kept before a closing punctuation mark, and none after an
// opening parenthesis.
func faqPlainText(b []byte) string {
	stripped := htmlTagsRE.ReplaceAllStringFunc(string(b), func(tag string) string {
		if faqInlineTagRE.MatchString(tag) {
			return ""
		}
		return " "
	})
	for ent, repl := range htmlEntities {
		stripped = strings.ReplaceAll(stripped, ent, repl)
	}
	// Collapse runs of whitespace into a single space.
	stripped = strings.Join(strings.Fields(stripped), " ")
	stripped = spaceBeforePunctRE.ReplaceAllString(stripped, "$1")
	stripped = spaceAfterOpenParenRE.ReplaceAllString(stripped, "(")
	return strings.TrimSpace(stripped)
}

// faqInlineTagRE matches an opening or closing inline HTML tag whose removal
// must not introduce a word boundary.
var faqInlineTagRE = regexp.MustCompile(`(?i)^</?(code|a|em|strong|span|b|i)\b`)

// spaceBeforePunctRE matches whitespace before a closing punctuation mark.
var spaceBeforePunctRE = regexp.MustCompile(`\s+([,.;:?!)])`)

// spaceAfterOpenParenRE matches whitespace after an opening parenthesis.
var spaceAfterOpenParenRE = regexp.MustCompile(`\(\s+`)

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
		URL  string `json:"url,omitempty"`
	}
	type doc struct {
		Context     string `json:"@context"`
		Type        string `json:"@type"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Steps       []step `json:"step"`
	}
	anchors := stepAnchors(in.RenderedHTML)
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
		// HowToStep.url is the page URL plus the step heading's rendered
		// anchor (spec/structured-data.md § HowTo). The anchor is read
		// from the rendered HTML, never recomputed, so it cannot drift
		// from the id the page carries; a step without a resolvable
		// anchor omits the field rather than inventing one.
		var url string
		if i < len(anchors) {
			url = in.Page.Canonical + "#" + anchors[i]
		}
		steps = append(steps, step{Type: "HowToStep", Name: name, Text: text, URL: url})
	}
	name := in.Page.Title
	if fixed, ok := howToNames[in.Page.Path]; ok {
		name = fixed
	}
	return mustJSON(doc{Context: schema, Type: "HowTo", Name: name, Description: in.Page.Description, Steps: steps})
}

// howToNames holds the HowTo.name values that the specification fixes for
// a given page path; every other page uses its title
// (specification/structured-data.md § HowTo, SD-QUERY-2).
var howToNames = map[string]string{
	"/docs/http-query-method": "How to serve the HTTP QUERY method (RFC 10008) with MuxMaster",
}

// stepHeadingHTMLRE matches a rendered `<h2 id="…">Step N — …</h2>`
// heading and captures its id.
var stepHeadingHTMLRE = regexp.MustCompile(`(?is)<h2\b[^>]*\bid="([^"]+)"[^>]*>\s*Step\s+\d+\s+(?:—|-|&mdash;)`)

// stepAnchors returns the id of every rendered `## Step N — …` heading, in
// document order. The Markdown step headings and the rendered step
// headings are the same sequence, so index i of the result belongs to the
// i-th step heading of the source.
func stepAnchors(html []byte) []string {
	if len(html) == 0 {
		return nil
	}
	var ids []string
	for _, m := range stepHeadingHTMLRE.FindAllSubmatch(html, -1) {
		ids = append(ids, string(m[1]))
	}
	return ids
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
