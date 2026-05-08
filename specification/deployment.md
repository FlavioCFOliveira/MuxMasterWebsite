---
title: Deployment
purpose: Define the Docker model, runtime contract, environment variables, reverse-proxy expectations, health endpoint, log shape, and the production launch gate.
owners: specification-manager; review by seo-specialist (HTTPS/HTTP/2/security headers), tailwind-specialist (CSS bundle delivery).
last-updated: 2026-05-08
status: ratified (production launch blocked on canonical-domain TBD)
---

# Deployment

## Container model

The site is shipped as a Docker image built with a multi-stage Dockerfile.

### Builder stage

- Base image: an official Go toolchain image matching MuxMaster's minimum (Go 1.26 or newer).
- Installs the **Tailwind CSS v4 standalone CLI binary** (downloaded for the target architecture).
- Compiles the CSS bundle from the project's templates and source files (see `brand-and-visual.md`).
- Generates favicons and the Open Graph image from `${MUXMASTER_SOURCE_DIR}/assets/logo-muxmaster.png`.
- Compiles the Go binary as a static, position-independent executable (`CGO_ENABLED=0`). The output binary is named `muxmaster-website`.

### Runtime stage

- Base image: distroless or scratch (preferred: `gcr.io/distroless/static-debian12:nonroot` or equivalent).
- Contains:
  - The compiled site binary (named `muxmaster-website`).
  - The compiled static assets (CSS bundle, favicons, OG image, hashed logo derivatives).
  - The site-original content tree (`/content/site/`).
  - The upstream MuxMaster source tree (copied from a known commit), or the directory is mounted at runtime.
- Runs as a non-root user.
- Exposes port `8080` (cleartext HTTP/1.1 and h2c HTTP/2).

### Upstream tree delivery

Two acceptable patterns; the deployment chooses one and the chosen pattern MUST be documented in the deploy runbook:

1. **Baked-in.** The build copies `${MUXMASTER_SOURCE_DIR}` into the runtime image at a fixed path (e.g. `/srv/muxmaster`). The image is rebuilt for each MuxMaster release that the site adopts.
2. **Volume-mounted.** The runtime mounts a read-only volume containing the upstream tree at `${MUXMASTER_SOURCE_DIR}`. The volume is updated independently of image rollouts.

In both patterns, the directory MUST satisfy the contract in `content-sources.md`.

## Runtime environment variables

| Variable | Purpose | Default | Required |
| --- | --- | --- | --- |
| `MUXMASTER_SOURCE_DIR` | Path to the upstream MuxMaster source tree. | `../MuxMaster` (development); the absolute path baked into or mounted in the runtime image (production). | Yes |
| `SITE_BASE_URL` | Absolute base URL of the site, used for canonical, OG, JSON-LD, sitemap, and llms.txt. | `http://localhost:8080` | Yes |
| `LOG_LEVEL` | One of `debug`, `info`, `warn`, `error`. | `info` | No |
| `PORT` | TCP port to bind. | `8080` | No |
| `ENV` | One of `development`, `staging`, `production`. Controls whether `noindex` is forced when `SITE_BASE_URL` is not the canonical domain. | `development` | No |

The server MUST log all environment-variable values at startup (`info` level) except for any future secret. None are secret on day one.

## Reverse-proxy expectations

The container is intended to run behind a reverse proxy (nginx, Caddy, Traefik, or a cloud load balancer). The proxy is responsible for:

- TLS termination (HTTPS-only in production).
- Setting and forwarding `X-Forwarded-Proto: https`, `X-Forwarded-Host`, `X-Forwarded-For`.
- HTTP/2 (and HTTP/3 where the proxy supports it) on the public side. The container speaks h2c on its loopback or container network.
- **Brotli compression** on the public side. The container MAY emit `Content-Encoding: gzip` directly when the proxy is not configured for Brotli.
- HSTS preload (the container also emits `Strict-Transport-Security`, see `seo.md`; the proxy MAY override).

The container MUST trust the proxy's `X-Forwarded-*` headers only when configured to do so. The default in production MUST be that they are trusted; the development default MUST be that they are not.

## Health endpoint

- `GET /healthz` MUST return `200 OK` with body `ok\n` and `Content-Type: text/plain; charset=utf-8` once the server has finished startup checks (required upstream files present, templates parsed, CSS bundle resolved, version label read).
- Before startup checks complete, `GET /healthz` MUST return `503 Service Unavailable`.
- `Cache-Control: no-store` on this route.
- The route is excluded from `sitemap.xml`, `robots.txt`, `llms.txt`, and `llms-full.txt`.
- A separate **readiness** endpoint is not provided; `/healthz` doubles as readiness. A liveness/readiness split is a candidate for a later revision.

## Logs

- Format: JSON via `log/slog` (Go standard library). Output: stdout.
- Required fields per request log: `time` (RFC 3339), `level`, `msg`, `method`, `path`, `status`, `bytes`, `duration_ms`, `remote_addr` (post-proxy resolution), `user_agent`, `referer`, `route_id` (the matched route pattern).
- Errors include `err` and a stable `error_kind`.
- Logs MUST NOT include request bodies or response bodies.
- One log line per completed request. Startup, shutdown, and configuration logs are emitted at `info`.

## Graceful shutdown

- On `SIGTERM` or `SIGINT`, the server MUST stop accepting new connections, finish in-flight requests within a 30-second budget, then exit `0`.
- If the budget is exceeded, the server logs a warning and exits `0` regardless.

## Production launch gate

- The site MUST NOT be served to the public until the canonical domain is decided (`open-questions.md` item 1).
- Until then, all environments MUST set `ENV=staging` (or `development`) and the rendered HTML MUST include `<meta name="robots" content="noindex,nofollow">`.
- The `sitemap.xml` MUST be empty (no `<url>` entries) when `ENV` is not `production`.
- Once the canonical domain is decided, the following MUST be updated atomically: `<link rel=canonical>`, `og:url`, `og:image` absolute URLs, `sitemap.xml`, `llms.txt`, `llms-full.txt`, JSON-LD `@id` URIs.
