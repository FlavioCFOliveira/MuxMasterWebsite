---
title: GEO contract
purpose: Define the Generative Engine Optimization contract — llms.txt artefacts, Markdown companions, AI crawler allowlist, FAQPage and HowTo structured data, and content-shape rules.
owners: geo-specialist (final review); review by seo-specialist (structured-data overlap), ux-specialist (content-shape and tone).
last-updated: 2026-05-10
status: ratified
---

# GEO (Generative Engine Optimization) contract

## Purpose

The site MUST be ingestible by AI answer engines (ChatGPT, Claude, Perplexity, Google AI Overviews, and similar) without HTML noise, and MUST present its facts in a shape that maximises correct citation.

## /llms.txt

- Path: `/llms.txt`. Plain text, UTF-8.
- Convention: https://llmstxt.org.
- Required structure (in order):
  1. A top-level `# MuxMaster` heading.
  2. A one-paragraph blurb stating what MuxMaster is, in the form: "MuxMaster is a high-performance, zero-dependency HTTP router for Go. It provides a radix-tree implementation with O(k) lookups, zero allocations on static routes, and 100% compatibility with `net/http`. It supports the minimum Go version stated on `/compatibility`."
  3. A `## Documentation` section listing every documentation page on the site as Markdown links (`- [Title](https://<canonical>/path): one-line purpose`).
  4. A `## API` section linking to `/api`.
  5. An `## Examples` section listing every example page.
  6. A `## Reference` section linking to `/benchmarks`, `/changelog`, `/releases/v1.0.0`, `/security`, `/compatibility`, `/contributing`.
  7. An `## Optional` section containing the link to the GitHub repository.
- The list of links MUST be auto-generated from the registered routes at startup. A route added to the site that is part of a documentation family MUST appear in `/llms.txt` without manual edits.

## /llms-full.txt

`/llms-full.txt` is the **bundled** variant defined by the llmstxt.org convention: the navigation index of `/llms.txt` followed by the concatenated Markdown bodies of every content-backed page, so that a crawler can ingest the entire site in one request.

- Path: `/llms-full.txt`. Served with `Content-Type: text/plain; charset=utf-8`.
- The file MUST start with the same top-level `# MuxMaster` heading and the same one-paragraph project blurb as `/llms.txt`, so that a crawler that fetches only this file still has the project frame.
- Immediately after the blurb, the file MUST list the canonical URLs of every documentation page in the same sectioned form as `/llms.txt` (`## Documentation`, `## API`, `## Examples`, `## Reference`, `## Optional`), each entry written as a Markdown link with a one-line description. The bundled file is therefore also a navigation index — a superset of `/llms.txt`, not a different navigation product.
- The links in this navigation index MUST point at the canonical HTML URLs (the same URLs `/llms.txt` lists). The bundled file MUST NOT list `.md` companion URLs in this index.
- After the navigation index, the file MUST emit a `---` separator on its own line, followed by a `# Full content` heading on its own line, followed by the concatenation of every content-backed page's Markdown body.
- Inlined bodies MUST appear in the same order as the routes appear in the navigation index above.
- Each inlined body MUST be preceded by a heading line that names the route URL (for example `## /docs/routing`), so that a crawler can locate the body it cares about within the bundle.
- Only routes whose body comes from a single curated file under `/content/` are inlined. Pages without a backing content file (`/`, `/docs/`, `/examples/`) MUST NOT appear in the inlined section, even though they are listed in the navigation index above. The implementation gates inclusion on the presence of a content file path for the route.
- The file MUST be auto-generated from the registered routes at startup, on the same trigger as `/llms.txt`. A new content-backed route added to the site MUST appear in both the navigation index and the inlined section without manual edits.

## Markdown companions

- Defined in `content-sources.md`. Every documentation route, the API page, every example page, the benchmarks page, the changelog, the release-notes pages, security, compatibility, and contributing pages MUST expose a `.md` companion.
- The HTML and the `.md` representations MUST present the same canonical content.
- The `.md` companion MUST set `Content-Type: text/markdown; charset=utf-8`.
- Content negotiation via `Accept` is **not** used; the explicit `.md` URL is the only path.

## AI crawler allowlist (robots.txt)

The full `robots.txt` is co-owned with `seo.md`. The AI-crawler portion MUST explicitly allow the following user agents on all paths except `/healthz`:

- `GPTBot` (OpenAI)
- `ChatGPT-User` (OpenAI on-demand)
- `OAI-SearchBot` (OpenAI search)
- `ClaudeBot` (Anthropic)
- `anthropic-ai` (legacy Anthropic agent)
- `PerplexityBot` (Perplexity)
- `Google-Extended` (Google generative)
- `Applebot-Extended` (Apple Intelligence)
- `CCBot` (Common Crawl)
- `Bytespider` (ByteDance)
- `Diffbot` (Diffbot — single canonical spelling; the vendor's user-agent is `Diffbot/X.X`, the alternate `DiffBot` capitalisation is not emitted by the site)
- `OmgiliBot` (Webz)
- `Amazonbot` (Amazon)
- `meta-externalagent` (Meta)
- `Perplexity-User` (Perplexity on-demand fetch)
- `FacebookBot` (Meta crawler — explicit complement to `meta-externalagent`, which only covers Meta's generative agent)
- `cohere-ai` (Cohere)
- `MistralAI-User` (Mistral on-demand fetch)

For each, the entry takes the form:

```
User-agent: <name>
Allow: /
Disallow: /healthz
```

The allowlist MUST be re-evaluated at least once per release of the site. Removal of a bot from the allowlist requires explicit ratification.

## FAQPage and HowTo structured data

- Pages with three or more explicit Q→A pairs (a `<h2>` or `<h3>` phrased as a question, immediately followed by a paragraph answer) MUST emit a JSON-LD `FAQPage` block listing those pairs. Q→A pairs are not optional on docs, guides, examples, and API/reference pages: their presence and minimum counts are mandated by `## Question-Oriented Content` below.
- Pages with an ordered, named list of steps MUST emit a JSON-LD `HowTo` block listing those steps. The trigger is structural — the presence of a contiguous, 1-indexed `## Step N — <name>` heading sequence — not editorial. Two page families satisfy this trigger by construction: the Getting Started page in `/docs/`, and every page under `/examples/<name>` whose body is a runnable program (see `## Example walkthrough shape` below). Pages outside those two families MAY emit `HowTo` when they contain such a step sequence, but the structural rule, not the page family, is what compels emission.
- Both blocks MUST live alongside the SEO JSON-LD blocks defined in `seo.md`. Duplicate facts in two blocks are acceptable.

Field-level rules and the validation gate are defined in `specification/structured-data.md`.

## Question-Oriented Content

### Purpose

The site structures its content to answer real user questions — both direct, single-sentence questions and complex, multi-step questions — by simulating conversational flows between a reader and the documentation. This strategy is used in deliberate preference over loose keyword targeting, because generative engines cite answer-shaped paragraphs more reliably than keyword-stuffed prose, and human readers locate information faster when it is framed as the question they were already asking.

### Conversational chain

A conversational chain is a sequence of the form `Q → A → follow-up Q → A → follow-up Q → A`, with the following constraints:

- Each `Q` MUST be a complete interrogative sentence (ending with a question mark) and self-contained — readable on its own, without depending on the surrounding prose for its subject.
- The lead `Q` (the first question in the chain) MUST be direct and answerable in one sentence.
- Each `A` MUST start with a direct, complete answer in one sentence before any elaboration. Code, lists, or longer explanation MAY follow that opening sentence within the same answer block.
- Every follow-up `Q` MUST share the lead question's intent and deepen it (a clarification, a corner case, a related operation on the same subject). A question that introduces a new topic is NOT a follow-up; it MUST start a new chain.
- A chain has exactly one lead question. The minimum viable chain is one lead and two follow-ups (three Q→A pairs in total).

### Per-page minimums

The following minimums apply to content pages:

- **Docs and guides** (`/docs/*`) and **examples** (`/examples/*`): MUST contain at least **one conversational chain** with **at least three Q→A pairs** in total (one lead plus at least two follow-ups).
- **API and reference** (`/api`): MUST contain **at least one Q→A pair per documented endpoint, type, or topic**. Follow-ups are not mandatory on this page family, but the lead-question rules above still apply.
- **Exempt pages** (no minimum enforced): `/` (landing), `/changelog`, `/releases/*`, `/security`, `/compatibility`, `/contributing`. These pages MAY use Q→A where it fits naturally, but no count is required.

### HTML structure

Each chain MUST be emitted in the page's HTML as:

- A `<section data-conversation="<slug>">` wrapper around the entire chain. The `<slug>` MUST be unique within the page, written in kebab-case, and stable across edits — it is the citation anchor that human readers and AI engines may link to.
- The lead question and every follow-up question MUST use the same heading level. On a typical page where the `<h1>` is the page title and `<h2>` opens the section that contains the chain, every question in the chain MUST be `<h3>`.
- Each answer MUST follow its question and SHOULD be a `<p>` element. An answer MAY use `<ol>`, `<ul>`, `<pre>`, or `<table>` after the opening sentence where the content is naturally an ordered list, an unordered list, code, or tabular data.
- Pages with multiple chains MUST place each chain in its own `<section data-conversation="…">` wrapper. Chains MUST NOT be nested.

### JSON-LD coupling

- Every `Question` from every chain on the page MUST be flattened into the page's single `FAQPage` block, as a single `mainEntity` array. Exactly one `FAQPage` block is emitted per page.
- The JSON-LD does NOT distinguish lead questions from follow-up questions. The conversational grouping is preserved only in the HTML, via the `data-conversation` wrappers.
- This applies whenever the page contains at least three Q→A pairs in total across all its chains, consistent with the threshold in `## FAQPage and HowTo structured data` above.

All JSON-LD constraints (entity graph, field completeness, validation gate) are defined in `specification/structured-data.md`.

### Non-goals

The following techniques are NOT used by this site and MUST NOT be introduced:

- Keyword density targets, keyword repetition, or any form of keyword stuffing.
- Brand-term repetition for ranking purposes.
- Synonym variation for the purpose of matching search-query phrasings.

Content is optimised for the question it answers, not for the words it contains.

### Compliance window

This contract is forward-looking. Existing pages MUST be brought into compliance during the next content-curation pass. New pages MUST comply on first write.

## Example walkthrough shape

The example pages are the most program-shaped content on the site: each one corresponds to a real, runnable program in the upstream MuxMaster repository. A code dump is not didactic, however. A reader landing on an example wants to learn what each part of the program does and why it is written that way; an AI answer engine needs prose paragraphs it can quote when answering "how do I build X with MuxMaster?". This section defines the shape every example page MUST take so that both audiences are served.

The following nine sub-rules apply.

1. **Scope.** Applies to every page under `/examples/<name>` whose body is a runnable program. Does not apply to docs, the API page, the landing page, or any reference page.

2. **Required structure.** An example page MUST be authored as an ordered sequence of `## Step N — <name>` H2 headings (N is a 1-indexed contiguous integer, `<name>` a concise human-readable label). Headings start at `## Step 1 — …` and continue without gaps.

3. **Per-step body.** Each `## Step N — …` section MUST open with at least one paragraph of didactic prose stating the step's purpose in one sentence (definition-first), then expand as needed. After the prose, the section MUST contain at most one fenced Go code excerpt showing only the lines relevant to that step (typically 3–40 lines). A step MAY also contain follow-up prose after the code excerpt to explain non-obvious consequences (error handling, security implications, performance trade-offs).

4. **No full source dump.** The page MUST NOT contain the full program as a single fenced block. Readers who want the entire file follow the `## Upstream source` link at the end, which already points at the canonical file in `https://github.com/FlavioCFOliveira/MuxMaster/tree/v<version>/examples/<name>`. Duplicating the full source bloats the LCP, dilutes the prose-to-code ratio that drives AI citation, and creates a second copy that can drift from upstream.

5. **Step ordering.** Steps SHOULD reflect the order in which the program executes or the order in which a reader would write the program from scratch. Where two orderings are equally defensible (e.g. "configure logger" vs "configure router"), the curator chooses the one that minimises forward references in the prose.

6. **Code-excerpt rules.** Each excerpt MUST be valid Go (or the relevant language) on its own as far as a reader's eye can tell — incomplete bodies are signalled with `// …` so the elision is explicit; ellipsis MUST NOT silently truncate the middle of a function. Imports SHOULD be elided unless the step's purpose is to discuss imports. The excerpt SHOULD be lifted verbatim from the upstream source so a reader who follows the upstream link sees the same lines.

7. **JSON-LD coupling.** When an example complies with this shape, the renderer's existing HowTo emitter (see `## FAQPage and HowTo structured data`) emits a HowTo JSON-LD block listing every step. The list of pages that emit HowTo is therefore data-driven (every example), not hand-curated; `specification/structured-data.md` § HowTo and the validation gate enforce field completeness.

8. **Question-Oriented Content remains in force.** The per-page minimums in `## Question-Oriented Content` apply unchanged: every example MUST still carry at least one conversational chain of ≥3 Q→A pairs, wrapped in `<section data-conversation="…">`, after the walkthrough and before the upstream-source link.

9. **Length guidance.** A typical example walkthrough has 5–12 steps. Pages with fewer than 3 steps are rejected (the page is not a walkthrough, it is a single excerpt — promote it to a doc page or fold it into another example). Pages with more than 15 steps are reviewed for over-segmentation.

## Content-shape rules

These rules apply to every page in addition to the language and tone rules in `overview.md`.

- **Definition-first.** The first sentence of every section MUST state the fact. The mechanism, justification, or example follows.
- **Self-contained paragraphs.** Each paragraph MUST be readable on its own without depending on a prior paragraph for subject or definition. LLMs frequently quote single paragraphs.
- **Concrete numbers.** Replace "fast" with measured numbers, "small" with byte counts, "supported" with versions. Cite the source file and line for any number that could be checked (the benchmarks page is the model).
- **Inline citations.** When the site quotes upstream code or numbers, the citation appears inline as a relative reference (e.g. "from `mux.go`") and as a link to the upstream file on GitHub.
- **No marketing voice.** No comparisons against named competitors unless a benchmark or fact supports the claim.
- **Q-shaped FAQs.** Where a page contains an FAQ, each question MUST be phrased as a complete interrogative sentence and the answer MUST start with a direct, complete answer in one sentence before any elaboration. See `## Question-Oriented Content` above for the conversational chain structure and the per-page minimums that govern when Q→A content is required.

## Cite-ability

- Every page MUST include a stable canonical URL (HTML and `.md`) so that AI answer engines can cite it.
- The page MUST include a date of last update (rendered visibly in the footer of the article body), sourced from the underlying file's mtime in `/content/`.
- The page MUST include the upstream source path it derives from, when applicable, so that AI engines can attribute the original location. For pages whose `/content/` mirror was produced by the `content-curator` agent (see `content-sources.md`), the citation points at the upstream file on GitHub.
