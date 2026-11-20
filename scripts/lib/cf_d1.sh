#!/usr/bin/env bash
# Cloudflare D1 provisioning, wrangler config patching, and SQL migrations.

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
  local db_id="$1" w_name="$2" p_name="$3" gw_id="${AI_GATEWAY_ID:-}"
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

ensure_d1_and_migrate() {
  local db_id
  db_id="$(read_d1_id_from_config wrangler.jsonc)"
  if [[ -z "$db_id" || "$db_id" == "$PLACEHOLDER_UUID" ]]; then
    log "Checking/creating Cloudflare D1 database '$D1_DATABASE_NAME'..."
    if [[ "$DRY_RUN" == "true" ]]; then
      db_id="$PLACEHOLDER_UUID"
      echo "[dry-run] npx wrangler d1 create $D1_DATABASE_NAME"
    else
      local list_json create_out
      list_json="$(npx wrangler d1 list --json 2>/dev/null || echo '[]')"
      db_id="$(node -e '
        const list = JSON.parse(process.argv[1] || "[]");
        const found = list.find(d => d.name === process.argv[2]);
        console.log(found ? found.uuid : "");
      ' "$list_json" "$D1_DATABASE_NAME")"
      if [[ -z "$db_id" ]]; then
        create_out="$(npx wrangler d1 create "$D1_DATABASE_NAME")"
        db_id="$(printf '%s\n' "$create_out" | grep -Eo '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -n1 || true)"
        [[ -n "$db_id" ]] || die "Created D1 database '$D1_DATABASE_NAME' but failed to parse database_id."
        log "Created D1 database $D1_DATABASE_NAME ($db_id)"
      else
        log "Found existing D1 database $D1_DATABASE_NAME ($db_id)"
      fi
    fi
  fi

  if [[ "$DRY_RUN" != "true" ]]; then
    patch_wrangler_configs "$db_id" "$WORKER_NAME" "$PAGES_PROJECT_NAME"
  fi

  log "Applying D1 schema migration (migrations/0001_init.sql)..."
  run_cmd npx wrangler d1 execute "$D1_DATABASE_NAME" --remote --yes --file=./migrations/0001_init.sql

  if [[ "$SEED_DEMO" == "true" ]]; then
    warn "Seeding demo teams (migrations/0002_seed.sql) — do NOT enable SEED_DEMO_TEAMS for production!"
    run_cmd npx wrangler d1 execute "$D1_DATABASE_NAME" --remote --yes --file=./migrations/0002_seed.sql
  fi
}

