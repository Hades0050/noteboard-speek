GO ?= go
BIN ?= bin/noteboard
PKG := ./cmd/noteboard
DIST := internal/webui/dist

.PHONY: all web server run dev-api dev-web proto lint test clean

all: web server

web:
	cd web && bun install && bun run build
	rm -rf $(DIST)
	mkdir -p $(DIST)
	cp -r web/.output/public/. $(DIST)/

server:
	$(GO) build -o $(BIN) $(PKG)

run: all
	./$(BIN)

# dev: два терминала — Go на :8080, Nuxt на :3000 с проксёй /api
dev-api:
	$(GO) run $(PKG)

dev-web:
	cd web && bun run dev

# Кодогенерация из api/proto: Go-сервер и TypeScript-клиент за один проход.
# Нужны buf и плагины: make tools.
proto:
	buf lint
	buf generate

tools:
	$(GO) install github.com/bufbuild/buf/cmd/buf@latest
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

lint:
	$(GO) vet ./...
	gofmt -l . | grep -v '^internal/gen/' || true

test:
	$(GO) test ./...

clean:
	rm -rf bin web/.output web/.nuxt
	rm -rf $(DIST) && mkdir -p $(DIST) && printf '<!doctype html><title>noteboard</title><p>Frontend not built. Run: make web</p>\n' > $(DIST)/index.html
