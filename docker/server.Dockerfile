FROM golang:1.25.0-alpine AS builder

ARG GOPROXY=https://proxy.golang.org,direct
WORKDIR /src/server
ENV GODEBUG=http2client=0 \
    GOPROXY=${GOPROXY}
COPY server/go.mod server/go.sum ./
RUN --mount=type=cache,id=gim-go-mod,target=/go/pkg/mod \
    for attempt in 1 2 3 4 5; do go mod download && break; test "$attempt" = 5 && exit 1; sleep 3; done
COPY server/ ./

RUN --mount=type=cache,id=gim-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=gim-go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-admin-bootstrap ./cmd/admin-bootstrap && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-gateway ./cmd/gateway && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-user-rpc ./cmd/user-rpc && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-group-rpc ./cmd/group-rpc && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-user-api ./cmd/user-api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-auth-api ./cmd/auth-api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-chat-api ./cmd/chat-api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-group-api ./cmd/group-api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-file-api ./cmd/file-api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-settings-api ./cmd/settings-api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gim-logs-api ./cmd/logs-api

FROM alpine:3.22

RUN addgroup -g 65532 -S gim && adduser -u 65532 -S -D -H -G gim gim
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/ /app/
USER 65532:65532

ENTRYPOINT []
