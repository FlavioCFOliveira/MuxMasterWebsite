---
title: Deployment
purpose: Define the Docker model, runtime contract, environment variables, reverse-proxy expectations, health endpoint, log shape, and the production launch gate.
owners: specification-manager; review by seo-specialist (HTTPS/HTTP/2/security headers), tailwind-specialist (CSS bundle delivery).
last-updated: 2026-05-11
status: ratified
---

# Deployment

## Container model

The site is shipped as a Docker image built with a multi-stage Dockerfile.

### Builder stage

- Base image: an official Go toolchain image matching MuxMaster's minimum (Go 1.26 or newer).
- Installs the **Tailwind CSS v4 standalone CLI binary** (downloaded for the target architecture).
- Compiles the CSS bundle from the project's templates and source files (see `brand-and-visual.md`).
- Generates favicons and the Open Graph image from the canonical logo PNG bundled with the repository (sourced from the upstream MuxMaster repository at `assets/logo-muxmaster.png` — see `brand-and-visual.md`).
- Compiles the Go binary as a static, position-independent executable (`CGO_ENABLED=0`). The output binary is named `muxmaster-website`.
- The builder stage does **not** require the upstream `../MuxMaster` tree at build time. Content under `/content/` is committed to this repository by the `content-curator` agent (see `content-sources.md`); it is part of the repository's source, not a build-time external dependency.

### Runtime stage

- Base image: distroless or scratch (preferred: `gcr.io/distroless/static-debian12:nonroot` or equivalent).
- Contains:
  - The compiled site binary (named `muxmaster-website`).
  - The compiled static assets (CSS bundle, favicons, OG image, hashed logo derivatives).
  - The full `/content/` tree as committed in this repository (mirrored upstream content plus site-original content).
- Runs as a non-root user (the distroless image's UID 65532).
- Exposes port `80` (cleartext HTTP/1.1 and h2c HTTP/2). Binding to a privileged port from a non-root user is permitted by attaching the file capability `cap_net_bind_service=ep` to the binary in the builder stage; the capability is preserved across the BuildKit `COPY --from=builder` into the distroless runtime stage. No other capability is granted, and the runtime container has no shell, no package manager, and no writable filesystem beyond the kernel-mandated minimum.
- The runtime image is **self-contained**. It does not require any external filesystem mount, the upstream `../MuxMaster` tree, or any other directory beyond what the builder stage produced. Every public route is pre-rendered at startup from `/content/` (see `rendering-and-caching.md`).

### Content delivery

Content is part of the repository under `/content/` and is therefore embedded in the runtime image automatically when the builder stage runs from a clean checkout. There is no runtime upstream-tree mount, no read-only volume, and no separate content rollout path. A new MuxMaster release reaches production through the following sequence:

1. The `content-curator` agent runs at development time against `${MUXMASTER_SOURCE_DIR}` and proposes a diff under `/content/` (see `content-sources.md`).
2. The diff is reviewed and committed to this repository.
3. A new image is built; the new content is baked in.
4. The image is rolled out; the running process is restarted.

`MUXMASTER_SOURCE_DIR` is therefore a **development-time and agent-time** variable only, used by the curator agent to locate the upstream working tree. It MUST NOT appear in the runtime image or in the production environment.

## Runtime environment variables

| Variable | Purpose | Default | Required |
| --- | --- | --- | --- |
| `SITE_BASE_URL` | Absolute base URL of the site, used for canonical, OG, JSON-LD, sitemap, and llms.txt. In production this MUST be set to `https://muxmaster.net` (the canonical domain ratified on 2026-05-11). | `http://localhost:8080` | Yes |
| `LOG_LEVEL` | One of `debug`, `info`, `warn`, `error`. | `info` | No |
| `PORT` | TCP port to bind. The compiled binary defaults to `8080` for local development (`go run`, `make dev`); the production Docker image overrides this to `80` via the `ENV PORT=80` directive in the runtime stage. Either value can be replaced by setting `PORT` at runtime. | `8080` binary; `80` Docker image | No |
| `ENV` | One of `development`, `staging`, `production`. Controls whether `noindex` is forced when `SITE_BASE_URL` is not the canonical production domain (`https://muxmaster.net`). | `development` | No |

`MUXMASTER_SOURCE_DIR` is **not** a runtime variable. It is consumed only by the `content-curator` agent at development time (see `content-sources.md`).

The server MUST log all environment-variable values at startup (`info` level) except for any future secret. None are secret on day one.

## Reverse-proxy expectations

The container is intended to run behind a reverse proxy (nginx, Caddy, Traefik, or a cloud load balancer). The proxy is responsible for:

- TLS termination (HTTPS-only in production).
- Setting and forwarding `X-Forwarded-Proto: https`, `X-Forwarded-Host`, `X-Forwarded-For`.
- HTTP/2 (and HTTP/3 where the proxy supports it) on the public side. The container speaks h2c on its loopback or container network.
- **Brotli compression** on the public side. The container MAY emit `Content-Encoding: gzip` directly when the proxy is not configured for Brotli.
- HSTS preload (the container also emits `Strict-Transport-Security`, see `seo.md`; the proxy MAY override).
- A `301 Moved Permanently` redirect from `https://www.muxmaster.net` (and from `http://www.muxmaster.net` and `http://muxmaster.net`) to the apex `https://muxmaster.net`. The apex is the single canonical origin; the `www` host MUST NOT serve content directly. This redirect is implemented at the reverse-proxy layer and is not visible to the container.

The container MUST trust the proxy's `X-Forwarded-*` headers only when configured to do so. The trust is opt-in via the `TRUSTED_PROXY_CIDRS` environment variable (comma-separated list of CIDR prefixes). Empty (or unset) means no proxy is trusted and `r.RemoteAddr` is used verbatim — the safe default. In production deployments behind a reverse proxy, the operator MUST set `TRUSTED_PROXY_CIDRS` to the edge-proxy network(s); the binary then runs `mwm.RealIP(prefixes...)` before the access logger so a forged `X-Forwarded-For` from a non-trusted peer is ignored. In development the variable stays unset.

## Health endpoint

- `GET /healthz` MUST return `200 OK` with body `ok\n` and `Content-Type: text/plain; charset=utf-8` once the server has finished startup checks (required files under `/content/` present, templates parsed, CSS bundle resolved, version label read, every public route pre-rendered to bytes).
- The HTTP listener is **bound only after** startup checks complete. Any probe that arrives during the not-ready window sees a TCP connection refusal, which every container orchestrator (Kubernetes, Nomad, Docker Compose, systemd) already interprets as a not-ready signal and retries. A 503 response is therefore neither emitted nor required: by the time `/healthz` can answer at all, it answers `200 OK`.
- `Cache-Control: no-store` on this route.
- The route is excluded from `sitemap.xml`, `robots.txt`, `llms.txt`, and `llms-full.txt`.
- A separate **readiness** endpoint is not provided; `/healthz` doubles as readiness. A liveness/readiness split is a candidate for a later revision.

## Logs

- Format: JSON via `log/slog` (Go standard library). Output: stdout.
- Required fields per request log: `time` (RFC 3339), `level`, `msg`, `method`, `path`, `status`, `bytes`, `duration_ms`, `remote_addr` (post-proxy resolution), `user_agent`, `referer`, `route_id` (the matched route pattern).
- `bytes` is the **post-compression** body size — the value on the wire. The access logger wraps the response writer in the outermost middleware position; the compression middleware runs further down the Pre chain, so by the time bytes are counted they have already passed through gzip when the client advertised `Accept-Encoding: gzip`. Bandwidth accounting and CDN tuning therefore work on the value as-is; identity responses log their raw size.
- Errors include `err` and a stable `error_kind`.
- Logs MUST NOT include request bodies or response bodies.
- One log line per completed request. Startup, shutdown, and configuration logs are emitted at `info`.

## Graceful shutdown

- On `SIGTERM` or `SIGINT`, the server MUST stop accepting new connections, finish in-flight requests within a 30-second budget, then exit `0`.
- If the budget is exceeded, the server logs a warning and exits `0` regardless.

## Production launch gate

The canonical production domain was ratified on 2026-05-11 as `https://muxmaster.net` (see `open-questions.md` item 1, RESOLVED). The specification-level blocker on public launch is therefore closed; the remaining gate items below are an **implementation rollout** step, not a specification blocker.

- In any environment that is not the production deployment on `https://muxmaster.net`, `ENV` MUST be `staging` or `development` and the rendered HTML MUST include `<meta name="robots" content="noindex,nofollow">`.
- The `sitemap.xml` MUST be empty (no `<url>` entries) when `ENV` is not `production`.
- The transition from staging to production MUST update the following artefacts **atomically** in the same deploy (no partial-update state in which some artefacts already point at `https://muxmaster.net` while others still point at the staging origin): `<link rel=canonical>`, `og:url`, `og:image` absolute URLs, `sitemap.xml`, `llms.txt`, `llms-full.txt`, JSON-LD `@id` URIs.
- DNS for `muxmaster.net` and `www.muxmaster.net` MUST resolve to the production deployment before `ENV=production` is flipped, and the `www` host MUST already serve the `301` redirect described under `## Reverse-proxy expectations`.
- HSTS preload submission (https://hstspreload.org) is recommended once production has served `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload` for at least one continuous week from `https://muxmaster.net`.
