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
	Family        string    // "landing", "doc-article", "collection", "api", "error"
	BuildTime     time.Time // process start time, retained for diagnostics; MUST NOT be used as a date substitute (spec/structured-data.md § Date sources for embedded content).
	DatePublished time.Time // truthful first-publication date sourced from front-matter; zero means omit per the doctrine.
	DateModified  time.Time // truthful last-modified date sourced from front-matter or git-log build manifest; zero means omit per the doctrine.
	HowToSource   []byte    // optional; if present the generator scans for `## Step N — name` headings
	HasPart       []string  // for "collection" family: canonical absolute URLs of the items the page lists; emitted as CollectionPage.hasPart.
	RenderedHTML  []byte    // optional; when present the FAQPage scanner walks <section data-conversation> regions to extract Q→A pairs into a single flat FAQPage block (spec/geo.md § Question-Oriented Content).
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
	if (in.Family == "doc-article" || in.Family == "api") && len(out) > 0 {
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
const UpstreamMinimumGoVersion = "1.26"

// softwareRuntimePlatform formats the minimum supported Go version as a
// schema.org runtimePlatform value (e.g. "Go 1.26"). Returns "" when the
// caller did not thread a Go version through Deps/Page; the caller emits
// the field with omitempty so an absent value produces no fabrication.
func softwareRuntimePlatform(goVersion string) string {
	if goVersion == "" {
		return ""
	}
	return "Go " + goVersion
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
		Version:             in.Page.Version,
		Description:         in.Page.Description,
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
// sync; a TestLandingFAQHTMLAndJSONLDAgree test in jsonld_test.go
// enforces the invariant by comparing the question lists.
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
// The HTML inside an answer is allowed by schema.org FAQPage.acceptedAnswer
// (the text field accepts inline HTML markup); we embed the same
// formatting (<code>, <strong>, <a href>) that the rendered page shows.
// Absolute URLs in <a href> are required so AI ingestion pipelines that
// parse the JSON-LD in isolation can still resolve every link without
// needing the page context. The BuildJSONLD caller threads the site's
// canonical base URL so this stays correct in dev (http://localhost:8080)
// and production (https://muxmaster.net) alike.
var landingFAQEntries = []struct {
	Q, A string
}{
	{
		Q: "What is MuxMaster?",
		A: `MuxMaster is a high-performance, zero-dependency HTTP router for Go. It implements a radix tree with O(k) lookups, allocates zero bytes on the static-route hot path, and is fully compatible with the <code>net/http</code> <code>Handler</code> interface so existing handlers compile unchanged.`,
	},
	{
		Q: "What Go version does MuxMaster require?",
		A: `Go 1.26 or later. The minimum is set by the <code>go</code> directive in the upstream <code>go.mod</code>; see <a href="{base}/compatibility">Compatibility</a> for the full version policy.`,
	},
	{
		Q: "Is MuxMaster compatible with net/http?",
		A: `100%. Handlers remain <code>http.Handler</code> and middleware remains <code>func(http.Handler) http.Handler</code>. A <code>*muxmaster.Mux</code> is a drop-in replacement for <code>http.ServeMux</code> everywhere the standard library accepts an <code>http.Handler</code>, so adoption is incremental: a single route, a single sub-tree, or a whole service.`,
	},
	{
		Q: "How fast is MuxMaster?",
		A: `Static-route lookups complete in <strong>25 ns</strong> with zero allocations; one-parameter routes complete in <strong>112 ns</strong> with a single 416-byte allocation for the tiered request bundle. Numbers measured on AMD Ryzen 9 5900HX, Go 1.26.2; see <a href="{base}/benchmarks">Benchmarks</a> for the full table and the reproduce instructions.`,
	},
	{
		Q: "What is MuxMaster's license?",
		A: `MIT. The full text is in the upstream repository at <a href="https://github.com/FlavioCFOliveira/MuxMaster/blob/main/LICENSE">LICENSE</a>. MIT permits commercial use, modification, distribution, and private use; the only requirement is preserving the copyright notice.`,
	},
	{
		Q: "How does MuxMaster compare to chi, gin, gorilla/mux, and httprouter?",
		A: `MuxMaster keeps the <code>net/http</code> handler signature (unlike <code>gin</code>, which introduces a <code>gin.Context</code>) and ships zero external dependencies. It matches <code>httprouter</code>'s static-route latency while exposing a higher-level API (route groups, typed parameters, error-returning handlers). See the <a href="{base}/docs/migration">migration guide</a> for side-by-side equivalents.`,
	},
}

// buildLandingFAQPageJSONLD emits the homepage FAQPage block. Returns the
// JSON string ready to embed; never empty (landingFAQEntries is a
// compile-time constant with six entries, well above the FAQPage minimum
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
	mainEntity := make([]questionT, 0, len(landingFAQEntries))
	for _, e := range landingFAQEntries {
		mainEntity = append(mainEntity, questionT{
			Type:           "Question",
			Name:           e.Q,
			AcceptedAnswer: answerT{Type: "Answer", Text: strings.ReplaceAll(e.A, "{base}", base)},
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
	return []string{mustJSON(collection), breadcrumbJSON(in.Page)}
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
	apiRef := struct {
		Context              string `json:"@context"`
		Type                 string `json:"@type"`
		ID                   string `json:"@id"`
		Name                 string `json:"name"`
		Description          string `json:"description"`
		URL                  string `json:"url"`
		TargetPlatform       string `json:"targetPlatform"`
		ProgrammingModel     string `json:"programmingModel"`
		ExecutableLibraryName string `json:"executableLibraryName"`
		AssemblyVersion      string `json:"assemblyVersion,omitempty"`
		About                idRef  `json:"about"`
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
		AssemblyVersion:       in.Page.Version,
		About:                 idRef{ID: jsonSoftwareID(base)},
	}
	return append(out, mustJSON(apiRef))
}

// benchmarks graph: TechArticle + BreadcrumbList + Dataset. The Dataset
// surfaces the benchmark numbers as citeable measurements for AI
// ingestion and Google's Dataset rich result.
func buildBenchmarksJSONLD(in JSONLDInputs) []string {
	out := buildArticleJSONLD(in)
	base := in.Page.BaseURL
	canonical := in.Page.Canonical
	type distribution struct {
		Type           string `json:"@type"`
		EncodingFormat string `json:"encodingFormat"`
		ContentURL     string `json:"contentUrl"`
	}
	type variable struct {
		Type        string `json:"@type"`
		Name        string `json:"name"`
		Description string `json:"description"`
		UnitText    string `json:"unitText"`
	}
	dataset := struct {
		Context          string         `json:"@context"`
		Type             string         `json:"@type"`
		ID               string         `json:"@id"`
		Name             string         `json:"name"`
		Description      string         `json:"description"`
		URL              string         `json:"url"`
		InLanguage       string         `json:"inLanguage"`
		License          string         `json:"license"`
		Creator          idRef          `json:"creator"`
		TemporalCoverage string         `json:"temporalCoverage,omitempty"`
		VariableMeasured []variable     `json:"variableMeasured"`
		Distribution     []distribution `json:"distribution"`
	}{
		Context:     schema,
		Type:        "Dataset",
		ID:          canonical + "#dataset",
		Name:        in.Page.Title,
		Description: in.Page.Description,
		URL:         canonical,
		InLanguage:  "en",
		License:     "https://opensource.org/licenses/MIT",
		Creator:     idRef{ID: jsonOrgID(base)},
		// temporalCoverage is omitted (omitempty) until the benchmark
		// report's date range is threaded through Page; truthful value
		// not yet available, never fabricated.
		VariableMeasured: []variable{
			{Type: "PropertyValue", Name: "ns/op", Description: "Nanoseconds per operation; lower is better.", UnitText: "ns"},
			{Type: "PropertyValue", Name: "B/op", Description: "Bytes allocated per operation; lower is better.", UnitText: "B"},
			{Type: "PropertyValue", Name: "allocs/op", Description: "Heap allocations per operation; lower is better.", UnitText: "allocations"},
		},
		Distribution: []distribution{
			{
				Type:           "DataDownload",
				EncodingFormat: "text/x-go",
				ContentURL:     "https://github.com/FlavioCFOliveira/MuxMaster/blob/main/bench_test.go",
			},
		},
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
		Context  string `json:"@context"`
		Type     string `json:"@type"`
		ID       string `json:"@id"`
		Name     string `json:"name"`
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
// plain-text string suitable for FAQPage schema fields. Tags are removed,
// entities decoded, whitespace collapsed.
func faqPlainText(b []byte) string {
	stripped := htmlTagsRE.ReplaceAllString(string(b), " ")
	for ent, repl := range htmlEntities {
		stripped = strings.ReplaceAll(stripped, ent, repl)
	}
	// Collapse runs of whitespace into a single space.
	stripped = strings.Join(strings.Fields(stripped), " ")
	return strings.TrimSpace(stripped)
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
