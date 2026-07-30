NODE := /Users/tianfeng/.workbuddy/binaries/node/versions/22.22.2/bin/node
NPM := /Users/tianfeng/.workbuddy/binaries/node/versions/22.22.2/bin/npm

.PHONY: dev dev-server dev-web build build-web build-server test test-web test-all lint run clean

## 开发：分别启动后端(8080)与前端 vite(5173，代理 /api)
dev: dev-server

dev-server:
	BSS_ADDR=127.0.0.1:8080 BSS_DATA=./data go run ./cmd/server

dev-web:
	cd web && $(NPM) run dev

## 构建：前端 dist → Go embed → 单二进制 bss-server
build: build-web build-server

build-web:
	rm -rf web/dist && mkdir -p web/dist && touch web/dist/.gitkeep && cd web && $(NPM) run build

build-server:
	go build -o bss-server ./cmd/server

## 测试
test:
	go test ./...

test-web:
	cd web && pnpm test

test-all: test test-web

## 静态检查（需先安装 golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0）
## 重点启用 errcheck / errorlint，用于拦截「吞掉 error」类回归（如 .Count().Error 未处理）
lint:
	golangci-lint run ./...

## 运行产物
run: build
	BSS_ADDR=127.0.0.1:8080 BSS_DATA=./data ./bss-server

clean:
	rm -rf web/dist bss-server
