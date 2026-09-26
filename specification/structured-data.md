---
title: Structured Data Doctrine
purpose: Define the unified JSON-LD contract for the MuxMaster documentation website — schema-by-page-family table, entity graph, field-completeness rules, auxiliary schemas, and the blocking CI validation gate.
owners: seo-specialist (rich-result eligibility); geo-specialist (AI-ingestion accuracy). Co-owned. Both must approve any change to this file.
last-updated: 2026-09-26
status: ratified
---

# Structured Data Doctrine

## Purpose

The same JSON-LD blocks emitted by this site serve two distinct goals at once. For traditional search engines the goal is **rich-result eligibility**: search-engine result pages render breadcrumb trails, FAQ accordions, How-To carousels, code-repository panels, and dataset summaries when the underlying JSON-LD is well-formed and complete. For AI answer engines the goal is **ingestion accuracy**: facts about MuxMaster (versions, signatures, benchmark numbers, license, author, organisation) must be attributed to a single, stable entity graph rather than re-derived per page. Stable `@id` URIs and `sameAs` references are the mechanism that lets a generative engine recognise that two mentions of MuxMaster on two different pages are the same entity rather than two distinct ones. This file is the single source of truth for both goals; `seo.md` and `geo.md` reference it for any schema rule.

## Master schema table

The following table maps each page family to the JSON-LD types it MUST emit. Whenever one of the four reified entities (`SoftwareSourceCode`, `Organization`, `Person`, `WebSite`) appears in the `Required JSON-LD` column outside `/`, the cell MUST be read as **"reference by `@id`, do not redefine inline"**. The annotation `(by @id)` is appended to such cells to make this explicit, and the `Notes` column states which slot on the page-level type the reference fills (see `## Entity graph` and `## Non-negotiables` below).

| Page family | Required JSON-LD | Notes |
| --- | --- | --- |
| `/` | `WebSite`, `SoftwareSourceCode`, `Organization`, `Person` | The four reified entity nodes are emitted **in full** on this page only. Every other page references them by `@id`. |
| `/docs/<section>` | `TechArticle`, `BreadcrumbList`, `HowTo` (Getting Started) | `TechArticle.author` references `Person@id`; `TechArticle.publisher` references `Organization@id`; `TechArticle.isPartOf` references `WebSite@id`. The Getting Started page emits `HowTo` because its body is structured as an ordered, named step sequence; other doc pages MAY emit `HowTo` when their body has the same shape. |
| `/docs/http-query-method` | `TechArticle`, `BreadcrumbList`, `FAQPage`, `HowTo` | **SD-QUERY-1.** All four types are mandatory on this page. `TechArticle` carries the same entity references as `/docs/<section>` and, in addition, `about` referencing `SoftwareSourceCode@id`. `FAQPage` lists every Q→A pair of the page's `## Common questions` section (`geo.md` GEO-QUERY-4). `HowTo` lists the page's step sequence (`information-architecture.md` IA-QUERY-13); every step's named code block also emits `Code`. **SD-QUERY-2.** `HowTo.name` MUST be "How to serve the HTTP QUERY method (RFC 10008) with MuxMaster". |
| `/docs/` | `CollectionPage`, `BreadcrumbList` | `CollectionPage.isPartOf` references `WebSite@id`; `CollectionPage.publisher` references `Organization@id`. |
| `/api` | `TechArticle`, `SoftwareSourceCode` (by `@id`), `APIReference`, `DefinedTermSet` (curated public-API surface), `BreadcrumbList` | `SoftwareSourceCode` is referenced by `@id` (the MuxMaster module node) — this row MUST NOT cause inline redefinition of the entity on `/api`. `APIReference` is the auxiliary schema mandated for this page family — see `## Auxiliary schemas`. `DefinedTermSet` lists the curated public-API surface (functions, types, methods, middleware constructors) with one-sentence descriptions per `DefinedTerm`; the list is hand-curated rather than auto-extracted because the source `content/api.md` is rendered `go doc` plain text that does not map cleanly onto the `DefinedTerm.description` field. |
| `/docs/` | `CollectionPage`, `ItemList`, `BreadcrumbList` | `ItemList.itemListElement[*]` lists every doc page with `position`, `name`, `url`, and `description` so AI engines can enumerate the documentation surface and key on the curated order. Same entity references as `/docs/<section>`. |
| `/examples/` | `CollectionPage`, `ItemList`, `BreadcrumbList` | `ItemList.itemListElement[*]` lists every example page with `position`, `name`, `url`, and `description`; the position field carries the curated learning order (REST → auth family → operational concerns). Same entity references as `/docs/`. |
| `/examples/<name>` | `TechArticle`, `BreadcrumbList`, `HowTo` (every page complying with `geo.md` § Example walkthrough shape) | Every example page MUST comply with the walkthrough shape defined in `geo.md` § Example walkthrough shape; that shape is the trigger for `HowTo` emission, so emission is data-driven across the entire `/examples/` family rather than hand-curated. When `HowTo` is emitted, every step's named code block also emits `Code` (see `## Auxiliary schemas`). |
| `/benchmarks` | `TechArticle`, `BreadcrumbList`, `Dataset` | `TechArticle.about` references `SoftwareSourceCode@id` (`#muxmaster`). `Dataset.creator` references `Organization@id`. **SD-BENCH-1.** The `Dataset` describes the current campaign results only (`content-sources.md` CS-BENCH-1): `Dataset.distribution` links to the raw files of the campaign archive `reports/benchmarks-<YYYY-MM-DD>/` in this repository, pinned to the full commit SHA that contains it (`url-and-versioning.md` URL-EXT-1). Corroborating upstream sources MAY be listed under `Dataset.isBasedOn`, linked at the `v1.3.0` tag. Historical `v1.1.0`-era data MUST NOT be described by the `Dataset`. **SD-BENCH-2.** The `Dataset` MUST carry a descriptive `name`, `measurementTechnique`, `keywords`, and `isAccessibleForFree`, as defined in `### Dataset` below. |
| `/changelog` | `TechArticle`, `BreadcrumbList` | `TechArticle.about` references `SoftwareSourceCode@id` (the MuxMaster module). |
| `/releases/<v>` | `TechArticle`, `BreadcrumbList` | `TechArticle.about` references `SoftwareSourceCode@id`; `TechArticle.version` is the release version. Applies to `/releases/v1.0.0`, `/releases/v1.1.0`, `/releases/v1.2.0`, and `/releases/v1.3.0`. |
| `/security`, `/compatibility`, `/contributing` | `TechArticle`, `BreadcrumbList` | Same entity references as `/docs/<section>`. |
| `/built-with-muxmaster` | `TechArticle`, `BreadcrumbList` | Same entity references as `/docs/<section>`. |

Cross-cutting rules:

- `FAQPage` MUST be added to any page whose body contains at least three Q→A pairs, in addition to whatever the table requires for that page family. The threshold and the conversational-chain rules are defined in `geo.md`.
- `DefinedTermSet` plus one `DefinedTerm` per defined term MUST be added to any page that contains a glossary or that defines technical terms (see `## Auxiliary schemas`).
- `Code` MUST be added for every named, citeable code snippet (a snippet that has a heading and a stable anchor; see `## Auxiliary schemas`).

## Entity graph

The site reifies four project-level entities. Each is **emitted in full only on `/`**, with a stable `@id` URI of the form `https://muxmaster.net/#<fragment>`. Every other page references these nodes by `@id` instead of redefining them inline. The canonical domain was ratified on 2026-05-11 as `https://muxmaster.net` (HTTPS, apex, no trailing slash — see `open-questions.md` item 1, RESOLVED, and `deployment.md`). In `development` and `staging`, where `SITE_BASE_URL` is not the canonical production origin, the `@id` URIs MUST be rewritten to use `SITE_BASE_URL` so that staging never advertises canonical-production identifiers, and the pages MUST NOT be marked indexable.

### MuxMaster module — `SoftwareSourceCode`

- `@id`: `https://<canonical>/#muxmaster`.
- Required fields: `name`, `programmingLanguage`, `codeRepository`, `license`, `version` (latest release, sourced from `../MuxMaster/CHANGELOG.md`), `runtimePlatform` (minimum Go version, sourced from `../MuxMaster/go.mod`), `targetProduct.applicationCategory: "DeveloperApplication"`, `sameAs`.
- `sameAs` MUST point at authoritative third-party sources only — at minimum the GitHub repository (`https://github.com/FlavioCFOliveira/MuxMaster`) and the Go package index page (`https://pkg.go.dev/github.com/FlavioCFOliveira/MuxMaster`). Future package-tracking aliases are added here as they appear.
- Referenced from: `/api` (the page is the API surface of this entity), `/changelog`, `/releases/<v>`, and `/benchmarks` (`about`), and any page that names the module.

### Publishing organisation — `Organization`

- `@id`: `https://<canonical>/#org`.
- Required fields: `name`, `url`, `logo` (the canonical logo from the upstream repository — see `brand-and-visual.md`), `sameAs`.
- `sameAs` MUST point at authoritative third-party sources only — at minimum the GitHub organisation or user page that hosts the upstream repository. No social-only profiles.
- Referenced from: every `TechArticle` (`publisher`), every `Dataset` (`creator`), and the `WebSite` node (`publisher`).

### Author and maintainer — `Person`

- `@id`: `https://<canonical>/#author`.
- Required fields: `name`, `url` (GitHub profile), `sameAs`.
- `sameAs` MUST point at authoritative third-party profiles only. Authoritative means GitHub, project page, public technical blog, or similar. Social-only profiles (Twitter/X, Facebook, Instagram, TikTok, and the like) MUST NOT be listed under `sameAs`.
- Referenced from: every `TechArticle` (`author`).

### Site — `WebSite`

- `@id`: `https://<canonical>/#website`.
- Required fields: `name`, `url`, `inLanguage: "en"`, `publisher` (referencing `Organization@id`).
- Referenced from: every `TechArticle` and every `CollectionPage` (`isPartOf`).

The four nodes above MUST appear in full **only on `/`**. On every other page, the JSON-LD MUST reference them by `@id` and MUST NOT redefine them inline (see `## Non-negotiables`).

## @id migration policy

`@id` URIs are the durable handles that allow generative engines to consolidate facts about MuxMaster across the site and across the wider web. An engine that has accumulated statements under `https://<canonical>/#software` does not automatically reattribute them when the site renames the node to `https://<canonical>/#muxmaster`; without an explicit bridge the citation graph fragments silently. The rules in this section govern every present and future rename of an `@id` on this site.

### Stability rule

`@id` URIs are stable identifiers. Once an `@id` has been published on a deployed page, it MUST NOT change except through a documented migration plan ratified in this file. Renaming an `@id` without following the procedure below is a defect and is blocked by the SEO and GEO gatekeepers.

### Migration plan requirements

A ratified migration plan MUST specify, in this file, all four of the following:

1. The **legacy `@id`** — the URI that is being retired.
2. The **new `@id`** — the URI that replaces it.
3. The **transition window** — a contiguous period during which the bridging mechanism described below is in effect. The window MUST be **at least 90 days** long.
4. The **bridging mechanism** — the concrete steps defined in `### Bridging mechanism` below.

### Bridging mechanism

During the transition window, the following two conditions MUST hold simultaneously:

- The entity emitted at the **new** `@id` MUST list the **legacy** `@id` in its `sameAs` array, alongside any other `sameAs` entries already required by `## Entity graph`. This declares to consuming engines that the two URIs identify the same entity.
- The legacy `@id` URL fragment MUST resolve to the same canonical page as the new `@id` — the page MUST not return `404`. Because `@id` is a URL fragment and fragments are not transmitted to the server, no HTTP redirect is required; the requirement is only that the host page continues to be served.

After the transition window has elapsed, the legacy `@id` entry in `sameAs` MAY be removed. The host page MUST continue to be served (the stability rule for canonical pages is governed by `url-and-versioning.md`, independently of this section).

### Current migration in flight

One migration is currently in flight on this site:

- **Legacy `@id`:** `https://<canonical>/#software`.
- **New `@id`:** `https://<canonical>/#muxmaster`.
- **Transition window:** starts on the deploy of the renamed `@id`; ends 90 days after that deploy.
- **Bridging mechanism:** during the window, the `SoftwareSourceCode` node emitted on `/` MUST include `"sameAs": [ ..., "https://<canonical>/#software" ]`, in addition to the third-party `sameAs` entries already required by `## Entity graph`.

After the window ends, the legacy `@id` entry MAY be removed from the `SoftwareSourceCode` node's `sameAs` array. Until then, removing it is a defect.

## Field completeness

For every JSON-LD node emitted on the site, every field that schema.org documents as **required** OR **recommended** for the type MUST be populated when a truthful value is available. The list of fields treated as required-or-recommended for each type used on this site is enumerated in `## Required field-by-type expectations` below.

The following are forbidden:

- **Empty strings** as a substitute for an absent value (for example `"datePublished": ""`).
- **Placeholder URLs** (for example `"url": "https://example.com"`).
- **Fabricated values** (a guessed `version`, an invented `datePublished`, a benchmark number that is not from the campaign archive or from an attributed upstream source; see `content-sources.md` § Benchmarks — sources).

When a required-or-recommended field cannot be truthfully populated, the field MUST be omitted entirely. The omission MUST be justified in a one-line HTML comment placed immediately above the relevant `<script type="application/ld+json">` tag, in the form:

```html
<!-- omitted: <field> on <type> — <one-sentence reason> -->
```

A node MAY carry more than one such comment when several fields are omitted. The HTML comment is the audit trail; reviewers and the CI validators read it to confirm that the omission is intentional.

## Required field-by-type expectations

For each schema type used on this site, the fields below MUST be populated when a truthful value is available, subject to the rules in `## Field completeness`. The "Source" column states where the truthful value comes from.

### `TechArticle`

| Field | Source |
| --- | --- |
| `headline` | The page's `<h1>` text. |
| `description` | The page's `<meta name="description">` content. |
| `inLanguage` | `"en"` (the site is English-only on day one — see `geo.md`). |
| `datePublished` | A `datePublished` value in the page's front-matter, set at first authorship and never rewritten thereafter. See `### Date sources for embedded content` below for the full resolution order. |
| `dateModified` | The git-log commit time of the most recent change to the file under `/content/` that backs the route, resolved at build time and threaded into the renderer. See `### Date sources for embedded content` below. |
| `author` | Reference to `Person@id`. |
| `publisher` | Reference to `Organization@id`. |
| `mainEntityOfPage` | The canonical absolute URL of the page. |
| `isPartOf` | Reference to `WebSite@id`. |

### Date sources for embedded content

Content files served via Go's `embed.FS` carry a zero `mtime` because the embedding step erases filesystem timestamps. The renderer therefore MUST NOT read `mtime` directly. The following resolution order is mandatory and applies to both `datePublished` and `dateModified` on `TechArticle` (and on any other type that carries either field, such as `Dataset`).

**Resolution order for `datePublished`:**

1. The `datePublished` value from the page's front-matter (an ISO-8601 date or datetime), if present.
2. **Otherwise omit the field**, with the standard HTML-comment audit trail (see `## Field completeness`):
   `<!-- omitted: datePublished on TechArticle — front-matter date not yet authored -->`.

**Resolution order for `dateModified`:**

1. A `dateModified` value from the page's front-matter (an ISO-8601 date or datetime), if explicitly set there.
2. The git-log commit time of the most recent commit that touched the file under `/content/` that backs the route, resolved at build time and threaded into the renderer through a build-step manifest (the build pipeline, not the runtime, runs `git log -1 --format=%cI -- <path>`).
3. **Otherwise omit the field**, with the standard HTML-comment audit trail:
   `<!-- omitted: dateModified on TechArticle — neither front-matter nor git history available -->`.

**Forbidden fallbacks.** Substituting the process build timestamp (`BuildTime`), the deploy timestamp, the current wall-clock time, or any other "best-guess" value for either date is a fabricated value (see `## Field completeness`) and is a defect. The doctrine prefers an omitted field with an audit-trail comment to any guessed substitute.

**Front-matter authoring rule.** On first authorship of a content file, the curator MUST set `datePublished` in the front-matter to the ISO-8601 date of authorship. On subsequent edits the curator MAY also set `dateModified` in the front-matter; if not set, the build pipeline derives it from git history per step 2 above. `datePublished` MUST NOT be rewritten after the first authorship.

### `BreadcrumbList`

| Field | Source |
| --- | --- |
| `itemListElement` | An array. |
| Each element's `position` | A 1-indexed contiguous integer. The first crumb is `1`. |
| Each element's `item.@id` | The canonical absolute URL of the breadcrumb target. |
| Each element's `item.name` | The visible text of the crumb. |

### `SoftwareSourceCode`

| Field | Source |
| --- | --- |
| `name` | `"MuxMaster"`. |
| `programmingLanguage` | `"Go"`. |
| `codeRepository` | `"https://github.com/FlavioCFOliveira/MuxMaster"`. |
| `license` | `"https://opensource.org/licenses/MIT"`. |
| `version` | The latest release version, sourced from `../MuxMaster/CHANGELOG.md` (`1.3.0` after the v1.3.0 sync; see `url-and-versioning.md` URL-VER-1). |
| `runtimePlatform` | The minimum Go version, sourced from `../MuxMaster/go.mod` (Go 1.27.1 at `v1.3.0`). |
| `description` | **SD-LAND-1.** A description of MuxMaster that states its support for the HTTP QUERY method (RFC 10008) and carries a performance-positioning statement that complies with `overview.md` INT-PERF-3, INT-PERF-4, and INT-PERF-7 (categories named in words, no numbers). |
| `targetProduct.applicationCategory` | `"DeveloperApplication"`. |
| `sameAs` | An array including at minimum the GitHub repository URL and the `pkg.go.dev` URL for the module. |

### `Organization`

| Field | Source |
| --- | --- |
| `name` | The publishing organisation name, as declared in the upstream repository's `LICENSE` and `README`. |
| `url` | The site's canonical URL: `https://muxmaster.net` in production; the value of `SITE_BASE_URL` in `development` and `staging`. |
| `logo` | The canonical logo URL on the site (mirrored from the upstream `assets/logo-muxmaster.png` — see `brand-and-visual.md`). |
| `sameAs` | An array including at minimum the GitHub organisation or user page that hosts the upstream repository. |

### `Person`

| Field | Source |
| --- | --- |
| `name` | The maintainer's name, as declared in the upstream repository's commit history and `LICENSE`. |
| `url` | The maintainer's GitHub profile URL. |
| `sameAs` | An array of authoritative third-party profile URLs only. |

### `WebSite`

| Field | Source |
| --- | --- |
| `name` | `"MuxMaster"`. |
| `url` | The site's canonical URL. |
| `inLanguage` | `"en"`. |
| `publisher` | Reference to `Organization@id`. |

### `CollectionPage`

| Field | Source |
| --- | --- |
| `name` | The page's `<h1>` text. |
| `description` | The page's `<meta name="description">` content. |
| `inLanguage` | `"en"`. |
| `isPartOf` | Reference to `WebSite@id`. |
| `publisher` | Reference to `Organization@id`. |
| `hasPart` | An array of `@id` references to the items the index lists (the doc sub-section pages, the example pages). |

### `FAQPage`

| Field | Source |
| --- | --- |
| `mainEntity` | An array of `Question` nodes — one per Q→A pair across all conversational chains on the page (see `geo.md`, `## Question-Oriented Content`). |
| Each `Question.name` | The complete interrogative sentence, identical to the heading text in the HTML. |
| Each `Question.acceptedAnswer.@type` | `"Answer"`. |
| Each `Question.acceptedAnswer.text` | The answer body, with the opening direct-answer sentence preserved. |

### Text of `FAQPage` and `HowTo` nodes

These rules apply on every page that emits `FAQPage` or `HowTo`, including `/`. They govern `Question.name`, `Question.acceptedAnswer.text`, `HowTo.name`, `HowTo.description`, `HowToStep.name`, and `HowToStep.text`.

- **SD-TEXT-1.** Each of these strings MUST be plain text, with no HTML markup.
- **SD-TEXT-2.** No whitespace character MAY precede a punctuation mark (for example, a space before a full stop, comma, colon, semicolon, question mark, or closing parenthesis).
- **SD-TEXT-3.** Each `HowToStep.text` MUST be a complete sentence, or a sequence of complete sentences; it MUST NOT end in a sentence fragment.

### `HowTo`

| Field | Source |
| --- | --- |
| `name` | The page's `<h1>` text or the explicit "How to …" heading that introduces the steps. On `/docs/http-query-method`, the value fixed by SD-QUERY-2. |
| `description` | A one-sentence summary of the procedure. |
| `step` | An array of `HowToStep` nodes, in order. |
| Each `HowToStep.name` | The step's heading text. |
| Each `HowToStep.text` | The step's prose body. |
| Each `HowToStep.url` | The canonical URL of the page plus the step's stable anchor (`#<slug>`). |

### `Dataset`

| Field | Source |
| --- | --- |
| `name` | A descriptive name of the form "MuxMaster v<version> router benchmark campaign, <YYYY-MM-DD>", naming the documented release and the campaign date. For the current campaign: "MuxMaster v1.3.0 router benchmark campaign, 2026-09-26" (SD-BENCH-2). |
| `description` | The benchmarks page's `<meta name="description">` content. |
| `creator` | Reference to `Organization@id`. |
| `license` | The licence declared for the campaign archive in this repository. When none is declared, the field is omitted with the audit-trail comment of `## Field completeness`. |
| `temporalCoverage` | The date or date range of the campaign's measurements (ISO 8601), sourced from the campaign archive's host facts. |
| `variableMeasured` | An array describing each measured metric — at minimum `ns/op`, `B/op`, `allocs/op`. |
| `measurementTechnique` | The campaign's measurement method: Go benchmarks run with `go test -bench` at the `-count` recorded in the archive (`content-sources.md` CS-BENCH-5), compared with `benchstat` (CS-BENCH-7) (SD-BENCH-2). |
| `keywords` | An array of terms that describe the dataset's subject (SD-BENCH-2). |
| `isAccessibleForFree` | `true` (SD-BENCH-2). |
| `distribution` | An array of `DataDownload` nodes, each linking to a raw `go test` or `benchstat` output file in the campaign archive, pinned to the full commit SHA (SD-BENCH-1). |
| `isBasedOn` | Optional. The corroborating upstream sources, linked at the `v1.3.0` tag. |

### `APIReference`

| Field | Source |
| --- | --- |
| `name` | `"MuxMaster API reference"`. |
| `description` | The `/api` page's `<meta name="description">` content. |
| `targetPlatform` | `"Go"`. |
| `programmingModel` | A short string describing the API model (for example `"radix-tree HTTP router"`). |
| `executableLibraryName` | `"github.com/FlavioCFOliveira/MuxMaster"`. |
| `assemblyVersion` | The latest release version, sourced from `../MuxMaster/CHANGELOG.md`. |
| `about` | Reference to `SoftwareSourceCode@id`. |

### `DefinedTermSet` and `DefinedTerm`

For each glossary or page that defines technical terms:

| Field | Source |
| --- | --- |
| `DefinedTermSet.@id` | A stable URI on the page (canonical URL plus a `#glossary` fragment). |
| `DefinedTermSet.name` | The glossary heading text. |
| `DefinedTermSet.hasDefinedTerm` | An array of `DefinedTerm` nodes, one per defined term. |
| Each `DefinedTerm.name` | The term being defined. |
| Each `DefinedTerm.description` | The definition body. |
| Each `DefinedTerm.inDefinedTermSet` | Reference to `DefinedTermSet.@id`. |
| Each `DefinedTerm.@id` | A stable URI on the page (canonical URL plus the term's anchor). |

### `Code`

For each named, citeable code snippet:

| Field | Source |
| --- | --- |
| `name` | The snippet's heading text. |
| `@id` | The canonical URL of the page plus the snippet's stable anchor. |
| `programmingLanguage` | `"Go"`. |
| `codeSampleType` | `"code snippet"` for partial snippets; `"full"` for complete, runnable programs. |
| `text` | The snippet body, identical to the text inside the `<pre><code>` block. |

## Auxiliary schemas

### `APIReference` on `/api`

The `/api` page documents the MuxMaster public API surface. It MUST emit one `APIReference` node in addition to the `TechArticle`, `SoftwareSourceCode`, and `BreadcrumbList` nodes already required for that page family. The `APIReference.about` field MUST reference `SoftwareSourceCode@id` so that the API surface is unambiguously linked to the module entity.

### `DefinedTerm` and `DefinedTermSet` for glossaries

A page that contains a glossary section, or that introduces and defines technical terms in a structured way (one heading per term, followed by the definition prose), MUST emit one `DefinedTermSet` node grouping every defined term on that page, plus one `DefinedTerm` node per term. The trigger is the presence of explicitly-defined terms; a passing mention of a term in running prose does not trigger this schema.

### `Code` for named, citeable code snippets

A code snippet is "named and citeable" when it satisfies all of the following:

- It is preceded by a heading (typically `<h3>` or `<h4>`).
- It carries a stable anchor on that heading.
- It is intended as a citable reference (an example a reader or a generative engine may link to or quote).

A snippet that meets these conditions MUST emit one `Code` node. A snippet that is illustrative inline prose (no heading, no anchor, no citation intent) does not emit `Code`.

## Validation

Validation is a **blocking CI gate**. Every pull request that touches HTML templates, content files under `/content/`, or the JSON-LD generators MUST run the following two checks before merge:

1. **schema.org validator** (https://validator.schema.org) on every page that emits JSON-LD. The build MUST fail on any error or warning marked `Critical` by the validator.
2. **Google Rich Results Test** equivalent (the publicly available Rich Results Test API, or the analogous `@google-cloud/structured-data-testing-tool`) on every page that emits a Rich-Results-eligible type — at minimum `FAQPage`, `HowTo`, `BreadcrumbList`, `TechArticle`, `Dataset`, and `SoftwareSourceCode`. The build MUST fail on any reported error. Warnings are not blocking but MUST be recorded in the build artefact (see below).

The CI job MUST emit, as a build artefact, the validator output for every page validated, so that reviewers can audit warnings and confirm that intentional omissions (recorded as HTML comments per `## Field completeness`) are accepted by the validators.

Local pre-commit running of the same validators is RECOMMENDED but not required. Authors MAY run the validators against a local server before opening a pull request to shorten the feedback loop.

## Non-negotiables

The following behaviours are forbidden everywhere on the site:

- **Fabricated values** in any JSON-LD field (a guessed `version`, an invented `datePublished`, a benchmark number not present in the campaign archive or in an attributed upstream source).
- **Placeholder URLs** (for example `https://example.com`) in any JSON-LD field.
- **Empty strings** used as a substitute for an absent value (for example `"author": { "@id": "" }`).
- **Inline redefinition** of the four reified entities (`SoftwareSourceCode` / `Organization` / `Person` / `WebSite`) on any page other than `/`. Other pages MUST reference them by `@id`.
- **JSON-LD blocks emitted only client-side**. Every JSON-LD block MUST be present in the initial HTML response served by the server. JavaScript-injected JSON-LD is not allowed.

A change that violates any of the above is rejected by the SEO and GEO gatekeepers and the CI validation gate.

## Compliance window

This contract is forward-looking. Existing pages MUST be brought into compliance during the next content-curation pass. New pages MUST comply on first write.

## Cross-references

- `seo.md`, `## JSON-LD structured data` — references this file for the master table, field expectations, and validation gate. SEO's specific concern is rich-result eligibility.
- `geo.md`, `## FAQPage and HowTo structured data` — references this file for field-level rules and the validation gate.
- `geo.md`, `## Question-Oriented Content`, `### JSON-LD coupling` — references this file for the entity graph, field completeness, and the validation gate.
- `agents-and-gates.md` — records co-ownership of this file by `seo-specialist` and `geo-specialist`.
- `information-architecture.md` — page-template definitions list the JSON-LD types per template; the master table here is authoritative when the two appear to disagree.
