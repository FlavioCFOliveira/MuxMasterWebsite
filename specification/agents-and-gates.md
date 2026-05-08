---
title: Agents and gatekeeping
purpose: Record the coordinator agents — four gatekeepers and one content curator — their ownership areas, and the final-gate rule for ux-specialist.
owners: specification-manager.
last-updated: 2026-05-08
status: ratified
---

# Agents and gatekeeping

The project ships dedicated subagents under `.claude/agents/` (defined in `../CLAUDE.md`). The coordination model has two roles:

- **Gatekeepers** review proposed changes and issue `APPROVED`, `APPROVED WITH CHANGES`, or `REJECTED` verdicts. Blocking fixes from any gatekeeper MUST be applied before merge. There are four gatekeepers: `seo-specialist`, `geo-specialist`, `tailwind-specialist`, and `ux-specialist`.
- **Authors** produce changes for the gatekeepers and the project owner to review. There is one author: `content-curator`. Authors do not issue verdicts.

## Agents and ownership areas

### seo-specialist

Final review authority on traditional SEO, Core Web Vitals, web standards, and accessibility (WCAG 2.2 AA). Owns:

- `<head>` metadata: title, description, canonical, Open Graph, Twitter Card.
- `sitemap.xml` and the search-engine portion of `robots.txt`.
- JSON-LD for rich results (`TechArticle`, `BreadcrumbList`, `SoftwareSourceCode`, `Organization`, `Dataset`, `CollectionPage`, `FAQPage` overlap with `geo.md`).
- HTTP semantics: status codes, redirects, caching, compression.
- Security headers.
- Image, font, and asset performance budget.

Blocks merge when the SEO contract in `seo.md` is not met.

### geo-specialist

Final review authority on Generative Engine Optimization. Owns:

- `/llms.txt` and `/llms-full.txt`.
- Markdown companion representations (`.md` URLs) and their content equivalence with the HTML representations.
- The AI-crawler portion of `robots.txt` (`GPTBot`, `ClaudeBot`/`anthropic-ai`, `PerplexityBot`, `Google-Extended`, `Applebot-Extended`, `CCBot`, and so on, as enumerated in `geo.md`).
- Content shape: definition-first sentences, self-contained paragraphs, concrete numbers, inline citations.
- Comparison tables and `FAQPage` / `HowTo` JSON-LD.

Blocks merge when the GEO contract in `geo.md` is not met.

### tailwind-specialist

Final review authority on UI and visual design implemented with Tailwind CSS v4. Owns:

- Design tokens (`@theme`).
- `@source` configuration.
- Layout primitives.
- Component styling.
- Mobile-first responsive layout.
- Dark mode.
- Typography.
- No-JavaScript interaction patterns (`<details>`, `popover`, `:has()`, `:target`, `@starting-style`).
- Animation and motion respecting `prefers-reduced-motion`.

Hard rule: **no JavaScript dependency for content rendering or primary interaction**. Blocks merge when this rule or the visual contract in `brand-and-visual.md` and `mobile-first-and-responsive.md` is not met.

### ux-specialist (final gate)

User Experience and usability holistic coordinator. **Invoked last**, after the other three have completed their reviews and any blocking fixes have been applied. Owns:

- Information architecture.
- Navigation design (sidebar, breadcrumb, in-page TOC, prev/next).
- User flows: landing → quickstart → first running router; "evaluating routers"; "broke production, need fix".
- Microcopy and labels.
- Page-template purpose.
- Empty, error, and loading states.
- Code-example UX.
- Onboarding flow.
- Cognitive-load budget.
- Heuristic evaluation (Nielsen's ten).
- Cross-page experience consistency.

`ux-specialist` does not edit code. It produces holistic verdicts with exact rewrite proposals routed to the main session or to a peer agent. A `ux-specialist` REJECTED blocks merge even when the three peer agents have approved.

### content-curator (author, not gatekeeper)

Authoring agent responsible for keeping `/content/` in sync with the upstream `../MuxMaster` source of truth. **Not** a gatekeeper — it does not issue `APPROVED` / `REJECTED` verdicts on proposed changes; it produces them. Owns:

- Reading `../MuxMaster/` via the environment variable `MUXMASTER_SOURCE_DIR` (development-time / agent-time only — this variable is **not** read by the runtime binary).
- Writing Markdown files under `/content/` (and only under `/content/`) per the layout and per-file shape defined in `content-sources.md`.
- Producing a diff for the project owner and the gatekeepers to review. The agent MUST NOT auto-commit.
- Flagging any inconsistency it cannot resolve (for example, a new upstream document with no corresponding page template, or a `## Benchmarks` section that has been renamed or removed upstream) instead of silently dropping or transforming content.

The agent MUST NOT:

- Read or write Go source code, templates, static assets, or any path outside `/content/`.
- Modify the runtime binary, the build pipeline, or the deployment configuration.
- Make editorial decisions that change facts (it transforms format and structure; it does not paraphrase numbers, signatures, or behaviour claims).

The agent definition file is `.claude/agents/content-curator.md` (authored separately from this specification).

## Invocation order

- `content-curator` is invoked when a content sync is requested (see `content-sources.md`). It produces a diff under `/content/` and the diff then enters the gatekeeper review path. The curator runs before any gatekeeper is consulted on the resulting change.
- Among the four gatekeepers: the first three (`seo-specialist`, `geo-specialist`, `tailwind-specialist`) are technical-surface specialists. They MAY be invoked in any order, often in parallel.
- `ux-specialist` MUST be invoked **last**, after the other three have completed their reviews and any blocking fixes have been applied. Its job is to read the post-fix state of the change as an integrated user experience.

## Disputes

- A direct dispute between two agents MUST be **escalated to the user**. It MUST NOT be resolved unilaterally.
- The `specification-manager` is the only writer in `/specification/`. When a dispute requires a specification change to resolve, the `specification-manager` performs the edit after the user ratifies the resolution.

## Overlap zones (consult more than one)

Per `../CLAUDE.md`:

- English quality, factual integrity, contradiction sweep — all four (independent checks).
- Mobile-first compliance and touch-target sizing — `tailwind-specialist` primary, `seo-specialist` verifies WCAG, `ux-specialist` verifies experiential hit rate.
- Heading hierarchy and semantic landmarks — `seo-specialist` primary, `tailwind-specialist` must not break them for visual reasons, `ux-specialist` verifies they communicate page structure.
- Font loading strategy — `seo-specialist` primary (CLS, LCP), `tailwind-specialist` concurs on family/weight choice. (Note: the site uses no web fonts; this overlap exists in case that decision is revisited.)
- Structured-data design and dual SEO+GEO impact — `seo-specialist` and `geo-specialist`.
- Microcopy / labels / button-and-link text — `ux-specialist` primary, `geo-specialist` concurs on tone, `seo-specialist` concurs on anchor-text descriptiveness.
- Information architecture and URL structure — `ux-specialist` primary, `seo-specialist` verifies canonical/sitemap/redirect alignment.

When in doubt about which agent applies, invoke every relevant one. The cost of an extra review is trivial compared to a regression.
