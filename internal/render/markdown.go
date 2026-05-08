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
//   - parser.WithAutoHeadingID emits `<h2 id="kebab-case">` anchors for
//     the in-page TOC.
//   - html.WithUnsafe lets curated HTML comments survive (the curator may
//     embed targeted HTML; the corpus is fully trusted because it is
//     committed in this repository, not user input).
//
// Goldmark is concurrency-safe once configured.
var markdownEngine = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
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
	return buf.Bytes(), nil
}

// Heading is one entry in the in-page table of contents.
type Heading struct {
	Level int    // 2 or 3
	ID    string // kebab-case anchor
	Text  string
}

// h2h3RE matches "## heading" and "### heading" at the start of a line.
// We extract the in-page TOC from the source Markdown rather than the
// rendered HTML so the TOC stays cheap and template-free.
var h2h3RE = regexp.MustCompile(`(?m)^(#{2,3})\s+(.+?)\s*$`)

// inlineFormattingRE strips inline Markdown formatting markers (`**`, `__`,
// `*`, `_`, backticks) from a heading's display text. Anchors are still
// computed from the unstripped text so they match goldmark's
// WithAutoHeadingID output.
var inlineFormattingRE = regexp.MustCompile("[`*_]+")

// inlineLinkRE collapses `[label](url)` into `label`. Headings rarely
// contain links; this is a defensive simplification.
var inlineLinkRE = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// ExtractHeadings returns every H2 and H3 heading in the supplied source,
// in document order. The IDs are computed with the same algorithm goldmark
// uses for parser.WithAutoHeadingID so the TOC anchor matches the rendered
// HTML.
func ExtractHeadings(src []byte) []Heading {
	matches := h2h3RE.FindAllSubmatch(src, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]Heading, 0, len(matches))
	seen := make(map[string]int, len(matches))
	for _, m := range matches {
		level := len(m[1])
		if level != 2 && level != 3 {
			continue
		}
		raw := string(m[2])
		text := inlineLinkRE.ReplaceAllString(raw, "$1")
		text = inlineFormattingRE.ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		id := slugify(text)
		if id == "" {
			continue
		}
		// Disambiguate duplicate slugs the same way goldmark does:
		// append "-N" for the Nth duplicate (1-indexed after the first).
		if n, ok := seen[id]; ok {
			seen[id] = n + 1
			id = fmt.Sprintf("%s-%d", id, n)
		} else {
			seen[id] = 1
		}
		out = append(out, Heading{Level: level, ID: id, Text: text})
	}
	return out
}

// slugify converts heading text to the kebab-case anchor format goldmark
// emits with parser.WithAutoHeadingID. The algorithm: lowercase, replace
// any run of non-[a-z0-9] runes with a single dash, trim leading and
// trailing dashes. Unicode characters outside ASCII are dropped, matching
// goldmark's default identifier generator.
func slugify(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevDash := true // suppress leading dash
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := b.String()
	out = strings.TrimRight(out, "-")
	return out
}
