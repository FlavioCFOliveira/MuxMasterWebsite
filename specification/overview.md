---
title: Overview
purpose: Describe the website's purpose, audiences, missions, and the integrity rules that govern its content.
owners: specification-manager; review by seo-specialist, geo-specialist, tailwind-specialist, ux-specialist.
last-updated: 2026-09-26
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

1. **Document MuxMaster faithfully.** Every factual claim on the site MUST match the upstream source in `../MuxMaster`. Where they disagree, upstream wins by definition; the local mirror under `/content/` is updated to match through the sync workflow described in `content-sources.md`. The runtime binary never reads the upstream tree directly.
2. **Be a working proof of MuxMaster.** The site itself MUST be served by MuxMaster. The site source is read by integrators as a real-world reference implementation. Implementation shortcuts that would weaken the proof MUST NOT be taken (see `../CLAUDE.md` "MuxMaster as router" constraint).

## Operating principle: static-tending

All content is prepared editorially in this repository by the agents in this development workflow (see `agents-and-gates.md`) and committed under `/content/`. The runtime binary serves only what has been prepared. Synchronisation with the upstream `../MuxMaster` source of truth happens at development time through the `content-curator` agent — never at request time. The website is therefore essentially a static site whose authoritative source is this repository, not the upstream module's working tree. Every public route is pre-rendered at startup; the same URL returns the same bytes for the lifetime of the process, and `Accept-Encoding` is the only request header permitted to influence the response. The full architecture is defined in `rendering-and-caching.md`.

## Version cadence

The website carries its own semantic version (`vMAJOR.MINOR.PATCH`), tagged on this repository and listed in `/CHANGELOG.md` at the repository root (Keep a Changelog 1.1.0 format). The relationship between this version and the upstream MuxMaster version is governed by the rules below.

### Policy

- **MAJOR and MINOR are locked to MuxMaster.** When MuxMaster releases `vX.Y.<any>`, the next website release MUST adopt the same `X.Y` pair. The website never advances its MAJOR or MINOR independently, and never lags behind MuxMaster's MAJOR or MINOR once the documentation for the new release has been merged.
- **PATCH is independent.** The website MAY cut a new PATCH release at any time to ship website-only operational fixes — for example continuous-integration changes, Docker image fixes, deployment adjustments, accessibility corrections, copy edits, or site infrastructure work — without waiting for a MuxMaster release. The website's PATCH number is therefore not required to match MuxMaster's PATCH number, and the two will routinely diverge. A state in which the website is at `v1.0.2` while MuxMaster is at `v1.0.1` is expected, not anomalous.
- **MuxMaster releases still trigger a website release.** Every MuxMaster release MUST be followed by a website release that documents it. That release adopts MuxMaster's `MAJOR.MINOR` and sets the website's PATCH to the next value in the website's own sequence (it does not reset to MuxMaster's PATCH).

### Worked example

- Website at `v1.0.1`, MuxMaster at `v1.0.1`.
- The website ships a Docker fix. The website releases `v1.0.2`; MuxMaster remains at `v1.0.1`. This is permitted under the new policy.
- MuxMaster later releases `v1.1.0`. The next website release MUST adopt `1.1` and is tagged `v1.1.0` (the website's PATCH resets only because MINOR advanced, per standard SemVer rules; it does not mirror MuxMaster's PATCH).
- MuxMaster then releases `v1.1.1`. The next website release is tagged `v1.1.1` if no website-only PATCH was cut in between, or `v1.1.2` (or higher) if one or more website-only PATCH releases were already published on the `1.1` line.

### Supersession of the previous lockstep rule

The previous rule — that the website is released with the same full semantic version as the MuxMaster release it documents, in lockstep across all three digits — is **superseded** as of website `v1.0.2` (2026-05-11). The lockstep rule was ratified in the website `v1.0.1` CHANGELOG entry; that historical entry is immutable under Keep a Changelog and MUST NOT be edited retroactively. The supersession is recorded in the website `v1.0.2` CHANGELOG entry by the `release-manager` agent at tag time.

### Version label in the page chrome (unchanged)

The version *label* shown in the header and footer is a separate concept from the website's own release tag and is unaffected by this policy. The label is read at server startup from `/content/changelog.md` (rule: first Markdown heading of the form `## vMAJOR.MINOR.PATCH` or `## [MAJOR.MINOR.PATCH]` without a pre-release suffix). The `content-curator` agent commits `/content/changelog.md` mirrored from `../MuxMaster/CHANGELOG.md` during a sync (see `content-sources.md`). The label therefore tracks the **MuxMaster** version the site documents, not the website's own tag. A restart is required for the label to roll forward. See `url-and-versioning.md` § Version label rule.

### Current state

- MuxMaster's latest release as of 2026-09-26: **v1.3.0** (released 2026-09-26). It requires Go 1.27.1 or later (`go.mod` declares `go 1.27.1`).
- The content refresh recorded in "Content refresh for MuxMaster v1.3.0" below brings the site to v1.3.0. After the curator syncs `/content/changelog.md`, the version label shown in the page chrome MUST read **v1.3.0**, and every page that states the minimum Go version MUST state **Go 1.27.1**.
- Under the Policy above, the website release that documents v1.3.0 adopts `1.3` as its MAJOR.MINOR pair.

## Language and tone rules (per CLAUDE.md §0)

- All user-facing content MUST be in **English**, with zero spelling, grammar, syntax, or punctuation errors. Spell- and grammar-check before publishing; defects found post-merge are bugs.
- Audience and register: clear, simple, unambiguous technical language. Define terms before using them. Use jargon only when it carries information the plain word cannot.
- Tone: technical, didactic, simple, objective. Lead with the fact or instruction. No marketing hype ("blazing fast", "revolutionary"). No hedging ("maybe", "kind of").
- Prefer short sentences, concrete nouns, active voice. Numbers and signatures over adjectives.

## Integrity rules

- **Single source of truth.** Versions, API signatures, defaults, supported Go versions, and benchmark numbers MUST match across every page on the site, and MUST match `../MuxMaster` upstream.
- **Faithful to code.** Documentation MUST describe MuxMaster as it is implemented today. Verify against `../MuxMaster` source before publishing any factual claim. If the code is wrong, fix the code first, then document the fixed behaviour. Never describe planned, intended, or remembered behaviour.
- **No vague statements.** Replace "fast" with measured numbers, "supported" with the exact versions, "recommended" with the reason. If a fact cannot be stated precisely, omit it.
- **Performance claims.** Every statement about speed, allocation, or comparison with other routers is governed by "Performance claims" below.
- **Cross-page consistency on edits.** When a doc page is added or changed in `/content/`, related pages in `/content/` and the relevant upstream files (`../MuxMaster/README.md`, `CHANGELOG.md`, `api.md`) MUST be cross-checked for contradictions, and any contradictions MUST be resolved in the same change. Edits to `/content/` are normally produced by the `content-curator` agent during a sync; see `content-sources.md`.

## Performance claims

These rules govern every performance statement on every surface of the site: page bodies, Markdown companions, `<meta name="description">`, Open Graph and Twitter descriptions, JSON-LD strings, `/llms.txt`, `/llms-full.txt`, image alternative text, and route descriptions. The sources the numbers come from are defined in `content-sources.md` § Benchmarks — sources.

- **INT-PERF-1.** Every published performance number MUST state, in the same paragraph or in the caption of the same table: the host CPU model, the Go toolchain version, the measurement date, the `-count` value, and a link to the source. A table MAY state these once in its caption for all of its cells. INT-PERF-11 applies this rule to prose, FAQ answers, and lists.
- **INT-PERF-2.** A number taken from upstream rather than from this project's campaign MUST be attributed to upstream, with the upstream measurement date and sample count, and linked to the upstream file pinned to a release tag (see CS-BENCH-9 to CS-BENCH-11).
- **INT-PERF-3.** A superlative or comparative claim ("fastest", "faster than", "slower than", "level with") MUST be supported by per-category data published on `/benchmarks`. The claim MUST name the route categories it covers, the handler mode it applies to (default `Handle`, `Handle` with `PoolRequestBundle`, or `HandleFast`), and the set of routers measured. It MUST NOT extend to a category, mode, or router the data does not cover.
- **INT-PERF-4.** A claim that MuxMaster is faster or slower than another router in a category MUST be backed by a `benchstat` comparison that is significant at alpha = 0.05. A difference that is not significant MUST be stated as "no significant difference".
- **INT-PERF-5.** Where the published data shows MuxMaster slower than a measured competitor in a category or mode, `/benchmarks` MUST publish that result with the same prominence as the favourable ones. Unfavourable categories MUST NOT be omitted.
- **INT-PERF-6.** A change between two MuxMaster versions MUST be published as measured: a delta that is not significant at alpha = 0.05 is "no significant difference" (CS-BENCH-7); identical B/op and allocs/op are stated as identical. When `/benchmarks` publishes gains since `v1.1.0`, it MUST also publish the measured costs added by security fixes since `v1.1.0`, under the same citation rules.
- **INT-PERF-7.** Space-limited surfaces — `<meta name="description">`, Open Graph and Twitter descriptions, JSON-LD `description` strings, and one-line entries in `/llms.txt` and `/llms-full.txt` — MUST NOT contain performance numbers, because they cannot carry the citation INT-PERF-1 requires. They MAY carry a comparative claim that satisfies INT-PERF-3 and INT-PERF-4 in words (for example, the categories in which MuxMaster was fastest among the routers measured).
- **INT-PERF-8.** Historical figures measured on `v1.1.0`-era code MUST appear only in the section `## Historical (v1.1.0-era code)` defined in CS-BENCH-12, MUST be labelled as historical wherever they are quoted, and MUST NOT support a claim about the current release.
- **INT-PERF-9.** The following phrases, and any paraphrase with the same meaning, are forbidden: "fastest across every route category"; "the only … in the entire Go ecosystem" (or any other claim of uniqueness across the Go ecosystem); "the fastest Go router" or "the fastest HTTP router" without the category, mode, and measured-set qualifiers INT-PERF-3 requires; and any claim about a router that the cited data did not measure. The only exception is the mirrored historical text defined in INT-PERF-10.

- **INT-PERF-10.** The project owner decided on 2026-09-26 that mirrored historical upstream text containing a performance claim forbidden by INT-PERF-9 is kept verbatim. The rule applies to the v1.1.0 entry of `/content/changelog.md` (route `/changelog`) and to `/content/release-notes/v1.1.0.md` (route `/releases/v1.1.0`). Each paragraph that contains such a claim MUST carry, immediately after the claim, the inline marker "*(Historical v1.1.0 claim; not supported by the 2026-09-26 campaign — see [/benchmarks](/benchmarks).)*". A page-level note alone does not satisfy this rule; a page-level note MAY be kept in addition to the markers. Because the paragraph is kept verbatim, its text is exempt from INT-PERF-1, INT-PERF-8 (placement), INT-PERF-9, and INT-PERF-11; the marker is the historical label INT-PERF-8 requires. Text outside such a paragraph gains no exemption.
- **INT-PERF-11.** INT-PERF-1 applies to every numeric performance statement, wherever it appears: prose paragraphs, FAQ answers, and list items, not only tables. For a prose paragraph or an FAQ answer, the host CPU model, the Go toolchain version, the measurement date, the `-count` value, and a link to the source MUST appear in the same paragraph. For a list, they MUST appear either in the same list item or in a caption line placed directly before the list, which then covers every item of that list.

## Content refresh for MuxMaster v1.3.0

The project owner decided the following objectives on 2026-09-26 for the refresh of the site's content from MuxMaster v1.1.0 to v1.3.0. The rules that implement them are cited against each objective.

1. **Promote MuxMaster's features seriously and honestly.** Governed by "Language and tone rules", "Integrity rules", and "Performance claims" above.
2. **Surface HTTP QUERY method (RFC 10008) support strongly, for both SEO and GEO.** Implemented by `information-architecture.md` § HTTP QUERY method page, `seo.md` § Page-specific requirements, `geo.md` § HTTP QUERY method coverage, and `structured-data.md` (master schema table).
3. **Show the performance changes from v1.1.0 to v1.3.0 as measured.** Implemented by CS-BENCH-1 to CS-BENCH-18 and INT-PERF-6.
4. **Position MuxMaster honestly against the measured competitors.** MuxMaster is described as the fastest router only in the categories and modes where the campaign's data, tested for significance, supports it (INT-PERF-3 to INT-PERF-5). The upstream data at `v1.3.0` (fastest on static, not-found, and parallel static routes; with `PoolRequestBundle`, fastest on 1 and 3 parameters and parallel parameter routes; level with httprouter on 2 parameters; slower on catch-all; default mode slower than httprouter on parameterised routes) is the expected shape, but the published claims follow the campaign's results.
5. **Update all content to the v1.3.0 feature set.** Implemented by the content sync (`content-sources.md`), CS-REL-1, CS-QUERY-1, and "Current state" above.
6. **SEO and GEO at the maximum level.** Implemented by `seo.md`, `geo.md`, and `structured-data.md`.

## TBD register (initial)

The five items below were registered as TBDs. They are tracked in detail, with current status, in `open-questions.md`. As of 2026-05-11 the canonical-domain blocker is closed; the remaining open item from this initial register is item 5 (landing page copy).

1. **Canonical production domain.** Resolved on 2026-05-11 as `https://muxmaster.net` (HTTPS, apex, no trailing slash). Used for `<link rel=canonical>`, Open Graph `og:url` and absolute `og:image`, `sitemap.xml`, `llms.txt`, `llms-full.txt`, and JSON-LD `@id` URIs.
2. **Exact accent colour hexes.** Resolved on 2026-05-11 as Tailwind's stock cyan and yellow scales (see `brand-and-visual.md`).
3. **Go module path of this repository.** Resolved on 2026-05-08 as `github.com/FlavioCFOliveira/MuxMasterWebsite`.
4. **Binary name** for the compiled site server. Resolved on 2026-05-08 as `muxmaster-website`.
5. **Landing page copy** — value proposition headline, subhead, and primary CTA wording. Open.
