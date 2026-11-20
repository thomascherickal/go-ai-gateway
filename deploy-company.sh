#!/usr/bin/env bash
set -euo pipefail

# Enterprise / Company Deployment Script for Go AI Gateway.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_PREFIX="company-deploy"
# shellcheck source=scripts/lib/common.sh
source "$SCRIPT_DIR/scripts/lib/common.sh"
# shellcheck source=scripts/lib/company_teams.sh
source "$SCRIPT_DIR/scripts/lib/company_teams.sh"

ENV_FILE=""
TARGET="worker" # worker | pages | both
TEAMS_FILE=""
KEYS_OUT="company-team-keys.txt"
INIT_ONLY="false"
SKIP_TESTS="false"
DRY_RUN="false"

usage() {
  cat <<'EOF'
Usage: ./deploy-company.sh [options]
  --env <file>        Path to enterprise env file (default: .env.company, or .env)
  --target <mode>     worker (default) | pages | both
  --teams-file <json> Optional JSON array of teams to provision via POST /admin/teams
  --keys-out <file>   Output file (chmod 600) for newly generated team keys
  --init-only         Create a hardened .env.company template and exit
  --skip-tests        Skip `go test ./...` before building
  --dry-run           Validate guardrails and preview deployment
  -h, --help          Show this help message
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env) ENV_FILE="${2:-}"; shift 2 ;;
    --target) TARGET="${2:-}"; shift 2 ;;
    --teams-file) TEAMS_FILE="${2:-}"; shift 2 ;;
    --keys-out) KEYS_OUT="${2:-}"; shift 2 ;;
    --init-only) INIT_ONLY="true"; shift ;;
    --skip-tests) SKIP_TESTS="true"; shift ;;
    --dry-run) DRY_RUN="true"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

case "$TARGET" in
  worker|pages|both) ;;
  *) die "Invalid --target '$TARGET'. Expected: worker, pages, or both." ;;
esac

if [[ -z "$ENV_FILE" ]]; then
  if [[ -f ".env.company" ]]; then ENV_FILE=".env.company"
  elif [[ -f ".env" ]]; then ENV_FILE=".env"
  else ENV_FILE=".env.company"
  fi
fi

if [[ "$INIT_ONLY" == "true" ]]; then
  init_company_env
  log "Init complete. Add enterprise vendor keys to $ENV_FILE, then run ./deploy-company.sh."
  exit 0
fi

[[ -f "$ENV_FILE" ]] || die "Enterprise env file '$ENV_FILE' not found. Run: ./deploy-company.sh --init-only"
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

if [[ -z "${ADMIN_TOKEN:-}" || "${ADMIN_TOKEN}" == change_me* || "${ADMIN_TOKEN}" == "gw_admin_dev" ]]; then
  die "Enterprise deployment requires a strong non-default ADMIN_TOKEN in $ENV_FILE."
fi
[[ "${SEED_DEMO_TEAMS:-false}" == "true" ]] && die "SEED_DEMO_TEAMS=true is blocked in deploy-company.sh."

export PERSONAL_MODE=false
export SEED_DEMO_TEAMS=false
[[ -x "./deploy-cloudflare.sh" ]] || die "./deploy-cloudflare.sh not found or not executable."

DEPLOY_ARGS=(--env "$ENV_FILE" --target "$TARGET")
[[ "$SKIP_TESTS" == "true" ]] && DEPLOY_ARGS+=(--skip-tests)
[[ "$DRY_RUN" == "true" ]] && DEPLOY_ARGS+=(--dry-run)

log "Running enterprise Cloudflare deployment (target=$TARGET, env=$ENV_FILE)..."
./deploy-cloudflare.sh "${DEPLOY_ARGS[@]}"
provision_teams_manifest "https://${WORKER_NAME:-go-ai-gateway}.workers.dev"
log "Enterprise deployment finished. Protect / and /admin/* with Cloudflare Access (Zero Trust)."
