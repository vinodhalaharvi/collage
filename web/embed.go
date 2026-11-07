// Package web holds the browser UI. engine.wasm is built from ./cmd/wasm
// (run `make wasm`) and committed so `go build` alone produces a full binary.
package web

import "embed"

//go:embed index.html engine.wasm wasm_exec.js
var FS embed.FS
