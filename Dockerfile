# syntax=docker/dockerfile:1

# Build a static binary.
FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X github.com/gsoultan/proteus-server/internal/rpc.Version=${VERSION}" \
    -o /out/proteus-server ./cmd/proteus-server

# Collect fonts so output is identical on every host.
FROM debian:bookworm-slim AS fonts
RUN apt-get update \
 && apt-get install -y --no-install-recommends fonts-liberation2 fonts-dejavu-core \
 && rm -rf /var/lib/apt/lists/*

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=fonts /usr/share/fonts /usr/share/fonts
COPY --from=build /out/proteus-server /proteus-server
ENV PROTEUS_ADDR=:8080 \
    PROTEUS_ADMIN_ADDR=:9090 \
    PROTEUS_SYSTEM_FONTS=true
EXPOSE 8080 9090
USER nonroot:nonroot
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s CMD ["/proteus-server", "healthcheck"]
ENTRYPOINT ["/proteus-server"]
