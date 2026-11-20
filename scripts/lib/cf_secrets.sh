#!/usr/bin/env bash
# Cloudflare Workers/Pages secret upload and optional personal profile provisioning.

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
    local val="${!key:-}"
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
    local val="${!key:-}"
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
  local budget_usd="${PERSONAL_MONTHLY_BUDGET_USD:-30}" limit_micros payload
  limit_micros="$(node -e 'console.log(Math.round(Number(process.argv[1] || 30) * 1e6))' "$budget_usd")"
  payload="$(node -e '
    console.log(JSON.stringify({
      team_id: "personal", team_name: "Personal Workspace", api_key: process.argv[1],
      monthly_limit_micros: Number(process.argv[2]), max_tier: process.argv[3] || "reasoning",
      pii_mode: process.argv[4] || "redact", enabled: true
    }));
  ' "$PERSONAL_API_KEY" "$limit_micros" "${PERSONAL_MAX_TIER:-reasoning}" "${PERSONAL_PII_MODE:-redact}")"

  log "Provisioning 'personal' team profile against $base_url..."
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] curl -X POST $base_url/admin/teams"
  else
    curl -fsS -X POST "$base_url/admin/teams" \
      -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d "$payload" >/dev/null
    log "Personal profile provisioned."
  fi
}

