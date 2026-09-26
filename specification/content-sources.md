---
title: Content sources
purpose: Define where every public route gets its content from, and the workflow by which upstream MuxMaster information is curated into this repository.
owners: specification-manager; review by seo-specialist (canonical alignment), geo-specialist (Markdown companion mapping), content-curator (sync workflow).
last-updated: 2026-09-26
status: ratified
---

# Content sources

## Strategy

All content served by the website lives **inside this repository**, under `/content/`. The runtime binary does not read the upstream `../MuxMaster` source tree at request time, at startup, or at any other point. The website is therefore self-contained: clone this repository, build the binary, and every public route can render without access to any other directory.

There are two content categories on the site, distinguished by **origin** rather than by rendering behaviour:

1. **Curated upstream content.** Material whose factual source is the upstream MuxMaster repository (documentation, API reference, changelog, release notes, security, compatibility, contributing, examples). `/content/benchmarks.md` is also written by the curator, but its primary source is this project's own benchmark campaign (see "Benchmarks — sources"), with upstream data as corroboration only. It is mirrored into `/content/` by the **`content-curator` agent** during a development-time sync (see "Sync workflow" below). After the sync, the file in `/content/` is the only source the runtime binary consults.
2. **Site-original content.** Material authored specifically for the site (landing copy, optional introductory copy for the docs and examples index pages, page chrome, navigation labels, page metadata). This content lives under `/content/site/` in this repository and is authored directly without going through the curator.

When the same fact appears upstream and on the site, the upstream value is the **factual** source of truth — but the runtime always reads the local mirrored copy. Drift between upstream and the local mirror is corrected by re-running the sync workflow, never by the runtime binary fetching upstream files.

## Repository layout under `/content/`

The `/content/` tree mirrors the upstream MuxMaster structure where applicable, with site-original additions under `/content/site/`:

```
/content/
├── docs/
│   ├── getting-started.md
│   ├── routing.md
│   ├── http-query-method.md
│   ├── groups.md
│   ├── middleware.md
│   ├── error-handling.md
│   ├── configuration.md
│   ├── response-helpers.md
│   ├── performance.md
│   ├── max-performance.md
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
│   ├── v1.0.0.md
│   ├── v1.1.0.md
│   ├── v1.2.0.md
│   └── v1.3.0.md
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
| `/content/docs/http-query-method.md` | `/docs/http-query-method`. |
| `/content/docs/groups.md` | `/docs/groups`. |
| `/content/docs/middleware.md` | `/docs/middleware`. |
| `/content/docs/error-handling.md` | `/docs/error-handling`. |
| `/content/docs/configuration.md` | `/docs/configuration`. |
| `/content/docs/response-helpers.md` | `/docs/response-helpers`. |
| `/content/docs/performance.md` | `/docs/performance`. |
| `/content/docs/max-performance.md` | `/docs/max-performance`. |
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
| `/content/site/built-with-muxmaster.md` | `/built-with-muxmaster` (site-owned; see "Site-owned content"). |
| `/content/release-notes/v1.0.0.md` | `/releases/v1.0.0`. |
| `/content/release-notes/v1.1.0.md` | `/releases/v1.1.0`. |
| `/content/release-notes/v1.2.0.md` | `/releases/v1.2.0`. |
| `/content/release-notes/v1.3.0.md` | `/releases/v1.3.0`. |

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
| `/docs/http-query-method` | `/content/docs/http-query-method.md` |
| `/docs/groups` | `/content/docs/groups.md` |
| `/docs/middleware` | `/content/docs/middleware.md` |
| `/docs/error-handling` | `/content/docs/error-handling.md` |
| `/docs/configuration` | `/content/docs/configuration.md` |
| `/docs/response-helpers` | `/content/docs/response-helpers.md` |
| `/docs/performance` | `/content/docs/performance.md` |
| `/docs/max-performance` | `/content/docs/max-performance.md` |
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
| `/releases/v1.1.0` | `/content/release-notes/v1.1.0.md` |
| `/releases/v1.2.0` | `/content/release-notes/v1.2.0.md` |
| `/releases/v1.3.0` | `/content/release-notes/v1.3.0.md` |
| `/security` | `/content/security.md` |
| `/compatibility` | `/content/compatibility.md` |
| `/contributing` | `/content/contributing.md` |
| `/robots.txt` | Site-original; static text generated at startup. |
| `/llms.txt` | Site-original; built at startup from the registered route table. |
| `/llms-full.txt` | Site-original; built at startup from the registered route table. Bundles the `/llms.txt` navigation index with the concatenated Markdown bodies of every content-backed route (see `geo.md`). |
| `/sitemap.xml` | Site-original; built at startup from the registered route table. |
| `/404` | `/content/site/404.md` (optional, with fallback). Pre-rendered at startup. |
| `/500` | `/content/site/500.md` (optional, with fallback). Pre-rendered at startup. |

Every route in the table is pre-rendered at startup per `rendering-and-caching.md`. There is no lazy or per-request rendering path.

## Examples — file shape

Each example file (`/content/examples/<name>.md`) MUST contain, in this order:

1. An editorial intro paragraph (one or two sentences) stating what the example program does and the concrete capability it demonstrates.
2. The walkthrough body: an ordered sequence of `## Step N — <name>` H2 sections, where `N` is a 1-indexed contiguous integer starting at `1`. Each section opens with one or more paragraphs of didactic prose and contains at most one fenced ```` ```go ```` excerpt (typically 3–40 lines) showing only the lines relevant to that step. The excerpt MUST be lifted verbatim from `${MUXMASTER_SOURCE_DIR}/examples/<name>/main.go` (or another file in that example's upstream directory when the step concerns it); the curator MUST NOT invent code. Elisions inside an excerpt MUST be marked with `// …` and MUST NOT silently truncate the middle of a function. The page MUST NOT contain the full program as a single fenced block.
3. A `## Common questions` section carrying at least one conversational chain of three or more Q→A pairs wrapped in `<section data-conversation="…">`, per `geo.md` § Question-Oriented Content.
4. A trailing `## Upstream source` section: one paragraph plus a link to the canonical upstream directory, in the form: `Source: <https://github.com/FlavioCFOliveira/MuxMaster/tree/v<version>/examples/<name>>`.

The canonical contract for the page shape — the H2 sequencing rule, the per-step body rule, the no-full-source-dump rule, and the JSON-LD coupling — is `geo.md` § Example walkthrough shape. The list of constraints in this section is the file-on-disk view of that contract; if the two ever drift, `geo.md` is authoritative and this section MUST be brought back into alignment.

The curator does **not** copy the upstream `main.go` verbatim as a single block. The curator authors the editorial intro and the per-step prose, chooses the segmentation, and lifts each excerpt verbatim from the upstream source so a reader who follows `## Upstream source` sees the same lines. Every excerpt on the page MUST appear in the upstream file referenced by `## Upstream source`.

## Benchmarks — sources

`/benchmarks` reads `/content/benchmarks.md` as-is. The website does not extract or transform any source at request time, and the runtime binary never executes a benchmark. The rules in this section govern where the numbers in `/content/benchmarks.md` come from. The rules that govern how any performance claim is worded, on any page, are in `overview.md` § Performance claims.

### Primary source: the website benchmark campaign

- **CS-BENCH-1.** The primary source of every MuxMaster performance number published on the site MUST be a benchmark campaign run by this project and archived in this repository under `reports/benchmarks-<YYYY-MM-DD>/`, where the date is the day the campaign's measurements were taken. The campaign is a development-time activity; it is not run by the runtime binary and not by the `content-curator` agent.
- **CS-BENCH-2.** The campaign archive MUST contain: the raw `go test` output of every run; the `benchstat` output of every comparison; the exact commands used; the host facts (CPU model, core and thread count, CPU frequency governor, operating system, kernel version, Go toolchain version, and the value of `GOTOOLCHAIN`); the full commit SHA of every tag or commit measured; and a `README.md` that states the campaign's purpose, method, and caveats.
- **CS-BENCH-3.** The campaign MUST compare MuxMaster `v1.1.0` with MuxMaster `v1.3.0` on the upstream root-package benchmarks. Each version MUST be run from an isolated checkout of its tag, on the same host, in the same session, either interleaved or back to back.
- **CS-BENCH-4.** The campaign MUST run the upstream `competitor/` suite at the `v1.3.0` tag.
- **CS-BENCH-5.** Every run in the campaign MUST use `-count=10` or higher, so that `benchstat` can test significance.
- **CS-BENCH-6.** Every run in one campaign MUST use the same Go toolchain, recorded in the archive. `GOTOOLCHAIN` MUST be set so that the `go` command does not switch toolchains between runs (for example `GOTOOLCHAIN=local` with an installed toolchain of at least Go 1.27.1, the minimum that `v1.3.0` requires). Rationale: `v1.1.0` declares a lower Go minimum than `v1.3.0`; with automatic toolchain switching the two sides could be built by different compilers, and the comparison would then measure the compiler as well as the router.
- **CS-BENCH-7.** Every comparison MUST be processed with `benchstat`, and the published comparison MUST report the p-value for each delta. A delta that is not statistically significant at alpha = 0.05 MUST be published as "no significant difference", without a percentage or a direction.
- **CS-BENCH-8.** The campaign MAY also run the upstream `middleware` package benchmarks at `v1.3.0`. Where a benchmark has no counterpart in `v1.1.0`, only its absolute `v1.3.0` value MAY be published; it MUST NOT be presented as a change since `v1.1.0`.

### Corroborating and upstream sources

- **CS-BENCH-9.** The upstream `README.md` `## Benchmarks` section at tag `v1.3.0` and the upstream `reports/perf-lab-2026-09-26-docs/` archive MAY be cited on `/benchmarks` only as corroboration of the campaign's results. They MUST NOT replace a campaign figure.
- **CS-BENCH-10.** A figure from the upstream "Measured changes since v1.1.0" list (route registration, `Mount`, `CleanPath`, redirects, 405, automatic `OPTIONS`, the `Text` helper, middleware, and the costs added by security fixes) MAY be published only when either (a) the campaign re-measured it, in which case the campaign figure is published, or (b) it is explicitly attributed to upstream, with the upstream measurement date, the upstream sample count, and a link to the upstream source pinned to the `v1.3.0` tag. The "before" values in that upstream list were measured on 2026-09-24 against pre-change commits, not against `v1.1.0`; the page MUST NOT present them as `v1.1.0` values.
- **CS-BENCH-11.** When the upstream commit that a corroborating figure was measured at is not the tag being documented, the page MUST state the measured commit. (For example, the upstream `perf-lab-2026-09-26-docs` host table records commit `bc4edb7`, which precedes the `v1.2.0` tag.)

### Historical data

- **CS-BENCH-12.** Benchmark data measured on `v1.1.0`-era code — the upstream `reports/perf-audit-2026-05-12/` archive and the upstream platform files `reports/<platform>-benchmarks-<date>.md` (for example `apple-m4-benchmarks-2026-05-12.md` and `rpi5-benchmarks-2026-05-12.md`) — MAY appear on `/benchmarks` only inside one section headed exactly `## Historical (v1.1.0-era code)`, placed after the current campaign results and before the "Source" section.
- **CS-BENCH-13.** That section MUST open with a sentence stating that its figures were measured on `v1.1.0`-era code and were not re-measured on the current release. Every table in it MUST carry its platform, measurement date, Go version, sample count, and a provenance line of the form "Source: reports/<filename> (upstream MuxMaster, <date>)", with the file name set in code format and linked to the upstream file pinned to the `v1.3.0` tag.
- **CS-BENCH-14.** From each platform file, the curator MUST extract only the serial and parallel tables of its `## Internal benchmarks` section, covering MuxMaster `Handle` (default), `Handle` with `PoolRequestBundle = true`, and `HandleFast`, with ns/op, B/op, and allocs/op. The hardware metadata (platform name, CPU model, core count and type breakdown if provided, clock speed if provided, RAM, operating system and version, kernel version, Go version) MUST be read verbatim from the header block of the platform file. The curator MUST NOT include raw `go test` output blocks, competitor tables, architecture-comparison tables or ratio columns, or the extended benchmark sections (group dispatch, middleware, and similar).
- **CS-BENCH-15.** When no historical source is present upstream, the curator MUST omit the historical section entirely; it MUST NOT emit a placeholder. If a platform file's `## Internal benchmarks` section lacks a serial or parallel table, the curator MUST flag the inconsistency in its proposal and MUST NOT silently drop or fabricate data for that platform.
- **CS-BENCH-16.** Historical figures MUST NOT be compared with campaign figures in the same table, and MUST NOT support any positioning claim about the current release.

### Source section

- **CS-BENCH-17.** `/content/benchmarks.md` MUST end with a "Source" section that lists, for each source used: the campaign archive `reports/benchmarks-<YYYY-MM-DD>/`, linked on `https://github.com/FlavioCFOliveira/MuxMasterWebsite` at the full commit SHA that contains it; the upstream benchmark suites measured (`bench_test.go` at `v1.1.0` and `v1.3.0`, `competitor/bench_test.go` at `v1.3.0`, and `middleware` benchmarks at `v1.3.0` when run); and every corroborating or historical upstream source cited, linked to the upstream file at the `v1.3.0` tag.
- **CS-BENCH-18.** The refreshed `/content/benchmarks.md` MUST NOT be published before the first campaign archive under `reports/benchmarks-<YYYY-MM-DD>/` exists in this repository.

## Version label

The version label rendered in the header and footer is read at server startup from `/content/changelog.md`. The detection rule is defined in `url-and-versioning.md` § Version label rule: the **first** Markdown heading of the form `## v<MAJOR>.<MINOR>.<PATCH>` or `## [<MAJOR>.<MINOR>.<PATCH>]` (no pre-release suffix) at the top of the file. A restart is required to roll the label forward. The curator agent commits `/content/changelog.md` mirrored from `../MuxMaster/CHANGELOG.md`; that mirroring is what makes a new version visible to the site.

## Markdown companions

- For every HTML route in the mapping table whose source is a Markdown file, the companion at `<route>.md` MUST serve the source file (after stripping a leading frontmatter block if present) with `Content-Type: text/markdown; charset=utf-8`.
- For example pages, the `.md` companion MUST serve `/content/examples/<name>.md` directly: the editorial intro, the ordered `## Step N — <name>` walkthrough sections (each carrying its didactic prose and its small Go excerpt), the `## Common questions` section, and the `## Upstream source` link. The shape of the `.md` representation is identical to the shape of the HTML rendering. The companion MUST NOT extract a bare `.go` source dump and MUST NOT apply any transformation step beyond stripping a leading frontmatter block if present.
- Content negotiation via `Accept` headers is **not** used. The `.md` URL is the only way to reach the Markdown representation.
- The HTML and `.md` representations of the same route MUST present the same canonical information.

## Sync workflow (development time)

The website is updated to a new MuxMaster release through the **content sync workflow**. The workflow is invoked manually, per release; there is no automation in v1.

1. **Trigger.** The project owner asks for a sync ("sync content from upstream", "atualizar conteúdo do MuxMaster", or any equivalent natural-language request).
2. **Curator invocation.** The orchestrator invokes the **`content-curator` agent** (defined in `.claude/agents/content-curator.md`; see `agents-and-gates.md`).
3. **Read upstream.** The curator reads `../MuxMaster` via the environment variable `MUXMASTER_SOURCE_DIR` (development-time / agent-time only — this variable is **not** read by the runtime binary). Default: `../MuxMaster`.
4. **Transform and propose.** The curator transforms each upstream document into the corresponding `/content/...md` file:
   - Documentation files under `${MUXMASTER_SOURCE_DIR}/docs/` map one-to-one to `/content/docs/<name>.md`, except `/content/docs/http-query-method.md`, which is composed per CS-QUERY-1 below.
   - `${MUXMASTER_SOURCE_DIR}/api.md` maps to `/content/api.md`.
   - `${MUXMASTER_SOURCE_DIR}/CHANGELOG.md` maps to `/content/changelog.md`.
   - `${MUXMASTER_SOURCE_DIR}/SECURITY.md` maps to `/content/security.md`.
   - `${MUXMASTER_SOURCE_DIR}/COMPATIBILITY.md` maps to `/content/compatibility.md`.
   - `${MUXMASTER_SOURCE_DIR}/CONTRIBUTING.md` maps to `/content/contributing.md`.
   - `${MUXMASTER_SOURCE_DIR}/release-notes/<file>.md` map to `/content/release-notes/<simplified-name>.md` (the curator strips dated suffixes; for example `v1.0.0-20260508.md` becomes `v1.0.0.md`). **CS-REL-1.** `release-notes/v1.2.0-20260926.md` MUST map to `/content/release-notes/v1.2.0.md` (route `/releases/v1.2.0`), and `release-notes/v1.3.0-20260926.md` MUST map to `/content/release-notes/v1.3.0.md` (route `/releases/v1.3.0`).
   - **CS-QUERY-1.** `/content/docs/http-query-method.md` has no one-to-one upstream counterpart. The curator composes it from these upstream sources at the `v1.3.0` tag: `README.md`, `CHANGELOG.md` `## [1.2.0]`, `release-notes/v1.2.0-20260926.md`, `docs/routing.md`, and `query_method_test.go`. Every claim on the page MUST be verified against the upstream source code at `v1.3.0` (`mux.go`, `group.go`, and the tests), except the pre-v1.2.0 panic message (`information-architecture.md` IA-QUERY-15), which MUST be verified against the upstream source at the `v1.1.0` tag. The page's required content is defined in `information-architecture.md` § HTTP QUERY method page. The page MUST end with a `## Sources` section linking each upstream source it uses, pinned to the `v1.3.0` tag, or to the `v1.1.0` tag for the source of the pre-v1.2.0 panic message. When the page quotes RFC 10008 (IA-QUERY-14), the `## Sources` section MUST also link RFC 10008 on the RFC Editor site.
   - `${MUXMASTER_SOURCE_DIR}/examples/<name>/` maps to `/content/examples/<name>.md`. The curator produces a walkthrough whose excerpts are lifted verbatim from upstream `${MUXMASTER_SOURCE_DIR}/examples/<name>/main.go` (and other files in that directory when a step concerns them). The curator MUST NOT invent code; every excerpt MUST appear in the upstream source. The curator chooses the step segmentation, authors the editorial intro and the per-step didactic prose, and assembles the `## Common questions` chain and `## Upstream source` link as described in "Examples — file shape" above and in `geo.md` § Example walkthrough shape.
   - `/content/benchmarks.md` is written from this repository's campaign archive `reports/benchmarks-<YYYY-MM-DD>/` (primary source), with upstream data used only as corroboration or in the historical section, per "Benchmarks — sources" above. The "Source" section required by CS-BENCH-17 MUST be appended.
   The curator proposes the resulting diff for review. The curator does **not** auto-commit.
5. **Review.** The project owner reviews the diff. Optionally, the gatekeeper agents (`seo-specialist`, `geo-specialist`, `tailwind-specialist`, `ux-specialist`) review per the model in `agents-and-gates.md` — for example, when the sync introduces a new section that affects sitemap entries (`seo-specialist`), the AI-crawler allowlist (`geo-specialist`), or the page templates (`ux-specialist`).
6. **Commit.** The project owner commits the approved changes.
7. **Inconsistencies.** If the curator detects an inconsistency it cannot resolve (for example, a new upstream document that has no place in the current page-template inventory, or a `## Benchmarks` section that no longer exists upstream), it MUST flag the inconsistency in its proposal and refuse to silently drop or rename content.

The curator agent does **not** read or write this repository's Go source code, templates, or static assets. It reads upstream sources (including upstream Go source, to lift excerpts and verify claims) and, read-only, this repository's benchmark campaign archives under `reports/benchmarks-<YYYY-MM-DD>/`. It writes only Markdown under `/content/`.

## Audit reports

The `${MUXMASTER_SOURCE_DIR}/reports/` directory is **not** mirrored on the site. The `/security` page links to it on GitHub.

## MuxMaster's own internal specification

The `${MUXMASTER_SOURCE_DIR}/specification/` directory is **not** mirrored on the site. It is MuxMaster's internal specification, distinct from this website's specification.

## Site-owned content

Site-owned content is the class of files under `/content/site/` that are authored in this repository and are never synchronised from upstream. It is the site-original category described in "Strategy" above; this section states its rules normatively.

- The `content-curator` agent MUST NOT create, overwrite, or delete any file under `/content/site/`.
- `/content/site/built-with-muxmaster.md` is site-owned content. It is the source of `/built-with-muxmaster` and of its Markdown companion `/built-with-muxmaster.md` (see `information-architecture.md` § Built with MuxMaster page).
- Every code excerpt on `/built-with-muxmaster` MUST be quoted verbatim from this repository's source code.
- Every number on `/built-with-muxmaster` MUST come from a recorded run of this repository's `make bench` target (see `rendering-and-caching.md` § Performance budget), and MUST state the host CPU, the operating system, the Go version, and the date of that run.
