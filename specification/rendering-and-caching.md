---
title: Rendering and caching
purpose: Define the SSR pipeline, the static-tending architecture, the in-process render store, the HTTP cache headers per route family, and the ETag/Last-Modified strategy.
owners: specification-manager; review by seo-specialist (cache headers, Core Web Vitals).
last-updated: 2026-09-26
status: ratified
---

# Rendering and caching

## Rendering model

- The site uses **Server-Side Rendering** with Go's standard library `html/template`.
- Every HTML response is fully rendered server-side before being sent. Pages MUST be useful with JavaScript disabled.
- The site is served by **MuxMaster** (`github.com/FlavioCFOliveira/MuxMaster`). All routes are registered on a `*muxmaster.Mux`. This is non-negotiable per `../CLAUDE.md` ("MuxMaster as router" dogfooding).
- All content is read from `/content/` in this repository, populated by the `content-curator` agent at development time (see `content-sources.md`). The runtime binary never reads the upstream `../MuxMaster` tree.
- The Markdown-to-HTML pipeline MUST preserve fenced code blocks with language hints, and MUST emit anchored heading IDs (`<h2 id="kebab-case">`) so the in-page TOC and JSON-LD `BreadcrumbList` and FAQPage anchors work.
- The Markdown-to-HTML pipeline MUST strip any leading YAML frontmatter block (delimited by `---` lines, per `content-sources.md` "Markdown companions") **before** passing the source to the CommonMark renderer. Frontmatter is metadata consumed by the JSON-LD date resolver (`structured-data.md` § "Date sources for embedded content"); it MUST NOT appear in the rendered HTML body, MUST NOT be parsed as a thematic break + setext heading, and MUST NOT contribute an entry to the in-page TOC. This rule applies to every caller of the Markdown-to-HTML pipeline, including doc-family pages and any intro snippet folded into a section index.

## Static-tending architecture

The site is **static-tending**. Every public route is **pre-rendered at startup**; **per-request rendering does not exist**. The same URL MUST return the same bytes for the same build identity. The server introduces **no per-request dynamism** beyond what client capability headers (`Accept-Encoding`) require. Server-side templates are an implementation detail of how those bytes are produced; they are never a request-time decision the client perceives.

Concretely:

- The server materialises every public route to bytes once during startup (an identity and a gzip representation; see "Materialisation rule"), holds the bytes in memory, and serves the recorded bytes for every subsequent request.
- `html/template` execution and Markdown rendering happen during startup, never during request handling.
- There is no lazy cache, no `mtime` watching, and no live templating. The previous Category A vs Category B distinction has been removed: every route is materialised the same way.

Static assets are served at `/static/*filepath`. They are not rendered from `/content/`; they are loaded from the static directory into memory at startup and served from memory for the lifetime of the process, as defined in "Static assets" below.

The operational endpoint `/healthz` does not render content and MUST set `Cache-Control: no-store`.

## Router configuration

The site is a public showcase of MuxMaster v1.3.0's high-performance strategy (upstream `../MuxMaster/docs/max-performance.md`). The router MUST be configured as follows:

- The `*muxmaster.Mux` MUST set `PoolRequestBundle = true`.
- The `*muxmaster.Mux` MUST set `PoolFastParams = true`.
- Cross-cutting middleware (the policy chain that applies to every request) MUST be registered with `Mux.Pre` only. `Mux.Use` MUST NOT be called on the site's router. `Mux.Pre` wraps the whole dispatch, so every route, whether registered with `Handle` or with `HandleFast`, runs the same policy chain, and `HandleFast` routes remain legal (MuxMaster panics when a `HandleFast` route is registered after `Mux.Use`).

## Handler lifetime contract

Both pools recycle their objects the instant a handler returns (upstream `../MuxMaster/docs/max-performance.md` § Lifetime contract — what you must not do). Every handler and every middleware on the site MUST therefore honour the following rules:

- A handler or middleware MUST NOT retain the `*http.Request`, its context, its `Body`, or its `muxmaster.Params` after it returns.
- A handler MUST NOT spawn a goroutine that reads the request, its context, its `Body`, or its `muxmaster.Params`.
- `httputil.ReverseProxy` MUST NOT be used behind the pooled mux.

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
- `/built-with-muxmaster`, `/built-with-muxmaster.md`
- `/llms.txt`, `/llms-full.txt`, `/robots.txt`, `/sitemap.xml`
- `/404`, `/500` (pre-rendered error templates emitted by the corresponding error handlers)

## Materialisation rule

The server MUST render each route to a `[]byte` once during startup, after **all** of the following preconditions are satisfied, in this order:

1. The required files under `/content/` are present and readable (see `content-sources.md`).
2. The version label is parsed from `/content/changelog.md`.
3. Compiled-asset filenames (e.g. the hashed CSS bundle) have been resolved.
4. Routes are registered on the `*muxmaster.Mux` (so `/llms.txt`, `/llms-full.txt`, and `/sitemap.xml` can enumerate them).

After materialisation, the server MUST serve the recorded bytes for every request to a public route. Per-request `html/template` execution MUST NOT happen.

The `404` and `500` templates are pre-rendered to bytes in the same way and emitted by the corresponding error handlers; the HTTP status code is set on the response (see "Error responses" below).

For every pre-rendered route, materialisation MUST produce two representations of the body, both held in memory:

- the **identity representation**: the rendered bytes, uncompressed;
- the **gzip representation**: the identity bytes compressed once, at materialisation time, with the Go standard library package `compress/gzip`. It is kept only when it is smaller than the identity bytes; a route whose gzip bytes are not smaller has the identity representation only.

## Recompute trigger

- Pre-rendered bytes are recomputed only on **process restart**.
- A change to `/content/` (committed by the `content-curator` agent during a sync; see `content-sources.md`) requires a redeploy and restart to take effect on the served bytes.
- A change to a registered route or a template counts as a code-level event; it triggers a rebuild and a redeploy, not a hot recompute.
- The runtime MUST NOT watch `/content/` for changes. Deploys are the unit of update.

## In-process render store

- The render store MUST be process-local. There is no distributed cache.
- The render store MUST NOT be persisted to disk.
- Each public route maps to exactly one entry holding its identity and gzip representations (see "Materialisation rule"), populated at startup and never invalidated except by restart. There is no eviction policy.
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
| `/built-with-muxmaster`, `/built-with-muxmaster.md` | `public, max-age=600, stale-while-revalidate=120` |
| `/llms.txt`, `/llms-full.txt`, `/robots.txt` | `public, max-age=300` |
| `/sitemap.xml` | `public, max-age=300` |
| `/404`, `/500` (error responses) | `no-store` |
| Static asset: the hashed CSS bundle (`/static/css/app.<hash>.css`) | `public, max-age=31536000, immutable` |
| Static asset: every other file under `/static/` | `public, max-age=86400` |
| `/healthz` | `no-store` |

## ETag and Last-Modified

- For every HTML and Markdown response, the server MUST set:
  - `ETag: "<sha256 of body, base64-url, first 16 chars>"` (strong validator). Each representation (identity and gzip) has its own ETag, computed with this formula over that representation's own bytes. The body bytes are stable for the process lifetime.
  - `Last-Modified: <RFC 7231 date>`. The value MUST be the **process start time**, since pre-rendered bytes are recomputed only on restart.
- The server MUST honour `If-None-Match` and `If-Modified-Since` and respond `304 Not Modified` when validation succeeds and the response would otherwise be `2xx`. Preconditions are ignored for error responses such as the branded `404`, which also carry no `ETag` or `Last-Modified` (RFC 9110 § 13.2.1).
- Every static asset response MUST carry a strong `ETag` computed over the bytes of the representation served (see "Static assets" below). `Last-Modified` MAY be omitted on static assets.

## Compression

- A request **accepts gzip** when its `Accept-Encoding` header contains a `gzip` token whose q-value is not `0`. Content-coding names are case-insensitive, and `x-gzip` is treated as `gzip` (RFC 9110 § 8.4.1.3). When no `gzip` or `x-gzip` token is present, a `*` token whose q-value is not `0` also accepts gzip (RFC 9110 § 12.5.3).
- For a pre-rendered route that has a gzip representation, a request that accepts gzip MUST receive it with `Content-Encoding: gzip`. Any other request MUST receive the stored identity representation, without a `Content-Encoding` header.
- Compression MUST NOT happen during request handling. Every compressed body is produced once at startup (see "Materialisation rule" and "Static assets").
- The server MUST set `Vary: Accept-Encoding` on both representations of every pre-rendered route, including `304 Not Modified` responses, and on compressible responses in general.
- The server MUST NOT emit `Content-Encoding: br`. Brotli would require an external dependency, and the server keeps zero external dependencies. Brotli, when wanted, is the responsibility of the reverse proxy (see `deployment.md` § Reverse-proxy expectations).
- `Accept-Encoding` is the **only** request header that may influence the response bytes. No other client signal (`Accept`, `Accept-Language`, cookies, query strings, user-agent) is permitted to vary the body.

## Hot-path header values

- The values of `ETag`, `Last-Modified`, `Cache-Control`, `Content-Type`, `Vary`, `Content-Encoding`, `Content-Length`, and every security header (see `seo.md` § Security headers (mandatory on every response)) MUST be computed once at startup, for every representation that carries them.
- Request handling MUST NOT re-format these values. It selects and writes the values computed at startup.

## Static assets

- Static assets MUST be served at the route `/static/*filepath`, registered with `GETFast` and `HEADFast`. `Mux.ServeFiles` MUST NOT be used for them.
- At startup, every regular file under the static directory MUST be loaded into memory with:
  - its identity body;
  - a gzip body, for files of a compressible type, kept only when it is smaller than the identity body. The compressible types are `text/css`, `image/svg+xml`, `text/javascript`, `application/json`, `text/plain`, `application/xml`, and `application/manifest+json`;
  - a strong `ETag`;
  - a `Content-Type` determined by the file extension;
  - a `Cache-Control` value: `public, max-age=31536000, immutable` for the hashed CSS bundle, and `public, max-age=86400` for every other file.
- When a gzip body is stored for a file, a request that accepts gzip (see "Compression") MUST receive it with `Content-Encoding: gzip`; any other request MUST receive the identity body.
- `Vary: Accept-Encoding` MUST be set on every response for a file that has a stored gzip body, including `304 Not Modified`, and MUST NOT be set for a file without one, because its response does not vary.
- The server MUST honour `If-None-Match` on static assets and respond `304 Not Modified` when validation succeeds.
- Range requests are not supported. A request carrying a `Range` header receives the full body with status `200 OK`.
- A directory path, the bare `/static/`, a path that escapes the static directory, and a path that names no loaded file MUST each receive the branded `404` response (see "Error responses").
- The static directory MUST NOT be read from disk after startup.

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

## Route identity in logs

- The `route_id` field of the request log (see `deployment.md` § Logs) MUST equal the matched route pattern, for example `/docs/routing` or `/static/*filepath`.
- For an unmatched request (answered with `404 Not Found`), `route_id` MUST be empty.

## Performance budget

The following budget is a non-functional requirement. It is measured by the in-repository benchmark target `make bench`, which runs the `internal/server` benchmarks with `-benchmem -count=6`. Each benchmark exercises the full handler chain, in process.

| Request | Time budget | Allocation budget |
| --- | --- | --- |
| A pre-rendered page, served gzip | ≤ 10 µs/op | ≤ 5 allocs/op |
| `/healthz` | — | ≤ 5 allocs/op |
| A static CSS request that accepts gzip | — | ≤ 5 allocs/op |

A dash means that no time budget is set for that request.

## Error responses

- `404 Not Found` MUST render an HTML page from the same template family as the rest of the site (header, footer, breadcrumb up to `Home`), with a clear message and three suggested links: `/docs/`, `/api`, `/examples/`. The page is pre-rendered at startup and served as bytes by the `404` handler. `Cache-Control: no-store` on the response.
- `500 Internal Server Error` MUST render a minimal HTML page with the same chrome and a generic message. The page is pre-rendered at startup. `Cache-Control: no-store`.
- Both error pages MUST set the correct HTTP status code in addition to the body.
