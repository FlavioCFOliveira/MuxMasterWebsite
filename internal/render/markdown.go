package render

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// markdownEngine is the process-wide CommonMark renderer. We chose
// github.com/yuin/goldmark (option (a) in the round brief) over a
// build-time generator because goldmark is a single mature dependency,
// CommonMark + GFM compliant, and produces deterministic output for stable
// ETags. The runtime keeps the static-tending invariant: every Markdown
// body is rendered once at startup inside a recipe.
//
// The configuration:
//   - extension.GFM enables tables (used by /content/benchmarks.md and
//     several docs pages).
//   - extension.DefinitionList renders the CommonMark definition-list
//     syntax to <dl><dt>Term</dt><dd>Definition</dd></dl>. The
//     buildDefinedTermSetJSONLD scanner walks <dl> blocks to emit
//     schema.org DefinedTermSet (spec/structured-data.md § Auxiliary
//     schemas).
//   - parser.WithAutoHeadingID emits `<h2 id="kebab-case">` anchors for
//     the in-page TOC.
//   - html.WithUnsafe lets curated HTML comments survive (the curator may
//     embed targeted HTML; the corpus is fully trusted because it is
//     committed in this repository, not user input).
//
// Goldmark is concurrency-safe once configured.
var markdownEngine = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.DefinitionList),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// MarkdownToHTML converts CommonMark + GFM bytes to HTML. It is invoked
// only at startup, by recipes; per-request rendering is forbidden by
// specification/rendering-and-caching.md.
//
// Goldmark already emits `<pre><code class="language-<lang>">…</code></pre>`
// for fenced code blocks with a language hint, which matches the CSS
// selectors used by the doc-page template.
func MarkdownToHTML(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := markdownEngine.Convert(src, &buf); err != nil {
		return nil, fmt.Errorf("markdown: convert: %w", err)
	}
	return addTableHeaderScope(buf.Bytes()), nil
}

// thOpenRE matches a <th> opening tag without a scope attribute. Goldmark
// emits <th> only inside <thead> (the first row of a GFM table); body
// cells use <td>. So a global add-scope pass is safe.
var thOpenRE = regexp.MustCompile(`<th(\s|>)`)

// addTableHeaderScope post-processes Goldmark's output to add scope="col"
// to every <th> opening tag. spec/accessibility-and-standards.md requires
// the scope attribute on data-table headers for screen-reader navigation;
// Goldmark does not emit it natively. Existing scope attributes are
// preserved (the regex matches only <th> followed by whitespace or the
// closing >, never a scope= already present).
func addTableHeaderScope(html []byte) []byte {
	return thOpenRE.ReplaceAll(html, []byte(`<th scope="col"$1`))
}

// Heading is one entry in the in-page table of contents.
type Heading struct {
	Level int    // 2 or 3
	ID    string // anchor as emitted by goldmark
	Text  string
}

// headingHTMLRE matches the H2/H3 elements goldmark emits with
// parser.WithAutoHeadingID. The renderer's output for an auto-ID heading is
// always a single line of the form `<h2 id="…">text</h2>` (or h3) — no
// nested elements, no attribute reordering, and the id comes first because
// goldmark writes it before any class or other attribute.
//
// Why this path (vs. parsing the source Markdown): the previous slugifier
// approximated goldmark's identifier algorithm and drifted on inputs that
// included em-dashes, dots, slashes, asterisks, and Unicode. Extracting from
// the *rendered* HTML guarantees that every TOC entry's `href="#id"` matches
// an `id="id"` that actually exists in the body — the two cannot disagree
// because they come from the same string.
//
// The text capture stops at the first `<` so any inline elements goldmark
// generated inside the heading (e.g. `<code>`) are stripped from the TOC
// label without us needing a full HTML parser; the call site post-processes
// the captured text with stripInlineTags for that case.
var headingHTMLRE = regexp.MustCompile(`<h([23])\s+id="([^"]+)"[^>]*>(.*?)</h[23]>`)

// inlineTagRE matches simple inline tags goldmark may emit inside a heading
// (most commonly <code>…</code>). The replace strips the tags and keeps the
// inner text, leaving entities such as &amp; intact for the TOC label.
var inlineTagRE = regexp.MustCompile(`<[^>]+>`)

// ExtractHeadingsFromHTML returns every H2 and H3 heading present in the
// supplied rendered HTML, in document order. The IDs are read directly from
// the HTML the page will serve, so the TOC anchor cannot drift from the
// rendered output.
func ExtractHeadingsFromHTML(htmlBody []byte) []Heading {
	matches := headingHTMLRE.FindAllSubmatch(htmlBody, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]Heading, 0, len(matches))
	for _, m := range matches {
		level := 0
		switch m[1][0] {
		case '2':
			level = 2
		case '3':
			level = 3
		default:
			continue
		}
		id := string(m[2])
		text := stripInlineTags(string(m[3]))
		text = decodeBasicEntities(text)
		text = strings.TrimSpace(text)
		if id == "" || text == "" {
			continue
		}
		out = append(out, Heading{Level: level, ID: id, Text: text})
	}
	return out
}

// stripInlineTags removes any HTML tag from s, keeping the inner text. Used
// to flatten inline elements (<code>, <em>, <strong>) inside a heading down
// to plain text for the TOC label.
func stripInlineTags(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	return inlineTagRE.ReplaceAllString(s, "")
}

// decodeBasicEntities decodes the small set of HTML entities goldmark may
// emit inside a heading text run. We avoid html.UnescapeString to keep this
// helper allocation-light and predictable.
func decodeBasicEntities(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&apos;", "'",
	)
	return r.Replace(s)
}
