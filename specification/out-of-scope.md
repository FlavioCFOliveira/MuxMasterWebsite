---
title: Out of scope (v1)
purpose: Enumerate what v1 of the site explicitly does not do, so that future requests to add these features are recognised as expansions and trigger explicit ratification.
owners: specification-manager.
last-updated: 2026-05-08
status: ratified
---

# Out of scope (v1)

The following are deliberately **not** part of v1 of the website. Adding any of them is a scope expansion and MUST be ratified before implementation.

## Search

- No on-site search index.
- No client-side search widget.
- Browser `Ctrl-F` and the sitemap are the day-one substitutes.
- Re-evaluated when content volume justifies it.

## Analytics

- No analytics on day one. Privacy-first.
- When added, the preferred option is **self-hosted Plausible**. Any choice MUST be ratified.

## Audit-report mirroring

- The `${MUXMASTER_SOURCE_DIR}/reports/` directory is **not** mirrored on the site.
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
