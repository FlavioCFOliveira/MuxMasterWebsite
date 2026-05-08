---
title: Overview
purpose: Describe the website's purpose, audiences, missions, and the integrity rules that govern its content.
owners: specification-manager; review by seo-specialist, geo-specialist, tailwind-specialist, ux-specialist.
last-updated: 2026-05-08
status: ratified
---

# Overview

## Purpose

The website is the **official documentation site for the MuxMaster Go module** (`github.com/FlavioCFOliveira/MuxMaster`). It documents the public API, behaviour, and idioms of MuxMaster, and it serves as the public reference that human developers and AI answer engines consult to evaluate, learn, and integrate the module.

## Audiences

The site serves two audiences with equal priority:

1. **Human technical readers** — developers reading the docs to evaluate, learn, or integrate MuxMaster. Register: technical, didactic, plain.
2. **AI / LLM crawlers and answer engines** (ChatGPT, Claude, Perplexity, Google AI Overviews, and similar) that ingest the site to produce citations, summaries, and integration guidance.

Every page MUST be useful to both audiences. Content shape decisions that favour one MUST NOT degrade the other below the contract defined in `seo.md` and `geo.md`.

## Missions

The site has two missions, both first-class:

1. **Document MuxMaster faithfully.** Every factual claim on the site MUST match the upstream source in `../MuxMaster`. Where they disagree, upstream wins by definition; the site is updated to match (see `content-sources.md`).
2. **Be a working proof of MuxMaster.** The site itself MUST be served by MuxMaster. The site source is read by integrators as a real-world reference implementation. Implementation shortcuts that would weaken the proof MUST NOT be taken (see `../CLAUDE.md` "MuxMaster as router" constraint).

## Operating principle: static-tending

The site is **static-tending**. The same URL returns the same bytes for the same build identity and the same upstream-source `mtime`. The server introduces no per-request dynamism beyond what client capability headers (`Accept-Encoding`) require. Server-side templates are an implementation detail of how those bytes are produced, never a request-time decision the client perceives. Live templating exists exclusively to keep documentation in sync with the upstream `../MuxMaster` source of truth, not to enable per-request variability. The two route categories that derive from this principle (pre-rendered at startup; lazy-cache live templating) are defined in `rendering-and-caching.md`.

## Version cadence

- The site does not maintain its own semantic version separate from MuxMaster. It surfaces the **latest released MuxMaster version** as a plain text label in the header and footer (e.g. `v1.0.1`).
- The version label is read at server startup from `../MuxMaster/CHANGELOG.md` (rule: first heading of the form `## vMAJOR.MINOR.PATCH` that is not a pre-release suffix). Restart is required to roll the label forward.
- The current value as of 2026-05-08 is **v1.0.1** (released 2026-05-08).

## Language and tone rules (per CLAUDE.md §0)

- All user-facing content MUST be in **English**, with zero spelling, grammar, syntax, or punctuation errors. Spell- and grammar-check before publishing; defects found post-merge are bugs.
- Audience and register: clear, simple, unambiguous technical language. Define terms before using them. Use jargon only when it carries information the plain word cannot.
- Tone: technical, didactic, simple, objective. Lead with the fact or instruction. No marketing hype ("blazing fast", "revolutionary"). No hedging ("maybe", "kind of").
- Prefer short sentences, concrete nouns, active voice. Numbers and signatures over adjectives.

## Integrity rules

- **Single source of truth.** Versions, API signatures, defaults, supported Go versions, and benchmark numbers MUST match across every page on the site, and MUST match `../MuxMaster` upstream.
- **Faithful to code.** Documentation MUST describe MuxMaster as it is implemented today. Verify against `../MuxMaster` source before publishing any factual claim. If the code is wrong, fix the code first, then document the fixed behaviour. Never describe planned, intended, or remembered behaviour.
- **No vague statements.** Replace "fast" with measured numbers, "supported" with the exact versions, "recommended" with the reason. If a fact cannot be stated precisely, omit it.
- **Cross-page consistency on edits.** When a doc page is added or changed, related pages on the site and the relevant upstream files (`../MuxMaster/README.md`, `CHANGELOG.md`, `api.md`) MUST be cross-checked for contradictions, and any contradictions MUST be resolved in the same change.

## TBD register (initial)

The five items below were registered as TBDs. They are tracked in detail, with current status, in `open-questions.md`. Production launch is blocked by item 1.

1. **Canonical production domain.** Open. Required for `<link rel=canonical>`, Open Graph `og:url` and absolute `og:image`, `sitemap.xml`, `llms.txt`, `llms-full.txt`, JSON-LD `@id` URIs.
2. **Exact accent colour hexes** extracted from the logo PNG (cyan and yellow). Open. Required to lock the design tokens.
3. **Go module path of this repository.** Resolved on 2026-05-08 as `github.com/FlavioCFOliveira/MuxMasterWebsite`.
4. **Binary name** for the compiled site server. Resolved on 2026-05-08 as `muxmaster-website`.
5. **Landing page copy** — value proposition headline, subhead, and primary CTA wording. Open.
