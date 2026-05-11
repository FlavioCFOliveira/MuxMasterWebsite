# Changelog

All notable changes to the MuxMaster documentation website are recorded in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html). The website's `MAJOR.MINOR` mirrors the MuxMaster release it documents; the website's `PATCH` digit is independent and advances for website-only operational fixes. See `specification/overview.md § Version cadence` for the full cadence policy.

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
