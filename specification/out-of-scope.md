---
title: Out of scope (v1)
purpose: Enumerate what v1 of the site explicitly does not do, so that future requests to add these features are recognised as expansions and trigger explicit ratification.
owners: specification-manager.
last-updated: 2026-05-08
status: ratified
---

# Out of scope (v1)

The following are deliberately **not** part of v1 of the website. Adding any of them is a scope expansion and MUST be ratified before implementation.

## Per-request dynamism

The site is **static-tending** (see `overview.md` and `rendering-and-caching.md`). Every public route is pre-rendered at startup, and the same URL MUST return the same bytes for the lifetime of the process. The following are therefore explicitly out of scope for v1:

- No user-specific or personalised content. The same bytes are served to every visitor.
- No A/B testing, experiments, bucketing, or feature flags evaluated at request time.
- No content negotiation via `Accept` to switch between HTML and Markdown (`.md` URLs are the only Markdown entry point).
- No request-time variation by `Accept-Language`, cookies, query strings, IP, geo, or user-agent.
- The only request header permitted to influence response bytes is `Accept-Encoding` (for compression).
- Server-side templates are an implementation detail of how bytes are produced, never a request-time decision the client perceives.

## Runtime upstream filesystem dependency

The website does **not** read `../MuxMaster/` at request time, at startup, or at any other point during runtime. Documentation content is prepared in advance by the `content-curator` agent at development time and committed to this repository under `/content/`. The runtime binary serves only what is in this repository. The following are therefore explicitly out of scope for v1:

- No runtime mount of the upstream MuxMaster tree.
- No runtime read of any path under `../MuxMaster/`.
- No environment variable named `MUXMASTER_SOURCE_DIR` consumed by the runtime binary. The variable exists only at development / agent time, where the `content-curator` agent uses it to locate the upstream working tree.
- No filesystem watcher on `/content/` or anywhere else; pre-rendered bytes are recomputed only on process restart (see `rendering-and-caching.md`).

## Search

- No on-site search index.
- No client-side search widget.
- Browser `Ctrl-F` and the sitemap are the day-one substitutes.
- Re-evaluated when content volume justifies it.

## Analytics

- No analytics on day one. Privacy-first.
- When added, the preferred option is **self-hosted Plausible**. Any choice MUST be ratified.

## Audit-report mirroring

- The upstream `reports/` directory is **not** mirrored on the site (and is not part of `/content/`).
- The `/security` page links to it on GitHub.

## Benchmark re-execution

- The site does **not** re-run benchmarks.
- The `/benchmarks` page quotes upstream numbers verbatim with citations to the source files.

## Executable example playground

- The site does **not** execute, sandbox, or run the example code.
- Each example page renders the upstream `main.go` as syntax-highlighted text and links to the upstream directory for the rest of the files.

## Logo variants

- SVG logo variant is out of scope for v1.
- Horizontal-lockup variant (logo + wordmark) as a separate asset is out of scope for v1.
- Only the existing PNG and the build-time derivatives (favicons, OG image, header logo sizes) are produced.

## Compatibility matrix

- The site shows only the **minimum** Go version (currently `Go 1.26+`).
- A version-by-version compatibility matrix is out of scope.

## URL versioning

- No `/v1/...` URL prefix on day one.
- Re-evaluated when MuxMaster v2 ships (see `url-and-versioning.md`).

## Internationalisation

- The site is English-only on day one.
- No `hreflang` tags. No translations.
- A future internationalisation effort would re-evaluate URL structure and `<html lang>` per locale.

## Public forms

- No newsletter form, no feedback form, no contact form.
- Issue reporting goes to GitHub.

## Comments

- No comments on documentation pages.

## User accounts

- No authentication. No accounts. No personalisation.

## Web fonts

- No web fonts. System font stack only (see `brand-and-visual.md`).

## Service worker, offline, PWA

- No service worker, no manifest beyond what is needed for the favicon set, no installable PWA experience.

## Persistent dark-mode preference

- The dark-mode toggle does not persist across visits in v1 (a refresh resets to the system preference).
- Persistence is a candidate for the first JavaScript enhancement.
