---
title: Content sources
purpose: Define where every public route gets its content from, and the runtime contract for the upstream MuxMaster source tree.
owners: specification-manager; review by seo-specialist (canonical alignment), geo-specialist (Markdown companion mapping).
last-updated: 2026-05-08
status: ratified
---

# Content sources

## Strategy

There are two content categories on the site.

1. **Upstream content.** Anything that exists in `../MuxMaster` (documentation, API reference, changelog, release notes, security, compatibility, contributing, examples). This content is **read at runtime** from the upstream tree. It is never copied or duplicated into the website repository.
2. **Site-original content.** Anything authored specifically for the site (landing copy, page chrome, navigation labels, the Markdown of the docs index, the benchmarks article wrapper, page metadata). This content lives under `/content/site/` in the website repository.

When the same fact appears in both categories, the upstream value wins.

## Runtime contract: `MUXMASTER_SOURCE_DIR`

- The site reads upstream content from the directory pointed to by the environment variable `MUXMASTER_SOURCE_DIR`.
- Default value (development): `../MuxMaster`.
- The directory MUST be readable by the server process at startup and at request time.
- The site MUST NOT modify any file under `MUXMASTER_SOURCE_DIR`.

### Required upstream files

The server MUST refuse to start if any of the following paths are missing under `MUXMASTER_SOURCE_DIR`. These are the day-one minimum:

| Path | Used by |
| --- | --- |
| `README.md` | Optional landing-page excerpts; reference. |
| `api.md` | `/api`. |
| `CHANGELOG.md` | `/changelog`; version label in header/footer. |
| `COMPATIBILITY.md` | `/compatibility`. |
| `SECURITY.md` | `/security`. |
| `CONTRIBUTING.md` | `/contributing`. |
| `docs/getting-started.md` | `/docs/getting-started`. |
| `docs/routing.md` | `/docs/routing`. |
| `docs/groups.md` | `/docs/groups`. |
| `docs/middleware.md` | `/docs/middleware`. |
| `docs/error-handling.md` | `/docs/error-handling`. |
| `docs/configuration.md` | `/docs/configuration`. |
| `docs/response-helpers.md` | `/docs/response-helpers`. |
| `docs/performance.md` | `/docs/performance`. |
| `docs/observability.md` | `/docs/observability`. |
| `docs/migration.md` | `/docs/migration`. |
| `docs/cookbook.md` | `/docs/cookbook`. |
| `examples/rest-api/main.go` | `/examples/rest-api`. |
| `examples/authn/main.go` | `/examples/authn`. |
| `examples/jwt/main.go` | `/examples/jwt`. |
| `examples/oauth2/main.go` | `/examples/oauth2`. |
| `examples/cache/main.go` | `/examples/cache`. |
| `examples/graceful-shutdown/main.go` | `/examples/graceful-shutdown`. |
| `examples/server-side-render/main.go` | `/examples/server-side-render`. |
| `examples/static-site/main.go` | `/examples/static-site`. |
| `release-notes/v1.0.0-20260508.md` | `/releases/v1.0.0`. |
| `assets/logo-muxmaster.png` | Header logo, favicon source, OG image source. |

The server MUST log the missing path and exit with a non-zero status if any of these files are absent at startup.

### Fallback behaviour for non-required files

- If a non-required upstream path is missing at request time (for example, a future release-notes file referenced by the footer link before it has been created upstream), the server MUST return `404 Not Found` for the affected route. It MUST NOT serve a stale cached copy and MUST NOT synthesise content.
- The Examples index MUST list only the eight examples enumerated above. Additional upstream example directories are ignored on day one (re-evaluated when the catalogue grows).

### Reload semantics

- The server reads the **version label** from `CHANGELOG.md` once at startup. A restart is required to update the label.
- Page bodies MAY be re-read from disk on cache miss (see `rendering-and-caching.md` for the cache key and invalidation rules).
- The server MUST NOT watch the upstream tree for changes in production; deploys are the unit of update.

## Public route to source-of-truth mapping

The HTML and Markdown companion (`.md` suffix) of every route below render from the same Markdown source. The Markdown companion serves the source file unmodified except for a leading frontmatter block being stripped if present.

| Route (HTML and `.md`) | Source file |
| --- | --- |
| `/` | `/content/site/landing.md` (site-original) |
| `/docs/` | `/content/site/docs-index.md` (site-original; lists the eleven sections) |
| `/docs/getting-started` | `${MUXMASTER_SOURCE_DIR}/docs/getting-started.md` |
| `/docs/routing` | `${MUXMASTER_SOURCE_DIR}/docs/routing.md` |
| `/docs/groups` | `${MUXMASTER_SOURCE_DIR}/docs/groups.md` |
| `/docs/middleware` | `${MUXMASTER_SOURCE_DIR}/docs/middleware.md` |
| `/docs/error-handling` | `${MUXMASTER_SOURCE_DIR}/docs/error-handling.md` |
| `/docs/configuration` | `${MUXMASTER_SOURCE_DIR}/docs/configuration.md` |
| `/docs/response-helpers` | `${MUXMASTER_SOURCE_DIR}/docs/response-helpers.md` |
| `/docs/performance` | `${MUXMASTER_SOURCE_DIR}/docs/performance.md` |
| `/docs/observability` | `${MUXMASTER_SOURCE_DIR}/docs/observability.md` |
| `/docs/migration` | `${MUXMASTER_SOURCE_DIR}/docs/migration.md` |
| `/docs/cookbook` | `${MUXMASTER_SOURCE_DIR}/docs/cookbook.md` |
| `/api` | `${MUXMASTER_SOURCE_DIR}/api.md` |
| `/examples/` | `/content/site/examples-index.md` (site-original) plus per-example purpose lines. |
| `/examples/rest-api` | `${MUXMASTER_SOURCE_DIR}/examples/rest-api/main.go` (rendered as code) plus a site-authored purpose paragraph at `/content/site/examples/rest-api.md`. |
| `/examples/authn` | Source `${MUXMASTER_SOURCE_DIR}/examples/authn/main.go`; purpose `/content/site/examples/authn.md`. |
| `/examples/jwt` | Source `${MUXMASTER_SOURCE_DIR}/examples/jwt/main.go`; purpose `/content/site/examples/jwt.md`. |
| `/examples/oauth2` | Source `${MUXMASTER_SOURCE_DIR}/examples/oauth2/main.go`; purpose `/content/site/examples/oauth2.md`. |
| `/examples/cache` | Source `${MUXMASTER_SOURCE_DIR}/examples/cache/main.go`; purpose `/content/site/examples/cache.md`. |
| `/examples/graceful-shutdown` | Source `${MUXMASTER_SOURCE_DIR}/examples/graceful-shutdown/main.go`; purpose `/content/site/examples/graceful-shutdown.md`. |
| `/examples/server-side-render` | Source `${MUXMASTER_SOURCE_DIR}/examples/server-side-render/main.go`; purpose `/content/site/examples/server-side-render.md`. |
| `/examples/static-site` | Source `${MUXMASTER_SOURCE_DIR}/examples/static-site/main.go`; purpose `/content/site/examples/static-site.md`. |
| `/benchmarks` | `/content/site/benchmarks.md` (site-original wrapper) cites figures verbatim from `${MUXMASTER_SOURCE_DIR}/README.md` and `${MUXMASTER_SOURCE_DIR}/bench_test.go`. |
| `/changelog` | `${MUXMASTER_SOURCE_DIR}/CHANGELOG.md` |
| `/releases/v1.0.0` | `${MUXMASTER_SOURCE_DIR}/release-notes/v1.0.0-20260508.md` |
| `/security` | `${MUXMASTER_SOURCE_DIR}/SECURITY.md` |
| `/compatibility` | `${MUXMASTER_SOURCE_DIR}/COMPATIBILITY.md` |
| `/contributing` | `${MUXMASTER_SOURCE_DIR}/CONTRIBUTING.md` |

## Markdown companions

- For every HTML route in the table above whose source is a Markdown file, the companion at `<route>.md` MUST serve the source file with `Content-Type: text/markdown; charset=utf-8`.
- For example pages, the `.md` companion MUST serve the **rendered Markdown wrapper** (the site-authored purpose paragraph, followed by a fenced ```go``` block containing the upstream source). It MUST NOT serve the bare `.go` file.
- Content negotiation via `Accept` headers is **not** used. The `.md` URL is the only way to reach the Markdown representation.
- The HTML and `.md` representations of the same route MUST present the same canonical information.

## Audit reports

The `${MUXMASTER_SOURCE_DIR}/reports/` directory is **not** mirrored on the site. The `/security` page links to it on GitHub.

## MuxMaster's own internal specification

The `${MUXMASTER_SOURCE_DIR}/specification/` directory is **not** exposed on the site. It is MuxMaster's internal specification, distinct from this website's specification.
