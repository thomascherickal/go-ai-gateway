#!/usr/bin/env bash
set -euo pipefail

# Cross-platform build script using Go's native GOOS/GOARCH and TinyGo Wasm.
OUT_DIR="${OUT_DIR:-builds}"
LDFLAGS="-s -w"

build_bin() {
  local label="$1" goos="$2" goarch="$3" out="$4"
  mkdir -p "$(dirname "$out")"
  echo "==> Building ${label} (${goos}/${goarch}) -> ${out}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags="$LDFLAGS" -o "$out" .
}

build_bin "windows" "windows" "amd64" "${OUT_DIR}/windows-amd64/go-ai-gateway.exe"
build_bin "linux"   "linux"   "amd64" "${OUT_DIR}/linux-amd64/go-ai-gateway"
build_bin "arm"     "linux"   "arm64" "${OUT_DIR}/linux-arm64/go-ai-gateway"
build_bin "darwin"  "darwin"  "arm64" "${OUT_DIR}/darwin-arm64/go-ai-gateway"
build_bin "android" "android" "arm64" "${OUT_DIR}/android-arm64/go-ai-gateway"

# iOS requires Apple Xcode SDK (CGO_ENABLED=1 on macOS); emit build helper on Linux.
mkdir -p "${OUT_DIR}/ios-arm64"
if [[ "$(uname -s)" == "Darwin" ]] && command -v xcrun >/dev/null 2>&1; then
  echo "==> Building ios (ios/arm64) -> ${OUT_DIR}/ios-arm64/go-ai-gateway"
  CGO_ENABLED=1 GOOS=ios GOARCH=arm64 \
    go build -trimpath -ldflags="$LDFLAGS" -o "${OUT_DIR}/ios-arm64/go-ai-gateway" .
else
  cat > "${OUT_DIR}/ios-arm64/README.txt" <<'EOF'
iOS (GOOS=ios GOARCH=arm64) requires macOS with Xcode SDK (CGO_ENABLED=1) or gomobile.
Run on macOS:
  CGO_ENABLED=1 GOOS=ios GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o builds/ios-arm64/go-ai-gateway .
EOF
  echo "==> Skipping native ios/arm64 binary on $(uname -s) (wrote ${OUT_DIR}/ios-arm64/README.txt)"
fi

# Web (Cloudflare Workers / WebAssembly via TinyGo)
mkdir -p "${OUT_DIR}/web"
if command -v tinygo >/dev/null 2>&1; then
  echo "==> Building web (TinyGo Wasm) -> ${OUT_DIR}/web/app.wasm"
  go run github.com/syumai/workers-go/cmd/workers-assets-gen
  tinygo build -o "${OUT_DIR}/web/app.wasm" -target wasm -no-debug .
  cp -f ./build/worker.mjs ./build/runtime.mjs ./build/wasm_exec.js "${OUT_DIR}/web/" 2>/dev/null || true
else
  echo "==> Building web (Go js/wasm) -> ${OUT_DIR}/web/app.wasm"
  GOOS=js GOARCH=wasm go build -trimpath -ldflags="$LDFLAGS" -o "${OUT_DIR}/web/app.wasm" .
fi

echo "OK: cross-platform builds written to ${OUT_DIR}/"

