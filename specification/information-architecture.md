---
title: Information architecture
purpose: Define the sitemap, URL structure, navigation, page templates, and inter-page navigation rules.
owners: ux-specialist (primary); seo-specialist (canonical/sitemap alignment); geo-specialist (Markdown companions and llms.txt linkage).
last-updated: 2026-05-08
status: ratified
---

# Information architecture

## Sitemap (day-one)

Each route is annotated with its rendering category — `[A]` for **pre-rendered at startup**, `[B]` for **lazy-cache live templating** — as defined in `rendering-and-caching.md`. The annotation applies to both the HTML route and its `.md` companion (where one exists). Operational endpoints are marked `(op)`.

```
/                            [A]
├── /docs/                   [A]
│   ├── /docs/getting-started   [B]
│   ├── /docs/routing           [B]
│   ├── /docs/groups            [B]
│   ├── /docs/middleware        [B]
│   ├── /docs/error-handling    [B]
│   ├── /docs/configuration     [B]
│   ├── /docs/response-helpers  [B]
│   ├── /docs/performance       [B]
│   ├── /docs/observability     [B]
│   ├── /docs/migration         [B]
│   └── /docs/cookbook          [B]
├── /api                     [B]
├── /examples/               [A]
│   ├── /examples/rest-api          [B]
│   ├── /examples/authn             [B]
│   ├── /examples/jwt               [B]
│   ├── /examples/oauth2            [B]
│   ├── /examples/cache             [B]
│   ├── /examples/graceful-shutdown [B]
│   ├── /examples/server-side-render [B]
│   └── /examples/static-site       [B]
├── /benchmarks              [B]
├── /changelog               [B]
├── /releases/v1.0.0         [B]
├── /security                [B]
├── /compatibility           [B]
├── /contributing            [B]
├── /llms.txt                [A]
├── /llms-full.txt           [A]
├── /robots.txt              [A]
├── /sitemap.xml             [A]
└── /healthz                 (op; not indexed; not in sitemap)
```

In addition to the sitemap above, the server MUST pre-render two non-navigable error templates as Category A: `/404` and `/500`. These are emitted by the corresponding error handlers and never appear in `sitemap.xml` or in navigation.

Every listed HTML route also has a Markdown companion (see `geo.md`) at the same path with a `.md` suffix, except `/`, `/llms.txt`, `/llms-full.txt`, `/robots.txt`, `/sitemap.xml`, `/healthz`. The companion shares its rendering category with the HTML route.

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
- The current version label (plain text, e.g. `v1.0.1`).
- A dark-mode toggle (no-JS pattern; see `brand-and-visual.md`).

## Sidebar

A persistent sidebar appears **only on `/docs/` and its sub-pages**. It lists the eleven sub-sections in the following fixed order, which matches the prev/next chain:

1. Getting started → `/docs/getting-started`
2. Routing → `/docs/routing`
3. Groups → `/docs/groups`
4. Middleware → `/docs/middleware`
5. Error handling → `/docs/error-handling`
6. Configuration → `/docs/configuration`
7. Response helpers → `/docs/response-helpers`
8. Performance → `/docs/performance`
9. Observability → `/docs/observability`
10. Migration → `/docs/migration`
11. Cookbook → `/docs/cookbook`

The active page MUST be visually marked and exposed to assistive technology (`aria-current="page"`).

## Footer (secondary navigation)

Footer links, in order:

1. Changelog → `/changelog`
2. Releases → `/releases/v1.0.0` (the most recent release; the link target updates with each new release entry)
3. Security → `/security`
4. Compatibility → `/compatibility`
5. Contributing → `/contributing`
6. GitHub → `https://github.com/FlavioCFOliveira/MuxMaster`

The footer also shows:

- The current MuxMaster version label.
- A copyright line ("MuxMaster is MIT-licensed.").
- A link to `/llms.txt` for AI clients.

## Breadcrumbs

- Breadcrumbs MUST appear on every page except `/` and the operational endpoints.
- Breadcrumb format: `Home / Section / Page`.
- The breadcrumb MUST be exposed to search engines as JSON-LD `BreadcrumbList` (see `seo.md`).
- The breadcrumb separator is a literal `/` rendered with `aria-hidden="true"` and a visually equivalent semantic structure.

## In-page table of contents

- Pages with three or more `<h2>` sections MUST include an in-page table of contents.
- The TOC is generated from the heading structure of the rendered page, with anchor links (`#kebab-case-of-heading`).
- The TOC is rendered above the article body on small viewports and as a sticky right rail on viewports ≥ 1024 px.

## Prev / next navigation within `/docs/`

- Every `/docs/<section>` page ends with a prev/next block linking to the adjacent sections in the order listed under "Sidebar".
- The first page (`/docs/getting-started`) has no `prev`. The last page (`/docs/cookbook`) has no `next`.

## Page templates inventory

The following templates exist. Each template has a single, declared purpose. All templates inherit the global header, footer, and breadcrumb (where breadcrumbs apply).

1. **landing** (`/`) — value proposition, primary CTA to `/docs/getting-started`, secondary CTA to `/api`, three to five highlight blocks (zero dependencies, performance, idiomatic API, error handling, middleware), link to `/benchmarks`. No sidebar.
2. **doc-page** (`/docs/<section>`) — left sidebar, breadcrumb, in-page TOC, article body rendered from a Markdown source, prev/next block. JSON-LD `TechArticle` + `BreadcrumbList`.
3. **doc-index** (`/docs/`) — eleven cards (one per sub-section) with title and one-line description. JSON-LD `BreadcrumbList`.
4. **api-page** (`/api`) — single long article rendered from `../MuxMaster/api.md`, with sticky in-page TOC. JSON-LD `TechArticle` + `SoftwareSourceCode` (referencing the upstream module). No sidebar.
5. **example-index** (`/examples/`) — eight cards (one per upstream example) with title, one-line purpose statement, link to the example page, and link to the upstream directory. JSON-LD `BreadcrumbList`.
6. **example-page** (`/examples/<name>`) — title, one-paragraph purpose, syntax-highlighted source of the example's primary file (typically `main.go`), and a link to the upstream directory containing the rest of the files. The site does not execute or sandbox the example. JSON-LD `TechArticle`.
7. **benchmarks** (`/benchmarks`) — table of benchmark numbers quoted verbatim from upstream, with the source file path and line cited next to each row. JSON-LD `TechArticle` + `Dataset` (where the table is the dataset).
8. **changelog** (`/changelog`) — full upstream `CHANGELOG.md` rendered as a single long page, with one `<h2>` per version. JSON-LD `TechArticle`.
9. **release-notes** (`/releases/v1.0.0`) — single long article rendered from `../MuxMaster/release-notes/v1.0.0-20260508.md`. JSON-LD `TechArticle`.
10. **generic-text-page** (`/security`, `/compatibility`, `/contributing`) — breadcrumb, single article body. JSON-LD `TechArticle`.

## Language attribute

Every HTML page MUST declare `<html lang="en">`.
