#!/usr/bin/env bash
set -euo pipefail

# Single-command Cloudflare Workers/Pages + D1 + TinyGo deployment script.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_PREFIX="deploy"
# shellcheck source=scripts/lib/common.sh
source "$SCRIPT_DIR/scripts/lib/common.sh"
# shellcheck source=scripts/lib/cf_d1.sh
source "$SCRIPT_DIR/scripts/lib/cf_d1.sh"
# shellcheck source=scripts/lib/cf_secrets.sh
source "$SCRIPT_DIR/scripts/lib/cf_secrets.sh"
# shellcheck source=scripts/lib/cf_deploy.sh
source "$SCRIPT_DIR/scripts/lib/cf_deploy.sh"

ENV_FILE=".env"
TARGET="worker" # worker | pages | both | local
SEED_DEMO="false"
SKIP_TESTS="false"
DRY_RUN="false"

usage() {
  cat <<'EOF'
Usage: ./deploy-cloudflare.sh [options]
  --env <file>      Path to .env file with API keys (default: .env)
  --target <mode>   worker (default) | pages | both | local
  --seed-demo       Apply migrations/0002_seed.sql (non-production only)
  --skip-tests      Skip `go test ./...` before building
  --dry-run         Print steps without mutating Cloudflare resources
  -h, --help        Show this help message
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env) ENV_FILE="${2:-}"; shift 2 ;;
    --target) TARGET="${2:-}"; shift 2 ;;
    --seed-demo) SEED_DEMO="true"; shift ;;
    --skip-tests) SKIP_TESTS="true"; shift ;;
    --dry-run) DRY_RUN="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

case "$TARGET" in
  worker|pages|both|local) ;;
  *) die "Invalid --target '$TARGET'. Expected: worker, pages, both, or local." ;;
esac

[[ -f "$ENV_FILE" ]] || die "Environment file '$ENV_FILE' not found. Run: cp .env.example $ENV_FILE && chmod 600 $ENV_FILE"
log "Loading configuration from $ENV_FILE"
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

WORKER_NAME="${WORKER_NAME:-go-ai-gateway}"
PAGES_PROJECT_NAME="${PAGES_PROJECT_NAME:-go-ai-gateway-demo}"
D1_DATABASE_NAME="${D1_DATABASE_NAME:-ai-gateway-db}"
[[ "${SEED_DEMO_TEAMS:-false}" == "true" ]] && SEED_DEMO="true"

require_cmd go
if [[ "$SKIP_TESTS" != "true" ]]; then
  log "Running Go test suite..."
  run_cmd go test -race ./...
fi

if [[ "$TARGET" == "local" ]]; then
  log "Starting Go AI Gateway locally on http://localhost:${PORT:-9900}..."
  if [[ "$DRY_RUN" == "true" ]]; then echo "[dry-run] go run ."; else exec go run .; fi
  exit 0
fi

require_cmd tinygo; require_cmd node; require_cmd npm; require_cmd npx
[[ -d node_modules ]] || { log "Installing npm devDependencies (wrangler)..."; run_cmd npm install; }
if [[ "$DRY_RUN" != "true" ]]; then
  log "Verifying Cloudflare authentication..."
  npx wrangler whoami >/dev/null 2>&1 || die "Not authenticated to Cloudflare. Run 'npx wrangler login' or set CLOUDFLARE_API_TOKEN & CLOUDFLARE_ACCOUNT_ID in $ENV_FILE."
fi

log "Building TinyGo WebAssembly bundle (./build/app.wasm)..."
run_cmd go run github.com/syumai/workers-go/cmd/workers-assets-gen
run_cmd tinygo build -o ./build/app.wasm -target wasm -no-debug ./...
[[ "$DRY_RUN" != "true" ]] && ls -lh ./build/app.wasm

ensure_d1_and_migrate
[[ "$TARGET" == "worker" || "$TARGET" == "both" ]] && deploy_worker_target
[[ "$TARGET" == "pages" || "$TARGET" == "both" ]] && deploy_pages_target
log "Deployment complete!"
