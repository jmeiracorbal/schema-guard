# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/schema-guard ./cmd/schema-guard

FROM scratch
LABEL org.opencontainers.image.source="https://github.com/jmeiracorbal/schema-guard" \
      org.opencontainers.image.description="Read-only multi-engine database schema drift detection" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/schema-guard /schema-guard
USER 65532:65532
WORKDIR /workspace
ENTRYPOINT ["/schema-guard"]
