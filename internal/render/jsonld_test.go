package render

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestFAQPageEmitsWithThreeOrMorePairs verifies the FAQPage emitter
// honours the doctrine threshold (spec/structured-data.md cross-cutting
// rule + spec/geo.md § Question-Oriented Content § JSON-LD coupling): a
// flat FAQPage block when ≥3 Q→A pairs exist, nothing otherwise.
func TestFAQPageEmitsWithThreeOrMorePairs(t *testing.T) {
	t.Parallel()

	threePairsHTML := []byte(`
<section data-conversation="routing-params">
  <h3>How do I match a route param?</h3>
  <p>Declare it with a colon prefix.</p>
  <h3>What if the param is optional?</h3>
  <p>Mount two routes — one with and one without the param.</p>
  <h3>How do I validate it?</h3>
  <p>Read the param in the handler and apply your validation logic.</p>
</section>
`)
	out := buildFAQPageJSONLD(JSONLDInputs{RenderedHTML: threePairsHTML})
	if out == "" {
		t.Fatalf("expected FAQPage block for 3 pairs, got empty")
	}
	if !strings.Contains(out, `"FAQPage"`) {
		t.Errorf("output missing FAQPage type: %s", out)
	}
	if strings.Count(out, `"@type":"Question"`) != 3 {
		t.Errorf("expected 3 Question entries, got: %s", out)
	}
}

// TestFAQPageSuppressedBelowThreshold verifies that pages with fewer than
// three Q→A pairs do not emit the block.
func TestFAQPageSuppressedBelowThreshold(t *testing.T) {
	t.Parallel()

	twoPairsHTML := []byte(`
<section data-conversation="x">
  <h3>One?</h3><p>A.</p>
  <h3>Two?</h3><p>B.</p>
</section>
`)
	if out := buildFAQPageJSONLD(JSONLDInputs{RenderedHTML: twoPairsHTML}); out != "" {
		t.Errorf("expected empty for 2 pairs, got: %s", out)
	}
}

// TestFAQPageFlattensAcrossChains verifies that questions from multiple
// <section data-conversation> regions on the same page are flattened into
// a single FAQPage with one mainEntity array.
func TestFAQPageFlattensAcrossChains(t *testing.T) {
	t.Parallel()

	twoChainsHTML := []byte(`
<section data-conversation="a">
  <h3>A1?</h3><p>a1.</p>
  <h3>A2?</h3><p>a2.</p>
</section>
<section data-conversation="b">
  <h3>B1?</h3><p>b1.</p>
  <h3>B2?</h3><p>b2.</p>
</section>
`)
	out := buildFAQPageJSONLD(JSONLDInputs{RenderedHTML: twoChainsHTML})
	if out == "" {
		t.Fatalf("expected FAQPage block for 4 pairs across two chains, got empty")
	}
	if strings.Count(out, `"@type":"Question"`) != 4 {
		t.Errorf("expected 4 Question entries flattened, got: %s", out)
	}
	// Single FAQPage emission, not one per chain.
	if strings.Count(out, `"FAQPage"`) != 1 {
		t.Errorf("expected exactly one FAQPage block, got: %s", out)
	}
}

// TestFAQPageIgnoresHeadingsOutsideSection verifies that a question-shaped
// heading outside <section data-conversation> is not collected.
func TestFAQPageIgnoresHeadingsOutsideSection(t *testing.T) {
	t.Parallel()

	mixedHTML := []byte(`
<h3>Stray?</h3><p>not in a section.</p>
<section data-conversation="x">
  <h3>One?</h3><p>a.</p>
  <h3>Two?</h3><p>b.</p>
  <h3>Three?</h3><p>c.</p>
</section>
`)
	out := buildFAQPageJSONLD(JSONLDInputs{RenderedHTML: mixedHTML})
	if strings.Count(out, `"@type":"Question"`) != 3 {
		t.Errorf("expected exactly 3 Question entries (stray heading ignored), got: %s", out)
	}
}

// TestFAQPageRequiresQuestionMark verifies that a non-interrogative
// heading inside the section is skipped.
func TestFAQPageRequiresQuestionMark(t *testing.T) {
	t.Parallel()

	html := []byte(`
<section data-conversation="x">
  <h3>One?</h3><p>a.</p>
  <h3>Not a question</h3><p>still text.</p>
  <h3>Two?</h3><p>b.</p>
  <h3>Three?</h3><p>c.</p>
</section>
`)
	out := buildFAQPageJSONLD(JSONLDInputs{RenderedHTML: html})
	if strings.Count(out, `"@type":"Question"`) != 3 {
		t.Errorf("expected 3 Question entries, got: %s", out)
	}
}

// TestHowToStepURLsFromRenderedAnchors verifies that each HowToStep carries
// the page URL plus the rendered id of its step heading, and that the HowTo
// carries a description (spec/structured-data.md § HowTo).
func TestHowToStepURLsFromRenderedAnchors(t *testing.T) {
	t.Parallel()
	src := []byte("# Guide\n\n## Step 1 — Register a route\n\nRegister it.\n\n## Step 2 — Call it with `curl`\n\nCall it.\n")
	htmlBody, err := MarkdownToHTML(src)
	if err != nil {
		t.Fatalf("MarkdownToHTML: %v", err)
	}
	in := JSONLDInputs{HowToSource: src, RenderedHTML: htmlBody}
	in.Page.Title = "Guide"
	in.Page.Description = "A guide."
	in.Page.Canonical = "https://muxmaster.net/docs/guide"
	var doc struct {
		Description string `json:"description"`
		Step        []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"step"`
	}
	if err := json.Unmarshal([]byte(buildHowToJSONLD(in)), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Description != "A guide." {
		t.Errorf("description=%q", doc.Description)
	}
	if len(doc.Step) != 2 {
		t.Fatalf("steps=%d, want 2", len(doc.Step))
	}
	for i, st := range doc.Step {
		frag, ok := strings.CutPrefix(st.URL, in.Page.Canonical+"#")
		if !ok || !strings.Contains(string(htmlBody), `id="`+frag+`"`) {
			t.Errorf("step %d url %q does not point at a rendered anchor", i+1, st.URL)
		}
	}
}

// TestArticleAboutAndVersion verifies TechArticle.about and
// TechArticle.version are emitted only when requested.
func TestArticleAboutAndVersion(t *testing.T) {
	t.Parallel()
	in := JSONLDInputs{Family: "doc-article", AboutSoftware: true, ArticleVersion: "1.3.0"}
	in.Page.BaseURL = "https://muxmaster.net"
	in.Page.Canonical = "https://muxmaster.net/releases/v1.3.0"
	in.Page.Title = "Release notes — v1.3.0"
	art := buildArticleJSONLD(in)[0]
	if !strings.Contains(art, `"about":{"@id":"https://muxmaster.net/#muxmaster"}`) || !strings.Contains(art, `"version":"1.3.0"`) {
		t.Errorf("TechArticle missing about/version: %s", art)
	}
	in.AboutSoftware, in.ArticleVersion = false, ""
	if art := buildArticleJSONLD(in)[0]; strings.Contains(art, `"about"`) || strings.Contains(art, `"version"`) {
		t.Errorf("TechArticle emits about/version when not requested: %s", art)
	}
}
