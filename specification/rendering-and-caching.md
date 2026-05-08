---
title: Rendering and caching
purpose: Define the SSR pipeline, the static-tending architecture, the in-process render store, the HTTP cache headers per route family, and the ETag/Last-Modified strategy.
owners: specification-manager; review by seo-specialist (cache headers, Core Web Vitals).
last-updated: 2026-05-08
status: ratified
---

# Rendering and caching

## Rendering model

- The site uses **Server-Side Rendering** with Go's standard library `html/template`.
- Every HTML response is fully rendered server-side before being sent. Pages MUST be useful with JavaScript disabled.
- The site is served by **MuxMaster** (`github.com/FlavioCFOliveira/MuxMaster`). All routes are registered on a `*mux.Mux`. This is non-negotiable per `../CLAUDE.md` ("MuxMaster as router" dogfooding).
- All content is read from `/content/` in this repository, populated by the `content-curator` agent at development time (see `content-sources.md`). The runtime binary never reads the upstream `../MuxMaster` tree.
- The Markdown-to-HTML pipeline MUST preserve fenced code blocks with language hints, and MUST emit anchored heading IDs (`<h2 id="kebab-case">`) so the in-page TOC and JSON-LD `BreadcrumbList` and FAQPage anchors work.

## Static-tending architecture

The site is **static-tending**. Every public route is **pre-rendered at startup**; **per-request rendering does not exist**. The same URL MUST return the same bytes for the same build identity. The server introduces **no per-request dynamism** beyond what client capability headers (`Accept-Encoding`) require. Server-side templates are an implementation detail of how those bytes are produced; they are never a request-time decision the client perceives.

Concretely:

- The server materialises every public route to a `[]byte` once during startup, holds the bytes in memory, and serves the recorded bytes for every subsequent request.
- `html/template` execution and Markdown rendering happen during startup, never during request handling.
- There is no lazy cache, no `mtime` watching, and no live templating. The previous Category A vs Category B distinction has been removed: every route is materialised the same way.

Static assets (`/assets/<hash>/...`) are served directly from disk via `mux.ServeFiles` and are content-addressed (see `seo.md`, `brand-and-visual.md`, and `deployment.md`); the static-tending architecture does not apply to them.

The operational endpoint `/healthz` does not render content and MUST set `Cache-Control: no-store`.

## Pre-rendered routes (day-one)

Every route below is materialised to bytes once at startup. The list is exhaustive for v1.

- `/` (landing)
- `/docs/`, `/docs/.md` (docs index)
- `/docs/<section>`, `/docs/<section>.md` (eleven sections; see `information-architecture.md` for the list)
- `/api`, `/api.md`
- `/examples/`, `/examples/.md` (examples index)
- `/examples/<name>`, `/examples/<name>.md` (eight examples; see `information-architecture.md` for the list)
- `/benchmarks`, `/benchmarks.md`
- `/changelog`, `/changelog.md`
- `/releases/<v>`, `/releases/<v>.md`
- `/security`, `/security.md`
- `/compatibility`, `/compatibility.md`
- `/contributing`, `/contributing.md`
- `/llms.txt`, `/llms-full.txt`, `/robots.txt`, `/sitemap.xml`
- `/404`, `/500` (pre-rendered error templates emitted by the corresponding error handlers)

## Materialisation rule

The server MUST render each route to a `[]byte` once during startup, after **all** of the following preconditions are satisfied, in this order:

1. The required files under `/content/` are present and readable (see `content-sources.md`).
2. The version label is parsed from `/content/changelog.md`.
3. Compiled-asset filenames (e.g. the hashed CSS bundle) have been resolved.
4. Routes are registered on the `*mux.Mux` (so `/llms.txt`, `/llms-full.txt`, and `/sitemap.xml` can enumerate them).

After materialisation, the server MUST serve the recorded bytes for every request to a public route. Per-request `html/template` execution MUST NOT happen.

The `404` and `500` templates are pre-rendered to bytes in the same way and emitted by the corresponding error handlers; the HTTP status code is set on the response (see "Error responses" below).

## Recompute trigger

- Pre-rendered bytes are recomputed only on **process restart**.
- A change to `/content/` (committed by the `content-curator` agent during a sync; see `content-sources.md`) requires a redeploy and restart to take effect on the served bytes.
- A change to a registered route or a template counts as a code-level event; it triggers a rebuild and a redeploy, not a hot recompute.
- The runtime MUST NOT watch `/content/` for changes. Deploys are the unit of update.

## In-process render store

- The render store MUST be process-local. There is no distributed cache.
- The render store MUST NOT be persisted to disk.
- Each public route maps to exactly one `[]byte` entry, populated at startup and never invalidated except by restart. There is no eviction policy.
- The store covers the HTML and `.md` representations of every public route plus `/llms.txt`, `/llms-full.txt`, `/robots.txt`, `/sitemap.xml`, `/404`, and `/500`.

## HTTP cache headers per route family

| Route family | `Cache-Control` |
| --- | --- |
| `/` (landing) | `public, max-age=300, stale-while-revalidate=60` |
| `/docs/`, `/docs/.md`, `/examples/`, `/examples/.md` (section indexes) | `public, max-age=300, stale-while-revalidate=60` |
| `/docs/<section>`, `/docs/<section>.md` | `public, max-age=600, stale-while-revalidate=120` |
| `/api`, `/api.md` | `public, max-age=600, stale-while-revalidate=120` |
| `/examples/<name>`, `/examples/<name>.md` | `public, max-age=600, stale-while-revalidate=120` |
| `/benchmarks`, `/benchmarks.md` | `public, max-age=600, stale-while-revalidate=120` |
| `/changelog`, `/changelog.md` | `public, max-age=300, stale-while-revalidate=60` |
| `/releases/<v>`, `/releases/<v>.md` | `public, max-age=86400, immutable` (release notes are immutable per release) |
| `/security`, `/compatibility`, `/contributing` (and `.md` companions) | `public, max-age=600, stale-while-revalidate=120` |
| `/llms.txt`, `/llms-full.txt`, `/robots.txt` | `public, max-age=300` |
| `/sitemap.xml` | `public, max-age=300` |
| `/404`, `/500` (error responses) | `no-store` |
| Static assets under `/assets/<hash>/...` | `public, max-age=31536000, immutable` |
| `/healthz` | `no-store` |

## ETag and Last-Modified

- For every HTML and Markdown response, the server MUST set:
  - `ETag: "<sha256 of body, base64-url, first 16 chars>"` (strong validator). The body bytes are stable for the process lifetime.
  - `Last-Modified: <RFC 7231 date>`. The value MUST be the **process start time**, since pre-rendered bytes are recomputed only on restart.
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
- The server MUST NOT introduce any other form of per-request dynamism. The same URL returns the same bytes for the same build identity (see `out-of-scope.md`).

## Error responses

- `404 Not Found` MUST render an HTML page from the same template family as the rest of the site (header, footer, breadcrumb up to `Home`), with a clear message and three suggested links: `/docs/`, `/api`, `/examples/`. The page is pre-rendered at startup and served as bytes by the `404` handler. `Cache-Control: no-store` on the response.
- `500 Internal Server Error` MUST render a minimal HTML page with the same chrome and a generic message. The page is pre-rendered at startup. `Cache-Control: no-store`.
- Both error pages MUST set the correct HTTP status code in addition to the body.
