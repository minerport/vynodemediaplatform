FROM node:24-alpine AS web
WORKDIR /src
COPY package.json package-lock.json ./
COPY apps/web/package.json apps/web/package.json
RUN --mount=type=secret,id=custom_ca,required=false \
    if [ -f /run/secrets/custom_ca ]; then \
      NODE_EXTRA_CA_CERTS=/run/secrets/custom_ca npm ci --workspace @vynode/web; \
    else \
      npm ci --workspace @vynode/web; \
    fi
COPY apps/web apps/web
COPY packages/design-tokens packages/design-tokens
RUN npm run build --workspace @vynode/web

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
RUN CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/vynode/media/server/internal/buildinfo.Version=${VERSION} -X github.com/vynode/media/server/internal/buildinfo.Commit=${COMMIT}" -o /vynode-server ./server/cmd/vynode-server && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o /tmp/vynode-server.exe ./server/cmd/vynode-server && mkdir -p /runtime/config /runtime/media /runtime/transcode /runtime/optimized /runtime/downloads && touch /runtime/config/.keep /runtime/media/.keep /runtime/transcode/.keep /runtime/optimized/.keep /runtime/downloads/.keep
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg ca-certificates && rm -rf /var/lib/apt/lists/* && groupadd -g 65532 vynode && useradd -u 65532 -g 65532 -M -s /usr/sbin/nologin vynode
COPY --from=build /vynode-server /vynode-server
COPY --from=web --chown=65532:65532 /src/apps/web/dist /web
COPY --from=build --chown=65532:65532 /runtime/config /config
COPY --from=build --chown=65532:65532 /runtime/media /media
COPY --from=build --chown=65532:65532 /runtime/transcode /transcode
COPY --from=build --chown=65532:65532 /runtime/optimized /optimized
COPY --from=build --chown=65532:65532 /runtime/downloads /downloads
ENV VYNODE_HTTP_ADDRESS=0.0.0.0:8096 VYNODE_CONFIG_DIR=/config VYNODE_DATABASE_TYPE=sqlite VYNODE_WEB_DIR=/web
ENV VYNODE_FFPROBE_PATH=/usr/bin/ffprobe VYNODE_FFMPEG_PATH=/usr/bin/ffmpeg VYNODE_TRANSCODE_DIR=/transcode VYNODE_PROBE_CONCURRENCY=2 VYNODE_PLAYBACK_PIPELINES=2
ENV VYNODE_OPTIMIZED_DIR=/optimized
ENV VYNODE_DOWNLOADS_DIR=/downloads
VOLUME ["/config", "/media", "/transcode", "/optimized", "/downloads"]
EXPOSE 8096
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/vynode-server", "healthcheck"]
USER 65532:65532
ENTRYPOINT ["/vynode-server"]
