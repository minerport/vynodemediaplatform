FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=secret,id=custom_ca,required=false \
    if [ -f /run/secrets/custom_ca ]; then \
      cp /run/secrets/custom_ca /usr/local/share/ca-certificates/vynode-build-ca.crt && update-ca-certificates; \
    fi && go mod download
COPY server ./server
ARG VERSION=0.1.0
ARG COMMIT=development
RUN CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/vynode/media/server/internal/buildinfo.Version=${VERSION} -X github.com/vynode/media/server/internal/buildinfo.Commit=${COMMIT}" -o /vynode-server ./server/cmd/vynode-server && mkdir -p /runtime/config /runtime/media /runtime/transcode && touch /runtime/config/.keep /runtime/media/.keep /runtime/transcode/.keep
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /vynode-server /vynode-server
COPY --from=build --chown=65532:65532 /runtime/config /config
COPY --from=build --chown=65532:65532 /runtime/media /media
COPY --from=build --chown=65532:65532 /runtime/transcode /transcode
ENV VYNODE_HTTP_ADDRESS=0.0.0.0:8096 VYNODE_CONFIG_DIR=/config VYNODE_DATABASE_TYPE=sqlite
VOLUME ["/config", "/media", "/transcode"]
EXPOSE 8096
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/vynode-server", "healthcheck"]
ENTRYPOINT ["/vynode-server"]
