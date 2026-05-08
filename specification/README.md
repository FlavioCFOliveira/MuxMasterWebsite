---
title: Specification index
purpose: Single entry point and table of contents for the MuxMaster website functional specification.
owners: specification-manager (sole writer); seo-specialist, geo-specialist, tailwind-specialist, ux-specialist (review).
last-updated: 2026-05-08
status: ratified (initial)
---

# MuxMaster website — functional specification

This folder is the single source of truth for **what** the MuxMaster documentation website is, **what** it presents, and **what** contracts it must honour. It does not prescribe implementation details beyond what is necessary to make the contracts concrete.

The specification governs the repository at `/data/dev/github.com/FlavioCFOliveira/MuxMasterWebsite`. The repository's working rules (Default Ignorance Principle, workflow order, gatekeeper agents) are defined in `../CLAUDE.md` and take precedence over anything not explicitly captured here.

## Audience

- **Human implementers** scaffolding and maintaining the site.
- **AI agents** operating in agentic workflows that need a precise, unambiguous contract to act on.
- **Reviewers** (the four gatekeeper agents) who evaluate proposed changes against the rules below.

## Document conventions

- Each file carries a header block listing its purpose, owners (gatekeeper agents that apply), and last-updated date.
- File names are lowercase, kebab-case, and reflect a single functional area.
- The marker `TBD` denotes a decision that has not yet been ratified. Every `TBD` is also tracked in `open-questions.md`. Implementation must not invent values for `TBD` items; it must request a ratification first.
- The marker `MUST` denotes a non-negotiable contract. `SHOULD` denotes a strong preference that may be relaxed only with explicit ratification. `MAY` denotes a permitted option.
- Cross-references use relative links between files in this folder.

## Files in this specification

1. [overview.md](./overview.md) — purpose, audience, missions, version cadence, language and integrity rules.
2. [information-architecture.md](./information-architecture.md) — sitemap, URLs, navigation, page templates, breadcrumb and prev/next rules.
3. [content-sources.md](./content-sources.md) — mapping of every public route to its upstream source, runtime contract for the upstream tree.
4. [rendering-and-caching.md](./rendering-and-caching.md) — server-side rendering pipeline, render cache, HTTP cache headers, ETag and Last-Modified strategy.
5. [url-and-versioning.md](./url-and-versioning.md) — URL conventions, redirects, reserved paths, version label rule.
6. [seo.md](./seo.md) — SEO contract per page family, JSON-LD shapes, sitemap, robots, security headers, Core Web Vitals targets.
7. [geo.md](./geo.md) — Generative Engine Optimization contract, llms.txt artefacts, Markdown companions, AI crawler allowlist.
8. [accessibility-and-standards.md](./accessibility-and-standards.md) — WCAG 2.2 AA contract, semantics, keyboard, focus, contrast, reduced motion.
9. [mobile-first-and-responsive.md](./mobile-first-and-responsive.md) — breakpoint strategy, fluid layout primitives, touch targets, responsive images.
10. [brand-and-visual.md](./brand-and-visual.md) — logo, palette, dark mode, type, code-block style, asset generation.
11. [deployment.md](./deployment.md) — Docker model, runtime contract, environment variables, reverse-proxy expectations, health endpoint, logs.
12. [agents-and-gates.md](./agents-and-gates.md) — gatekeeper agents and their ownership areas; final-gate rule for ux-specialist.
13. [out-of-scope.md](./out-of-scope.md) — what v1 of the site explicitly does not do.
14. [open-questions.md](./open-questions.md) — register of TBD items with owners, blocking impact, and resolution path.

## Ratification status

The decisions captured in this initial specification were agreed with the owner on 2026-05-08. The items listed in `open-questions.md` are explicitly not yet ratified; no implementation decision may close them silently.

## Related external references

- Project rules: `../CLAUDE.md`
- Upstream MuxMaster repository: `https://github.com/FlavioCFOliveira/MuxMaster` (local checkout `../MuxMaster`).
- llms.txt convention: `https://llmstxt.org`.
- WCAG 2.2: `https://www.w3.org/TR/WCAG22/`.
