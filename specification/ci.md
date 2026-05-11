---
title: CI pipeline contract
purpose: Define the continuous-integration pipeline that gates every pull request, with particular focus on the blocking structured-data validation step mandated by structured-data.md § Validation.
owners: seo-specialist + geo-specialist (jointly own the structured-data validation gate); release-manager (owns the rest of the pipeline).
last-updated: 2026-05-11
status: ratified
---

# CI pipeline contract

## Purpose

Every pull request that merges into `main` MUST pass a deterministic continuous-integration pipeline before merge. The pipeline runs the test suite, the structured-data validation gate ratified in `structured-data.md` § Validation, and the standard build steps the release-manager agent uses for tagging.

This file pins the technical choices behind each gate so the pipeline is reproducible, auditable, and free of drift between specification and runtime.

## Structured-data validation — toolchain choice

`structured-data.md` § Validation mandates a blocking gate using the schema.org validator AND the Google Rich Results Test. The available off-the-shelf options were evaluated:

| Tool | API stability | Coverage | Authentication | Verdict |
| --- | --- | --- | --- | --- |
| `validator.schema.org` web form | No public stable API; HTML POST works but is undocumented and rate-limited | Schema.org compliance | None | Brittle; rejected as the primary path |
| Google Rich Results Test API | Requires Search Console OAuth; per-property quotas | Rich-results eligibility for FAQPage / HowTo / BreadcrumbList / Article / Dataset | OAuth, per-domain quota | Heavy; deferred (see optional path below) |
| `@google-cloud/structured-data-testing-tool` | Unmaintained since 2021 | Wraps the deprecated Google testing-tool API | None | Rejected (deprecated upstream) |
| Custom Go-based linter | Stable (we own it) | Configurable; matches the doctrine exactly | None | **Selected** |

**Decision.** The primary validator is a **custom Go-based JSON-LD linter** in `internal/lint/jsonld/` (or similar) that:

1. Walks every prerendered HTML page in the build artefact.
2. Extracts every `<script type="application/ld+json">` block.
3. Asserts the JSON parses, the `@type` is one of the types named in `structured-data.md` § Master schema table for the page family, and every required-or-recommended field listed in § Required field-by-type expectations is present (or accompanied by the audit-trail HTML comment for an intentional omission).
4. Asserts the four reified entity nodes appear in full **only on `/`** and are referenced by `@id` on every other page.
5. Asserts `@id` URIs use the canonical absolute URL and that every internal `@id` reference resolves to a node emitted somewhere in the same prerendered tree.
6. Fails the build with a non-zero exit code and a per-page error summary on any defect.

This linter is the Critical-error gate. It runs in CI on every PR; pull requests with any reported defect are blocked from merging.

**Optional secondary path.** When the rich-results-eligibility signal is needed beyond the doctrine's structural rules (e.g. a release sweep before a public announcement), the Google Rich Results Test web form is consulted manually by the release-manager. Automating this through the API is tracked separately and is not part of the merge gate.

## CI host

GitHub Actions, running on `ubuntu-latest`, Go version pinned to the `go.mod` directive (currently 1.26).

The workflow lives at `.github/workflows/ci.yml`. The structured-data step depends on `make prerender` (or equivalent) so the linter sees the same byte-for-byte HTML the runtime would serve.

## Required steps (every pull request)

1. **Setup.** Checkout, install Go, restore the module cache.
2. **Vet & test.** `go vet ./...` followed by `go test ./...` with the race detector. Coverage threshold is not enforced today (TBD).
3. **Build.** `go build ./...` with the version label injected via `-ldflags`.
4. **Prerender.** Materialise every public route to bytes by running the binary in `--prerender-only` mode (or equivalent) and dumping the prerender map to a temporary directory.
5. **Structured-data validation.** Run the JSON-LD linter against the prerender directory. Exit non-zero on any reported defect.
6. **Artefact upload.** Attach the linter output (one report per page, plus a summary index) as a build artefact with a 30-day retention. Reviewers MUST be able to download it from any PR.

Steps 5 and 6 are the structured-data validation gate; failure of either MUST block merge.

## Warning-only quality gates

A second CI job runs after the test-and-validate job and reports — but does not block — additional quality signals:

1. **Lighthouse CI.** `@lhci/cli` runs against eight representative pages (landing, docs index, getting-started, routing, API, examples index, jwt example, benchmarks) with the desktop preset. Thresholds: performance ≥ 0.90, accessibility ≥ 0.95, best-practices ≥ 0.95, SEO ≥ 0.95. Reports are uploaded as a build artefact (30-day retention).
2. **pa11y.** WCAG 2.2 AA scan across the same set plus `/404`.

Both run with `continue-on-error: true` so a regression is visible (warning annotation on the PR) without blocking the merge. After one green week on `main`, both gates flip to blocking (the `continue-on-error` lines come off).

## Out of scope (today)

- HTML5 validity gate via `nu-validator` (planned; not blocking today).

These items live under `out-of-scope.md` until they are scheduled.
