#!/usr/bin/env bash
# Enterprise env initialization and team manifest provisioning helpers.

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
ADMIN_TOKEN=$admin_tok
SEED_DEMO_TEAMS=false
PERSONAL_MODE=false
# PROVIDER_LIMITS_USD=anthropic=500,openai=500,gemini=300,grok=100
ANTHROPIC_API_KEY=
GEMINI_API_KEY=
OPENAI_API_KEY=
XAI_API_KEY=
DEEPSEEK_API_KEY=
QWEN_API_KEY=
MISTRAL_API_KEY=
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

  touch "$KEYS_OUT" && chmod 600 "$KEYS_OUT"
  local rows
  rows="$(node -e '
    const fs = require("fs"), crypto = require("crypto");
    const list = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
    if (!Array.isArray(list)) throw new Error("teams-file must be a JSON array");
    for (const item of list) {
      if (!item.team_id) throw new Error("each team requires team_id");
      const generated = !item.api_key;
      const apiKey = item.api_key || ("gw_corp_" + item.team_id + "_" + crypto.randomBytes(16).toString("hex"));
      const payload = {
        team_id: item.team_id, team_name: item.team_name || item.team_id, api_key: apiKey,
        monthly_limit_micros: Number(item.monthly_limit_micros || 50000000),
        max_tier: item.max_tier || "balanced", pii_mode: item.pii_mode || "redact",
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
      -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d "$payload" >/dev/null
    log "  provisioned team: $tid"
    [[ "$generated" == "true" ]] && printf '%s=%s\n' "$tid" "$api_key" >> "$KEYS_OUT"
  done <<< "$rows"
  log "Any newly generated team keys were saved to $KEYS_OUT (chmod 600)."
}

