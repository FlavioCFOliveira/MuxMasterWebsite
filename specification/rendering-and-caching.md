---
title: Rendering and caching
purpose: Define the SSR pipeline, the static-tending architecture, the in-process render cache, the HTTP cache headers per route family, and the ETag/Last-Modified strategy.
owners: specification-manager; review by seo-specialist (cache headers, Core Web Vitals).
last-updated: 2026-05-08
status: ratified
---

# Rendering and caching

## Rendering model

- The site uses **Server-Side Rendering** with Go's standard library `html/template`.
- Every HTML response is fully rendered server-side before being sent. Pages MUST be useful with JavaScript disabled.
- The site is served by **MuxMaster** (`github.com/FlavioCFOliveira/MuxMaster`). All routes are registered on a `*mux.Mux`. This is non-negotiable per `../CLAUDE.md` ("MuxMaster as router" dogfooding).
- The renderer is single-pass per execution: the handler resolves the source file (per `content-sources.md`), parses Markdown to HTML (for Markdown routes), populates the page-template data, and produces the response bytes.
- The Markdown-to-HTML pipeline MUST preserve fenced code blocks with language hints, and MUST emit anchored heading IDs (`<h2 id="kebab-case">`) so the in-page TOC and JSON-LD `BreadcrumbList` and FAQPage anchors work.

## Static-tending architecture

The site is **static-tending**. The same URL MUST return the same bytes for the same build identity and the same upstream-source `mtime`. The server introduces **no per-request dynamism** beyond what client capability headers (`Accept-Encoding`) require. Server-side templates are an implementation detail of how those bytes are produced; they are never a request-time decision the client perceives. Live templating exists exclusively to keep documentation in sync with the upstream `../MuxMaster` source of truth, not to enable per-request variability.

To honour that principle, every public route belongs to exactly one of two categories:

- **Category A — Pre-rendered at startup.** Routes whose output does not depend on any live upstream file. The server materialises these to byte-buffers in memory once during startup and serves the cached bytes thereafter.
- **Category B — Lazy-cache live templating.** Routes whose output is derived from files under `${MUXMASTER_SOURCE_DIR}`. The server runs `html/template` (and Markdown rendering, where applicable) on the first request and on every subsequent request whose cached entry has been invalidated by an upstream `mtime` change. Cached bytes are held in memory and served until invalidated.

Static assets (`/assets/<hash>/...`) are served directly from disk via `mux.ServeFiles` and are content-addressed (see `seo.md` and `deployment.md`); they belong to neither category.

The operational endpoint `/healthz` is unaffected by either category; it does not render content and MUST set `Cache-Control: no-store`.

## Category A — Pre-rendered at startup

### Routes (day-one)

- `/` (landing)
- `/docs/`, `/docs/.md` (docs index; generated at startup from the registered route table, optionally prepended with `/content/site/docs-index.md` when present — see `content-sources.md`)
- `/examples/`, `/examples/.md` (examples index; generated at startup from the registered route table, optionally prepended with `/content/site/examples-index.md` when present — see `content-sources.md`)
- `/404` (error template)
- `/500` (error template)
- `/robots.txt`
- `/llms.txt`
- `/llms-full.txt`
- `/sitemap.xml`

### Materialisation rule

- The server MUST render each Category A route to a `[]byte` once during startup, after **all** of the following preconditions are satisfied, in this order:
  1. Routes have been registered on the `*mux.Mux` (so `/llms.txt`, `/llms-full.txt`, and `/sitemap.xml` can enumerate them).
  2. Upstream metadata required by the recipe has been loaded (currently only the version label parsed from `../MuxMaster/CHANGELOG.md`; see `content-sources.md`).
  3. Compiled-asset filenames (e.g. the hashed CSS bundle) have been resolved.
- After materialisation, the server MUST serve the recorded bytes for every request to a Category A route. Per-request `html/template` execution MUST NOT happen for Category A.
- The `404` and `500` templates are pre-rendered to bytes in the same way and emitted by the corresponding error handlers; the HTTP status code is set on the response (per "Error responses" below).

### Recompute trigger

- Category A bytes are recomputed only on **process restart**.
- A change to a registered route counts as a code-level event; it triggers a rebuild and a redeploy, not a hot recompute.
- `mtime` changes under `${MUXMASTER_SOURCE_DIR}` MUST NOT invalidate Category A entries. The only upstream input today (the version label) is captured at startup and a restart is required to roll it forward (see `content-sources.md`).

## Category B — Lazy-cache live templating

### Routes (day-one)

- `/docs/<section>`, `/docs/<section>.md` (the docs index `/docs/` is Category A — see above)
- `/api`, `/api.md`
- `/examples/<name>`, `/examples/<name>.md` (the examples index `/examples/` is Category A — see above)
- `/benchmarks`, `/benchmarks.md` (extracted live from `${MUXMASTER_SOURCE_DIR}/README.md` per the contract in `content-sources.md`)
- `/changelog`, `/changelog.md`
- `/releases/<v>`, `/releases/<v>.md`
- `/security`, `/security.md`
- `/compatibility`, `/compatibility.md`
- `/contributing`, `/contributing.md`

### Cache key and invalidation

- The cache key for each entry is the triple `(route, source-file mtime, build-id)`. The `build-id` is a constant generated at compile time and changes on every deploy.
- An entry is valid only while the source file's `mtime` matches the entry's recorded `mtime`. On mismatch, the server MUST recompute the entry and replace it.
- The first request to a cold entry pays the render cost; every subsequent request served from a still-valid entry returns cached bytes without executing `html/template`.

### Why this category exists

- Category B exists because the upstream MuxMaster repository is the single source of truth and its files MAY change between deploys (for example, when the project owner updates `../MuxMaster` locally before cutting a release). Live templating keeps the site faithful to upstream without requiring a website redeploy for every documentation edit.

## In-process render cache

- The cache MUST be process-local. There is no distributed cache.
- The cache MUST NOT be persisted to disk.
- The cache covers both categories: Category A entries are populated at startup and never invalidated except by restart; Category B entries are populated lazily and invalidated by `mtime` mismatch.
- Day-one sizing rule: at most one entry per known route, with no eviction policy beyond invalidation. Page count is small and bounded by the registered route table.

## HTTP cache headers per route family

| Category | Route family | `Cache-Control` |
| --- | --- | --- |
| A | `/` (landing) | `public, max-age=300, stale-while-revalidate=60` |
| A | `/docs/`, `/docs/.md`, `/examples/`, `/examples/.md` (section indexes) | `public, max-age=300, stale-while-revalidate=60` |
| B | `/docs/<section>`, `/docs/<section>.md` | `public, max-age=600, stale-while-revalidate=120` |
| B | `/api`, `/api.md` | `public, max-age=600, stale-while-revalidate=120` |
| B | `/examples/<name>`, `/examples/<name>.md` | `public, max-age=600, stale-while-revalidate=120` |
| B | `/benchmarks`, `/benchmarks.md` | `public, max-age=600, stale-while-revalidate=120` |
| B | `/changelog`, `/changelog.md` | `public, max-age=300, stale-while-revalidate=60` |
| B | `/releases/<v>`, `/releases/<v>.md` | `public, max-age=86400, immutable` (release notes are immutable per release) |
| B | `/security`, `/compatibility`, `/contributing` (and `.md` companions) | `public, max-age=600, stale-while-revalidate=120` |
| A | `/llms.txt`, `/llms-full.txt`, `/robots.txt` | `public, max-age=300` |
| A | `/sitemap.xml` | `public, max-age=300` |
| A | `/404`, `/500` (error responses) | `no-store` |
| (assets) | Static assets under `/assets/<hash>/...` | `public, max-age=31536000, immutable` |
| (op) | `/healthz` | `no-store` |

## ETag and Last-Modified

- For every HTML and Markdown response, regardless of category, the server MUST set:
  - `ETag: "<sha256 of body, base64-url, first 16 chars>"` (strong validator). The body bytes are stable for both categories — Category A bytes are fixed for the process lifetime; Category B bytes are fixed for the lifetime of a cache entry.
  - `Last-Modified: <RFC 7231 date>`. For Category B the value is the source file's `mtime`. For Category A the value is the process start time, since Category A bytes are recomputed only on restart.
- The server MUST honour `If-None-Match` and `If-Modified-Since` and respond `304 Not Modified` when validation succeeds.
- For static assets the filename is hashed at build time, so `ETag` and `Last-Modified` MAY be omitted in favour of `Cache-Control: immutable`.

## Compression

- The server MUST emit `Content-Encoding: gzip` when the client advertises it via `Accept-Encoding`.
- The server MAY emit `Content-Encoding: br` when the reverse proxy is not handling Brotli; in production deployments, the reverse proxy is expected to handle Brotli (see `deployment.md`).
- The server MUST set `Vary: Accept-Encoding` on compressible responses.
- `Accept-Encoding` is the **only** request header that may influence the response bytes. No other client signal (`Accept`, `Accept-Language`, cookies, query strings, user-agent) is permitted to vary the body.

## Content type

- HTML responses: `Content-Type: text/html; charset=utf-8`.
- Markdown companions: `Content-Type: text/markdown; charset=utf-8`.
- `llms.txt`, `llms-full.txt`, `robots.txt`: `Content-Type: text/plain; charset=utf-8`.
- `sitemap.xml`: `Content-Type: application/xml; charset=utf-8`.
- Static CSS: `Content-Type: text/css; charset=utf-8`.
- PNG images: `Content-Type: image/png`. WebP: `image/webp`. AVIF: `image/avif`.

## Content negotiation

- Content negotiation via `Accept` is **not** used to switch between HTML and Markdown. The Markdown representation is reachable only at the explicit `.md` URL.
- The server MUST NOT introduce any other form of per-request dynamism. The same URL returns the same bytes for the same build identity and the same upstream-source `mtime` (see `out-of-scope.md`).

## Error responses

- `404 Not Found` MUST render an HTML page from the same template family as the rest of the site (header, footer, breadcrumb up to `Home`), with a clear message and three suggested links: `/docs/`, `/api`, `/examples/`. The page is Category A: pre-rendered at startup and served as bytes by the `404` handler. `Cache-Control: no-store` on the response.
- `500 Internal Server Error` MUST render a minimal HTML page with the same chrome and a generic message. The page is Category A: pre-rendered at startup. `Cache-Control: no-store`.
- Both error pages MUST set the correct HTTP status code in addition to the body.
