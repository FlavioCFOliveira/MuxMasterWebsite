package render

import (
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
