#!/usr/bin/env bash
set -euo pipefail

# Full personal deployment automation for Go AI Gateway.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_PREFIX="personal-deploy"
# shellcheck source=scripts/lib/common.sh
source "$SCRIPT_DIR/scripts/lib/common.sh"
# shellcheck source=scripts/lib/personal_env.sh
source "$SCRIPT_DIR/scripts/lib/personal_env.sh"
# shellcheck source=scripts/lib/personal_service.sh
source "$SCRIPT_DIR/scripts/lib/personal_service.sh"

MODE="local" # local | cloudflare | pages
ENV_FILE=""
INSTALL_SERVICE="false"
UNINSTALL_SERVICE="false"
INIT_ONLY="false"
STATUS_ONLY="false"
SKIP_TESTS="false"
DRY_RUN="false"

CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/go-ai-gateway"
DEFAULT_ENV_FILE="$CONFIG_DIR/env"
DEFAULT_STATE_FILE="$CONFIG_DIR/state.json"
INSTALL_BIN="$HOME/.local/bin/go-ai-gateway"
SYSTEMD_UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
SYSTEMD_UNIT_FILE="$SYSTEMD_UNIT_DIR/go-ai-gateway.service"

usage() {
  cat <<'EOF'
Usage: ./deploy-personal.sh [options]
  --mode <target>       local (default) | cloudflare | pages
  --env <file>          Path to personal env file
  --init-only           Create/verify personal env file and exit
  --install-service     (local) Install binary & systemd --user service
  --uninstall-service   Remove systemd --user service and binary
  --status              Check local service /healthz and /admin/providers
  --skip-tests          Skip `go test ./...` before building
  --dry-run             Print actions without mutating files/Cloudflare
  -h, --help            Show this help message
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="${2:-}"; shift 2 ;;
    --env) ENV_FILE="${2:-}"; shift 2 ;;
    --init-only) INIT_ONLY="true"; shift ;;
    --install-service) INSTALL_SERVICE="true"; shift ;;
    --uninstall-service) UNINSTALL_SERVICE="true"; shift ;;
    --status) STATUS_ONLY="true"; shift ;;
    --skip-tests) SKIP_TESTS="true"; shift ;;
    --dry-run) DRY_RUN="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

case "$MODE" in
  local|cloudflare|pages) ;;
  *) die "Invalid --mode '$MODE'. Expected: local, cloudflare, or pages." ;;
esac

[[ "$UNINSTALL_SERVICE" == "true" ]] && { uninstall_systemd_service; exit 0; }
[[ "$STATUS_ONLY" == "true" ]] && { check_personal_status; exit 0; }

resolve_personal_env
init_personal_env
[[ "$INIT_ONLY" == "true" ]] && { log "Init complete. Edit $ENV_FILE to add your AI vendor API keys, then re-run ./deploy-personal.sh."; exit 0; }

load_personal_env
require_cmd go
[[ "$SKIP_TESTS" != "true" ]] && { log "Running Go test suite..."; run_cmd go test -race ./...; }

case "$MODE" in
  local)
    if [[ "$INSTALL_SERVICE" == "true" ]]; then
      install_systemd_service
    else
      print_tool_exports "http://localhost:${PORT:-9900}"
      log "Starting personal gateway in foreground (Ctrl+C to stop)..."
      if [[ "$DRY_RUN" == "true" ]]; then echo "[dry-run] exec go run ."; else exec go run .; fi
    fi ;;
  cloudflare|pages)
    [[ -x "./deploy-cloudflare.sh" ]] || die "./deploy-cloudflare.sh not found or not executable."
    CF_TARGET="worker"; [[ "$MODE" == "pages" ]] && CF_TARGET="both"
    DEPLOY_ARGS=(--env "$ENV_FILE" --target "$CF_TARGET" --skip-tests)
    [[ "$DRY_RUN" == "true" ]] && DEPLOY_ARGS+=(--dry-run)
    ./deploy-cloudflare.sh "${DEPLOY_ARGS[@]}"
    print_tool_exports "https://${WORKER_NAME:-go-ai-gateway-personal}.workers.dev"
    if [[ "$MODE" == "pages" ]]; then
      log "Pages Dashboard URL: https://${PAGES_PROJECT_NAME:-go-ai-gateway-personal-ui}.pages.dev"
    fi ;;
esac
