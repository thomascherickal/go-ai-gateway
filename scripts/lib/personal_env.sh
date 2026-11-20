#!/usr/bin/env bash
# Personal env resolution, initialization, and tool export helpers.

resolve_personal_env() {
  [[ -n "$ENV_FILE" ]] && return
  if [[ -f ".env.personal" ]]; then
    ENV_FILE=".env.personal"
  elif [[ -f ".env" ]]; then
    ENV_FILE=".env"
  else
    ENV_FILE="$DEFAULT_ENV_FILE"
  fi
}

init_personal_env() {
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
PROVIDER_LIMITS_USD=anthropic=5,openai=5,gemini=3,grok=3,deepseek=2,qwen=2
WORKER_NAME=go-ai-gateway-personal
PAGES_PROJECT_NAME=go-ai-gateway-personal-ui
D1_DATABASE_NAME=personal-ai-gateway-db
SEED_DEMO_TEAMS=false
ANTHROPIC_API_KEY=
GEMINI_API_KEY=
XAI_API_KEY=
OPENAI_API_KEY=
DEEPSEEK_API_KEY=
QWEN_API_KEY=
MISTRAL_API_KEY=
OLLAMA_BASE_URL=http://localhost:11434/v1
EOF
  chmod 600 "$ENV_FILE"
  log "Generated new personal credentials in $ENV_FILE (not printed for safety)."
}

load_personal_env() {
  if [[ ! -f "$ENV_FILE" ]]; then
    [[ "$DRY_RUN" == "true" ]] && return
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

