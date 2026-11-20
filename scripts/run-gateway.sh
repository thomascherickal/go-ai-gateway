#!/usr/bin/env bash
set -euo pipefail

# Dynamically selects and launches the matching prebuilt binary from builds/.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILDS_DIR="${ROOT_DIR}/builds"

if [[ "${1:-}" == "--web" ]]; then
  shift
  echo "==> Launching WebAssembly build (${BUILDS_DIR}/web/app.wasm) via Wrangler..."
  exec npx wrangler dev "$@"
fi

detect_target() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  if [[ -n "${TERMUX_VERSION:-}" ]] || [[ -f /system/build.prop ]]; then
    echo "android-arm64/go-ai-gateway"
    return
  fi
  case "$os" in
    mingw*|msys*|cygwin*) echo "windows-amd64/go-ai-gateway.exe"; return ;;
    darwin*)              echo "darwin-arm64/go-ai-gateway"; return ;;
    linux*)
      case "$arch" in
        aarch64|arm64|armv8*) echo "linux-arm64/go-ai-gateway"; return ;;
        *)                    echo "linux-amd64/go-ai-gateway"; return ;;
      esac
      ;;
  esac
  echo "unsupported:${os}/${arch}"
}

TARGET="$(detect_target)"
if [[ "$TARGET" == unsupported:* ]]; then
  echo "ERROR: unsupported platform (${TARGET#unsupported:}). Use --web or run 'go run .'" >&2
  exit 1
fi

BIN="${BUILDS_DIR}/${TARGET}"
if [[ ! -x "$BIN" ]]; then
  echo "Binary ${BIN} not found; building cross-platform binaries..." >&2
  "${ROOT_DIR}/scripts/build-all.sh"
fi

echo "==> Running ${BIN}"
exec "$BIN" "$@"

