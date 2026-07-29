NODE := /Users/tianfeng/.workbuddy/binaries/node/versions/22.22.2/bin/node
NPM := /Users/tianfeng/.workbuddy/binaries/node/versions/22.22.2/bin/npm

.PHONY: dev dev-server dev-web build build-web build-server test run clean

## 开发：分别启动后端(8080)与前端 vite(5173，代理 /api)
dev: dev-server

dev-server:
	BSS_ADDR=127.0.0.1:8080 BSS_DATA=./data go run ./cmd/server

dev-web:
	cd web && $(NPM) run dev

## 构建：前端 dist → Go embed → 单二进制 bss-server
build: build-web build-server

build-web:
	cd web && $(NPM) run build

build-server:
	go build -o bss-server ./cmd/server

## 测试
test:
	go test ./...

## 运行产物
run: build
	BSS_ADDR=127.0.0.1:8080 BSS_DATA=./data ./bss-server

clean:
	rm -rf web/dist bss-server
