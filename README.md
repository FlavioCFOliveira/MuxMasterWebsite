# MuxMaster website

Official documentation site for the [MuxMaster](https://github.com/FlavioCFOliveira/MuxMaster) Go HTTP router. The site itself is served by MuxMaster — it documents the module and acts as a real-world reference implementation.

## Run locally

```sh
make tailwind-install   # one-off: downloads the Tailwind v4 standalone CLI into ./bin/
make dev                # builds the CSS once, then runs the binary with `go run`
```

The site listens on `:8080` by default. Override via `PORT`.

Required environment:

| Variable | Default | Purpose |
| --- | --- | --- |
| `MUXMASTER_SOURCE_DIR` | `../MuxMaster` | Upstream MuxMaster source tree (must satisfy `specification/content-sources.md`). |
| `SITE_BASE_URL` | `http://localhost:8080` | Absolute base URL used for canonical, OG, JSON-LD, and sitemap. |
| `PORT` | `8080` | TCP port. |
| `LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, `error`. |
| `ENV` | `development` | One of `development`, `staging`, `production`. |

## Source of truth

The contract this code satisfies lives in `specification/`. Read it before editing.

This scaffold round delivers a runnable binary with the landing page wired and every other route registered as a "coming soon" placeholder. The following routes are stubs that will be filled in by later rounds (geo / seo / content):

- `/llms.txt`, `/llms-full.txt`, `/sitemap.xml`, `/robots.txt` — minimal valid bodies, not yet auto-generated from the route table.
- `/docs/*`, `/api`, `/examples/*`, `/benchmarks`, `/changelog`, `/releases/*`, `/security`, `/compatibility`, `/contributing` — render the "coming soon" template with correct `<head>` and breadcrumb.
- `static/favicon/` is reserved; the build-time favicon generation pipeline is not yet implemented.

## Build the binary

```sh
make build              # produces ./bin/muxmaster-website + hashed CSS bundle
make test               # `go test -race ./...`
make vet                # `go vet ./...`
```

## Docker

The Dockerfile is multi-stage. The MuxMaster source tree must be supplied at build time. Recommended invocation (run from inside this repository):

```sh
docker build \
  --build-context muxmaster=../MuxMaster \
  -t muxmaster-website:dev .
```

The runtime stage is `gcr.io/distroless/static-debian12:nonroot`, listens on `:8080` (h2c), and reads the upstream source tree from `MUXMASTER_SOURCE_DIR=/muxmaster`.
