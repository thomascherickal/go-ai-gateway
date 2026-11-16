#!/usr/bin/env bash
set -euo pipefail

# Enterprise / Company Deployment Script for Go AI Gateway:
#   - Enforces production guardrails (strong ADMIN_TOKEN, no demo seed keys, no personal bypass)
#   - Delegates TinyGo Wasm build, D1 migration, secrets upload, and Worker/Pages deploy to ./deploy-cloudflare.sh
#   - Optionally provisions department/team budgets from a JSON manifest (--teams-file)

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

Options:
  --env <file>        Path to enterprise env file (default: .env.company, or .env)
  --target <mode>     Deployment target: worker (default), pages, or both
  --teams-file <json> Optional JSON array of teams to provision via POST /admin/teams
  --keys-out <file>   Output file (chmod 600) for newly generated team keys (default: company-team-keys.txt)
  --init-only         Create a hardened .env.company template with generated ADMIN_TOKEN and exit
  --skip-tests        Skip `go test ./...` before building
  --dry-run           Validate guardrails and preview deployment without mutating Cloudflare
  -h, --help          Show this help message
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env)
      ENV_FILE="${2:-}"
      shift 2
      ;;
    --target)
      TARGET="${2:-}"
      shift 2
      ;;
    --teams-file)
      TEAMS_FILE="${2:-}"
      shift 2
      ;;
    --keys-out)
      KEYS_OUT="${2:-}"
      shift 2
      ;;
    --init-only)
      INIT_ONLY="true"
      shift
      ;;
    --skip-tests)
      SKIP_TESTS="true"
      shift
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

case "$TARGET" in
  worker|pages|both) ;;
  *)
    echo "Invalid --target '$TARGET'. Expected: worker, pages, or both." >&2
    exit 1
    ;;
esac

log()  { printf '\033[1;34m[company-deploy]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[warn]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Required command '$1' not found in PATH."
}

gen_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 24
  else
    od -An -tx1 -N24 /dev/urandom | tr -d ' \n'
  fi
}

resolve_env_file() {
  if [[ -n "$ENV_FILE" ]]; then
    return
  fi
  if [[ -f ".env.company" ]]; then
    ENV_FILE=".env.company"
  elif [[ -f ".env" ]]; then
    ENV_FILE=".env"
  else
    ENV_FILE=".env.company"
  fi
}

init_company_env() {
  if [[ -f "$ENV_FILE" ]]; then
    log "Using existing enterprise env file: $ENV_FILE"
    return
  fi
  log "Creating enterprise env file at $ENV_FILE (chmod 600)..."
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] write $ENV_FILE (chmod 600)"
    return
  fi
  local admin_tok
  admin_tok="gw_corp_admin_$(gen_secret)"
  cat > "$ENV_FILE" <<EOF
# Go AI Gateway — Enterprise Production Configuration
WORKER_NAME=go-ai-gateway-prod
PAGES_PROJECT_NAME=go-ai-gateway-portal
D1_DATABASE_NAME=ai-gateway-prod-db
AI_GATEWAY_ID=enterprise-ai-gateway

# Enterprise Guardrails
ADMIN_TOKEN=$admin_tok
SEED_DEMO_TEAMS=false
PERSONAL_MODE=false

# Optional global monthly vendor caps across all teams (USD):
# PROVIDER_LIMITS_USD=anthropic=500,openai=500,gemini=300,grok=100

# Enterprise Vendor API Keys (leave blank for any unused vendor):
ANTHROPIC_API_KEY=
GEMINI_API_KEY=
OPENAI_API_KEY=
XAI_API_KEY=
DEEPSEEK_API_KEY=
QWEN_API_KEY=
MISTRAL_API_KEY=
# UPSTREAM_BASE_URL=
# UPSTREAM_API_KEY=
EOF
  chmod 600 "$ENV_FILE"
  log "Created $ENV_FILE with a generated ADMIN_TOKEN (not printed for safety)."
}

provision_teams_manifest() {
  local base_url="$1"
  [[ -n "$TEAMS_FILE" ]] || return 0
  [[ -f "$TEAMS_FILE" ]] || die "Teams manifest '$TEAMS_FILE' not found."
  require_cmd node

  log "Provisioning enterprise teams from $TEAMS_FILE against $base_url..."
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] provision teams from $TEAMS_FILE via POST $base_url/admin/teams (generated keys -> $KEYS_OUT)"
    return 0
  fi

  touch "$KEYS_OUT"
  chmod 600 "$KEYS_OUT"

  local rows
  rows="$(node -e '
    const fs = require("fs");
    const crypto = require("crypto");
    const list = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
    if (!Array.isArray(list)) throw new Error("teams-file must be a JSON array");
    for (const item of list) {
      if (!item.team_id) throw new Error("each team requires team_id");
      const generated = !item.api_key;
      const apiKey = item.api_key || ("gw_corp_" + item.team_id + "_" + crypto.randomBytes(16).toString("hex"));
      const payload = {
        team_id: item.team_id,
        team_name: item.team_name || item.team_id,
        api_key: apiKey,
        monthly_limit_micros: Number(item.monthly_limit_micros || 50000000),
        max_tier: item.max_tier || "balanced",
        pii_mode: item.pii_mode || "redact",
        enabled: item.enabled !== undefined ? Boolean(item.enabled) : true
      };
      console.log(JSON.stringify({ team_id: item.team_id, generated, api_key: apiKey, payload }));
    }
  ' "$TEAMS_FILE")"

  while IFS= read -r row; do
    [[ -n "$row" ]] || continue
    local tid generated api_key payload
    tid="$(node -e 'console.log(JSON.parse(process.argv[1]).team_id)' "$row")"
    generated="$(node -e 'console.log(JSON.parse(process.argv[1]).generated)' "$row")"
    api_key="$(node -e 'console.log(JSON.parse(process.argv[1]).api_key)' "$row")"
    payload="$(node -e 'console.log(JSON.stringify(JSON.parse(process.argv[1]).payload))' "$row")"

    curl -fsS -X POST "$base_url/admin/teams" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d "$payload" >/dev/null
    log "  provisioned team: $tid"

    if [[ "$generated" == "true" ]]; then
      printf '%s=%s\n' "$tid" "$api_key" >> "$KEYS_OUT"
    fi
  done <<< "$rows"
  log "Any newly generated team keys were saved to $KEYS_OUT (chmod 600)."
}

resolve_env_file
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

# Enterprise production guardrails
if [[ -z "${ADMIN_TOKEN:-}" || "${ADMIN_TOKEN}" == change_me* || "${ADMIN_TOKEN}" == "gw_admin_dev" ]]; then
  die "Enterprise deployment requires a strong non-default ADMIN_TOKEN in $ENV_FILE."
fi
if [[ "${SEED_DEMO_TEAMS:-false}" == "true" ]]; then
  die "SEED_DEMO_TEAMS=true is blocked in deploy-company.sh to prevent exposing demo keys in production."
fi

export PERSONAL_MODE=false
export SEED_DEMO_TEAMS=false

[[ -x "./deploy-cloudflare.sh" ]] || die "./deploy-cloudflare.sh not found or not executable."

DEPLOY_ARGS=(--env "$ENV_FILE" --target "$TARGET")
if [[ "$SKIP_TESTS" == "true" ]]; then
  DEPLOY_ARGS+=(--skip-tests)
fi
if [[ "$DRY_RUN" == "true" ]]; then
  DEPLOY_ARGS+=(--dry-run)
fi

log "Running enterprise Cloudflare deployment (target=$TARGET, env=$ENV_FILE)..."
./deploy-cloudflare.sh "${DEPLOY_ARGS[@]}"

WORKER_URL="https://${WORKER_NAME:-go-ai-gateway}.workers.dev"
provision_teams_manifest "$WORKER_URL"

log "Enterprise deployment finished."
log "Next: protect / and /admin/* with Cloudflare Access (Zero Trust) in the Cloudflare Dashboard."

