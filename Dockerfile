ARG GO_VERSION=1.25

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=2.0.0-dev

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -trimpath \
    -ldflags="-s -w -X github.com/rest-sh/restish/v2/internal/cli.Version=${VERSION}" \
    -o /out/restish \
    ./cmd/restish

RUN mkdir -p /out/home /out/config/restish /out/cache/restish

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/restish /usr/local/bin/restish
COPY --from=build --chown=65532:65532 /out/home /home/nonroot
COPY --from=build --chown=65532:65532 /out/config /config
COPY --from=build --chown=65532:65532 /out/cache /cache

ENV HOME=/home/nonroot
ENV XDG_CONFIG_HOME=/config
ENV XDG_CACHE_HOME=/cache

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/restish"]
