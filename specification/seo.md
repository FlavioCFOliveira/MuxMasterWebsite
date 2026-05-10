---
title: SEO contract
purpose: Define the search-engine optimisation contract for every page family — head metadata, structured data, sitemap, robots, security headers, and Core Web Vitals targets.
owners: seo-specialist (final review); review by ux-specialist (anchor-text descriptiveness), geo-specialist (structured-data overlap with GEO).
last-updated: 2026-05-10
status: ratified
---

# SEO contract

## Per-page head metadata (mandatory on every indexable page)

Every indexable HTML page MUST include the following inside `<head>`:

- `<meta charset="utf-8">`.
- `<meta name="viewport" content="width=device-width, initial-scale=1">`.
- A unique `<title>`. Format: `<Page title> — MuxMaster`. Maximum 60 visible characters where practical.
- A unique `<meta name="description" content="...">`. 110 to 160 characters. Plain, factual, no ellipsis-truncation.
- `<link rel="canonical" href="<absolute URL on the canonical domain>">`. The canonical domain is **TBD** (`open-questions.md` item 1). Until it is decided, the canonical link MUST use the value of `SITE_BASE_URL` and the page MUST NOT be marked indexable in production.
- Open Graph: `og:type` (`website` for `/`, `article` everywhere else), `og:title`, `og:description`, `og:url` (absolute), `og:image` (absolute, pointing at the generated 1200×630 OG image — see `brand-and-visual.md`), `og:site_name` ("MuxMaster"), `og:locale` ("en_US").
- Twitter Card: `twitter:card` (`summary_large_image`), `twitter:title`, `twitter:description`, `twitter:image`.
- `<meta name="theme-color" content="...">` matching the dark and light themes (two entries with `media` attributes).
- Favicons (see `brand-and-visual.md`).

Pages that MUST NOT be indexed (until the canonical domain is decided, or because they are operational): the page MUST emit `<meta name="robots" content="noindex,nofollow">` and the route MUST be excluded from `sitemap.xml`. `/healthz` is in this category permanently.

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
- Excluded from the sitemap: `/healthz`, `/robots.txt`, `/sitemap.xml`, `/llms.txt`, `/llms-full.txt`, all `.md` companions, all `/assets/...` paths.
- Each entry includes `<loc>` (absolute URL on the canonical domain), `<lastmod>` (the mtime of the underlying file in `/content/` in W3C datetime format; for routes whose source is the registered route table rather than a single file — `/`, `/docs/`, `/examples/` — `<lastmod>` MUST be the process start time), `<changefreq>` (`monthly` for documentation, `weekly` for `/changelog` and `/`), and `<priority>` (1.0 for `/`, 0.8 for `/docs/`, `/api`, `/examples/`, 0.6 for `/docs/<section>`, `/examples/<name>`, `/benchmarks`, 0.4 for the rest).

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
