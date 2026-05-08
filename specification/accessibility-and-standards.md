---
title: Accessibility and web standards
purpose: Define the WCAG 2.2 AA contract, web-standards compliance, semantic landmarks, keyboard navigation, focus, contrast, reduced motion, and the no-console-errors rule.
owners: seo-specialist (final review on standards and accessibility); ux-specialist (experiential verification); tailwind-specialist (visual implementation must not break landmarks).
last-updated: 2026-05-08
status: ratified
---

# Accessibility and web standards

## Compliance target

- The site MUST conform to **WCAG 2.2 Level AA** for every page, including dynamic states.
- The site MUST emit **valid HTML5** and **valid CSS** (validated with the W3C validators or an equivalent maintained tool).
- The browser console MUST be free of errors and warnings under the supported browsers (latest two major versions of Chromium, Firefox, Safari).

## Semantic landmarks

- Every page MUST include `<header>`, `<main>`, `<nav>`, and `<footer>` landmarks. Each occurs at most once at the page level (multiple `<nav>` regions are permitted if each carries a distinct `aria-label`, e.g. "Primary", "Documentation", "Footer").
- The skip link "Skip to main content" MUST be the first focusable element on every page and MUST move focus to `<main>`.
- `<main>` MUST have `id="main"`.

## Headings

- Each page has exactly one `<h1>`. Heading levels follow strict hierarchy (no skipping).
- Headings communicate page structure to assistive technology and to GEO consumers; their text MUST be informative on its own.

## Keyboard navigation

- Every interactive element MUST be reachable by `Tab` in a logical order matching the visual order.
- Focus MUST be visible at all times for keyboard users; the focus style is high-contrast and distinct from hover.
- The dark-mode toggle MUST be operable by keyboard. The no-JS pattern (CSS-only toggle via `:has()` on a checkbox) MUST expose a labelled `<input type="checkbox">` with an associated `<label>`.
- Disclosure widgets MUST use native `<details>`/`<summary>` to inherit keyboard semantics for free.
- No keyboard trap is permitted.

## Focus management

- Focus styles MUST meet WCAG 2.4.11 (Focus Not Obscured) and 2.4.13 (Focus Appearance) at AA: a 2 px solid outline, contrast ratio ≥ 3:1 against the adjacent background.
- Focus MUST NOT be removed (`outline: none`) without an equivalent replacement.

## Colour and contrast

- Body text MUST meet contrast ratio ≥ 4.5:1 against its background.
- Large text (≥ 24 px or ≥ 18.66 px bold) MUST meet ≥ 3:1.
- Non-text UI elements (focus rings, form borders, the dark-mode toggle states) MUST meet ≥ 3:1 against their adjacent background.
- The neutral palette (zinc/grey scale) and accent colours (cyan, yellow) MUST be selected so these ratios are met in both light and dark themes (see `brand-and-visual.md`).

## Reduced motion

- All transitions and animations MUST respect `prefers-reduced-motion: reduce`. When set, motion is replaced with an instant state change.
- No autoplay video, no auto-rotating carousels.

## Language attribute

- `<html lang="en">` MUST be set on every page.

## Images and media

- Every `<img>` MUST have an `alt` attribute. Decorative images use `alt=""` and `aria-hidden="true"`. Informative images describe the information they convey, not the picture (the logo's alt is `"MuxMaster"` on the header link, but `""` if it sits next to a visible "MuxMaster" wordmark).
- SVG icons used decoratively MUST have `aria-hidden="true"` and `focusable="false"`.

## Forms

- The site has no public form on day one. If a form is added (newsletter, search), every input MUST have a programmatically associated `<label>`, errors MUST be announced via `aria-live`, and validation MUST work without JavaScript.

## Tables

- Data tables MUST use `<th scope="col|row">` and a `<caption>` describing the table's content.

## Code blocks

- Code blocks MUST be wrapped in `<pre><code>` with a language class (`language-go`, `language-bash`, `language-text`).
- Long code blocks MUST be horizontally scrollable on small viewports without overflowing the page.
- Code blocks MUST NOT be the sole carriers of essential information; an explanatory paragraph precedes or follows.

## Touch and pointer

- See `mobile-first-and-responsive.md` for the touch-target rule (44 × 44 CSS px).
- No interaction MAY require hover. Hover-only enhancements (e.g. a tooltip) MUST also be reachable on focus and on touch.

## Error pages

- The 404 and 500 pages MUST follow the same accessibility contract as content pages, including landmarks, heading hierarchy, focus management, and skip link.

## Validation tooling

The following checks MUST run in CI before any release:

- W3C HTML validator (or `html-validate`, or equivalent) on every rendered page.
- Lighthouse Accessibility category ≥ 95 on every page.
- `axe-core` audit (zero serious or critical violations).
- Link checker over all internal links.
- Lighthouse Core Web Vitals targets (see `seo.md`).
