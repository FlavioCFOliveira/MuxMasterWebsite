---
title: SEO contract
purpose: Define the search-engine optimisation contract for every page family — head metadata, structured data, sitemap, robots, security headers, and Core Web Vitals targets.
owners: seo-specialist (final review); review by ux-specialist (anchor-text descriptiveness), geo-specialist (structured-data overlap with GEO).
last-updated: 2026-09-26
status: ratified
---

# SEO contract

## Per-page head metadata (mandatory on every indexable page)

Every indexable HTML page MUST include the following inside `<head>`:

- `<meta charset="utf-8">`.
- `<meta name="viewport" content="width=device-width, initial-scale=1">`.
- A unique `<title>`. Format: `<Page title> — MuxMaster`, except on release-notes pages, whose format is defined in SEO-REL-1. Maximum 60 visible characters where practical.
- A unique `<meta name="description" content="...">`. 110 to 160 characters. Plain, factual, no ellipsis-truncation.
- `<link rel="canonical" href="<absolute URL on the canonical domain>">`. The canonical domain was ratified on 2026-05-11 as `https://muxmaster.net` (HTTPS, apex, no trailing slash — see `open-questions.md` item 1, RESOLVED, and `deployment.md` § Runtime environment variables). The `<link rel="canonical">` value MUST therefore be `https://muxmaster.net<path>` on every indexable page in production. In `development` and `staging` (where `SITE_BASE_URL` is not the canonical production domain), the page MUST NOT be marked indexable and the canonical link MUST mirror the value of `SITE_BASE_URL` so that staging and production never advertise the same canonical URL.
- Open Graph: `og:type` (`website` for `/`, `article` everywhere else), `og:title`, `og:description`, `og:url` (absolute), `og:image` (absolute, pointing at the generated 1200×630 OG image — see `brand-and-visual.md`), `og:site_name` ("MuxMaster"), `og:locale` ("en_US").
- Twitter Card: `twitter:card` (`summary_large_image`), `twitter:title`, `twitter:description`, `twitter:image`.
- `<meta name="theme-color" content="...">` matching the dark and light themes (two entries with `media` attributes).
- Favicons (see `brand-and-visual.md`).

Pages that MUST NOT be indexed (because the deployment is not the canonical production origin at `https://muxmaster.net`, or because they are operational): the page MUST emit `<meta name="robots" content="noindex,nofollow">` and the route MUST be excluded from `sitemap.xml`. `/healthz` is in this category permanently.

## Page-specific requirements

These rules add to the per-page head metadata above. Every description they govern is also subject to `overview.md` § Performance claims (INT-PERF-7: no performance numbers in space-limited surfaces).

- **SEO-QUERY-1.** `/docs/http-query-method` MUST use the `<title>` "HTTP QUERY method (RFC 10008) in Go — MuxMaster". Its `<h1>` and sidebar label are defined in `information-architecture.md` IA-QUERY-2. Its `<meta name="description">` MUST name the HTTP QUERY method, RFC 10008, and MuxMaster, and MUST state that QUERY is a safe, idempotent method that carries a request body.
- **SEO-QUERY-2.** Links to `/docs/http-query-method` MUST use anchor text that contains "HTTP QUERY method" (for example "HTTP QUERY method (RFC 10008)").
- **SEO-QUERY-3.** On `/releases/v1.2.0` and on `/changelog`, the first mention of the HTTP QUERY method in the page body MUST link to `/docs/http-query-method`, with anchor text that complies with SEO-QUERY-2.
- **SEO-LAND-1.** The `<meta name="description">` of `/` MUST mention MuxMaster's support for the HTTP QUERY method (RFC 10008) and MUST carry a performance-positioning statement that complies with INT-PERF-3, INT-PERF-4, and INT-PERF-7 (categories named in words, no numbers). The same applies to the `og:description` and `twitter:description` of `/`.
- **SEO-REL-1.** Every release-notes page (`/releases/v<version>`) MUST carry a unique `<title>` of the form "MuxMaster v<version> release notes" (for example "MuxMaster v1.2.0 release notes"). `/releases/v1.2.0` and `/releases/v1.3.0` MUST each also carry a unique `<meta name="description">`. The description of `/releases/v1.2.0` MUST mention the HTTP QUERY method (RFC 10008); the description of `/releases/v1.3.0` MUST mention the Go 1.27.1 minimum.
- **SEO-BENCH-1.** The `<meta name="description">` of `/benchmarks` MUST name MuxMaster v1.3.0 and state that the results come from a measurement campaign comparing v1.1.0 with v1.3.0 and with other Go routers; it MUST NOT contain numbers (INT-PERF-7).
- **SEO-VER-1.** Every page that states the MuxMaster version or the minimum Go version MUST use the values in `url-and-versioning.md` URL-VER-1 (v1.3.0; Go 1.27.1).

## Semantic HTML5

- Each page MUST contain exactly one `<h1>`, located in the main article region.
- Heading hierarchy MUST be strict: `<h1>` → `<h2>` → `<h3>` with no skipped levels.
- The page MUST include `<header>`, `<main>`, `<nav>`, `<footer>` landmarks. The `<main>` element MUST wrap the primary article.
- Documentation pages MUST wrap their article body in `<article>`.
- Lists, tables, and code blocks MUST use the corresponding semantic elements (`<ul>`, `<ol>`, `<table>`, `<pre><code>`). No `<div>`-only structures where a semantic element exists.

## JSON-LD structured data

The master schema-by-page-family table, the entity graph (the four reified nodes referenced by `@id` from every page), the per-type field expectations, the auxiliary schemas (`APIReference`, `DefinedTerm`/`DefinedTermSet`, `Code`), and the blocking CI validation gate are defined in `structured-data.md`. SEO's specific concern in that contract is **rich-result eligibility**: search-engine result pages render breadcrumb trails, FAQ accordions, How-To carousels, code-repository panels, and dataset summaries when the JSON-LD is well-formed and complete. See `structured-data.md` for the master table, the field-completeness rules, and the validation gate.

## sitemap.xml

- The server MUST generate `/sitemap.xml` from the registered route list at startup.
- Excluded from the sitemap: `/healthz`, `/robots.txt`, `/sitemap.xml`, `/llms.txt`, `/llms-full.txt`, all `.md` companions, all `/static/...` paths.
- Each entry includes `<loc>` (absolute URL on the canonical domain), `<lastmod>` (see SEO-MAP-1), `<changefreq>`, and `<priority>`.
- **SEO-MAP-1.** `<lastmod>` MUST be the later of `dateModified` and `datePublished` in the front matter of the content file that backs the route, formatted `YYYY-MM-DD`. The index routes `/`, `/docs/`, and `/examples/` use the dates in their own front matter when present; otherwise they use the latest `<lastmod>` among their child routes (for `/docs/`, the doc pages; for `/examples/`, the example pages; for `/`, every other route in the sitemap). A route with no date from either source MUST omit `<lastmod>`; it MUST NOT substitute the process start time or any other value. The HTTP `Last-Modified` header is unaffected: it remains the process start time (`rendering-and-caching.md` § ETag and Last-Modified).
- The `<priority>` value is `1.0` for `/`, `0.8` for `/docs/`, `/api`, `/examples/`, `0.6` for `/docs/<section>`, `/examples/<name>`, `/benchmarks`, and `0.4` for the rest.
- The `<changefreq>` value is governed by the cadence at which the page's primary content is expected to change, not by the page's importance:

| Page family | `<changefreq>` | Rationale |
| --- | --- | --- |
| `/` | `weekly` | The hero, headline benchmarks, and "what's new" panel may shift between releases. |
| `/changelog` | `weekly` | Updated on every release of MuxMaster. |
| `/docs/` and `/examples/` | `monthly` | These are stable section indexes — the cards listed do not churn often, only when a new doc or example is added. |
| All other documentation pages (`/docs/<section>`, `/examples/<name>`, `/api`, `/benchmarks`, `/security`, `/compatibility`, `/contributing`, `/built-with-muxmaster`, `/releases/<v>`) | `monthly` | Documentation cadence; corrections and clarifications land at most monthly. |

## robots.txt (search-engine portion)

The full `robots.txt` is co-owned with `geo.md`. The search-engine portion MUST:

- Allow all paths by default for `User-agent: *`.
- Disallow `/healthz`.
- Reference the canonical sitemap: `Sitemap: <absolute URL>/sitemap.xml`.

The AI-crawler portion is defined in `geo.md`.

## Security headers (mandatory on every response)

| Header | Value |
| --- | --- |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains; preload` (in production, when served over HTTPS by the reverse proxy). |
| `Content-Security-Policy` | `default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; upgrade-insecure-requests`. No inline scripts. No inline styles unless the design tokens require a single nonce-controlled `<style>` element; in that case the CSP is updated and re-ratified. |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `accelerometer=(), camera=(), geolocation=(), gyroscope=(), microphone=(), payment=(), usb=()` |
| `X-Frame-Options` | `DENY` (also covered by CSP `frame-ancestors`; both set for defence-in-depth). |
| `Cross-Origin-Opener-Policy` | `same-origin` |
| `Cross-Origin-Resource-Policy` | `same-origin` |

## Core Web Vitals targets

- **LCP** (Largest Contentful Paint): < 2.5 s on the 75th percentile, on a simulated 4G connection with a moderate device.
- **INP** (Interaction to Next Paint): < 200 ms on the 75th percentile.
- **CLS** (Cumulative Layout Shift): < 0.1.
- The site is server-rendered and dependency-light; meeting these targets is feasible without a JavaScript framework. Any change that pushes a page above any threshold is a regression and MUST be addressed before merge (`seo-specialist` REJECTED).

## Image and media

- Every `<img>` MUST have explicit `width` and `height` to prevent CLS.
- `loading="lazy"` MUST be set on images below the fold; the logo and OG image are above the fold and MUST NOT be lazy-loaded.
- `decoding="async"` SHOULD be set on all images.
- Modern formats (AVIF, WebP) with PNG fallback SHOULD be used for the logo derivatives. `srcset` and `sizes` MUST be set when multiple sizes are provided.

## Anchor text

- Every link MUST have descriptive anchor text. "Click here" and "read more" are forbidden.
- External links indicate that they are external in their accessible name (text or `aria-label`).

## Pagination, infinite scroll

- Not used on day one. Documentation pages are single-page articles.
