VERSION ?= dev

.PHONY: build wasm test clean
build:
	go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o collage ./cmd/collage

wasm:
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o web/engine.wasm ./cmd/wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" web/

test:
	go vet ./...
	go test ./...

clean:
	rm -f collage
