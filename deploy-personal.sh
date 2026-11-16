#!/usr/bin/env bash
set -euo pipefail

# Full personal deployment automation for Go AI Gateway:
#   - Initializes ~/.config/go-ai-gateway/env (chmod 600) with random tokens if absent
#   - Supports local foreground, Linux systemd --user background service, or Cloudflare Workers/Pages Free Tier
#   - Reuses ./deploy-cloudflare.sh for Cloudflare Wasm + D1 + secrets + personal profile provisioning
#   - Prints copy-pasteable tool exports (Claude Code, Antigravity CLI, Gemini CLI, OpenAI tools)

MODE="local"          # local | cloudflare | pages
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

Options:
  --mode <target>       Deployment mode:
                          local       Build & run locally with FileStore persistence (default)
                          cloudflare  Deploy personal Worker + D1 + Workers AI via ./deploy-cloudflare.sh
                          pages       Deploy personal Worker + Cloudflare Pages dashboard via ./deploy-cloudflare.sh
  --env <file>          Path to personal env file (default: ~/.config/go-ai-gateway/env, or ./.env.personal)
  --init-only           Create/verify personal env file with generated tokens and exit
  --install-service     (local mode) Install ~/.local/bin/go-ai-gateway & enable systemd --user service
  --uninstall-service   Stop, disable, and remove the systemd --user service
  --status              Check local service /healthz and /admin/providers status
  --skip-tests          Skip `go test ./...` before building
  --dry-run             Print actions without modifying files, services, or Cloudflare
  -h, --help            Show this help message
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode)
      MODE="${2:-}"
      shift 2
      ;;
    --env)
      ENV_FILE="${2:-}"
      shift 2
      ;;
    --init-only)
      INIT_ONLY="true"
      shift
      ;;
    --install-service)
      INSTALL_SERVICE="true"
      shift
      ;;
    --uninstall-service)
      UNINSTALL_SERVICE="true"
      shift
      ;;
    --status)
      STATUS_ONLY="true"
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

case "$MODE" in
  local|cloudflare|pages) ;;
  *)
    echo "Invalid --mode '$MODE'. Expected: local, cloudflare, or pages." >&2
    exit 1
    ;;
esac

log()  { printf '\033[1;34m[personal-deploy]\033[0m %s\n' "$*"; }
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
  if [[ -f ".env.personal" ]]; then
    ENV_FILE=".env.personal"
  elif [[ -f ".env" ]]; then
    ENV_FILE=".env"
  elif [[ -f "$DEFAULT_ENV_FILE" ]]; then
    ENV_FILE="$DEFAULT_ENV_FILE"
  else
    ENV_FILE="$DEFAULT_ENV_FILE"
  fi
}

init_env_file() {
  if [[ -f "$ENV_FILE" ]]; then
    log "Using existing personal env file: $ENV_FILE"
    return
  fi

  local parent_dir
  parent_dir="$(dirname "$ENV_FILE")"
  log "Creating personal env file at $ENV_FILE (chmod 600) with generated secrets..."
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] mkdir -p $parent_dir && write $ENV_FILE (chmod 600)"
    return
  fi

  mkdir -p "$parent_dir"
  local admin_tok personal_key
  admin_tok="gw_admin_$(gen_secret)"
  personal_key="gw_personal_$(gen_secret)"

  cat > "$ENV_FILE" <<EOF
# Go AI Gateway — Personal Multi-Subscription Configuration
PERSONAL_MODE=true
PERSONAL_API_KEY=$personal_key
PERSONAL_MONTHLY_BUDGET_USD=30
PERSONAL_PII_MODE=redact
PERSONAL_MAX_TIER=reasoning
ADMIN_TOKEN=$admin_tok
GATEWAY_DATA_FILE=$DEFAULT_STATE_FILE
PORT=9900

# Per-provider monthly caps in USD (overflows to Workers AI or local Ollama when reached):
PROVIDER_LIMITS_USD=anthropic=5,openai=5,gemini=3,grok=3,deepseek=2,qwen=2

# Cloudflare Worker / Pages / D1 settings (used for --mode cloudflare or --mode pages):
WORKER_NAME=go-ai-gateway-personal
PAGES_PROJECT_NAME=go-ai-gateway-personal-ui
D1_DATABASE_NAME=personal-ai-gateway-db
SEED_DEMO_TEAMS=false

# Fill in keys for the AI companies you subscribe to (leave others blank):
ANTHROPIC_API_KEY=
GEMINI_API_KEY=
XAI_API_KEY=
OPENAI_API_KEY=
DEEPSEEK_API_KEY=
QWEN_API_KEY=
MISTRAL_API_KEY=

# Optional local Ollama ($0/token overflow & offline inference):
OLLAMA_BASE_URL=http://localhost:11434/v1

# Optional Cloudflare Workers AI REST credentials (for local mode):
# CF_ACCOUNT_ID=
# CF_API_TOKEN=
EOF
  chmod 600 "$ENV_FILE"
  log "Generated new personal credentials in $ENV_FILE (not printed for safety)."
}

load_env_file() {
  if [[ ! -f "$ENV_FILE" ]]; then
    if [[ "$DRY_RUN" == "true" ]]; then
      return
    fi
    die "Env file '$ENV_FILE' does not exist."
  fi
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
}

print_tool_exports() {
  local base_url="$1"
  cat <<EOF

===============================================================================
Personal AI Gateway Ready: $base_url
===============================================================================
Read your PERSONAL_API_KEY from: $ENV_FILE

Add these to ~/.bashrc or ~/.zshrc so all developer tools route through it:
  export GW_KEY="\$(grep -E '^PERSONAL_API_KEY=' '$ENV_FILE' | cut -d= -f2-)"
  export OPENAI_BASE_URL="$base_url/v1"
  export OPENAI_API_KEY="\$GW_KEY"
  export ANTHROPIC_BASE_URL="$base_url"
  export ANTHROPIC_API_KEY="\$GW_KEY"
  export GOOGLE_GEMINI_BASE_URL="$base_url"
  export GEMINI_API_KEY="\$GW_KEY"

Dashboard UI: $base_url/
===============================================================================
EOF
}

uninstall_systemd_service() {
  log "Removing systemd user service..."
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] systemctl --user disable --now go-ai-gateway"
    echo "[dry-run] rm -f $SYSTEMD_UNIT_FILE $INSTALL_BIN"
    return
  fi
  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user disable --now go-ai-gateway >/dev/null 2>&1 || true
  fi
  rm -f "$SYSTEMD_UNIT_FILE" "$INSTALL_BIN"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user daemon-reload >/dev/null 2>&1 || true
  fi
  log "Removed personal systemd service and binary."
}

install_systemd_service() {
  require_cmd systemctl
  log "Installing binary to $INSTALL_BIN..."
  run_cmd mkdir -p "$(dirname "$INSTALL_BIN")" "$SYSTEMD_UNIT_DIR" "$(dirname "${GATEWAY_DATA_FILE:-$DEFAULT_STATE_FILE}")"
  run_cmd go build -o "$INSTALL_BIN" .

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] write $SYSTEMD_UNIT_FILE"
    echo "[dry-run] systemctl --user daemon-reload && systemctl --user enable --now go-ai-gateway"
    return
  fi

  cat > "$SYSTEMD_UNIT_FILE" <<EOF
[Unit]
Description=Go AI Gateway (Personal Multi-Subscription Control Plane)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=$ENV_FILE
ExecStart=$INSTALL_BIN
Restart=on-failure
RestartSec=3

[Install]
WantedBy=default.target
EOF

  systemctl --user daemon-reload
  systemctl --user enable --now go-ai-gateway

  local port="${PORT:-9900}"
  local url="http://127.0.0.1:$port"
  for _ in $(seq 1 20); do
    if curl -fsS "$url/healthz" >/dev/null 2>&1; then
      log "Service is healthy at $url"
      curl -fsS "$url/healthz" && echo
      print_tool_exports "http://localhost:$port"
      return
    fi
    sleep 0.25
  done
  warn "Service started; check logs with: journalctl --user -u go-ai-gateway -f"
}

check_status() {
  resolve_env_file
  load_env_file
  local port="${PORT:-9900}"
  local url="http://127.0.0.1:$port"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user status go-ai-gateway --no-pager || true
  fi
  log "Checking $url/healthz..."
  curl -fsS "$url/healthz" && echo
  if [[ -n "${ADMIN_TOKEN:-}" ]]; then
    log "Per-provider monthly spend & caps:"
    curl -fsS "$url/admin/providers" -H "Authorization: Bearer $ADMIN_TOKEN" && echo
  fi
}

if [[ "$UNINSTALL_SERVICE" == "true" ]]; then
  uninstall_systemd_service
  exit 0
fi

if [[ "$STATUS_ONLY" == "true" ]]; then
  check_status
  exit 0
fi

resolve_env_file
init_env_file

if [[ "$INIT_ONLY" == "true" ]]; then
  log "Init complete. Edit $ENV_FILE to add your AI vendor API keys, then re-run ./deploy-personal.sh."
  exit 0
fi

load_env_file
require_cmd go

if [[ "$SKIP_TESTS" != "true" ]]; then
  log "Running Go test suite..."
  run_cmd go test -race ./...
fi

case "$MODE" in
  local)
    if [[ "$INSTALL_SERVICE" == "true" ]]; then
      install_systemd_service
    else
      PORT_VAL="${PORT:-9900}"
      print_tool_exports "http://localhost:$PORT_VAL"
      log "Starting personal gateway in foreground (Ctrl+C to stop)..."
      if [[ "$DRY_RUN" == "true" ]]; then
        echo "[dry-run] exec go run ."
      else
        exec go run .
      fi
    fi
    ;;

  cloudflare)
    [[ -x "./deploy-cloudflare.sh" ]] || die "./deploy-cloudflare.sh not found or not executable."
    DEPLOY_ARGS=(--env "$ENV_FILE" --target worker --skip-tests)
    if [[ "$DRY_RUN" == "true" ]]; then
      DEPLOY_ARGS+=(--dry-run)
    fi
    ./deploy-cloudflare.sh "${DEPLOY_ARGS[@]}"
    print_tool_exports "https://${WORKER_NAME:-go-ai-gateway-personal}.workers.dev"
    ;;

  pages)
    [[ -x "./deploy-cloudflare.sh" ]] || die "./deploy-cloudflare.sh not found or not executable."
    DEPLOY_ARGS=(--env "$ENV_FILE" --target both --skip-tests)
    if [[ "$DRY_RUN" == "true" ]]; then
      DEPLOY_ARGS+=(--dry-run)
    fi
    ./deploy-cloudflare.sh "${DEPLOY_ARGS[@]}"
    print_tool_exports "https://${WORKER_NAME:-go-ai-gateway-personal}.workers.dev"
    log "Pages Dashboard URL: https://${PAGES_PROJECT_NAME:-go-ai-gateway-personal-ui}.pages.dev"
    ;;
esac
