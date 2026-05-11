# syntax=docker/dockerfile:1.7
#
# Multi-stage build for the MuxMaster website.
#
# Recommended invocation (from this repository's root):
#   docker build -t muxmaster-website:dev .
#
# The runtime binary is self-contained: every public route is pre-rendered
# at startup from the /content/ tree embedded in the binary. The image does
# not require the upstream MuxMaster source tree at build time or at
# runtime — the curated copy under /content/ is committed to this
# repository (specification/deployment.md, specification/content-sources.md).

# Builder runs on Debian-based golang:1.26 (glibc) rather than the alpine
# variant. The Tailwind v4 standalone binary published at
# github.com/tailwindlabs/tailwindcss/releases is linked against glibc;
# downloading it inside an alpine (musl) image yields a binary that
# silently fails to execute. The Debian-based image also ships make,
# curl, bash, and git out of the box. The runtime stage below is still
# distroless static-debian12, so the production image size is unaffected.
FROM golang:1.26 AS builder

WORKDIR /workspace

COPY go.mod go.sum* ./
RUN go mod download || true

# Project sources, including /content/ embedded by go:embed at build time.
COPY . .

# Build the Tailwind bundle, the derived image assets, and the binary.
# `make assets` builds the in-tree tools/imagegen helper (Go-only, no
# CGO, no system image libraries) and derives the sized logos,
# favicons, apple-touch-icons, and the 1200x630 Open Graph composition
# from static/img/logo.png. These derived files are gitignored, so the
# only way they can land in the runtime stage is via this RUN step;
# otherwise every <link>/<img> the templates emit for them resolves to
# a 404 at request time.
RUN make tailwind-install && make css && make assets
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w -X main.buildID=$(date -u +%Y%m%dT%H%M%SZ)" \
        -o /out/muxmaster-website ./cmd/muxmaster-website

# Grant the binary the minimum capability needed to bind to a privileged
# port (< 1024). The runtime image runs as the nonroot UID 65532, which by
# default cannot call `bind(2)` on port 80; `setcap cap_net_bind_service=ep`
# attaches a file capability that authorises this single operation and
# nothing else. The capability is preserved across BuildKit's COPY into
# the distroless stage below; the smoke step on every release verifies
# this by booting the image and exercising /healthz on the configured port.
RUN apt-get update \
 && apt-get install -y --no-install-recommends libcap2-bin \
 && rm -rf /var/lib/apt/lists/* \
 && setcap 'cap_net_bind_service=+ep' /out/muxmaster-website \
 && getcap /out/muxmaster-website

# Runtime stage: distroless static, non-root.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /srv

COPY --from=builder /out/muxmaster-website /srv/muxmaster-website
COPY --from=builder /workspace/static       /srv/static
COPY --from=builder /workspace/templates    /srv/templates

ENV PORT=80 \
    LOG_LEVEL=info \
    ENV=production \
    SITE_BASE_URL=https://muxmaster.net

EXPOSE 80

# The distroless runtime has neither curl nor wget; the binary itself
# does the HTTP self-GET via --healthcheck. interval/timeout/start-period
# are set so a hung process is detected within ~90s of failure while
# allowing a generous 5s window for startup prerender.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/srv/muxmaster-website", "--healthcheck"]

ENTRYPOINT ["/srv/muxmaster-website"]
