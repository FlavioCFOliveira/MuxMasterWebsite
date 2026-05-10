# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Purpose

This repository is the **official documentation website for the MuxMaster Go module** (https://github.com/FlavioCFOliveira/MuxMaster — also available locally at `../MuxMaster`).

MuxMaster is a high-performance, zero-dependency HTTP router for Go (radix tree, O(k) lookups, zero-alloc static routes, 100% `net/http` compatible). The website's job is to present, document, and promote the module — API reference, guides, benchmarks, examples, release notes — to two audiences: human developers and AI/LLM crawlers.

## Project assets

- **Logo:** the canonical project logo lives in the upstream MuxMaster repository at https://github.com/FlavioCFOliveira/MuxMaster/blob/main/assets/logo-muxmaster.png (raw: https://raw.githubusercontent.com/FlavioCFOliveira/MuxMaster/main/assets/logo-muxmaster.png). Use this as the source of truth for any logo usage on the site (header, favicon source, Open Graph image base, etc.).

## Roadmap

**Name:** muxmasterwebsite

All technical roadmap work for this project (sprints, tasks, backlog, audit) lives in the `muxmasterwebsite` roadmap, managed via the `rmp` (Groadmap) CLI. Use the `roadmap-manager` skill for any roadmap operation.

## Default Ignorance Principle

**This is the founding rule of how you operate in this repository. Every other rule in this file — Collaboration rules, Workflow, the non-negotiable constraints, agent boundaries — derives from it. When any other rule is silent, this one applies. When any other rule conflicts with this one, this one wins.**

**By default, you know nothing.** The only information you are allowed to treat as true is what is explicitly written in the project specification at `/specification/` (the canonical source of truth, managed by the `specification-manager` agent). Anything not in the specification is, by default, **unknown** — regardless of what your training data, intuition, the surrounding code, prior conversations, your memory store, or your general knowledge of the topic might suggest. Unknown does not mean "guess sensibly"; it means "do not assume — ask".

You may only operate, act, write code, edit files, run commands, or produce documentation when the instructions and information you have are **clear, specific, and concrete**. Whenever you encounter any of the following — no matter how minor it appears:

- A doubt of any kind,
- Missing information,
- Incomplete information,
- A contradiction (within the specification, between specification and code, between rules, between this file and another, between any two sources),
- An ambiguity, or
- Multiple plausible interpretations of a user request,

you must **stop immediately** and **ask the user for clarification before taking any action**. This applies even when the gap seems trivial, the answer seems obvious, the user appears to be in a hurry, or proceeding would feel more helpful. "Probably means X", "the obvious choice is Y", and "I'll just go with the standard pattern" are **never** acceptable grounds to proceed.

This applies to the user's own messages too. If a user request is unclear, vague, under-specified, or admits multiple reasonable interpretations, ask whatever questions are needed to remove the ambiguity **before** acting on the request.

### How to seek clarification

When clarification is needed:

1. **Gather every reasonable option you can identify** — not just the first that comes to mind. List the genuinely best alternatives (typically 2 to 4), each described concisely with its trade-offs. Do not omit a viable option to steer the user toward a preferred answer.
2. **State your recommendation** clearly, and explain the reason for it.
3. **Ask one question at a time, sequentially** — never bundle multiple clarifications into a single prompt; each answer may change the framing of the next question, and bundling forces the user to commit before seeing how their choices interact.

### After clarification — persist the answer

Once the user has answered, you must persist the result before continuing the work, choosing the right destination:

1. If the clarification expands, refines, or contradicts the project's functional requirements → **update the specification** under `/specification/` (delegate to the `specification-manager` agent when in scope) so the new fact becomes part of the single source of truth. The next time the same question arises, the answer must already be in the spec.
2. If the clarification is a durable preference, working-style instruction, recurring constraint, or non-spec context (tooling preference, factual context about the user or project, external resource pointer) → **save it to the auto-memory store** under the appropriate type (`user`, `feedback`, `project`, or `reference`).
3. If the clarification is genuinely scoped to the current task and has no future relevance → no persistence is needed; proceed with the work.

Only after the relevant persistence step is complete may you resume the actual task. Skipping clarification, skipping persistence, or proceeding on assumptions is a violation of this principle and must be treated as such.

## Collaboration rules

The mechanics for asking the user are defined in the **Default Ignorance Principle** above. In short: stop on any doubt, gather every reasonable option with a clear recommendation, ask one question at a time. **You are not authorized to make decisions on your own** — these rules govern *how* you work and apply to every task, regardless of size or apparent obviousness.

## Workflow

Every non-trivial change must follow this fixed order:

1. **Specify** — capture the requirement as an explicit, written specification under `/specification/` (managed by the `specification-manager` agent) before any code is written. Resolve every ambiguity per the **Default Ignorance Principle** above. No code starts until the specification is agreed and committed.
2. **Implement** — write the code strictly against the agreed specification. No silent scope expansion. If reality forces a deviation, return to step 1, update the specification, and only then continue.
3. **Test** — verify that the implemented behaviour matches the specification with appropriate automated tests (unit, integration, end-to-end, accessibility, Lighthouse / Core Web Vitals, structured-data validation, link checks, etc., as applicable to the change). "Looks right" is not acceptance.
4. **Document** — update every affected piece of documentation (site pages, `llms.txt`, `llms-full.txt`, markdown companions, JSON-LD, README, CHANGELOG, agent prompts, this CLAUDE.md when applicable) so it reflects the code as actually shipped. Documentation lagging behind code is treated as a defect, not a follow-up.

Skipping, reordering, or merging steps is not permitted. If a step appears unnecessary for a given change, stop and ask before proceeding.

## Stack

- **Language:** Go (matches MuxMaster's own minimum, currently Go 1.26+).
- **HTTP router:** **MuxMaster itself** is used as the router for this site — the project is therefore also a real-world dogfooding example of MuxMaster. Treat any router-related work as both a website task and a public showcase of the module.
- **Dependencies:** keep them as minimal as MuxMaster does (zero external dependencies preferred for the server side). Static assets and templates over heavy frameworks.

When adding routing/middleware code, mirror MuxMaster's idioms (`mux.Group`, `Use`, `HandlerFuncE`, typed param helpers, FastHandler where appropriate). The site's source is meant to be readable as a reference implementation.

## Non-negotiable constraints

These are hard requirements that override convenience and must be considered for **every** change:

### 0. Documentation content — language, tone, integrity
- **Language:** all project documentation — every user-facing page, page copy, code comment shown to readers, alt text, meta description, JSON-LD string, `llms.txt`/`llms-full.txt` content, README, CHANGELOG, release notes, and any other artifact a reader may encounter — **must always be written in English**, in the most exemplary English achievable, with **zero spelling, grammar, syntax, or punctuation errors**. Run a spell/grammar check before shipping; treat any error found post-merge as a defect.
- **Audience & register:** documentation is written for a **human technical audience** (developers reading the docs to learn, evaluate, or integrate MuxMaster). Use **clear, simple, unambiguous technical language**. Define terms before using them. Prefer plain words over jargon when the plain word is equally precise; use jargon only when it carries information the plain word does not.
- **Tone:** technical, didactic, simple, objective. Lead with the fact or instruction; explain mechanism after the claim. No marketing fluff, no hype adjectives ("blazing fast", "revolutionary"), no hedging ("maybe", "kind of"). Prefer short sentences and concrete nouns over abstractions.
- **Faithful to code:** documentation must describe the code **exactly as it is implemented today** — not as planned, intended, hoped for, or remembered from a previous version. Before publishing or updating any factual claim (API signature, default value, behaviour, benchmark, supported Go version, configuration option), verify it against the actual source in `../MuxMaster` (or this repository, as applicable). If the code and the docs disagree, the code is correct by definition: update the docs, never the other way around. If the code is wrong, fix the code first, then document the fixed behaviour.
- **Integrity:** the site is a single source of truth. **No contradictory information anywhere** — versions, API signatures, benchmark numbers, defaults, supported Go versions, behavior descriptions must match across every page (and match `../MuxMaster` upstream). **No vague statements** — replace "fast" with measured numbers, "supported" with the exact versions, "recommended" with the reason. If a fact cannot be stated precisely, omit it rather than approximate.
- Whenever a doc page is added or edited, cross-check related pages (and the `../MuxMaster` README/CHANGELOG/api.md) for any statement that the change would now contradict, and update them in the same commit.
- The website's dual mission is **(a) to document MuxMaster** and **(b) to be itself a working proof of MuxMaster's viability and production-readiness** — by being built on MuxMaster. Every implementation decision should be defensible under both lenses; if a shortcut on the site would weaken the proof, don't take it.

### 1. SEO (Search Engine Optimization) — ultra-optimized
Every page must ship with: a unique, descriptive `<title>`; a unique `<meta name="description">`; correct `<link rel="canonical">`; Open Graph + Twitter Card tags; semantic HTML5 (`<main>`, `<article>`, `<nav>`, `<header>`, `<footer>`, headings in correct hierarchy, exactly one `<h1>` per page); `JSON-LD` structured data (e.g. `SoftwareSourceCode`, `TechArticle`, `BreadcrumbList`, `FAQPage` where applicable); `robots.txt`; XML `sitemap.xml` (auto-generated, kept in sync with routes); `hreflang` if/when localized; clean human-readable URLs (kebab-case, no query strings for content); fast Core Web Vitals (LCP, INP, CLS); pre-rendered HTML — no client-side-only content for indexable pages.

### 2. GEO (Generative Engine Optimization) — first-class
The site must also be optimized for ingestion by LLMs / AI answer engines (ChatGPT, Claude, Perplexity, Google AI Overviews, etc.):
- Serve a top-level `/llms.txt` and `/llms-full.txt` (llmstxt.org convention) listing the canonical documentation URLs and a digestible knowledge map.
- Provide machine-readable variants of doc pages (e.g. `.md` companion at `/<path>.md` or `Accept: text/markdown` content negotiation) so LLMs can ingest content without HTML noise.
- Keep paragraphs self-contained and answer-shaped (lead with the claim, then evidence). Use clear `Q → A` structures for FAQs.
- Include explicit, factual statements (versions, benchmark numbers, supported Go versions, API signatures) rather than marketing prose — generative engines cite concrete facts.
- Allow reputable AI crawlers in `robots.txt` unless the user instructs otherwise; never silently block them.
- JSON-LD `FAQPage` and `HowTo` schemas wherever they fit naturally.

### 3. Web standards — 100%
- Valid HTML5, valid CSS, no console errors.
- WCAG 2.2 AA accessibility (semantic landmarks, `alt` text, sufficient contrast, focus states, keyboard navigation, ARIA only when native HTML can't express the semantics).
- Progressive enhancement — pages must be useful with JavaScript disabled. JS is an enhancement, never a requirement for content rendering or navigation.
- Correct HTTP semantics: status codes, `Cache-Control`, `ETag`/`Last-Modified`, `Content-Type`, `Content-Encoding` (gzip/br), `Vary`, security headers (`Content-Security-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`).
- HTTPS-only in production, HTTP/2 (or HTTP/3) where the deploy target supports it.

### 4. Mobile-first & natively responsive
- CSS authored mobile-first (base styles target small viewports; `min-width` media queries scale up). No desktop-first cascades.
- Fluid layouts (CSS Grid / Flexbox / `clamp()` / container queries). No fixed pixel widths for content regions.
- Touch targets ≥ 44×44 CSS px; no hover-only interactions.
- Responsive images (`srcset`, `sizes`, modern formats — AVIF/WebP with fallbacks, `loading="lazy"` for below-the-fold, explicit `width`/`height` to prevent CLS).
- Test changes at narrow widths (≤ 360px) before wide ones.

## Working in this repo

- The repo is currently empty (no Go module, no build/test commands yet). When bootstrapping, initialize a Go module under `github.com/FlavioCFOliveira/MuxMasterWebsite` and add MuxMaster as a dependency.
- When this section becomes outdated (real `go.mod`, `Makefile`, or scripts land), update CLAUDE.md with the actual build/lint/test/dev commands.
- The neighboring MuxMaster repo (`../MuxMaster`) is the source of truth for API surface, examples, and version numbers shown on the site — read from there rather than duplicating documentation, and keep content in sync with its `CHANGELOG.md` and `release-notes/`.

## Coordinator agents

This project ships dedicated subagents under `.claude/agents/`. Treat them as gatekeepers, not optional helpers. The four agents are peers and may all need to review the same change; each issues `APPROVED` / `APPROVED WITH CHANGES` / `REJECTED` verdicts, and blocking fixes from any agent must be applied before merge.

The first three agents are technical-surface specialists and may be invoked in any order (often in parallel). The fourth agent (`ux-specialist`) is the **final holistic gate** and must be invoked **last**, after the other three have completed their reviews and any blocking fixes have been applied — its job is to read the post-fix state of the change as an integrated user experience.

- **`seo-specialist`** — Traditional SEO + Core Web Vitals + web-standards + accessibility (WCAG 2.2 AA). Owns: `<head>` metadata, canonical/OG/Twitter, `sitemap.xml`, search-engine portion of `robots.txt`, JSON-LD for rich results (`TechArticle`, `BreadcrumbList`, `SoftwareSourceCode`, `Organization`), HTTP semantics (status, redirects, caching, compression), security headers, image/font/asset performance.
- **`geo-specialist`** — Generative Engine Optimization. Owns: `llms.txt`, `llms-full.txt`, markdown companion representations (`Accept: text/markdown` and/or `<path>.md`), AI-crawler portion of `robots.txt` (`GPTBot`, `ClaudeBot`/`anthropic-ai`, `PerplexityBot`, `Google-Extended`, `Applebot-Extended`, `CCBot`, etc.), content shape (definition-first sentences, self-contained paragraphs, statistics, quotations, inline citations), comparison tables, `FAQPage` and `HowTo` JSON-LD.
- **`tailwind-specialist`** — UI / visual design with Tailwind CSS v4. Owns: design tokens (`@theme`), `@source` configuration, layout primitives, component styling, mobile-first responsive layout, dark mode, typography, no-JavaScript interaction patterns (`<details>`, `popover`, `:has()`, `:target`, `@starting-style`), animation/motion respecting `prefers-reduced-motion`. Hard rule: no JavaScript dependency for content rendering or primary interaction.
- **`ux-specialist`** — User Experience and usability holistic coordinator. **Final gate**, invoked last. Owns: information architecture, navigation design (sidebar, breadcrumb, in-page TOC, prev/next), user flows (landing → quickstart → first running router; "evaluating routers"; "broke production, need fix"), microcopy and labels, page-template purpose, empty/error/loading states, code-example UX, onboarding flow, cognitive-load budget, heuristic evaluation (Nielsen's 10), cross-page experience consistency. Does **not** edit code — produces holistic verdicts with exact rewrite proposals routed to the main session or to a peer agent. Has blocking power: a `ux-specialist` REJECTED blocks merge even when the three peer agents have approved. Direct disputes with a peer agent are **escalated to the user** — never resolved unilaterally.

**Overlap zones** (consult more than one):
- English quality, factual integrity, contradiction sweep — all four (independent checks).
- Mobile-first compliance and touch-target sizing — `tailwind-specialist` primary, `seo-specialist` verifies WCAG, `ux-specialist` verifies experiential hit rate.
- Heading hierarchy and semantic landmarks — `seo-specialist` primary, `tailwind-specialist` must not break them for visual reasons, `ux-specialist` verifies they communicate the page's structure to a reader.
- Font loading strategy — `seo-specialist` primary (CLS, LCP), `tailwind-specialist` concurs on family/weight choice.
- Structured-data design and dual SEO+GEO impact — `seo-specialist` and `geo-specialist`.
- Microcopy / labels / button-and-link text — `ux-specialist` primary, `geo-specialist` concurs on tone, `seo-specialist` concurs on anchor-text descriptiveness.
- Information architecture and URL structure — `ux-specialist` primary, `seo-specialist` verifies canonical/sitemap/redirect alignment.

Invoke via natural language ("have the seo-specialist review …", "ask the tailwind-specialist to audit …", "have the ux-specialist run the final gate …") or `@seo-specialist` / `@geo-specialist` / `@tailwind-specialist` / `@ux-specialist`. When in doubt about which agent applies, invoke every relevant one — the cost of an extra review is trivial compared to shipping a regression in search visibility, AI citation rate, web-standards compliance, visual/responsive quality, or end-to-end usability.

## When in doubt

If a proposed change would compromise SEO, GEO, accessibility, mobile-first behavior, end-to-end UX, or the "MuxMaster as router" dogfooding constraint — stop and flag it before implementing. These five pillars are the reason the site exists in this form.
