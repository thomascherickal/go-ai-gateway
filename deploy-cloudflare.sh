#!/usr/bin/env bash
set -euo pipefail

# Single-file deployment script for Go AI Gateway:
#   - Loads API keys and config from .env (never echoes secrets)
#   - Runs Go tests & compiles TinyGo WebAssembly bundle
#   - Provisions & migrates Cloudflare D1 database
#   - Pushes secrets to Cloudflare Workers and/or Cloudflare Pages
#   - Deploys Worker API and/or Cloudflare Pages interactive demo

ENV_FILE=".env"
TARGET="worker" # worker | pages | both | local
SEED_DEMO="false"
SKIP_TESTS="false"
DRY_RUN="false"

usage() {
  cat <<'EOF'
Usage: ./deploy-cloudflare.sh [options]

Options:
  --env <file>      Path to .env file with API keys (default: .env)
  --target <mode>   Deployment target:
                      worker  Deploy Cloudflare Worker API + D1 + Workers AI (default)
                      pages   Deploy Cloudflare Pages demo UI + Pages Functions
                      both    Deploy Worker API AND Cloudflare Pages demo UI
                      local   Run locally on http://localhost:9900 using .env
  --seed-demo       Apply migrations/0002_seed.sql (demo keys; non-production only)
  --skip-tests      Skip `go test ./...` before building
  --dry-run         Print steps without mutating Cloudflare resources
  -h, --help        Show this help message
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
    --seed-demo)
      SEED_DEMO="true"
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
  worker|pages|both|local) ;;
  *)
    echo "Invalid --target '$TARGET'. Expected: worker, pages, both, or local." >&2
    exit 1
    ;;
esac

log()  { printf '\033[1;34m[deploy]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[warn]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[error]\033[0m %s\n' "$*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Required command '$1' not found in PATH."
}

run_cmd() {
  if [[ "$DRY_RUN" == "true" ]]; then
    printf '[dry-run] %s\n' "$*"
  else
    "$@"
  fi
}

# 1. Load .env
if [[ ! -f "$ENV_FILE" ]]; then
  die "Environment file '$ENV_FILE' not found. Run: cp .env.example $ENV_FILE && chmod 600 $ENV_FILE"
fi
log "Loading configuration from $ENV_FILE"
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

WORKER_NAME="${WORKER_NAME:-go-ai-gateway}"
PAGES_PROJECT_NAME="${PAGES_PROJECT_NAME:-go-ai-gateway-demo}"
D1_DATABASE_NAME="${D1_DATABASE_NAME:-ai-gateway-db}"
if [[ "${SEED_DEMO_TEAMS:-false}" == "true" ]]; then
  SEED_DEMO="true"
fi

# 2. Check prerequisites
require_cmd go
if [[ "$SKIP_TESTS" != "true" ]]; then
  log "Running Go test suite..."
  run_cmd go test -race ./...
fi

if [[ "$TARGET" == "local" ]]; then
  log "Starting Go AI Gateway locally on http://localhost:${PORT:-9900}..."
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] go run ."
  else
    exec go run .
  fi
  exit 0
fi

require_cmd tinygo
require_cmd node
require_cmd npm
require_cmd npx

if [[ ! -d node_modules ]]; then
  log "Installing npm devDependencies (wrangler)..."
  run_cmd npm install
fi

if [[ "$DRY_RUN" != "true" ]]; then
  log "Verifying Cloudflare authentication..."
  npx wrangler whoami >/dev/null 2>&1 || die "Not authenticated to Cloudflare. Run 'npx wrangler login' or set CLOUDFLARE_API_TOKEN & CLOUDFLARE_ACCOUNT_ID in $ENV_FILE."
fi

# 3. Build TinyGo Wasm bundle
log "Building TinyGo WebAssembly bundle (./build/app.wasm)..."
run_cmd go run github.com/syumai/workers-go/cmd/workers-assets-gen
run_cmd tinygo build -o ./build/app.wasm -target wasm -no-debug ./...
if [[ "$DRY_RUN" != "true" ]]; then
  ls -lh ./build/app.wasm
fi

# 4. Provision D1 database and patch wrangler configs if placeholder UUID is present
PLACEHOLDER_UUID="00000000-0000-0000-0000-000000000000"

read_d1_id_from_config() {
  node -e '
    const fs = require("fs");
    const cfg = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
    const db = (cfg.d1_databases || [])[0];
    console.log(db && db.database_id ? db.database_id : "");
  ' "$1"
}

patch_wrangler_configs() {
  local db_id="$1"
  local w_name="$2"
  local p_name="$3"
  local gw_id="${AI_GATEWAY_ID:-}"
  node -e '
    const fs = require("fs");
    function patch(file, name, dbName, dbId, gwId) {
      if (!fs.existsSync(file)) return;
      const cfg = JSON.parse(fs.readFileSync(file, "utf8"));
      cfg.name = name;
      if (cfg.d1_databases && cfg.d1_databases[0]) {
        cfg.d1_databases[0].database_name = dbName;
        cfg.d1_databases[0].database_id = dbId;
      }
      cfg.vars = cfg.vars || {};
      cfg.vars.AI_GATEWAY_ID = gwId;
      fs.writeFileSync(file, JSON.stringify(cfg, null, 2) + "\n");
    }
    patch("wrangler.jsonc", process.argv[2], process.argv[4], process.argv[1], process.argv[5]);
    patch("wrangler.pages.jsonc", process.argv[3], process.argv[4], process.argv[1], process.argv[5]);
  ' "$db_id" "$w_name" "$p_name" "$D1_DATABASE_NAME" "$gw_id"
}

DB_ID="$(read_d1_id_from_config wrangler.jsonc)"
if [[ -z "$DB_ID" || "$DB_ID" == "$PLACEHOLDER_UUID" ]]; then
  log "Checking/creating Cloudflare D1 database '$D1_DATABASE_NAME'..."
  if [[ "$DRY_RUN" == "true" ]]; then
    DB_ID="$PLACEHOLDER_UUID"
    echo "[dry-run] npx wrangler d1 create $D1_DATABASE_NAME"
  else
    LIST_JSON="$(npx wrangler d1 list --json 2>/dev/null || echo '[]')"
    DB_ID="$(node -e '
      const list = JSON.parse(process.argv[1] || "[]");
      const found = list.find(d => d.name === process.argv[2]);
      console.log(found ? found.uuid : "");
    ' "$LIST_JSON" "$D1_DATABASE_NAME")"

    if [[ -z "$DB_ID" ]]; then
      CREATE_OUT="$(npx wrangler d1 create "$D1_DATABASE_NAME")"
      DB_ID="$(printf '%s\n' "$CREATE_OUT" | grep -Eo '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -n1 || true)"
      [[ -n "$DB_ID" ]] || die "Created D1 database '$D1_DATABASE_NAME' but failed to parse database_id."
      log "Created D1 database $D1_DATABASE_NAME ($DB_ID)"
    else
      log "Found existing D1 database $D1_DATABASE_NAME ($DB_ID)"
    fi
  fi
fi

if [[ "$DRY_RUN" != "true" ]]; then
  patch_wrangler_configs "$DB_ID" "$WORKER_NAME" "$PAGES_PROJECT_NAME"
fi

# 5. Apply D1 migrations
log "Applying D1 schema migration (migrations/0001_init.sql)..."
run_cmd npx wrangler d1 execute "$D1_DATABASE_NAME" --remote --yes --file=./migrations/0001_init.sql

if [[ "$SEED_DEMO" == "true" ]]; then
  warn "Seeding demo teams (migrations/0002_seed.sql) — do NOT enable SEED_DEMO_TEAMS for production!"
  run_cmd npx wrangler d1 execute "$D1_DATABASE_NAME" --remote --yes --file=./migrations/0002_seed.sql
fi

# 6. Helper to push non-empty secrets cleanly
SECRET_KEYS=(
  ADMIN_TOKEN
  PROVIDER_LIMITS_USD
  ANTHROPIC_API_KEY
  GEMINI_API_KEY
  XAI_API_KEY
  DEEPSEEK_API_KEY
  QWEN_API_KEY
  DASHSCOPE_API_KEY
  OPENAI_API_KEY
  MISTRAL_API_KEY
  UPSTREAM_BASE_URL
  UPSTREAM_API_KEY
)

push_worker_secrets() {
  log "Uploading configured secrets to Worker '$WORKER_NAME'..."
  for key in "${SECRET_KEYS[@]}"; do
    val="${!key:-}"
    if [[ -n "$val" ]]; then
      if [[ "$DRY_RUN" == "true" ]]; then
        echo "[dry-run] printf '***' | npx wrangler secret put $key --name $WORKER_NAME"
      else
        printf '%s' "$val" | npx wrangler secret put "$key" --name "$WORKER_NAME" >/dev/null
        log "  set Worker secret: $key"
      fi
    fi
  done
}

push_pages_secrets() {
  log "Uploading configured secrets to Pages project '$PAGES_PROJECT_NAME'..."
  for key in "${SECRET_KEYS[@]}"; do
    val="${!key:-}"
    if [[ -n "$val" ]]; then
      if [[ "$DRY_RUN" == "true" ]]; then
        echo "[dry-run] printf '***' | npx wrangler pages secret put $key --project-name $PAGES_PROJECT_NAME"
      else
        printf '%s' "$val" | npx wrangler pages secret put "$key" --project-name "$PAGES_PROJECT_NAME" >/dev/null
        log "  set Pages secret: $key"
      fi
    fi
  done
}

provision_personal_profile() {
  local base_url="$1"
  if [[ "${PERSONAL_MODE:-false}" != "true" || -z "${PERSONAL_API_KEY:-}" || -z "${ADMIN_TOKEN:-}" ]]; then
    return
  fi
  local budget_usd="${PERSONAL_MONTHLY_BUDGET_USD:-30}"
  local limit_micros
  limit_micros="$(node -e 'console.log(Math.round(Number(process.argv[1] || 30) * 1e6))' "$budget_usd")"
  local payload
  payload="$(node -e '
    console.log(JSON.stringify({
      team_id: "personal",
      team_name: "Personal Workspace",
      api_key: process.argv[1],
      monthly_limit_micros: Number(process.argv[2]),
      max_tier: process.argv[3] || "reasoning",
      pii_mode: process.argv[4] || "redact",
      enabled: true
    }));
  ' "$PERSONAL_API_KEY" "$limit_micros" "${PERSONAL_MAX_TIER:-reasoning}" "${PERSONAL_PII_MODE:-redact}")"

  log "Provisioning 'personal' team profile against $base_url..."
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] curl -X POST $base_url/admin/teams"
  else
    curl -fsS -X POST "$base_url/admin/teams" \
      -H "Authorization: Bearer $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d "$payload" >/dev/null
    log "Personal profile provisioned."
  fi
}

# 7. Deploy Worker and/or Pages
DEPLOYED_WORKER_URL=""
if [[ "$TARGET" == "worker" || "$TARGET" == "both" ]]; then
  if [[ -z "${ADMIN_TOKEN:-}" ]]; then
    warn "ADMIN_TOKEN is empty in $ENV_FILE; /admin/* endpoints will remain locked until ADMIN_TOKEN is set."
  fi
  log "Deploying Cloudflare Worker '$WORKER_NAME'..."
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] npx wrangler deploy"
    DEPLOYED_WORKER_URL="https://${WORKER_NAME}.workers.dev"
  else
    DEPLOY_OUT="$(npx wrangler deploy)"
    printf '%s\n' "$DEPLOY_OUT"
    DEPLOYED_WORKER_URL="$(printf '%s\n' "$DEPLOY_OUT" | grep -Eo 'https://[a-zA-Z0-9._-]+\.workers\.dev' | head -n1 || true)"
  fi
  push_worker_secrets
  if [[ -n "$DEPLOYED_WORKER_URL" ]]; then
    log "Checking Worker health: $DEPLOYED_WORKER_URL/healthz"
    if [[ "$DRY_RUN" != "true" ]]; then
      curl -fsS "$DEPLOYED_WORKER_URL/healthz" && echo
    fi
    provision_personal_profile "$DEPLOYED_WORKER_URL"
  fi
fi

if [[ "$TARGET" == "pages" || "$TARGET" == "both" ]]; then
  log "Ensuring Cloudflare Pages project '$PAGES_PROJECT_NAME' exists..."
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] npx wrangler pages project create $PAGES_PROJECT_NAME --production-branch main"
  else
    npx wrangler pages project create "$PAGES_PROJECT_NAME" --production-branch main >/dev/null 2>&1 || true
  fi

  if [[ "$TARGET" == "pages" ]]; then
    log "Deploying full-stack Pages demo (UI + TinyGo Wasm Pages Functions)..."
    run_cmd npx wrangler pages deploy --project-name "$PAGES_PROJECT_NAME" --config wrangler.pages.jsonc
    push_pages_secrets
  else
    log "Deploying static demo UI to Cloudflare Pages (pointing at Worker backend $DEPLOYED_WORKER_URL)..."
    if [[ "$DRY_RUN" == "true" ]]; then
      echo "[dry-run] npx wrangler pages deploy <static-pages-dir> --project-name $PAGES_PROJECT_NAME"
    else
      TMP_PAGES="$(mktemp -d)"
      trap 'rm -rf "$TMP_PAGES"' EXIT
      cp -R ./pages/* "$TMP_PAGES"/
      if [[ -n "$DEPLOYED_WORKER_URL" ]]; then
        sed -i "s|localStorage.getItem(\"gw_pages_url\") || \"\"|localStorage.getItem(\"gw_pages_url\") || \"$DEPLOYED_WORKER_URL\"|" "$TMP_PAGES/index.html"
      fi
      local wrangler_bin="$PWD/node_modules/.bin/wrangler"
      (cd "$TMP_PAGES" && "$wrangler_bin" pages deploy . --project-name "$PAGES_PROJECT_NAME")
    fi
  fi
fi

log "Deployment complete!"
