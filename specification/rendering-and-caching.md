---
title: Rendering and caching
purpose: Define the SSR pipeline, the in-process render cache, the HTTP cache headers per route family, and the ETag/Last-Modified strategy.
owners: specification-manager; review by seo-specialist (cache headers, Core Web Vitals).
last-updated: 2026-05-08
status: ratified
---

# Rendering and caching

## Rendering model

- The site uses **Server-Side Rendering** with Go's standard library `html/template`.
- Every HTML response is fully rendered server-side before being sent. Pages MUST be useful with JavaScript disabled.
- The site is served by **MuxMaster** (`github.com/FlavioCFOliveira/MuxMaster`). All routes are registered on a `*mux.Mux`. This is non-negotiable per `../CLAUDE.md` ("MuxMaster as router" dogfooding).
- The renderer is single-pass per request: handler resolves the source file (per `content-sources.md`), parses Markdown to HTML (for Markdown routes), populates the page-template data, and writes the response.
- The Markdown-to-HTML pipeline MUST preserve fenced code blocks with language hints, and MUST emit anchored heading IDs (`<h2 id="kebab-case">`) so the in-page TOC and JSON-LD `BreadcrumbList` and FAQPage anchors work.

## Computed at startup

The following data is computed once at startup and held in memory; a restart is required to refresh it:

- The current MuxMaster version label (read from `CHANGELOG.md`, see `overview.md`).
- The list of registered routes (used by `sitemap.xml`, `llms.txt`, and `llms-full.txt`).
- The hashed asset filename for the compiled CSS bundle (see `brand-and-visual.md`).
- The required-file presence check for `MUXMASTER_SOURCE_DIR` (see `content-sources.md`).

## Computed per request

- Resolution of the source file path.
- Markdown rendering (if not cached, see below).
- Template execution and final HTML assembly.

## In-process render cache

- The server MAY hold a per-route render cache in memory.
- Cache key: `(route, source-file mtime, build-id)`. The `build-id` is a constant generated at compile time and changes on every deploy.
- Cache invalidation: a cache entry is valid only while the source file's `mtime` matches the entry's recorded `mtime`. On mismatch, the entry is recomputed and replaced.
- The cache MUST NOT be persisted to disk.
- The cache MUST be process-local. There is no distributed cache.
- The cache size limit is **TBD** during implementation; a sensible day-one default is "at most one entry per known route, no eviction policy beyond invalidation". Page count is small and bounded.

## HTTP cache headers per route family

| Route family | `Cache-Control` |
| --- | --- |
| `/` (landing) | `public, max-age=300, stale-while-revalidate=60` |
| `/docs/`, `/docs/<section>`, `/docs/<section>.md` | `public, max-age=600, stale-while-revalidate=120` |
| `/api`, `/api.md` | `public, max-age=600, stale-while-revalidate=120` |
| `/examples/`, `/examples/<name>`, `/examples/<name>.md` | `public, max-age=600, stale-while-revalidate=120` |
| `/benchmarks`, `/benchmarks.md` | `public, max-age=600, stale-while-revalidate=120` |
| `/changelog`, `/changelog.md` | `public, max-age=300, stale-while-revalidate=60` |
| `/releases/<v>`, `/releases/<v>.md` | `public, max-age=86400, immutable` (release notes are immutable per release) |
| `/security`, `/compatibility`, `/contributing` (and `.md` companions) | `public, max-age=600, stale-while-revalidate=120` |
| `/llms.txt`, `/llms-full.txt`, `/robots.txt` | `public, max-age=300` |
| `/sitemap.xml` | `public, max-age=300` |
| Static assets under `/assets/<hash>/...` | `public, max-age=31536000, immutable` |
| `/healthz` | `no-store` |

## ETag and Last-Modified

- For every HTML and Markdown response, the server MUST set:
  - `ETag: "<sha256 of body, base64-url, first 16 chars>"` (strong validator).
  - `Last-Modified: <RFC 7231 date of the source file's mtime>`.
- The server MUST honour `If-None-Match` and `If-Modified-Since` and respond `304 Not Modified` when validation succeeds.
- For static assets the filename is hashed at build time, so `ETag` and `Last-Modified` MAY be omitted in favour of `Cache-Control: immutable`.

## Compression

- The server MUST emit `Content-Encoding: gzip` when the client advertises it via `Accept-Encoding`.
- The server MAY emit `Content-Encoding: br` when the reverse proxy is not handling Brotli; in production deployments, the reverse proxy is expected to handle Brotli (see `deployment.md`).
- The server MUST set `Vary: Accept-Encoding` on compressible responses.

## Content type

- HTML responses: `Content-Type: text/html; charset=utf-8`.
- Markdown companions: `Content-Type: text/markdown; charset=utf-8`.
- `llms.txt`, `llms-full.txt`, `robots.txt`: `Content-Type: text/plain; charset=utf-8`.
- `sitemap.xml`: `Content-Type: application/xml; charset=utf-8`.
- Static CSS: `Content-Type: text/css; charset=utf-8`.
- PNG images: `Content-Type: image/png`. WebP: `image/webp`. AVIF: `image/avif`.

## Content negotiation

- Content negotiation via `Accept` is **not** used to switch between HTML and Markdown. The Markdown representation is reachable only at the explicit `.md` URL.

## Error responses

- `404 Not Found` MUST render an HTML page from the same template family as the rest of the site (header, footer, breadcrumb up to `Home`), with a clear message and three suggested links: `/docs/`, `/api`, `/examples/`. `Cache-Control: no-store` on the response.
- `500 Internal Server Error` MUST render a minimal HTML page with the same chrome and a generic message. `Cache-Control: no-store`.
- Both error pages MUST set the correct HTTP status code in addition to the body.
