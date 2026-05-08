---
title: Brand and visual identity
purpose: Define the visual identity — logo handling, palette, dark mode, typography, code-block style, and asset generation.
owners: tailwind-specialist (primary); ux-specialist (experiential verification); seo-specialist (font loading, CLS).
last-updated: 2026-05-08
status: ratified (TBD on exact accent hexes)
---

# Brand and visual identity

## Visual character

- Restrained technical canvas. Neutral greys and zincs dominate. The logo carries the brand energy on the header, on the OG image, and as the favicon source.
- Two accent colours are used: cyan (links and primary CTAs) and yellow (occasional highlights, badges, version label backdrop).
- The visual character MUST defer to content; decoration that competes with code or copy is forbidden.

## Logo

- Source of truth: `${MUXMASTER_SOURCE_DIR}/assets/logo-muxmaster.png` (1024 × 1024 PNG, RGBA, mascot Gopher).
- The header MUST display the logo to the left of the navigation, sized at 32 × 32 CSS px on small viewports and 40 × 40 CSS px on `md` and above.
- The logo image MUST be served from `/assets/<hash>/logo.<size>.png` with long-cache headers (see `rendering-and-caching.md`).
- SVG variant and horizontal-lockup variant are out of scope for v1 (see `out-of-scope.md`).

## Colour palette

### Neutrals

- The neutral palette uses Tailwind v4's `zinc` scale by default. Concrete tokens:
  - Light theme: page background `zinc-50`, surface `white`, body text `zinc-900`, secondary text `zinc-600`, border `zinc-200`.
  - Dark theme: page background `zinc-950`, surface `zinc-900`, body text `zinc-100`, secondary text `zinc-400`, border `zinc-800`.

### Accents (TBD on exact hexes)

- **Cyan** (links, primary CTAs, focus ring tint): derived from the cyan in the logo. Exact hex is **TBD** (`open-questions.md` item 2).
- **Yellow** (badges, highlights, version label backdrop): derived from the yellow in the logo. Exact hex is **TBD** (`open-questions.md` item 2).
- Both accents MUST be tested against the neutral palette in light and dark themes for WCAG AA contrast (see `accessibility-and-standards.md`).

## Dark mode

- Dark mode follows the system preference by default (`prefers-color-scheme`).
- A toggle in the header overrides the system preference for the duration of the visit. The toggle MUST work without JavaScript.
- No-JS pattern: a hidden `<input type="checkbox">` whose state is persisted via the CSS `:has()` selector, with the cascade flipping a custom property on `:root`. Persistence across requests is **not** required on day one (a refresh resets to the system preference). Persistence is a candidate for re-evaluation when the site supports JavaScript-based enhancements.
- The toggle's accessible name is "Toggle dark mode" and the toggle MUST be keyboard operable.
- Both `theme-color` meta tags (light and dark, via `media`) MUST be present.

## Typography

- **System font stack only.** No web fonts. No `@font-face` rules.
- Sans-serif stack (body, headings):
  `ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif`.
- Monospace stack (code blocks, inline code):
  `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace`.
- Headings scale (refer to `mobile-first-and-responsive.md` for fluid scaling):
  - `<h1>`: `clamp(1.875rem, 1.5rem + 1.5vw, 2.5rem)`, weight 700.
  - `<h2>`: `clamp(1.5rem, 1.25rem + 1vw, 2rem)`, weight 700.
  - `<h3>`: `clamp(1.25rem, 1.125rem + 0.5vw, 1.5rem)`, weight 600.
  - `<h4>`: 1.125rem, weight 600.
- Body line height: 1.6. Heading line height: 1.2.

## Link styles

- Default: cyan accent text, underlined with `text-underline-offset: 0.2em` and `text-decoration-thickness: 1px`.
- Hover: thicker underline (`2px`).
- Focus: 2 px solid outline using the cyan accent at AA-compliant contrast.
- Visited links MUST be visually distinct only when within prose body — navigation and footer links do not change after visit.

## Code blocks

- Monospace stack defined above. Base size 14 px on small viewports, scaling up to 15 px on `lg`.
- Background: a slightly elevated surface against the page (light: `zinc-100`; dark: `zinc-900`).
- Inline `<code>` shares the same background with reduced padding.
- Syntax highlighting palette MUST be selected to meet AA contrast in both themes; the highlighter is implementation-defined but MUST run server-side (no client-side highlighter) so pages are useful with JavaScript disabled.

## Asset generation (build-time)

A build-time script MUST generate the following from `${MUXMASTER_SOURCE_DIR}/assets/logo-muxmaster.png`:

| Asset | Use |
| --- | --- |
| `favicon.ico` (16, 32, 48 multi-size) | `<link rel="icon">` for legacy browsers. |
| `favicon-32.png`, `favicon-192.png`, `favicon-512.png` | `<link rel="icon" sizes="...">`. |
| `apple-touch-icon-180.png` | `<link rel="apple-touch-icon">`. |
| `og-image-1200x630.png` | Open Graph and Twitter Card image. The image MUST include the logo and the wordmark "MuxMaster" with the tagline derived from the landing page (TBD). |
| `logo-32.png`, `logo-40.png`, `logo-80.png` (and AVIF/WebP equivalents) | Header logo (responsive `srcset`). |

Generated assets MUST be content-hashed (`logo.<hash>.png`) and served from `/assets/<hash>/...`.

## CSS toolchain

- The CSS bundle is built with the **Tailwind CSS v4 standalone CLI binary**. No Node, no npm at runtime.
- The CLI is invoked at build time. The `@source` directive(s) MUST cover all template files and any inline class strings used by handlers.
- Output: a single content-hashed file at `/assets/<hash>/app.css`.
- The bundle is served by MuxMaster via its file-serving primitive with `Cache-Control: public, max-age=31536000, immutable`.
- No second-stage CSS framework or runtime CSS-in-JS.

## No JavaScript dependency for content

- Content rendering and primary interaction MUST NOT depend on JavaScript. The dark-mode toggle, the sidebar disclosure on small viewports, and the in-page TOC MUST work with JavaScript disabled.
- JavaScript MAY be added later as an enhancement (for example, a copy-to-clipboard button on code blocks). It MUST be additive, not load-bearing.

## Motion

- Transitions are limited to `200ms` ease-out on hover, focus, and theme switch.
- All motion MUST respect `prefers-reduced-motion: reduce`.
