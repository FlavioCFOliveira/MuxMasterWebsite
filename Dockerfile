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

FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates curl make bash git

WORKDIR /workspace

COPY go.mod go.sum* ./
RUN go mod download || true

# Project sources, including /content/ embedded by go:embed at build time.
COPY . .

# Build the Tailwind bundle and the binary.
RUN make tailwind-install && make css
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w -X main.buildID=$(date -u +%Y%m%dT%H%M%SZ)" \
        -o /out/muxmaster-website ./cmd/muxmaster-website

# Runtime stage: distroless static, non-root.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /srv

COPY --from=builder /out/muxmaster-website /srv/muxmaster-website
COPY --from=builder /workspace/static       /srv/static
COPY --from=builder /workspace/templates    /srv/templates

ENV PORT=8080 \
    LOG_LEVEL=info \
    ENV=production

EXPOSE 8080

# The distroless runtime has neither curl nor wget; the binary itself
# does the HTTP self-GET via --healthcheck. interval/timeout/start-period
# are set so a hung process is detected within ~90s of failure while
# allowing a generous 5s window for startup prerender.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/srv/muxmaster-website", "--healthcheck"]

ENTRYPOINT ["/srv/muxmaster-website"]
