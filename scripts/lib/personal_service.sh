#!/usr/bin/env bash
# Linux systemd --user service installation, removal, and status checks.

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

  local port="${PORT:-9900}" url="http://127.0.0.1:${PORT:-9900}"
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

check_personal_status() {
  resolve_personal_env
  load_personal_env
  local port="${PORT:-9900}" url="http://127.0.0.1:${PORT:-9900}"
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

