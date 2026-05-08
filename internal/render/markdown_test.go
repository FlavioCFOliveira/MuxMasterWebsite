package render

import (
	"strings"
	"testing"
)

// TestMarkdownToHTMLFencedCodeBlock pins the language-class hint goldmark
// emits, which the doc-page CSS depends on.
func TestMarkdownToHTMLFencedCodeBlock(t *testing.T) {
	t.Parallel()
	src := []byte("```go\nfunc main() {}\n```\n")
	out, err := MarkdownToHTML(src)
	if err != nil {
		t.Fatalf("MarkdownToHTML: %v", err)
	}
	if !strings.Contains(string(out), `class="language-go"`) {
		t.Errorf("expected class=\"language-go\"; got %s", string(out))
	}
}

// TestMarkdownToHTMLAnchoredHeadings pins the kebab-case ID emission used
// by the in-page TOC and JSON-LD anchors.
func TestMarkdownToHTMLAnchoredHeadings(t *testing.T) {
	t.Parallel()
	src := []byte("## Pattern Priority\n\n## Path Normalization\n")
	out, err := MarkdownToHTML(src)
	if err != nil {
		t.Fatalf("MarkdownToHTML: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `id="pattern-priority"`) {
		t.Errorf("missing id=pattern-priority; got %s", s)
	}
	if !strings.Contains(s, `id="path-normalization"`) {
		t.Errorf("missing id=path-normalization; got %s", s)
	}
}

// TestMarkdownToHTMLGFMTables exercises the GFM table extension used by
// /content/benchmarks.md and several docs pages.
func TestMarkdownToHTMLGFMTables(t *testing.T) {
	t.Parallel()
	src := []byte("| A | B |\n|---|---|\n| 1 | 2 |\n")
	out, err := MarkdownToHTML(src)
	if err != nil {
		t.Fatalf("MarkdownToHTML: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<table>") {
		t.Errorf("expected <table>; got %s", s)
	}
	if !strings.Contains(s, "<th>A</th>") {
		t.Errorf("expected <th>A</th>; got %s", s)
	}
}

// TestMarkdownToHTMLDeterministic ensures repeated conversion yields
// identical bytes — required for stable ETags.
func TestMarkdownToHTMLDeterministic(t *testing.T) {
	t.Parallel()
	src := []byte("# Title\n\n## Section\n\n```go\nfunc Foo() {}\n```\n\n| A | B |\n|---|---|\n| 1 | 2 |\n")
	a, err := MarkdownToHTML(src)
	if err != nil {
		t.Fatalf("MarkdownToHTML a: %v", err)
	}
	b, err := MarkdownToHTML(src)
	if err != nil {
		t.Fatalf("MarkdownToHTML b: %v", err)
	}
	if string(a) != string(b) {
		t.Error("Markdown rendering must be deterministic")
	}
}

// TestExtractHeadingsFromHTML verifies the HTML-side TOC extraction returns
// every H2 and H3 in document order with the IDs goldmark actually emitted.
func TestExtractHeadingsFromHTML(t *testing.T) {
	t.Parallel()
	src := []byte("# H1 ignored\n\n## Pattern Priority\n\nText.\n\n### Sub heading\n\n## Path Normalization\n")
	rendered, err := MarkdownToHTML(src)
	if err != nil {
		t.Fatalf("MarkdownToHTML: %v", err)
	}
	got := ExtractHeadingsFromHTML(rendered)
	want := []Heading{
		{Level: 2, ID: "pattern-priority", Text: "Pattern Priority"},
		{Level: 3, ID: "sub-heading", Text: "Sub heading"},
		{Level: 2, ID: "path-normalization", Text: "Path Normalization"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d headings, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("heading %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestHeadingsAndAnchorsAgree exercises the property that motivates the new
// extraction path: the headings ExtractHeadingsFromHTML returns MUST every
// match an `id="…"` in the rendered HTML for the SAME source. The set of
// inputs is deliberately hostile — em-dashes, dots, slashes, asterisks,
// non-ASCII Unicode — exactly the cases the previous slugifier got wrong.
func TestHeadingsAndAnchorsAgree(t *testing.T) {
	t.Parallel()
	src := []byte(`# Title

## Step 1 — Hello, World

Text.

### Pattern Priority

Text.

## Path/Normalization

Text.

## API.Reference

Text.

## **Strong** heading

Text.

## café — special

Text.
`)
	rendered, err := MarkdownToHTML(src)
	if err != nil {
		t.Fatalf("MarkdownToHTML: %v", err)
	}
	headings := ExtractHeadingsFromHTML(rendered)
	if len(headings) == 0 {
		t.Fatalf("no headings extracted; rendered=%s", rendered)
	}
	rs := string(rendered)
	for _, h := range headings {
		marker := `id="` + h.ID + `"`
		if !strings.Contains(rs, marker) {
			t.Errorf("heading %+v has no matching %s in body", h, marker)
		}
	}
}

// TestStripFrontmatter pins the YAML frontmatter strip on the .md companion
// path. Files without a frontmatter block must pass through unchanged.
func TestStripFrontmatter(t *testing.T) {
	t.Parallel()
	withFM := []byte("---\ntitle: x\n---\n# Body\n")
	if got := string(stripFrontmatter(withFM)); got != "# Body\n" {
		t.Errorf("stripFrontmatter with FM = %q", got)
	}
	withoutFM := []byte("# Body\n\n## Section\n")
	if got := string(stripFrontmatter(withoutFM)); got != string(withoutFM) {
		t.Error("file without frontmatter must pass through unchanged")
	}
}
