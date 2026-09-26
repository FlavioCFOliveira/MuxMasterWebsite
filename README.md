# MuxMaster website

Official documentation site for the [MuxMaster](https://github.com/FlavioCFOliveira/MuxMaster) Go HTTP router. The site itself is served by MuxMaster — it documents the module and acts as a real-world reference implementation.

## Run locally

```sh
make tailwind-install   # one-off: downloads the Tailwind v4 standalone CLI into ./bin/
make dev                # builds the CSS once, then runs the binary with `go run`
```

The site listens on `:8080` by default when run from source. Override via `PORT`.

Runtime environment variables resolve in three layers — binary default (compiled in), image default (baked into the official Docker image), runtime override (always wins):

| Variable | Binary default | Image default | Purpose |
| --- | --- | --- | --- |
| `SITE_BASE_URL` | `http://localhost:8080` | `https://muxmaster.net` | Absolute base URL used for canonical, OG, JSON-LD, sitemap, and `llms.txt`. |
| `PORT` | `8080` | `80` | TCP port. |
| `LOG_LEVEL` | `info` | `info` | One of `debug`, `info`, `warn`, `error`. |
| `ENV` | `development` | `production` | One of `development`, `staging`, `production`. |

The binary defaults serve local development (`go run`, `make dev`). The image defaults serve the canonical production deployment at `https://muxmaster.net`. Override any value at container runtime (`docker run -e SITE_BASE_URL=https://staging.example …`) for staging or preview environments; when overriding `SITE_BASE_URL` away from production, also set `ENV=staging` or `ENV=development` so the site emits `noindex`.

`MUXMASTER_SOURCE_DIR` is **not** a runtime variable. It is a development-time and agent-time variable consumed only by the `content-curator` agent to locate the upstream working tree. The runtime binary is self-contained: every public route is pre-rendered at startup from `/content/`, embedded into the binary at build time.

## Source of truth

The contract this code satisfies lives in `specification/`. Read it before editing.

## Build the binary

```sh
make build              # produces ./bin/muxmaster-website + hashed CSS bundle
make test               # `go test -race ./...`
make vet                # `go vet ./...`
make bench              # request-path benchmarks: -benchmem -count=6
```

`make bench` drives each request type through the complete handler (the `Pre` middleware chain, the MuxMaster router, and the handler) with the production logger configuration. The numbers on `/built-with-muxmaster` come from this target.

## How the server uses MuxMaster

- `Mux.PoolRequestBundle` and `Mux.PoolFastParams` are on. Every handler must honour the lifetime contract: never keep the request, its context, its body, or `Params` after returning, and never start a goroutine that reads them.
- Cross-cutting middleware is registered with `Mux.Pre` only. Never call `Mux.Use`: MuxMaster panics when a `HandleFast` route is registered after it.
- Pre-rendered routes and static files are served from `render.Response` values computed once at startup (identity and gzip bodies, ETags, header values). No response is compressed or formatted per request.
- `/static/*filepath` is a `FastHandler` registered with `GETFast` and `HEADFast`, served from memory.

The normative rules are in `specification/rendering-and-caching.md`.

## Docker

The Dockerfile is multi-stage and self-contained — `/content/` is committed in this repository and embedded into the binary at build time, so no external build context is required:

```sh
docker build -t muxmaster-website:dev .
```

The runtime stage is `gcr.io/distroless/static-debian12:nonroot` and listens on `:80` (h2c). The image bakes `SITE_BASE_URL=https://muxmaster.net` and `ENV=production`; override either at `docker run` time for staging or preview deployments.
