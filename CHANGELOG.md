# Changelog

All notable changes to the MuxMaster documentation website are recorded in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html). The website's release tag mirrors the MuxMaster release it documents; see `specification/overview.md § Version cadence` for the cadence policy.

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

[v1.0.1]: https://github.com/FlavioCFOliveira/MuxMasterWebsite/releases/tag/v1.0.1
