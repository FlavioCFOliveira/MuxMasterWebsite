# syntax=docker/dockerfile:1.7
#
# Multi-stage build for the MuxMaster website.
#
# Recommended invocation (from this repository's root):
#   docker build --build-context muxmaster=../MuxMaster -t muxmaster-website:dev .
#
# The `muxmaster` build context provides the upstream MuxMaster source tree,
# which is both a build-time dependency (for `go build`, via `replace`) and a
# runtime dependency (per `specification/content-sources.md`).

FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates curl make bash git

WORKDIR /workspace

# Pull in the upstream MuxMaster source first so the Go module replace target
# resolves during `go mod download`.
COPY --from=muxmaster . /muxmaster
COPY go.mod go.sum* ./
RUN go mod download || true

# Project sources.
COPY . .

# Replace the dev-time relative path with the absolute path inside the build
# image. The substitution is a no-op when go.sum already pins the version.
RUN sed -i 's|replace github.com/FlavioCFOliveira/MuxMaster => ../MuxMaster|replace github.com/FlavioCFOliveira/MuxMaster => /muxmaster|' go.mod

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
COPY --from=builder /workspace/content      /srv/content

# Bake the upstream MuxMaster source tree at a stable path. Per
# specification/deployment.md, the alternative is a runtime volume mount;
# operators can override MUXMASTER_SOURCE_DIR to point at a mount.
COPY --from=muxmaster . /muxmaster

ENV MUXMASTER_SOURCE_DIR=/muxmaster \
    PORT=8080 \
    LOG_LEVEL=info \
    ENV=production

EXPOSE 8080

ENTRYPOINT ["/srv/muxmaster-website"]
