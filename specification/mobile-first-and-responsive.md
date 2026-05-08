---
title: Mobile-first and responsive layout
purpose: Define breakpoint strategy, fluid layout primitives, touch-target rules, and the responsive-images contract.
owners: tailwind-specialist (primary); ux-specialist (experiential verification); seo-specialist (CLS and image performance).
last-updated: 2026-05-08
status: ratified
---

# Mobile-first and responsive layout

## Authoring direction

- All CSS MUST be authored mobile-first: base styles target the smallest supported viewport (320 px wide), and `min-width` media queries scale up.
- Desktop-first cascades (using `max-width` to remove styles for small viewports) are forbidden.

## Supported viewports

- Smallest viewport supported: **320 px** wide. The site MUST be usable and legible at this width with no horizontal scroll on the body.
- Test pass MUST cover the following widths before any change is merged: 320, 375, 414, 768, 1024, 1280, 1440 px.

## Breakpoints

- The site uses Tailwind v4's default breakpoints unless overridden in the design tokens:
  - `sm` ≥ 640 px
  - `md` ≥ 768 px
  - `lg` ≥ 1024 px
  - `xl` ≥ 1280 px
  - `2xl` ≥ 1536 px
- Sidebar (on `/docs/`) collapses below `lg`. Below `lg`, the sidebar MUST be reachable via a native `<details>`/`<summary>` disclosure (no JS dependency).
- In-page TOC sticky right rail appears at `lg` and above; below `lg`, the TOC is rendered above the article body as a collapsible block.

## Fluid layout primitives

- Layouts MUST use CSS Grid, Flexbox, `clamp()`, and container queries.
- Fixed pixel widths for content regions are forbidden. Maximum content width is expressed in `ch` or fluid units (`min(72ch, 100% - 2rem)`).
- Reading-measure for prose body MUST be between 60 and 80 characters at the comfortable line length.

## Touch targets

- Every interactive element (link, button, toggle, disclosure summary) MUST have a hit area of **at least 44 × 44 CSS px**, including padding.
- Adjacent touch targets MUST be separated by at least 8 px or be visually grouped to avoid mis-taps.
- Hover-only interactions are forbidden (also stated in `accessibility-and-standards.md`).

## Responsive images

- Every `<img>` MUST set explicit `width` and `height` attributes that match the intrinsic ratio of the image, to prevent CLS.
- Below-the-fold images MUST set `loading="lazy"`. Above-the-fold images (logo, hero illustration) MUST NOT lazy-load.
- `decoding="async"` SHOULD be set on all images.
- Multiple sizes MUST be provided via `srcset` and `sizes` for the logo derivatives (32, 48, 192, 512, 1024 px) and the OG image (1200 × 630 px PNG only — Open Graph consumers do not require `srcset`).
- Modern formats: AVIF preferred, WebP fallback, PNG fallback. The `<picture>` element MUST be used when more than one format is offered.

## Typography scaling

- Base font size on body text: 16 px on small viewports, scaling up via `clamp(1rem, 1rem + 0.25vw, 1.125rem)` to a maximum of 18 px.
- Headings scale fluidly via `clamp()` between a small-viewport size and a large-viewport size.
- Line height: 1.6 for body, 1.2 for headings.

## Content density

- Content density MUST NOT increase on small viewports beyond what is comfortable for thumb scrolling.
- Tables that overflow the viewport MUST scroll horizontally inside their own container; the page itself MUST NOT scroll horizontally.

## Orientation

- The site MUST work in portrait and landscape orientations across all supported viewports.
- No content is gated behind a specific orientation.
