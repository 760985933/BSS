# syntax=docker/dockerfile:1

# ---------- 1) 前端构建（pnpm） ----------
FROM node:22-alpine AS web
WORKDIR /web
RUN corepack enable && corepack prepare pnpm@10.4.1 --activate
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# ---------- 2) 后端构建（纯 Go，无 CGO） ----------
FROM golang:1.25-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bss-server ./cmd/server

# ---------- 3) 运行镜像 ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 bss
WORKDIR /app
COPY --from=server /out/bss-server /app/bss-server
ENV BSS_ADDR=0.0.0.0:8080 \
    BSS_DATA=/data \
    BSS_JWT_SECRET=""
VOLUME ["/data"]
EXPOSE 8080
USER bss
ENTRYPOINT ["/app/bss-server"]
