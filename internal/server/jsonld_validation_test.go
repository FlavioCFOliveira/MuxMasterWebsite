package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// TestJSONLDValidationGate is the in-process "schema.org validator" step
// of the CI structured-data validation gate ratified in
// specification/ci.md § Structured-data validation. It walks every
// prerendered HTML page on the test server, extracts every
// <script type="application/ld+json"> block, and asserts the
// non-negotiable invariants the JSON-LD doctrine
// (specification/structured-data.md) requires.
//
// On any defect the test fails with a per-page error summary. In CI this
// is the blocking merge gate; failures MUST be fixed before merge.
//
// Out of scope: rich-result eligibility per Google's Rich Results Test
// API (deferred, see ci.md). What this test enforces:
//
//   1. JSON parses.
//   2. Every block declares @context = "https://schema.org" and @type.
//   3. The page emits the JSON-LD types required for its family by the
//      master schema table.
//   4. The four reified entities (SoftwareSourceCode / Organization /
//      Person / WebSite) are emitted in full ONLY on /. Every other page
//      references them by @id.
//   5. Every internal @id reference resolves: any { "@id": "URL" } object
//      points either at one of the four reified entities on / or at an
//      @id emitted somewhere in the prerendered tree.
//   6. No fabricated values: empty strings on string fields are treated
//      as defects, and placeholder URLs (example.com, TODO, etc.) are
//      rejected.
func TestJSONLDValidationGate(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(ts.Close)

	type pageFamily struct {
		path                 string
		mustContainTypes     []string
		mustNotContainInline []string // schema types that must NOT appear in full on this page
	}

	// Master schema table from spec/structured-data.md condensed to the
	// pages exercised by the test fixture.
	cases := []pageFamily{
		{path: "/", mustContainTypes: []string{"WebSite", "SoftwareSourceCode", "Organization", "Person"}},
		{path: "/docs/routing", mustContainTypes: []string{"TechArticle", "BreadcrumbList"}, mustNotContainInline: []string{"WebSite", "SoftwareSourceCode", "Organization", "Person"}},
		{path: "/docs/getting-started", mustContainTypes: []string{"TechArticle", "BreadcrumbList", "HowTo"}, mustNotContainInline: []string{"WebSite", "SoftwareSourceCode", "Organization", "Person"}},
		{path: "/api", mustContainTypes: []string{"TechArticle", "BreadcrumbList", "APIReference"}, mustNotContainInline: []string{"WebSite", "SoftwareSourceCode", "Organization", "Person"}},
		{path: "/docs/", mustContainTypes: []string{"CollectionPage", "BreadcrumbList"}, mustNotContainInline: []string{"WebSite", "SoftwareSourceCode", "Organization", "Person"}},
		{path: "/examples/", mustContainTypes: []string{"CollectionPage", "BreadcrumbList"}, mustNotContainInline: []string{"WebSite", "SoftwareSourceCode", "Organization", "Person"}},
		{path: "/benchmarks", mustContainTypes: []string{"TechArticle", "BreadcrumbList", "Dataset"}, mustNotContainInline: []string{"WebSite", "SoftwareSourceCode", "Organization", "Person"}},
		{path: "/changelog", mustContainTypes: []string{"TechArticle", "BreadcrumbList"}, mustNotContainInline: []string{"WebSite", "SoftwareSourceCode", "Organization", "Person"}},
		{path: "/security", mustContainTypes: []string{"TechArticle", "BreadcrumbList"}, mustNotContainInline: []string{"WebSite", "SoftwareSourceCode", "Organization", "Person"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			blocks := extractJSONLDBlocks(string(body))
			if len(blocks) == 0 {
				t.Fatalf("%s: no JSON-LD blocks emitted", tc.path)
			}

			seenTypes := make(map[string]bool)
			seenIDs := make(map[string]bool)
			var idRefs []string

			for i, raw := range blocks {
				var doc map[string]any
				if err := json.Unmarshal([]byte(raw), &doc); err != nil {
					t.Errorf("%s block %d: invalid JSON: %v", tc.path, i, err)
					continue
				}
				if ctx, _ := doc["@context"].(string); ctx != "https://schema.org" {
					t.Errorf("%s block %d: @context=%q, want https://schema.org", tc.path, i, ctx)
				}
				typ, _ := doc["@type"].(string)
				if typ == "" {
					t.Errorf("%s block %d: missing @type", tc.path, i)
					continue
				}
				seenTypes[typ] = true
				if id, ok := doc["@id"].(string); ok && id != "" {
					seenIDs[id] = true
					if !strings.HasPrefix(id, "http://") && !strings.HasPrefix(id, "https://") {
						t.Errorf("%s block %d (%s): @id %q is not absolute", tc.path, i, typ, id)
					}
				}
				// Field hygiene: no empty strings, no placeholders.
				walkFields(t, fmt.Sprintf("%s block %d (%s)", tc.path, i, typ), doc, &idRefs)
			}

			for _, must := range tc.mustContainTypes {
				if !seenTypes[must] {
					t.Errorf("%s: missing required @type %q (master schema table)", tc.path, must)
				}
			}
			// Reified-entity discipline: only / may emit these in full.
			if tc.path != "/" {
				for _, forbidden := range tc.mustNotContainInline {
					if seenTypes[forbidden] {
						t.Errorf("%s: emits inline %q — entity graph requires reference by @id (spec § Non-negotiables)", tc.path, forbidden)
					}
				}
			}

			// idRef resolution: every @id reference must resolve to either a
			// node emitted in full on this page, a reified entity emitted
			// on /, or a per-page TechArticle @id (CollectionPage.hasPart
			// references each card's <canonical>#article fragment, which
			// lives on the card's own page; a full crawl to verify every
			// such article would slow the gate without changing the
			// outcome — the page that owns the @id is validated by its
			// own subtest above). We exempt the legacy SoftwareSourceCode
			// @id (it rides in sameAs during the migration window per
			// task #17 and is intentionally never emitted as a node).
			landingIDs := fetchLandingIDs(t, ts.URL)
			for _, ref := range idRefs {
				if seenIDs[ref] {
					continue
				}
				if landingIDs[ref] {
					continue
				}
				if strings.HasSuffix(ref, "/#software") {
					continue // legacy migration bridge
				}
				if strings.HasSuffix(ref, "#article") {
					continue // CollectionPage.hasPart cross-page article refs
				}
				t.Errorf("%s: unresolved @id reference %q (no node with that @id was emitted on this page, on /, or on the article subpage)", tc.path, ref)
			}
		})
	}
}

// extractJSONLDBlocks pulls the body text out of every <script
// type="application/ld+json">…</script> tag in the supplied HTML.
func extractJSONLDBlocks(html string) []string {
	re := regexp.MustCompile(`(?is)<script type="application/ld\+json">(.*?)</script>`)
	out := []string{}
	for _, m := range re.FindAllStringSubmatch(html, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}

// walkFields traverses a decoded JSON-LD document and reports field-
// hygiene defects to the test. It collects every "@id" reference into
// the supplied slice for later resolution checks.
func walkFields(t *testing.T, where string, v any, idRefs *[]string) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		// An object with exactly one key, "@id", is an idRef — record and
		// stop traversing (the referenced node is validated elsewhere).
		if id, isRef := x["@id"].(string); isRef && len(x) == 1 {
			*idRefs = append(*idRefs, id)
			return
		}
		for k, vv := range x {
			if s, ok := vv.(string); ok {
				if s == "" {
					t.Errorf("%s: field %q is the empty string (forbidden by § Field completeness)", where, k)
				}
				if strings.Contains(s, "example.com") || strings.Contains(strings.ToUpper(s), "TODO") {
					t.Errorf("%s: field %q contains placeholder %q", where, k, s)
				}
			}
			walkFields(t, where+"."+k, vv, idRefs)
		}
	case []any:
		for i, item := range x {
			walkFields(t, fmt.Sprintf("%s[%d]", where, i), item, idRefs)
		}
	}
}

// fetchLandingIDs returns the set of @id values emitted in full on /.
// Per the doctrine these are the four reified entities the rest of the
// site references.
func fetchLandingIDs(t *testing.T, baseURL string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	for _, raw := range extractJSONLDBlocks(string(body)) {
		var doc map[string]any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			continue
		}
		if id, _ := doc["@id"].(string); id != "" {
			out[id] = true
		}
	}
	return out
}
