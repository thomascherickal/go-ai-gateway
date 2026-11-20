#!/usr/bin/env bash
# Cloudflare Workers and Cloudflare Pages deployment routines.

DEPLOYED_WORKER_URL=""

deploy_worker_target() {
  if [[ -z "${ADMIN_TOKEN:-}" ]]; then
    warn "ADMIN_TOKEN is empty in $ENV_FILE; /admin/* endpoints will remain locked until ADMIN_TOKEN is set."
  fi
  log "Deploying Cloudflare Worker '$WORKER_NAME'..."
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "[dry-run] npx wrangler deploy"
    DEPLOYED_WORKER_URL="https://${WORKER_NAME}.workers.dev"
  else
    local deploy_out
    deploy_out="$(npx wrangler deploy)"
    printf '%s\n' "$deploy_out"
    DEPLOYED_WORKER_URL="$(printf '%s\n' "$deploy_out" | grep -Eo 'https://[a-zA-Z0-9._-]+\.workers\.dev' | head -n1 || true)"
  fi
  push_worker_secrets
  if [[ -n "$DEPLOYED_WORKER_URL" ]]; then
    log "Checking Worker health: $DEPLOYED_WORKER_URL/healthz"
    if [[ "$DRY_RUN" != "true" ]]; then
      curl -fsS "$DEPLOYED_WORKER_URL/healthz" && echo
    fi
    provision_personal_profile "$DEPLOYED_WORKER_URL"
  fi
}

deploy_pages_target() {
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
      local tmp_pages wrangler_bin="$PWD/node_modules/.bin/wrangler"
      tmp_pages="$(mktemp -d)"
      trap 'rm -rf "$tmp_pages"' EXIT
      cp -R ./pages/* "$tmp_pages"/
      if [[ -n "$DEPLOYED_WORKER_URL" ]]; then
        sed -i "s|localStorage.getItem(\"gw_pages_url\") || \"\"|localStorage.getItem(\"gw_pages_url\") || \"$DEPLOYED_WORKER_URL\"|" "$tmp_pages/index.html"
      fi
      (cd "$tmp_pages" && "$wrangler_bin" pages deploy . --project-name "$PAGES_PROJECT_NAME")
    fi
  fi
}

