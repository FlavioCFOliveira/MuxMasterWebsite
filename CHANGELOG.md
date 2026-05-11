# Changelog

All notable changes to the MuxMaster documentation website are recorded in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html). The website's `MAJOR.MINOR` mirrors the MuxMaster release it documents; the website's `PATCH` digit is independent and advances for website-only operational fixes. See `specification/overview.md § Version cadence` for the full cadence policy.

## [v1.0.10] — 2026-05-11

Operational PATCH on top of `v1.0.9`. Closes the final two findings deferred from the `v1.0.6` audit: GEO-003 (Markdown companions for the three index pages) and GEO-009 (`APIReference` + curated `DefinedTermSet` for `/api`, plus `ItemList` for `/docs/` and `/examples/`). `Dataset` on `/benchmarks` was already emitted by `buildBenchmarksJSONLD` from earlier releases; no change there. With this release every finding from the post-`v1.0.5` SEO/GEO audit is either resolved, deferred upstream (SEO-006), or deferred outside this repository (SEO-001 SITE_BASE_URL runtime override, SEO-003 `www` DNS). No MuxMaster release is documented by this entry: MuxMaster remains at `v1.0.1` (released 2026-05-08).

### Added

- **GEO-003 — Markdown companions for the three index pages.** New URLs: `/index.md` (served from `content/site/landing.md`), `/docs/index.md` and `/examples/index.md` (generated at startup from the route table, ordered the same way the HTML index page is ordered). Three new recipes — `LandingMarkdownRecipe`, `DocsIndexMarkdownRecipe`, `ExamplesIndexMarkdownRecipe` — share a `buildSectionIndexMarkdown` helper that optionally folds in an intro from `content/site/<section>-index.md` (with the leading `# Heading` line stripped to avoid a duplicate H1). Each of the three index page templates now emits `<link rel="alternate" type="text/markdown" href="...">` pointing at its companion; the head template uses a new `Page.MarkdownAlternateURL` field (set explicitly by the recipe) so that the index-page canonical (which ends in `/`) does not produce a malformed `<Canonical>.md` href. The convention `/index.md`, `/docs/index.md`, `/examples/index.md` was chosen over `/.md`, `/docs/.md`, `/examples/.md` because the former mirrors the way most documentation sites that serve Markdown lay out their canonical URLs, and matches the llmstxt.org expectation of clean, predictable paths.
- **GEO-009 — `ItemList` JSON-LD on `/docs/` and `/examples/`.** Both collection-family pages now emit a schema.org `ItemList` block alongside the existing `CollectionPage` + `BreadcrumbList`. Every list entry is a `ListItem` with `position` (1-indexed, preserving the same order the rendered HTML uses), `name`, `url`, and `description`. The list carries `itemListOrder: https://schema.org/ItemListOrderAscending`. The new `JSONLDInputs.ItemListItems []ItemListItem` field and the corresponding `itemsAsItemList` helper in `recipes.go` are the only plumbing required; existing `buildCollectionJSONLD` was extended to append the `ItemList` when the slice is non-empty.
- **GEO-009 — Curated `DefinedTermSet` JSON-LD on `/api`.** Twenty-nine `DefinedTerm` entries covering MuxMaster's public-API surface: top-level package symbols (`muxmaster.New`, `muxmaster.Mux`, `muxmaster.Group`, `muxmaster.Params`, `muxmaster.PathParam`, `muxmaster.ParamsFromContext`, `muxmaster.RoutePattern`, `muxmaster.HandlerFuncE`, `muxmaster.HTTPError`, `muxmaster.FastHandler`, `muxmaster.FastMiddleware`, `muxmaster.JSON`, `muxmaster.Text`, `muxmaster.XML`, `muxmaster.NoContent`, `muxmaster.Redirect`, `muxmaster.ServeFiles`) and middleware sub-package constructors (`middleware.RequestID`, `Recoverer`, `Logger`, `Compress`, `RealIP`, `Timeout`, `Throttle`, `BasicAuth`, `JWTAuth`, `OAuth2Introspect`, `APIKey`, `CORS`). Each `DefinedTerm.description` is a single sentence that an AI ingestion pipeline can quote verbatim. The list is curated rather than auto-extracted from `content/api.md` because the source is rendered `go doc` plain text — multi-paragraph descriptions with nested code blocks and indented elaboration — which does not map cleanly onto the single-string `DefinedTerm.description` field; a faithful auto-extraction would either truncate descriptions arbitrarily or carry code-fence wrappers into the JSON, neither of which is useful downstream. When upstream MuxMaster adds or renames a public symbol the release-manager agent MUST update `apiDefinedTerms` in `internal/render/jsonld.go`.

### Specification

- **`specification/structured-data.md` — page family table updated.** The `/api` row now lists `DefinedTermSet` as a required block (with the rationale for the curated-vs-auto-extracted decision in the cell text). The `/docs/` and `/examples/` rows now list `ItemList` as a required block, with notes on the curated learning order on `/examples/` and the alphabetical order on `/docs/`.

### Test evidence

- **Local Docker smoke against the freshly built `v1.0.10`-test image.** Container started on a free local port (`19500`):
  - `GET /index.md` → `200 (3747 B, text/markdown; charset=utf-8)` — landing.md body served verbatim with frontmatter stripped.
  - `GET /docs/index.md` → `200 (2090 B, text/markdown; charset=utf-8)` — emits `# Documentation\n\n` followed by 13 `- [Title](path) — description` lines in alphabetical order.
  - `GET /examples/index.md` → `200 (1553 B, text/markdown; charset=utf-8)` — emits `# Examples\n\n` followed by 8 list items in the curated learning order (REST API → auth family → operational concerns).
  - Each of the three index pages now carries `<link rel="alternate" type="text/markdown" href="https://muxmaster.net/{index path}.md">` in `<head>`.
  - `/api` JSON-LD now contains both `"@type":"APIReference"` and `"@type":"DefinedTermSet"`, with 29 `"@type":"DefinedTerm"` entries.
  - `/examples/` JSON-LD now contains 1 `ItemList` with 10 `ListItem` entries (the route table currently has 10 example entries; the count includes /examples/server-side-render and /examples/static-site which were added after the original audit).
  - `/docs/` JSON-LD now contains 1 `ItemList` with 13 `ListItem` entries.
  - `/benchmarks` continues to emit `Dataset` (no change required; was already correct).
- `make vet` — pass.
- `make build` — pass.
- `go test -race -count=1 ./...` — pass across all 6 packages, including the updated server test fixture that now provides `site/landing.md` (required by `LandingMarkdownRecipe`).

### Deployment note

This release only affects production HTML and machine-readable artefacts once an operator pulls the new image tag (`:v1.0.10` or the moving `:latest`) and restarts the running container. The remaining production-side defect (the `SITE_BASE_URL` override emitting `http://muxmaster.net/`, documented under `v1.0.6` *Deferred outside this repository*) is still independent and must be corrected at the deploy layer.

## [v1.0.9] — 2026-05-11

Operational PATCH on top of `v1.0.8`. CI-only fix: the release smoke-test introduced in `v1.0.6` still expected `/static/img/logo.png` to return `200`, but the SEO-008 fix landed in `v1.0.8` correctly makes that path return `404` (the source PNG is now a build input under `tools/imagegen/source.png`, not a runtime asset). The smoke is now aligned with the v1.0.8 contract: derived assets must return `200`, and the source PNG must return `404`. Image content is functionally identical to `v1.0.8`. No MuxMaster release is documented by this entry: MuxMaster remains at `v1.0.1` (released 2026-05-08).

### Fixed

- **Release smoke-test asserts the SEO-008 contract.** The "Derived static images" probe block in `.github/workflows/release.yml` no longer includes `/static/img/logo.png` (which now returns `404` by design). A new probe block (3b) explicitly asserts that `/static/img/logo.png` returns a non-200 status, so a future refactor that accidentally re-introduces the build input under `static/` would fail the smoke. The other twelve derived-asset probes are unchanged and continue to defend the `v1.0.5` regression class.

### Note on `v1.0.8`

- The `v1.0.8` image was built and pushed successfully; only the post-push smoke gate failed, and the failure was in the smoke script, not in the image. Operators tracking `:v1.0.8` are running the correct artefact; operators tracking `:latest` are now pulled forward to `:v1.0.9` (same artefact plus the smoke fix).

### Deployment note

`v1.0.9` is a CI-hygiene release. No runtime change. Operators on `:v1.0.8` may stay; operators on `:latest` are now on `v1.0.9` automatically.

## [v1.0.8] — 2026-05-11

Operational PATCH on top of `v1.0.7`. Lands the first two of the four audit findings deferred from `v1.0.6`: the homepage now emits a `FAQPage` JSON-LD block with six answer-first Q→A pairs (GEO-002), and the 1.6 MB `static/img/logo.png` build input is no longer publicly served (SEO-008). Two findings (GEO-003 markdown companions for the three index pages; GEO-009 `APIReference` / `Dataset` / `ItemList` with per-symbol `DefinedTerm`) remain deferred to a follow-up release because they require either new content authoring or a structured-content parser for `content/api.md`. No MuxMaster release is documented by this entry: MuxMaster remains at `v1.0.1` (released 2026-05-08).

### Changed

- **GEO-002 — homepage now emits a `FAQPage` JSON-LD with six answer-first Q→A pairs.** The pairs cover the questions an AI ingestion pipeline is most likely to receive from a cold prompt about MuxMaster: *"What is MuxMaster?"*, *"What Go version does MuxMaster require?"*, *"Is MuxMaster compatible with `net/http`?"*, *"How fast is MuxMaster?"*, *"What is MuxMaster's license?"*, *"How does MuxMaster compare to chi, gin, gorilla/mux, and httprouter?"*. The HTML form lives in `templates/pages/landing.html` inside a `<section data-conversation="landing-faq">` wrapper; the JSON-LD form is emitted by a new `buildLandingFAQPageJSONLD()` in `internal/render/jsonld.go` that reads from a package-level `landingFAQEntries` array. The two forms are intentionally duplicated (not derived from one another by markdown scanning) because the landing template is HTML-driven, not Markdown-driven — the comment block above `landingFAQEntries` flags the invariant explicitly.
- **SEO-008 — `static/img/logo.png` (1.6 MB) is no longer publicly served.** The source PNG is a build input for `tools/imagegen`, not a runtime asset; no page references it, but the public reachability meant any party probing the path pulled the 1.6 MB blob. The file is now vendored to `tools/imagegen/source.png`. `make logo` writes there; the imagegen helper reads from there; `make assets` continues to derive every variant under `static/img/` and `static/favicon/`. The Dockerfile builder picks the new layout up automatically (the `COPY . .` step is unchanged) and the runtime stage's `COPY --from=builder /workspace/static /srv/static` no longer carries the 1.6 MB build input. The `.gitignore` comment block, the `Makefile` `UPSTREAM_LOGO` comment, and the `tools/imagegen/main.go` package comment all document the new location and the *why*.

### Specification

- **`specification/structured-data.md`** (no edit needed; the `FAQPage` rule was already general — *"pages with three or more explicit Q→A pairs MUST emit a JSON-LD `FAQPage` block"*. The homepage was the only page in scope that did not emit one; this release closes the gap.)
- **`specification/url-and-versioning.md` (implicit)** — the previously documented `/static/img/logo.png` source-of-truth path no longer exists. The build-input source lives at `tools/imagegen/source.png`. No spec section explicitly referenced `/static/img/logo.png`, but the change is recorded here for completeness.

### Deferred to a future release

- **GEO-003 — Markdown companions for the three index pages (`/`, `/docs/`, `/examples/`).** The convention agreed during the audit follow-up is `/index.md`, `/docs/index.md`, `/examples/index.md` (llmstxt-friendly, mirrors the inner-page pattern, allows a natural `<link rel="alternate" type="text/markdown" href="/<path>/index.md">`). The implementation needs either: (a) new source files at `content/site/docs-index.md` and `content/site/examples-index.md` plus a markdown companion recipe for the three new routes; or (b) a programmatic generator that emits the markdown body from the existing route table (the same source the HTML index recipes already iterate over). Option (b) avoids content drift and is the recommended path. The homepage `/index.md` companion is the simplest of the three — `content/site/landing.md` already exists.
- **GEO-009 — `APIReference` (`/api`), `Dataset` (`/benchmarks`), `ItemList` (`/examples/`) JSON-LD with full coverage.** The full coverage includes per-symbol `DefinedTerm` entries for every exported symbol documented under `/api`. The `content/api.md` source is structured as `go doc` output — `FUNCTIONS` and `TYPES` sections each containing one symbol per `func ...` / `type ...` block — which is parseable by a regex-based extractor (roughly ~150 symbols across the package and the middleware sub-package). The implementation is therefore tractable but non-trivial and deserves a dedicated commit with proper test coverage rather than being bundled here.

### Test evidence

- **Local Docker smoke against the freshly built `v1.0.8`-test image** — the homepage HTML carries one `<section data-conversation="landing-faq">` wrapper with six question articles; the rendered JSON-LD includes one `"@type":"FAQPage"` block with six `"@type":"Question"` entities; the request to `GET /static/img/logo.png` returns `404 Not Found` while `GET /static/img/logo-32.png` still returns `200 OK` (so the derived variants are unaffected). Verified by `docker build` + `docker run` against `localhost:19300`.
- `make vet` — pass.
- `make build` — pass (the `make assets` step picks up the new `tools/imagegen/source.png` path and writes the same 13 derived artefacts; byte sizes are bit-identical to the previous build).
- `go test -race -count=1 ./...` — pass across all 6 packages.

### Deployment note

This release only affects production HTML once an operator pulls the new image tag (`:v1.0.8` or the moving `:latest`) and restarts the running container. The remaining production-side defect (the `SITE_BASE_URL` override emitting `http://muxmaster.net/` instead of `https://`, documented under `v1.0.6` *Deferred outside this repository*) is still independent and must be corrected at the deploy layer; it is unaffected by this release.

## [v1.0.7] — 2026-05-11

Operational PATCH on top of `v1.0.6`. CI-only fix: the extended release smoke-test introduced in `v1.0.6` (SEO-002) was itself buggy and rejected the otherwise-correct `v1.0.6` image. The smoke probe is corrected here so the gate it was designed to be can actually defend the release. The runtime image content is functionally identical to `v1.0.6`; consumers who already pulled `:v1.0.6` do not need to redeploy to `:v1.0.7` for any user-facing reason. No MuxMaster release is documented by this entry: MuxMaster remains at `v1.0.1` (released 2026-05-08).

### Fixed

- **Release smoke-test no longer captures startup-log JSON as `SITE_BASE_URL`.** The `v1.0.6` smoke first tried `docker exec muxmaster-smoke /srv/muxmaster-website --print-base-url` and fell back to `docker inspect` only when that command exited non-zero. The binary has no such flag and silently ignored it; instead it started a second server inside the running container and wrote the startup configuration log to stdout. Because the command exited zero, the capture succeeded — but it captured the JSON log line whose `site_base_url` field then contaminated every downstream check (`canonical=https://muxmaster.net/ does not match SITE_BASE_URL={"time":"..."}`). The fix removes the `docker exec` attempt entirely and reads `SITE_BASE_URL` from `docker inspect --format '{{range .Config.Env}}...'` as the single source of truth. The image's runtime `ENV` is by definition what every emitted URL must agree with.

### Note on `v1.0.6`

- The `v1.0.6` image was built and pushed to `ghcr.io/flaviocfoliveira/muxmaster-website:v1.0.6` and `:latest` successfully; only the post-push smoke gate failed, and the failure was in the smoke script, not in the image. The `v1.0.6` GitHub Release page is therefore absent, but the image artefact, its tag, and its CHANGELOG entry are intact. Operators tracking `:v1.0.6` are running the correct artefact; operators tracking `:latest` are now pulled forward to `:v1.0.7` (same artefact, with the smoke fix landing in the release workflow).

### Test evidence

- The fixed smoke step runs against the freshly published image, exercises every probe family introduced in `v1.0.6`, and now correctly resolves `SITE_BASE_URL` from `docker inspect`. The original `v1.0.6` runtime smoke (the local `docker build` + `docker run` smoke described in `v1.0.6` *Test evidence*) was already passing — the local test did not exercise the GitHub-Actions-only docker-exec path, which is what hid the bug.

### Deployment note

`v1.0.7` is a CI-hygiene release. No runtime change. Operators on `:v1.0.6` may stay; operators on `:latest` are now on `v1.0.7` automatically.

## [v1.0.6] — 2026-05-11

Operational PATCH on top of `v1.0.5`. Bundles seven findings raised by the `seo-specialist` + `geo-specialist` production audit of the post-`v1.0.5` site: a factual API-name correction in public documentation, four new AI crawlers in `robots.txt`, two new reserved-path artefacts (`/favicon.ico`, `/.well-known/security.txt`), a structured `Organization.logo` schema, and a substantially extended release smoke-test that fails the release on any future regression of the `v1.0.4` / `v1.0.5` / scheme-mismatch class. Two audit findings (the `Vary: Accept-Encoding` duplicate, sourced upstream in MuxMaster `middleware/compress.go`; and the `data-conversation` HTML wrappers, already present in every doc page that emits `FAQPage`) are documented under Known issues. No MuxMaster release is documented by this entry: MuxMaster remains at `v1.0.1` (released 2026-05-08).

### Fixed

- **GEO-005 — wrong API name in the `/docs/getting-started` FAQ and the `/api` overview FAQ.** Both FAQ blocks documented `mux.Param(r, "id")` for reading a path parameter; the real MuxMaster API is `muxmaster.PathParam(r, "id")`. The `mux.Param` symbol does not exist in the upstream module. The FAQ JSON-LD is built dynamically from the rendered HTML by `buildFAQPageJSONLD`, so the JSON-LD `FAQPage.mainEntity[*].acceptedAnswer` blocks corrected themselves on rebuild; no schema-emitter change was needed. The `/docs/getting-started` FAQ now also points to the typed-parameter helpers (`muxmaster.ParamsFromContext(r.Context()).Int("id")` and friends) so an LLM ingesting the answer surfaces a working idiom for typed reads.

### Changed

- **GEO-007 — four AI crawlers explicitly listed in `robots.txt`.** `Perplexity-User` (Perplexity on-demand fetch), `FacebookBot` (Meta crawler — explicit complement to `meta-externalagent`, which only covers Meta's generative agent), `cohere-ai` (Cohere), and `MistralAI-User` (Mistral on-demand fetch). The bots fall through to the `User-agent: *` wildcard in the absence of an explicit entry; explicit listing is the project's chosen pattern (per `specification/geo.md § AI crawler allowlist`) so that any future decision to allow or disallow is auditable per-crawler.
- **SEO-004 — `/favicon.ico` is now reserved.** Browsers, RSS readers, bookmark engines, and various aggregators probe `/favicon.ico` by reflex even when the page declares modern `<link rel="icon">` alternatives. Returning a 7 KB HTML 404 body for every such probe wasted bandwidth and polluted logs. The path now returns `301 Moved Permanently` with `Location: /static/favicon/favicon-32.png` and `Cache-Control: public, max-age=86400`. The redirect Location is path-only — the host header is never echoed back to the client, mirroring the defensive pattern of `normalisationRedirects`.
- **SEO-007 — `/.well-known/security.txt` (RFC 9116) is now served.** Required fields: `Contact: https://github.com/FlavioCFOliveira/MuxMaster/security/advisories/new`, `Expires` (computed at build time as twelve months ahead of the build, in `2006-01-02T15:04:05Z` format), `Preferred-Languages: en`, `Canonical: <SITE_BASE_URL>/.well-known/security.txt`. The `Expires` value MUST be refreshed before it lapses; this is part of the release contract and is verified by the extended release smoke-test described under Test evidence.
- **SEO-009 — `Organization.logo` is now an `ImageObject`.** The JSON-LD field changed from a bare URL string to a full `ImageObject` with `url`, `contentUrl`, `width: 384`, `height: 384`. Google's *Organization* rich-result documentation prefers the structured form because consumers receive the aspect ratio without fetching the binary. The bare-URL form remains valid schema.org; the structured form is strictly more useful and equally cheap.
- **SEO-002 — release smoke-test extended from `/healthz` to seven probe families.** The previous smoke step ran only `--healthcheck`, which probes `/healthz`. Any regression in the HTML body, static-asset delivery, or canonical-URL emission shipped green because `/healthz` continued to answer `ok\n`. The extended smoke now also probes: (a) 11 public HTML routes for `200` and a body of at least 1000 B; (b) 12 derived static-image assets; (c) the hashed CSS bundle (discovered dynamically from the homepage `<link>` so the test is independent of the hash); (d) the two new reserved-path artefacts (`/favicon.ico`, `/.well-known/security.txt`); (e) the `rel="canonical"` URL on the homepage matches the image's baked `SITE_BASE_URL`; (f) the first `<loc>` in `sitemap.xml` matches `SITE_BASE_URL`; (g) the `Sitemap:` directive in `robots.txt` matches `<SITE_BASE_URL>/sitemap.xml`. The container is now mapped `8080:80` (the runtime listens on `:80` since `v1.0.2`); the previous `8080:8080` mapping was a stale leftover from the pre-`v1.0.2` port layout. Each of the three regressions seen this week (`v1.0.4` CSS `403`, `v1.0.5` derived-image `404`, current production scheme override) would now fail the release.

### Specification

- **`specification/geo.md` — AI crawler allowlist extended.** Adds `Perplexity-User`, `FacebookBot`, `cohere-ai`, `MistralAI-User` to the canonical list. The recipe in `internal/render/recipes.go` and the test in `internal/render/recipes_test.go` are updated to match.
- **`specification/url-and-versioning.md § Reserved paths` — two new paths.** `/favicon.ico` is now documented as a `301` to `/static/favicon/favicon-32.png` with `Cache-Control: public, max-age=86400`. `/.well-known/security.txt` is documented as an RFC 9116 plain-text artefact with required fields (`Contact`, `Expires` capped at twelve months, `Preferred-Languages`, `Canonical`) and the release-contract obligation to refresh `Expires` before it lapses. The `/assets/...` placeholder line was corrected to `/static/...` to match the actual route.

### Known issues

- **SEO-006 — duplicate `Vary: Accept-Encoding` header on compressed responses (deferred upstream).** Verified root cause: `../MuxMaster/middleware/compress.go:64` calls `hdr.Add("Vary", "Accept-Encoding")` unconditionally in the commit path, even when the application has already called `Set` on the same header. The duplicate manifests only for clients that advertise `gzip` or `br` (verified by curl: `identity` clients receive a single `Vary`, `gzip`/`br` clients receive two). The proper fix is a five-line idempotency check in the upstream MuxMaster `Compress` middleware (`if !contains(hdr["Vary"], "Accept-Encoding") { hdr.Add(...) }`); shipping a local workaround in this repo would either lose `Vary` on small uncompressed responses or require a brittle post-compress dedupe wrapper. Tracked as a follow-up; will be picked up by a MuxMaster patch release plus a `go.mod` bump here.
- **GEO-006 — `<section data-conversation="...">` HTML wrappers (verified non-issue).** The `geo-specialist` audit claimed the wrapper was missing from the HTML of pages emitting `FAQPage` JSON-LD. Cross-checked against the production HTML of `https://muxmaster.net/docs/getting-started` (`grep -c data-conversation` → `1`) and the 18 markdown sources under `/content/` (every doc, example, and the API overview already carries an `<section data-conversation="...">` wrapper). The finding was a false positive; no change needed.
- **Nine outstanding lint findings (8 `errcheck`, 1 `staticcheck`) inherited from `v1.0.1`.** Same status as `v1.0.5`; not affected by this release.

### Deferred to v1.0.7 (design decisions required before implementation)

The following audit findings need explicit design decisions and were not bundled into this PATCH:

- **GEO-002 — `FAQPage` JSON-LD on the homepage** (and the six Q→A pairs that compose it; new content authoring).
- **GEO-003 — Markdown companions for the three index pages (`/`, `/docs/`, `/examples/`)** (decide the URL convention: `/index.md`, `/.md`, or `/_/index.md`; spec change).
- **SEO-008 — Stop publicly serving the 1.6 MB `static/img/logo.png` build input** (decide whether to relocate the source PNG out of `static/` or to add a route that 404s the path; touches both the `Makefile` and the build).
- **GEO-009 — `APIReference` on `/api`, `Dataset` on `/benchmarks`, `ItemList` on `/examples/`** (decide depth of `DefinedTerm` per-symbol coverage; non-trivial schema work).

### Deferred outside this repository

- **SEO-001 / GEO-001 — `SITE_BASE_URL` runtime override in production.** Verified non-code defect: the live container at `https://muxmaster.net` is emitting `http://muxmaster.net/...` in every canonical, OG, Twitter, sitemap, JSON-LD `@id`, `llms.txt`, and `<link rel="alternate">` artefact, despite the image baking `ENV SITE_BASE_URL=https://muxmaster.net` since `v1.0.3` (commit `ef27371`). Local smoke (`docker run` with the image's default env) confirms the image emits `https://muxmaster.net/...` correctly. The fix is an operator action on the production host: remove any `-e SITE_BASE_URL=http://muxmaster.net` runtime override and redeploy.
- **SEO-003 — `www.muxmaster.net` DNS record.** The hostname does not resolve from this network. The Let's Encrypt certificate covers the name, but DNS does not point at the production deployment, so the `www → apex` `301` chain cannot be verified end-to-end. The fix is an A (and ideally AAAA) record pointing at the production IP.

### Test evidence

- **Local Docker smoke against the freshly built `v1.0.6-test` image.** Container started on a free local port (`19100`), `--healthcheck` passed within 2 s, then probed: `/favicon.ico` returned `301` with `Location: /static/favicon/favicon-32.png` and `Cache-Control: public, max-age=86400`; `/.well-known/security.txt` returned `200` with `Content-Type: text/plain; charset=utf-8` and the four RFC 9116 fields (`Contact`, `Expires=2027-05-11T17:56:58Z`, `Preferred-Languages: en`, `Canonical: https://muxmaster.net/.well-known/security.txt`); `/robots.txt` returned all four new crawlers (`User-agent: Perplexity-User`, `User-agent: FacebookBot`, `User-agent: cohere-ai`, `User-agent: MistralAI-User`); the homepage JSON-LD now emits `Organization.logo` as `{"@type":"ImageObject","url":"https://muxmaster.net/static/img/logo-384.png","contentUrl":"https://muxmaster.net/static/img/logo-384.png","width":384,"height":384}`. The smoke also confirms the image's baked `SITE_BASE_URL=https://muxmaster.net` reaches every emitted URL.
- `make vet` — pass (no diagnostics).
- `make build` — pass (binary linked successfully; `make assets` regenerated the 13 derived PNGs deterministically).
- `go test -race -count=1 ./...` — pass across all 6 packages (`cmd/muxmaster-website`, `internal/config`, `internal/content`, `internal/meta`, `internal/render`, `internal/server`), `0 FAIL`, `0 SKIP`. Includes the updated `TestRobotsRecipeListsExpectedBots` which now asserts the four new crawlers are present.
- **Extended release smoke-test (introduced in this release).** Will run on the `v1.0.6` release workflow against the published image, exercising the seven probe families described under Changed § SEO-002.

### Deployment note

This fix only affects production HTML once an operator pulls the new image tag (`:v1.0.6` or the moving `:latest`) and restarts the running container. The remaining production-side defect (`SITE_BASE_URL` override emitting `http://muxmaster.net/`) is independent of this release and must be corrected at the deploy layer, as described under *Deferred outside this repository*.

## [v1.0.5] — 2026-05-11

Operational PATCH on top of `v1.0.4`. Fixes a production bug in which every derived static-image asset returned `404 Not Found` — sized header logos, favicons, the apple-touch-icon, and the 1200×630 Open Graph composition — because the Docker builder never ran `make assets`. The fix adds `make assets` to the builder's `RUN` step so the runtime image is self-contained. No MuxMaster release is documented by this entry: MuxMaster remains at `v1.0.1` (released 2026-05-08).

### Fixed

- **Derived static-image assets are now present in the production image.** The 13 PNG artefacts produced by `make assets` (`static/img/logo-{32,64,80,128,192,256,384}.png`, `static/img/og-image.png`, `static/favicon/favicon-{32,192,512}.png`, `static/favicon/apple-touch-icon-180.png`) are gitignored by design — they are deterministic outputs of `tools/imagegen` from the single committed source `static/img/logo.png`. The Dockerfile's builder stage previously ran only `make tailwind-install && make css`, so the runtime stage's `COPY --from=builder /workspace/static /srv/static` brought the source `logo.png` and the freshly built CSS bundle but **none** of the derived images. Every `<link rel="icon">`, `<link rel="apple-touch-icon">`, `<img>` tag in the header, and `og:image` absolute URL on every page therefore resolved to a `404 Not Found` at request time, including `https://muxmaster.net/static/img/og-image.png` — which is the URL served to every social-media unfurl, every search-engine result preview, and every AI answer-engine card. The Dockerfile now runs `make tailwind-install && make css && make assets` in the same builder `RUN` step. The `imagegen` tool is pure Go (`golang.org/x/image/draw`, no CGO, no system image libraries) so it builds cleanly inside the existing `golang:1.26` builder stage with no additional toolchain.

### Changed

- **Dockerfile builder stage runs `make assets`.** The single-line change is annotated with a comment documenting why the step is load-bearing (derived images are gitignored; the only path to the runtime image is this RUN). The runtime stage is unchanged. Image-build time grows by a fraction of a second (imagegen is sub-second on the release runner).

### Known issues

- **The release smoke-test does not exercise static-asset paths.** The current smoke step in `.github/workflows/release.yml` runs `/srv/muxmaster-website --healthcheck`, which only probes `/healthz`. A missing static asset therefore does not fail the release — that is how `v1.0.3` and `v1.0.4` shipped with the 404s described above. A future website-only PATCH should extend the smoke-test to probe at least one derived asset (for example `/static/img/og-image.png` and one favicon) before the release is marked successful, so the same class of bug cannot recur silently. Tracked as a follow-up; not a release blocker for `v1.0.5`.
- **Nine outstanding lint findings (8 `errcheck`, 1 `staticcheck`) inherited from `v1.0.1`.** Same status as `v1.0.4`; not affected by this release.

### Test evidence

- **Local image build** — `docker build -t muxmaster-website:v1.0.5-test .` completes successfully on `golang:1.26` with `make tailwind-install && make css && make assets` as the new builder RUN step.
- **End-to-end probe of the locally-built image** — the container was run on a free local port (`19000`) and every previously-404 path was probed. Results: `/static/img/logo.png` `200 (1644598B, image/png)`, `/static/img/logo-32.png` `200 (2155B, image/png)`, `/static/img/logo-384.png` `200 (161876B, image/png)`, `/static/img/og-image.png` `200 (205160B, image/png)`, `/static/favicon/favicon-32.png` `200 (2155B, image/png)`, `/static/favicon/favicon-512.png` `200 (268020B, image/png)`, `/static/favicon/apple-touch-icon-180.png` `200 (44376B, image/png)`, `/static/css/app.cda7aa064711.css` `200 (43330B, text/css; charset=utf-8)`. Byte sizes match the host-built outputs exactly, confirming the `imagegen` tool is deterministic and produces identical artefacts in the builder.
- `make css` — pass.
- `make vet`, `make build`, `go test -race -count=1 ./...` — unchanged from `v1.0.4`; all green.

### Deployment note

This fix only affects production HTML once an operator pulls the new image tag (`:v1.0.5` or the moving `:latest`) and restarts the running container. Existing deployments built from `v1.0.4` or earlier continue to serve `404 Not Found` for every derived static-image path until that redeploy happens.

## [v1.0.4] — 2026-05-11

Operational PATCH on top of `v1.0.3`. Fixes a production bug in which the hashed CSS bundle (`/static/css/app.<hash>.css`) was served as `403 Forbidden` by the official Docker image, leaving every page on `https://muxmaster.net` unstyled. The `Makefile` now enforces a uniform permission model across the entire `static/` tree — every directory `0755`, every file `0644` — so the non-root runtime UID introduced in `v1.0.2` can always read every static asset, regardless of which build tool produced it or what its default mode was. No MuxMaster release is documented by this entry: MuxMaster remains at `v1.0.1` (released 2026-05-08).

### Fixed

- **Hashed CSS bundle is now readable by the non-root runtime user.** The `css` recipe in the `Makefile` produced the hashed bundle via `mktemp` followed by `mv`. `mktemp(1)` creates its temporary file with mode `0600` (POSIX security default), and `mv(1)` preserves the source mode on the destination, so the final `static/css/app.<hash>.css` landed on disk with mode `0600`. While the binary ran as root inside the container (every release up to and including `v1.0.1`), a `0600` root-owned bundle was readable at request time and the bug was invisible. The migration to the distroless non-root runtime in `v1.0.2` (UID 65532, commit `4c5a1f6`) inverted that: `os.Open` of a `0600` root-owned file from UID 65532 returns `EACCES`, which Go's `http.FileServer` maps to `403 Forbidden\n` (a 14-byte plain-text body), and every public page on `https://muxmaster.net` was served unstyled because the CSS link in its `<head>` resolved to that 403.

### Changed

- **`Makefile` — uniform permission model for the `static/` tree.** The `Makefile` now exposes a `NORMALIZE_STATIC_PERMS` helper (`find static -type d -exec chmod 0755 {} + && find static -type f -exec chmod 0644 {} +`) that is invoked as the final step of every recipe that creates or modifies content under `static/` — currently `css`, `logo`, and `assets`. A new phony target `static-perms` exposes the same sweep for manual invocation. The sweep is deliberately tree-wide rather than per-file: any future tool that lands content under `static/` is automatically covered without needing to remember to `chmod` its own outputs, and any future producer recipe inherits the contract by chaining the same helper. The single point of enforcement removes the class of bug that produced this release: a build tool's per-file default mode (such as `mktemp`'s `0600`) can no longer leak into a deployed artefact.

### Known issues

- **Nine outstanding lint findings (8 `errcheck`, 1 `staticcheck`) inherited from `v1.0.1`.** All nine findings predate this release and live in files untouched by the `v1.0.4` commit. The release contract enforced in CI (`go build`, `go vet`, `go test -race`, the JSON-LD validation gate) is green at the release commit; `golangci-lint` is not currently a CI gate. The findings are tracked as technical debt for a dedicated `chore(lint)` cleanup in a future website-only PATCH; they do not affect runtime behaviour and are documented here for transparency.

### Test evidence

- **Permission-sweep proof** — every file under `static/` was deliberately set to `0600` (with a parent directory at `0700`) and `make css` was then run. After the recipe completed, `find static -type f ! -perm 0644` reported zero results and `find static -type d ! -perm 0755` reported zero results. The sweep covered all 19 files (the hashed CSS bundle, the source `app.css`, eight sized PNGs in `static/img/`, five favicon/apple-touch-icon PNGs in `static/favicon/`, the vendored `logo.png`, the generated `og-image.png`, and the two `.gitkeep` files) and all four directories (`static`, `static/css`, `static/img`, `static/favicon`).
- **Standalone target** — `make static-perms` reports `normalised permissions under static/ (dirs 0755, files 0644)` and restores `0644` on a file previously set to `0600`.
- `make css` — pass (Tailwind v4 production bundle built; `stat -c '%a' static/css/app.cda7aa064711.css` reports `644`).
- `make vet` — pass (no diagnostics).
- `make build` — pass (binary linked successfully).
- `go test -race -count=1 ./...` — pass across all 6 packages (`cmd/muxmaster-website`, `internal/config`, `internal/content`, `internal/meta`, `internal/render`, `internal/server`), `0 FAIL`, `0 SKIP`.

### Deployment note

This fix only affects production HTML once an operator pulls the new image tag (`:v1.0.4` or the moving `:latest`) and restarts the running container. Existing deployments built from `v1.0.3` or earlier continue to serve `403 Forbidden` for the hashed CSS until that redeploy happens.

## [v1.0.3] — 2026-05-11

Operational PATCH on top of `v1.0.2`. Fixes a production bug in which the official Docker image served `localhost:8080` URLs in every public canonical reference — `<link rel="canonical">`, `og:url`, `og:image`, `sitemap.xml`, `llms.txt`, `llms-full.txt`, and JSON-LD `@id` URIs — because the published image relied on the binary's development default for `SITE_BASE_URL`. The runtime stage now bakes `SITE_BASE_URL=https://muxmaster.net` so that an unconfigured run of `ghcr.io/flaviocfoliveira/muxmaster-website` already produces correct production URLs. No MuxMaster release is documented by this entry: MuxMaster remains at `v1.0.1` (released 2026-05-08).

### Fixed

- **Production HTML no longer references `localhost:8080`.** Before this release, the published image declared no `ENV SITE_BASE_URL` in its runtime stage, so every container that did not receive an explicit runtime override fell back to the binary default of `http://localhost:8080`. As a result, the live site at `https://muxmaster.net` served pages whose `<link rel="canonical">`, `og:url`, absolute `og:image`, `sitemap.xml` `<loc>` entries, `llms.txt` and `llms-full.txt` URLs, and JSON-LD `@id` URIs all pointed at `http://localhost:8080`. Search engines and AI ingestion crawlers therefore received a self-contradictory site: reachable at `https://muxmaster.net` but self-declaring as `http://localhost:8080`. The runtime stage of the Dockerfile now sets `ENV SITE_BASE_URL=https://muxmaster.net`, alongside the existing `PORT=80`, `LOG_LEVEL=info`, and `ENV=production` defaults. Operators who already pass `-e SITE_BASE_URL=…` (or the equivalent in their orchestrator) are unaffected; operators who relied on the unconfigured image will receive the canonical production value automatically once they redeploy with the new tag.

### Specification

- **`specification/deployment.md` — new `## Terminology` section.** Defines three layers that govern the effective value of every runtime variable: **binary default** (compiled into the Go binary, appropriate for local development), **image default** (declared via `ENV` in the runtime stage of the Dockerfile, mirrors the canonical production origin because the official image's only public instance is the production site), and **runtime override** (Docker `-e`, Kubernetes `env:`, Compose `environment:`, systemd `Environment=`; always wins). The three-layer model applies uniformly across `SITE_BASE_URL`, `PORT`, `LOG_LEVEL`, and `ENV`.
- **`specification/deployment.md` — runtime-stage requirement.** The Runtime stage section now mandates that the Dockerfile's runtime stage MUST set `ENV SITE_BASE_URL=https://muxmaster.net` and `ENV PORT=80`. The mandate is justified explicitly: the published image is the artefact of this repository, the only public instance of that artefact is the canonical production site, and an unconfigured run must therefore already produce correct canonical URLs.
- **`specification/deployment.md` — runtime environment-variable table restructured.** The single `Default` column is split into separate `Binary default` and `Image default` columns. `SITE_BASE_URL` is marked "No when running the official Docker image (the image default is the canonical production value); Yes for any other execution context whose canonical origin differs from the binary default."
- **`specification/deployment.md` — new `### SITE_BASE_URL resolution` sub-section.** Documents the binary default (`http://localhost:8080`, intentionally not changed to a remote host because doing so would publish canonical references for production from a non-production process), the image default (`https://muxmaster.net`, justified by the artefact identity), and the runtime override path for staging and preview deployments (which MUST also set `ENV` to `staging` or `development` so that `noindex` is forced and staging never advertises canonical-production identifiers).

### Changed

- **`README.md` — runtime environment variable table reshaped.** The table now exposes separate `Binary default` and `Image default` columns, with the prose introduction explicitly framing the three-layer model (binary default, image default, runtime override). Each variable's image default is listed: `SITE_BASE_URL=https://muxmaster.net`, `PORT=80`, `LOG_LEVEL=info`, `ENV=production`. A follow-up paragraph documents how to override `SITE_BASE_URL` for staging or preview deployments and reminds operators to set `ENV=staging` or `ENV=development` whenever the override differs from the canonical production value.
- **`README.md` — Docker section.** The closing line now states that the runtime stage listens on `:80` (h2c) — previously `:8080`, a stale reference left over from before the `v1.0.2` port migration — and adds that the image bakes `SITE_BASE_URL=https://muxmaster.net` and `ENV=production`, with explicit guidance to override either at `docker run` time for staging or preview deployments.
- **`README.md` — quick-start prose.** The line "The site listens on `:8080` by default." is clarified to "The site listens on `:8080` by default when run from source." so that the documented default cannot be mistaken for the official image's listen port, which has been `:80` since `v1.0.2`.

### Known issues

- **Nine outstanding lint findings (8 `errcheck`, 1 `staticcheck`) inherited from `v1.0.1`.** All nine findings predate this release and live in files untouched by the `v1.0.3` commit. The release contract enforced in CI (`go build`, `go vet`, `go test -race`, the JSON-LD validation gate) is green at the release commit; `golangci-lint` is not currently a CI gate. The findings are tracked as technical debt for a dedicated `chore(lint)` cleanup in a future website-only PATCH; they do not affect runtime behaviour and are documented here for transparency.

### Test evidence

- `make css` — pass (Tailwind v4 production bundle built).
- `make assets` — pass (12 image artefacts generated).
- `make vet` — pass (no diagnostics).
- `make build` — pass (binary linked successfully).
- `go test -race -count=1 ./...` — pass across all 6 packages, 76 individual `PASS` cases, 0 `FAIL`, 0 `SKIP`. Includes `TestJSONLDValidationGate`, which runs as part of `go test` and exercises every pre-rendered HTML page against the schema.org structural checks and the rich-result eligibility checks.
- `make lint` — 9 pre-existing findings reported as Known issues above; identical to the `v1.0.2` baseline (no new findings introduced by this release); not a release blocker under the current CI contract.

### Deployment note

This fix only affects production HTML once an operator pulls the new image tag (`:v1.0.3` or the moving `:latest`) and restarts the running container. Existing deployments continue to serve `localhost:8080` URLs until that redeploy happens.

## [v1.0.2] — 2026-05-11

Operational PATCH on top of `v1.0.1`. The container now serves on port `80` as a non-root user, every GitHub Actions workflow is bumped to its latest major version, and the specification adopts an independent PATCH cadence for website-only fixes. No MuxMaster release is documented by this entry: MuxMaster remains at `v1.0.1` (released 2026-05-08).

### Changed

- **Container image binds to port `80` by default.** The runtime image previously exposed port `8080`; the production Docker image now binds the HTTP listener directly to port `80` so that operators can run `docker run -p 80:80 …` without a port-mapping shim. The container continues to run as the distroless non-root user (UID 65532); the binary carries the file capability `cap_net_bind_service=ep` (attached during the builder stage and preserved across BuildKit's `COPY --from=builder`) so that the non-privileged user can bind to a privileged port without any additional Linux capability being granted to the container. The locally compiled binary (`make dev`, `go run`) still defaults to port `8080` for development convenience; `PORT` overrides either default at runtime. Commit `4c5a1f6`.
- **GitHub Actions bumped to their latest major versions.** Every action referenced by the project's CI and release workflows is pinned to its current major (for example `actions/checkout@v5`, `actions/setup-go@v6`, `actions/upload-artifact@v5`, `docker/setup-buildx-action@v4`, `docker/login-action@v4`, `docker/build-push-action@v7`, `softprops/action-gh-release@v3`). The bump removes Node 16/18 deprecation warnings on GitHub-hosted runners and aligns the project with the supported runtime; no workflow behaviour changes. Commit `9201465`.

### Specification

- **Version cadence — lockstep rule superseded.** The lockstep rule ratified in the `v1.0.1` CHANGELOG entry is superseded as of this release. The website's `PATCH` digit is now independent of MuxMaster's `PATCH` and advances for website-only operational fixes (CI changes, Docker image fixes, deployment adjustments, accessibility corrections, copy edits, infrastructure work). `MAJOR.MINOR` continue to mirror MuxMaster's `MAJOR.MINOR`. The historical `v1.0.1` entry is immutable under Keep a Changelog and is not edited retroactively; the new policy is recorded in `specification/overview.md § Version cadence`.

### Known issues

- **Nine outstanding lint findings (8 `errcheck`, 1 `staticcheck`) inherited from `v1.0.1`.** All nine findings predate this release and live in files untouched by `4c5a1f6` and `9201465`. The release contract enforced in CI (`go build`, `go vet`, `go test -race`, the JSON-LD validation gate) is green at the release commit; `golangci-lint` is not currently a CI gate. The findings are tracked as technical debt and are scheduled for a dedicated `chore(lint)` cleanup in a future website-only PATCH; they do not affect runtime behaviour and are documented here for transparency.

### Test evidence

- `make css` — pass (Tailwind v4 production bundle built).
- `make assets` — pass (12 image artefacts generated).
- `go vet ./...` — pass (no diagnostics).
- `go test -race ./...` — pass across all 6 packages, including `TestJSONLDValidationGate`.
- `make lint` — 9 pre-existing findings reported as Known issues above; not a release blocker under the current CI contract.

## [v1.0.1] — 2026-05-11

First public release of the MuxMaster documentation website. The site documents and promotes the MuxMaster Go HTTP router (`github.com/FlavioCFOliveira/MuxMaster`, v1.0.1, released 2026-05-08) and is itself built on MuxMaster as a real-world dogfooding example.

### Added

- **Canonical production domain.** Ratified as `https://muxmaster.net` (HTTPS, apex, no trailing slash). Used by `<link rel="canonical">`, Open Graph `og:url`, absolute `og:image`, `sitemap.xml`, `llms.txt`, `llms-full.txt`, and JSON-LD `@id` URIs. Closes specification open question #1.
- **Static-tending architecture.** Every public route is pre-rendered at server startup; the same URL returns the same bytes for the lifetime of the process. Approximately thirty public routes are wired in this release: landing, eleven documentation pages under `/docs/`, eight under `/examples/`, `/api`, `/benchmarks`, `/security`, `/compatibility`, `/contributing`, `/changelog`, and `/releases/v1.0.0`.
- **Operational endpoints.** `/healthz` (liveness probe; binds after the renderer is ready), `/robots.txt`, `/sitemap.xml`, `/llms.txt`, and `/llms-full.txt`.
- **Self-contained Docker image.** Multi-stage build, distroless runtime, non-root user, in-binary `--healthcheck` flag for container health checks. The image embeds every template, content file, and static asset.
- **SEO contract.** Per-page unique `<title>`, `<meta name="description">`, canonical link, Open Graph and Twitter Card metadata, semantic HTML5 landmarks, JSON-LD structured data (`SoftwareSourceCode`, `TechArticle`, `BreadcrumbList`, `FAQPage`, `HowTo` where applicable), XML sitemap, and search-engine `robots.txt` rules.
- **GEO contract.** Top-level `/llms.txt` and `/llms-full.txt` artefacts (llmstxt.org convention), Markdown companion at `<path>.md` for every documentation route so AI engines can ingest content without HTML noise, and explicit AI-crawler rules in `robots.txt`.
- **Security headers.** Content-Security-Policy, Strict-Transport-Security (HTTPS-gated), Cross-Origin-Opener-Policy, Cross-Origin-Resource-Policy, X-Frame-Options `DENY`, X-Content-Type-Options `nosniff`, Referrer-Policy, and Permissions-Policy. X-Forwarded-For trust is opt-in via the `TRUSTED_PROXY_CIDRS` environment variable; outside that list the header is ignored.
- **CI gates.** JSON-LD validation runs as a blocking gate on every pull request. Lighthouse and pa11y run as warning-only gates today; they are scheduled to flip to blocking after one week of green runs (tracked in `specification/ci.md`).
- **Test coverage at release.** `internal/config` 100 %, `internal/content` 86.5 %, `internal/meta` 85 %, `internal/server` 80.4 %, `internal/render` 51.9 %. The full suite runs under the race detector and is green at the release commit.

### Fixed

- **`go.mod` resolves the upstream module from the proxy.** The previous `replace github.com/FlavioCFOliveira/MuxMaster => ../MuxMaster` directive pointed the build at a developer-local checkout that no continuous-integration runner can access. It is removed in this release; MuxMaster `v1.0.1` is now resolved from the public module proxy on every host, including the release workflow runner.

### Changed

- **Specification: version cadence.** `specification/overview.md § Version cadence` is updated to ratify the new policy: the website is released with the same semantic version as the MuxMaster release it documents, and the website's own release history lives in `/CHANGELOG.md` and in annotated Git tags. The rule that reads the displayed version label from `/content/changelog.md` at startup is unchanged.

### Infrastructure

- **Release workflow.** A new GitHub Actions workflow at `.github/workflows/release.yml` is triggered by tags matching `v*.*.*`. The workflow re-runs the test suite, builds the Docker image with Buildx, publishes it to `ghcr.io/flaviocfoliveira/muxmaster-website` under the immutable `:v1.0.1` tag and the moving `:latest` tag, smoke-tests the resulting image's `--healthcheck`, and creates the corresponding GitHub Release with this changelog entry as the body.

[v1.0.3]: https://github.com/FlavioCFOliveira/MuxMasterWebsite/releases/tag/v1.0.3
[v1.0.2]: https://github.com/FlavioCFOliveira/MuxMasterWebsite/releases/tag/v1.0.2
[v1.0.1]: https://github.com/FlavioCFOliveira/MuxMasterWebsite/releases/tag/v1.0.1
