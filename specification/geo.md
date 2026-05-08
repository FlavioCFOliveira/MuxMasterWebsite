---
title: GEO contract
purpose: Define the Generative Engine Optimization contract — llms.txt artefacts, Markdown companions, AI crawler allowlist, FAQPage and HowTo structured data, and content-shape rules.
owners: geo-specialist (final review); review by seo-specialist (structured-data overlap), ux-specialist (content-shape and tone).
last-updated: 2026-05-08
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
- `DiffBot` / `Diffbot` (Diffbot)
- `OmgiliBot` (Webz)
- `Amazonbot` (Amazon)
- `meta-externalagent` (Meta)

For each, the entry takes the form:

```
User-agent: <name>
Allow: /
Disallow: /healthz
```

The allowlist MUST be re-evaluated at least once per release of the site. Removal of a bot from the allowlist requires explicit ratification.

## FAQPage and HowTo structured data

- Pages with three or more explicit Q→A pairs (a `<h2>` or `<h3>` phrased as a question, immediately followed by a paragraph answer) MUST emit a JSON-LD `FAQPage` block listing those pairs.
- Pages with an ordered, named list of steps (typically the Getting Started page and some examples) MUST emit a JSON-LD `HowTo` block listing those steps.
- Both blocks MUST live alongside the SEO JSON-LD blocks defined in `seo.md`. Duplicate facts in two blocks are acceptable.

## Content-shape rules

These rules apply to every page in addition to the language and tone rules in `overview.md`.

- **Definition-first.** The first sentence of every section MUST state the fact. The mechanism, justification, or example follows.
- **Self-contained paragraphs.** Each paragraph MUST be readable on its own without depending on a prior paragraph for subject or definition. LLMs frequently quote single paragraphs.
- **Concrete numbers.** Replace "fast" with measured numbers, "small" with byte counts, "supported" with versions. Cite the source file and line for any number that could be checked (the benchmarks page is the model).
- **Inline citations.** When the site quotes upstream code or numbers, the citation appears inline as a relative reference (e.g. "from `mux.go`") and as a link to the upstream file on GitHub.
- **No marketing voice.** No comparisons against named competitors unless a benchmark or fact supports the claim.
- **Q-shaped FAQs.** Where a page contains an FAQ, each question MUST be phrased as a complete interrogative sentence and the answer MUST start with a direct, complete answer in one sentence before any elaboration.

## Cite-ability

- Every page MUST include a stable canonical URL (HTML and `.md`) so that AI answer engines can cite it.
- The page MUST include a date of last update (rendered visibly in the footer of the article body), sourced from the underlying file's mtime in `/content/`.
- The page MUST include the upstream source path it derives from, when applicable, so that AI engines can attribute the original location. For pages whose `/content/` mirror was produced by the `content-curator` agent (see `content-sources.md`), the citation points at the upstream file on GitHub.
