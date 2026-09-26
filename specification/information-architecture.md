---
title: Information architecture
purpose: Define the sitemap, URL structure, navigation, page templates, and inter-page navigation rules.
owners: ux-specialist (primary); seo-specialist (canonical/sitemap alignment); geo-specialist (Markdown companions and llms.txt linkage).
last-updated: 2026-09-26
status: ratified
---

# Information architecture

## Sitemap (day-one)

Every public route is pre-rendered at startup (see `rendering-and-caching.md`). Operational endpoints are marked `(op)`.

```
/
├── /docs/
│   ├── /docs/getting-started
│   ├── /docs/routing
│   ├── /docs/http-query-method
│   ├── /docs/groups
│   ├── /docs/middleware
│   ├── /docs/error-handling
│   ├── /docs/configuration
│   ├── /docs/response-helpers
│   ├── /docs/performance
│   ├── /docs/max-performance
│   ├── /docs/observability
│   ├── /docs/migration
│   └── /docs/cookbook
├── /api
├── /examples/
│   ├── /examples/rest-api
│   ├── /examples/authn
│   ├── /examples/jwt
│   ├── /examples/oauth2
│   ├── /examples/cache
│   ├── /examples/graceful-shutdown
│   ├── /examples/server-side-render
│   └── /examples/static-site
├── /benchmarks
├── /changelog
├── /releases/v1.0.0
├── /releases/v1.1.0
├── /releases/v1.2.0
├── /releases/v1.3.0
├── /security
├── /compatibility
├── /contributing
├── /built-with-muxmaster
├── /llms.txt
├── /llms-full.txt
├── /robots.txt
├── /sitemap.xml
└── /healthz                 (op; not indexed; not in sitemap)
```

In addition to the sitemap above, the server MUST pre-render two non-navigable error templates: `/404` and `/500`. These are emitted by the corresponding error handlers and never appear in `sitemap.xml` or in navigation.

Every listed HTML route also has a Markdown companion (see `geo.md`) at the same path with a `.md` suffix, except `/`, `/llms.txt`, `/llms-full.txt`, `/robots.txt`, `/sitemap.xml`, `/healthz`.

## URL conventions

- Lowercase, kebab-case path segments. No camelCase, no underscores.
- No query strings for content. Query strings MAY appear only for transient operational parameters (none defined today).
- No trailing slash on leaf URLs (`/docs/routing`, not `/docs/routing/`).
- Index URLs (parents of a section) MUST end with a trailing slash (`/docs/`, `/examples/`).
- No file extensions on HTML routes. Markdown companions use the `.md` suffix on the same path (`/docs/routing.md`).
- No URL versioning today. The day MuxMaster v2 ships, the URL strategy is re-evaluated explicitly.

## Top navigation

Header navigation, in order, on every page:

1. `Docs` → `/docs/`
2. `API` → `/api`
3. `Examples` → `/examples/`
4. `Benchmarks` → `/benchmarks`
5. `GitHub` → `https://github.com/FlavioCFOliveira/MuxMaster` (external, opens in new tab with `rel="noopener"`).

The header also displays:

- The MuxMaster logo (link to `/`).
- The current version label (plain text, e.g. `v1.3.0`).
- A dark-mode toggle (no-JS pattern; see `brand-and-visual.md`).

## Sidebar

A persistent sidebar appears **only on `/docs/` and its sub-pages**. It lists the thirteen sub-sections in the following fixed order, which matches the prev/next chain:

1. Getting started → `/docs/getting-started`
2. Routing → `/docs/routing`
3. HTTP QUERY method (RFC 10008) → `/docs/http-query-method`
4. Groups → `/docs/groups`
5. Middleware → `/docs/middleware`
6. Error handling → `/docs/error-handling`
7. Configuration → `/docs/configuration`
8. Response helpers → `/docs/response-helpers`
9. Performance → `/docs/performance`
10. Maximum performance → `/docs/max-performance`
11. Observability → `/docs/observability`
12. Migration → `/docs/migration`
13. Cookbook → `/docs/cookbook`

The active page MUST be visually marked and exposed to assistive technology (`aria-current="page"`).

## Footer (secondary navigation)

Footer links, in order:

1. Changelog → `/changelog`
2. Releases → `/releases/v1.3.0` (the most recent release; the link target updates with each new release entry)
3. Security → `/security`
4. Compatibility → `/compatibility`
5. Contributing → `/contributing`
6. Built with MuxMaster → `/built-with-muxmaster`
7. GitHub → `https://github.com/FlavioCFOliveira/MuxMaster`

The footer also shows:

- The current MuxMaster version label.
- A copyright line ("MuxMaster is MIT-licensed.").
- A link to `/llms.txt` for AI clients.

## Built with MuxMaster page

- `/built-with-muxmaster` is an HTML page with a Markdown companion at `/built-with-muxmaster.md`. It documents how this website itself uses MuxMaster. Its source is the site-owned file `/content/site/built-with-muxmaster.md` (see `content-sources.md` § Site-owned content).
- It is reachable from the footer (see "Footer (secondary navigation)").
- The `/benchmarks` page MUST link to `/built-with-muxmaster`.
- The `/examples/max-performance` page MUST link to `/built-with-muxmaster`.
- It uses the documentation page template (the same template as `/benchmarks`), without the docs sidebar.
- Its breadcrumb is `Home › Built with MuxMaster`.

## HTTP QUERY method page

`/docs/http-query-method` is the site's dedicated page for MuxMaster's support of the HTTP QUERY method, added in MuxMaster v1.2.0. Its source is `/content/docs/http-query-method.md`, composed by the curator per `content-sources.md` CS-QUERY-1.

- **IA-QUERY-1.** The page MUST be served at `/docs/http-query-method`, with a Markdown companion at `/docs/http-query-method.md`. It uses the doc-page template, sits third in the sidebar and in the prev/next chain (after `/docs/routing`, before `/docs/groups`), and its breadcrumb is `Home / Docs / HTTP QUERY method (RFC 10008)`.
- **IA-QUERY-2.** The page's `<h1>` and sidebar label MUST be "HTTP QUERY method (RFC 10008)". The `<title>` is defined in `seo.md` SEO-QUERY-1.
- **IA-QUERY-3.** The page MUST open with a definition-first paragraph that answers "What is the HTTP QUERY method?" in its first sentence: QUERY is a safe and idempotent HTTP method, like GET, that carries request content in the body, like POST. The same paragraph MUST cite RFC 10008 (June 2026), linked to its RFC Editor page, and state that MuxMaster has supported QUERY since v1.2.0.
- **IA-QUERY-4.** The page MUST contain a table of MuxMaster's QUERY API with one row per symbol, giving its exact Go signature and its purpose: `MethodQuery` (the constant `"QUERY"`), `Mux.QUERY`, `Mux.QUERYE`, `Mux.QUERYFast`, `Group.QUERY`, and `Group.QUERYE`. The page MUST state that `Group` has no `QUERYFast` method and that a fast QUERY route on a group is registered with `Group.HandleFast(muxmaster.MethodQuery, …)`.
- **IA-QUERY-5.** The page MUST contain a registration example: a short Go excerpt that registers a QUERY route and reads the request body in the handler. Every line of it MUST be verified against the upstream source at the `v1.3.0` tag.
- **IA-QUERY-6.** The page MUST state the router's method-dispatch behaviour for QUERY: `Mux.ANY` and `Group.ANY` register QUERY; the `Allow` header of a `405 Method Not Allowed` response and of an automatic `OPTIONS` response lists QUERY when the path has a QUERY route; and the `Allow` order is GET, HEAD, POST, PUT, PATCH, DELETE, CONNECT, TRACE, QUERY, OPTIONS.
- **IA-QUERY-7.** The page MUST state the redirect behaviour: with `RedirectCode` left at `0`, `RedirectTrailingSlash` and `RedirectFixedPath` redirect a QUERY request with `307 Temporary Redirect`, which preserves the method and the body, whereas GET and HEAD receive `301 Moved Permanently`; a non-zero `RedirectCode` applies to every method.
- **IA-QUERY-8.** The page MUST state the handler's responsibilities: the router performs no validation of a QUERY request's `Content-Type` or body, so the handler MUST validate them; and response caching for QUERY follows RFC 10008, which the page cites by section.
- **IA-QUERY-9.** The page MUST state that Go 1.27's `net/http` does not define `http.MethodQuery` (tracked by golang/go#80058, linked), that MuxMaster therefore defines `MethodQuery`, and that its value is `"QUERY"` and will remain equal to any future `http.MethodQuery`.
- **IA-QUERY-10.** The page MUST show how to call a QUERY route with `curl` (a command that sends the QUERY method with a request body and a `Content-Type` header).
- **IA-QUERY-11.** The page MUST carry a `## Common questions` section with at least one conversational chain of three or more Q→A pairs, per `geo.md` § Question-Oriented Content.
- **IA-QUERY-12.** The page MUST end with the `## Sources` section required by CS-QUERY-1.
- **IA-QUERY-13.** The registration example (IA-QUERY-5), the handler's validation of the request content (IA-QUERY-8), and the `curl` call (IA-QUERY-10) MUST be presented as one contiguous `## Step N — <name>` sequence starting at `## Step 1 — …`. This sequence is the structural trigger for the page's `HowTo` block (`geo.md` § FAQPage and HowTo structured data; `structured-data.md` SD-QUERY-1).
- **IA-QUERY-14.** The page SHOULD include a verbatim quotation of RFC 10008's definition of the QUERY method, attributed to RFC 10008 by section and linked to its RFC Editor page.
- **IA-QUERY-15.** The page SHOULD quote, verbatim, the panic message that MuxMaster releases before v1.2.0 raised when a route was registered for the QUERY method. The message MUST be taken from the upstream source at the `v1.1.0` tag, which the page's `## Sources` section links (CS-QUERY-1).

## Breadcrumbs

- Breadcrumbs MUST appear on every page except `/` and the operational endpoints.
- Breadcrumb format: `Home / Section / Page`.
- The breadcrumb MUST be exposed to search engines as JSON-LD `BreadcrumbList` (see `seo.md`).
- The breadcrumb separator is a literal `/` rendered with `aria-hidden="true"` and a visually equivalent semantic structure.
- **IA-BC-1.** The breadcrumb of every release-notes page (`/releases/v<version>`) MUST be `Home / Changelog / <release>`, where `Changelog` links to `/changelog` and the last crumb names the release version.
- **IA-BC-2.** In the release-notes breadcrumb, `aria-current="page"` MUST be set on the last crumb only.

## In-page table of contents

- Pages with three or more `<h2>` sections MUST include an in-page table of contents.
- The TOC is generated from the heading structure of the rendered page, with anchor links (`#kebab-case-of-heading`).
- The TOC is rendered above the article body on small viewports and as a sticky right rail on viewports ≥ 1024 px.

## Prev / next navigation within `/docs/`

- Every `/docs/<section>` page ends with a prev/next block linking to the adjacent sections in the order listed under "Sidebar".
- The first page (`/docs/getting-started`) has no `prev`. The last page (`/docs/cookbook`) has no `next`.

## Page templates inventory

The following templates exist. Each template has a single, declared purpose. All templates inherit the global header, footer, and breadcrumb (where breadcrumbs apply).

1. **landing** (`/`) — value proposition, primary CTA to `/docs/getting-started`, secondary CTA to `/api`, three to five highlight blocks (candidate topics: zero dependencies, performance, HTTP QUERY method, idiomatic API, error handling, middleware), link to `/benchmarks`. No sidebar. **IA-LAND-1.** One highlight block MUST present HTTP QUERY method (RFC 10008) support and link to `/docs/http-query-method`. **IA-LAND-2.** Any performance statement on the landing page MUST comply with `overview.md` § Performance claims and link to `/benchmarks`.
2. **doc-page** (`/docs/<section>`) — left sidebar, breadcrumb, in-page TOC, article body rendered from a Markdown source, prev/next block. JSON-LD `TechArticle` + `BreadcrumbList`.
3. **doc-index** (`/docs/`) — thirteen cards (one per sub-section) with title and one-line description. JSON-LD `BreadcrumbList`.
4. **api-page** (`/api`) — single long article rendered from `/content/api.md`, with sticky in-page TOC. JSON-LD `TechArticle` + `SoftwareSourceCode` (referencing the upstream module). No sidebar.
5. **example-index** (`/examples/`) — eight cards (one per example mirrored into `/content/examples/`) with title, one-line purpose statement, link to the example page, and link to the upstream directory. JSON-LD `BreadcrumbList`.
6. **example-page** (`/examples/<name>`) — title (`<h1>`), a short editorial intro paragraph (one or two sentences) stating what the example program does and the concrete capability it demonstrates, and the walkthrough body. The walkthrough body is an ordered sequence of `## Step N — <name>` H2 sections; each step opens with one or more paragraphs of didactic prose and contains at most one small fenced Go excerpt (3–40 lines) lifted verbatim from the upstream source. The full program source is **not** rendered on this template; readers who want the entire file follow the `## Upstream source` link at the foot of the page, which targets `https://github.com/FlavioCFOliveira/MuxMaster/tree/v<version>/examples/<name>`. The page MUST also carry a `## Common questions` section with at least one conversational chain of three or more Q→A pairs wrapped in `<section data-conversation="…">` (per `geo.md` § Question-Oriented Content), placed after the walkthrough and before `## Upstream source`. The canonical contract for the page shape is `geo.md` § Example walkthrough shape; this template inventory references it rather than restating it. The site does not execute or sandbox the example. JSON-LD `TechArticle`, `BreadcrumbList`, `HowTo`, `FAQPage`.
7. **benchmarks** (`/benchmarks`) — benchmark results from this project's campaign archive (written into `/content/benchmarks.md` by the curator agent per `content-sources.md` § Benchmarks — sources), with the "Source" section of CS-BENCH-17 and, when present, the `## Historical (v1.1.0-era code)` section. JSON-LD `TechArticle` + `Dataset` (where the table is the dataset).
8. **changelog** (`/changelog`) — full mirrored `/content/changelog.md` rendered as a single long page, with one `<h2>` per version. JSON-LD `TechArticle`.
9. **release-notes** (`/releases/v<version>`: `/releases/v1.0.0`, `/releases/v1.1.0`, `/releases/v1.2.0`, `/releases/v1.3.0`) — single long article rendered from `/content/release-notes/v<version>.md`. JSON-LD `TechArticle`.
10. **generic-text-page** (`/security`, `/compatibility`, `/contributing`) — breadcrumb, single article body. JSON-LD `TechArticle`.

## Language attribute

Every HTML page MUST declare `<html lang="en">`.
