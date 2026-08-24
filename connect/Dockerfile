FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=secret,id=custom_ca,required=false if [ -f /run/secrets/custom_ca ]; then cp /run/secrets/custom_ca /usr/local/share/ca-certificates/vynode-build-ca.crt && update-ca-certificates; fi && go mod download
COPY connect ./connect
RUN CGO_ENABLED=0 go test -timeout=60s ./connect/... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /vynode-connect ./connect/cmd/vynode-connect

FROM alpine:3.22
RUN addgroup -g 65532 vynode && adduser -D -H -u 65532 -G vynode vynode && mkdir -p /data && chown 65532:65532 /data
COPY --from=build /vynode-connect /vynode-connect
ENV VYNODE_CONNECT_LISTEN=:8090 VYNODE_CONNECT_DATA_DIR=/data VYNODE_CONNECT_ISSUER=https://connect.example.invalid
VOLUME ["/data"]
EXPOSE 8090
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/vynode-connect", "healthcheck"]
USER 65532:65532
ENTRYPOINT ["/vynode-connect"]
