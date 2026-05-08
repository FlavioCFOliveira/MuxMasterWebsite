---
title: Content sources
purpose: Define where every public route gets its content from, and the workflow by which upstream MuxMaster information is curated into this repository.
owners: specification-manager; review by seo-specialist (canonical alignment), geo-specialist (Markdown companion mapping), content-curator (sync workflow).
last-updated: 2026-05-08
status: ratified
---

# Content sources

## Strategy

All content served by the website lives **inside this repository**, under `/content/`. The runtime binary does not read the upstream `../MuxMaster` source tree at request time, at startup, or at any other point. The website is therefore self-contained: clone this repository, build the binary, and every public route can render without access to any other directory.

There are two content categories on the site, distinguished by **origin** rather than by rendering behaviour:

1. **Curated upstream content.** Material whose factual source is the upstream MuxMaster repository (documentation, API reference, changelog, release notes, security, compatibility, contributing, examples, benchmarks). It is mirrored into `/content/` by the **`content-curator` agent** during a development-time sync (see "Sync workflow" below). After the sync, the file in `/content/` is the only source the runtime binary consults.
2. **Site-original content.** Material authored specifically for the site (landing copy, optional introductory copy for the docs and examples index pages, page chrome, navigation labels, page metadata). This content lives under `/content/site/` in this repository and is authored directly without going through the curator.

When the same fact appears upstream and on the site, the upstream value is the **factual** source of truth — but the runtime always reads the local mirrored copy. Drift between upstream and the local mirror is corrected by re-running the sync workflow, never by the runtime binary fetching upstream files.

## Repository layout under `/content/`

The `/content/` tree mirrors the upstream MuxMaster structure where applicable, with site-original additions under `/content/site/`:

```
/content/
├── docs/
│   ├── getting-started.md
│   ├── routing.md
│   ├── groups.md
│   ├── middleware.md
│   ├── error-handling.md
│   ├── configuration.md
│   ├── response-helpers.md
│   ├── performance.md
│   ├── observability.md
│   ├── migration.md
│   └── cookbook.md
├── api.md
├── examples/
│   ├── rest-api.md
│   ├── authn.md
│   ├── jwt.md
│   ├── oauth2.md
│   ├── cache.md
│   ├── graceful-shutdown.md
│   ├── server-side-render.md
│   └── static-site.md
├── benchmarks.md
├── changelog.md
├── security.md
├── compatibility.md
├── contributing.md
├── release-notes/
│   └── v1.0.0.md
└── site/
    ├── landing.md            (optional, prepended to landing page chrome)
    ├── docs-index.md         (optional intro for /docs/)
    ├── examples-index.md     (optional intro for /examples/)
    ├── 404.md                (error template body)
    └── 500.md                (error template body)
```

### Required files

The server MUST refuse to start if any of the following paths are missing under `/content/`. These are the day-one minimum:

| Path | Used by |
| --- | --- |
| `/content/api.md` | `/api`. |
| `/content/changelog.md` | `/changelog`; version label in header/footer. |
| `/content/compatibility.md` | `/compatibility`. |
| `/content/security.md` | `/security`. |
| `/content/contributing.md` | `/contributing`. |
| `/content/benchmarks.md` | `/benchmarks`. |
| `/content/docs/getting-started.md` | `/docs/getting-started`. |
| `/content/docs/routing.md` | `/docs/routing`. |
| `/content/docs/groups.md` | `/docs/groups`. |
| `/content/docs/middleware.md` | `/docs/middleware`. |
| `/content/docs/error-handling.md` | `/docs/error-handling`. |
| `/content/docs/configuration.md` | `/docs/configuration`. |
| `/content/docs/response-helpers.md` | `/docs/response-helpers`. |
| `/content/docs/performance.md` | `/docs/performance`. |
| `/content/docs/observability.md` | `/docs/observability`. |
| `/content/docs/migration.md` | `/docs/migration`. |
| `/content/docs/cookbook.md` | `/docs/cookbook`. |
| `/content/examples/rest-api.md` | `/examples/rest-api`. |
| `/content/examples/authn.md` | `/examples/authn`. |
| `/content/examples/jwt.md` | `/examples/jwt`. |
| `/content/examples/oauth2.md` | `/examples/oauth2`. |
| `/content/examples/cache.md` | `/examples/cache`. |
| `/content/examples/graceful-shutdown.md` | `/examples/graceful-shutdown`. |
| `/content/examples/server-side-render.md` | `/examples/server-side-render`. |
| `/content/examples/static-site.md` | `/examples/static-site`. |
| `/content/release-notes/v1.0.0.md` | `/releases/v1.0.0`. |

The server MUST log the missing path and exit with a non-zero status if any of these files are absent at startup.

### Optional files

The following site-original files are optional. The server MUST start successfully when they are absent:

- `/content/site/landing.md` — when present, prepended to the landing page chrome as introductory copy. When absent, the landing page renders from chrome alone.
- `/content/site/docs-index.md` — when present, prepended to the docs index above the list of sections.
- `/content/site/examples-index.md` — when present, prepended to the examples index above the list of examples.
- `/content/site/404.md`, `/content/site/500.md` — error template bodies. When absent, the server uses a built-in fallback that meets the contract in `accessibility-and-standards.md`.

The logo asset (used by header, favicons, and Open Graph image) is delivered as a binary file under the static-asset pipeline (see `brand-and-visual.md` and `deployment.md`); it is not part of `/content/`.

## Public route to local-file mapping

Every public route reads from a single local file (or a startup-only data structure). Both the HTML representation and its `.md` companion derive from the same source.

| Route (HTML and `.md`) | Source |
| --- | --- |
| `/` | `/content/site/landing.md` (optional) plus landing-page chrome generated at startup. |
| `/docs/` | The registered route table held in process memory; optional `/content/site/docs-index.md` prepended when present. |
| `/docs/getting-started` | `/content/docs/getting-started.md` |
| `/docs/routing` | `/content/docs/routing.md` |
| `/docs/groups` | `/content/docs/groups.md` |
| `/docs/middleware` | `/content/docs/middleware.md` |
| `/docs/error-handling` | `/content/docs/error-handling.md` |
| `/docs/configuration` | `/content/docs/configuration.md` |
| `/docs/response-helpers` | `/content/docs/response-helpers.md` |
| `/docs/performance` | `/content/docs/performance.md` |
| `/docs/observability` | `/content/docs/observability.md` |
| `/docs/migration` | `/content/docs/migration.md` |
| `/docs/cookbook` | `/content/docs/cookbook.md` |
| `/api` | `/content/api.md` |
| `/examples/` | The registered route table held in process memory; optional `/content/site/examples-index.md` prepended when present. |
| `/examples/rest-api` | `/content/examples/rest-api.md` |
| `/examples/authn` | `/content/examples/authn.md` |
| `/examples/jwt` | `/content/examples/jwt.md` |
| `/examples/oauth2` | `/content/examples/oauth2.md` |
| `/examples/cache` | `/content/examples/cache.md` |
| `/examples/graceful-shutdown` | `/content/examples/graceful-shutdown.md` |
| `/examples/server-side-render` | `/content/examples/server-side-render.md` |
| `/examples/static-site` | `/content/examples/static-site.md` |
| `/benchmarks` | `/content/benchmarks.md` |
| `/changelog` | `/content/changelog.md` |
| `/releases/v1.0.0` | `/content/release-notes/v1.0.0.md` |
| `/security` | `/content/security.md` |
| `/compatibility` | `/content/compatibility.md` |
| `/contributing` | `/content/contributing.md` |
| `/robots.txt` | Site-original; static text generated at startup. |
| `/llms.txt` | Site-original; built at startup from the registered route table. |
| `/llms-full.txt` | Site-original; built at startup from the registered route table. |
| `/sitemap.xml` | Site-original; built at startup from the registered route table. |
| `/404` | `/content/site/404.md` (optional, with fallback). Pre-rendered at startup. |
| `/500` | `/content/site/500.md` (optional, with fallback). Pre-rendered at startup. |

Every route in the table is pre-rendered at startup per `rendering-and-caching.md`. There is no lazy or per-request rendering path.

## Examples — file shape

Each example file (`/content/examples/<name>.md`) MUST contain:

1. An editorial intro paragraph (one or two sentences) stating the example's purpose.
2. A fenced ```` ```go ```` code block containing the **embedded `main.go` source** of that example, copied verbatim by the curator agent from `${MUXMASTER_SOURCE_DIR}/examples/<name>/main.go`.
3. A trailing "Source" line linking to the upstream directory, in the form: `Source: <https://github.com/FlavioCFOliveira/MuxMaster/tree/v<version>/examples/<name>>`.

The curator agent embeds the source code rather than linking to it externally, so the runtime page is self-contained and survives upstream availability incidents.

## Benchmarks — source citation

`/benchmarks` reads `/content/benchmarks.md` as-is. The website does not extract or transform the upstream README at request time; the curator agent performs that extraction at sync time and writes the result into `/content/benchmarks.md`.

`/content/benchmarks.md` MUST include, after the article body, a "Source" line citing the upstream file, version, and section, in the form:

> This page reflects the upstream README's `## Benchmarks` section as of YYYY-MM-DD at commit `<short-sha>`. Source: <https://github.com/FlavioCFOliveira/MuxMaster/blob/v<version>/README.md#benchmarks>.

The date and commit are filled in by the curator agent during sync.

## Version label

The version label rendered in the header and footer is read at server startup from `/content/changelog.md`. The detection rule (unchanged from the previous specification) is the **first** Markdown heading of the form `## v<MAJOR>.<MINOR>.<PATCH>` (no pre-release suffix) at the top of the file. A restart is required to roll the label forward. The curator agent commits `/content/changelog.md` mirrored from `../MuxMaster/CHANGELOG.md`; that mirroring is what makes a new version visible to the site.

## Markdown companions

- For every HTML route in the mapping table whose source is a Markdown file, the companion at `<route>.md` MUST serve the source file (after stripping a leading frontmatter block if present) with `Content-Type: text/markdown; charset=utf-8`.
- For example pages, the `.md` companion MUST serve `/content/examples/<name>.md` directly: the editorial intro, the fenced ```` ```go ```` block, and the trailing "Source" line. The companion MUST NOT attempt to extract the bare `.go` source.
- Content negotiation via `Accept` headers is **not** used. The `.md` URL is the only way to reach the Markdown representation.
- The HTML and `.md` representations of the same route MUST present the same canonical information.

## Sync workflow (development time)

The website is updated to a new MuxMaster release through the **content sync workflow**. The workflow is invoked manually, per release; there is no automation in v1.

1. **Trigger.** The project owner asks for a sync ("sync content from upstream", "atualizar conteúdo do MuxMaster", or any equivalent natural-language request).
2. **Curator invocation.** The orchestrator invokes the **`content-curator` agent** (defined in `.claude/agents/content-curator.md`; see `agents-and-gates.md`).
3. **Read upstream.** The curator reads `../MuxMaster` via the environment variable `MUXMASTER_SOURCE_DIR` (development-time / agent-time only — this variable is **not** read by the runtime binary). Default: `../MuxMaster`.
4. **Transform and propose.** The curator transforms each upstream document into the corresponding `/content/...md` file:
   - Documentation files under `${MUXMASTER_SOURCE_DIR}/docs/` map one-to-one to `/content/docs/<name>.md`.
   - `${MUXMASTER_SOURCE_DIR}/api.md` maps to `/content/api.md`.
   - `${MUXMASTER_SOURCE_DIR}/CHANGELOG.md` maps to `/content/changelog.md`.
   - `${MUXMASTER_SOURCE_DIR}/SECURITY.md` maps to `/content/security.md`.
   - `${MUXMASTER_SOURCE_DIR}/COMPATIBILITY.md` maps to `/content/compatibility.md`.
   - `${MUXMASTER_SOURCE_DIR}/CONTRIBUTING.md` maps to `/content/contributing.md`.
   - `${MUXMASTER_SOURCE_DIR}/release-notes/<file>.md` map to `/content/release-notes/<simplified-name>.md` (the curator strips dated suffixes; for example `v1.0.0-20260508.md` becomes `v1.0.0.md`).
   - `${MUXMASTER_SOURCE_DIR}/examples/<name>/main.go` maps to `/content/examples/<name>.md`, with the curator authoring the editorial intro paragraph and embedding the `.go` source as described in "Examples — file shape" above.
   - The `## Benchmarks` section of `${MUXMASTER_SOURCE_DIR}/README.md` is extracted (heading inclusive, up to but not including the next heading of equal or higher level) and written to `/content/benchmarks.md`, with the source citation appended.
   The curator proposes the resulting diff for review. The curator does **not** auto-commit.
5. **Review.** The project owner reviews the diff. Optionally, the gatekeeper agents (`seo-specialist`, `geo-specialist`, `tailwind-specialist`, `ux-specialist`) review per the model in `agents-and-gates.md` — for example, when the sync introduces a new section that affects sitemap entries (`seo-specialist`), the AI-crawler allowlist (`geo-specialist`), or the page templates (`ux-specialist`).
6. **Commit.** The project owner commits the approved changes.
7. **Inconsistencies.** If the curator detects an inconsistency it cannot resolve (for example, a new upstream document that has no place in the current page-template inventory, or a `## Benchmarks` section that no longer exists upstream), it MUST flag the inconsistency in its proposal and refuse to silently drop or rename content.

The curator agent does **not** read or write Go source code, templates, or static assets. It writes only Markdown under `/content/`.

## Audit reports

The `${MUXMASTER_SOURCE_DIR}/reports/` directory is **not** mirrored on the site. The `/security` page links to it on GitHub.

## MuxMaster's own internal specification

The `${MUXMASTER_SOURCE_DIR}/specification/` directory is **not** mirrored on the site. It is MuxMaster's internal specification, distinct from this website's specification.
