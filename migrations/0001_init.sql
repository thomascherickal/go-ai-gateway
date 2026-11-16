-- Cloudflare D1 schema for AI Traffic Control Plane

CREATE TABLE IF NOT EXISTS team_budgets (
    team_id TEXT PRIMARY KEY,
    team_name TEXT NOT NULL,
    api_key_hash TEXT NOT NULL UNIQUE,
    monthly_limit_micros INTEGER NOT NULL,
    spent_micros INTEGER NOT NULL DEFAULT 0,
    period_ym TEXT NOT NULL,
    max_tier TEXT NOT NULL DEFAULT 'reasoning',
    pii_mode TEXT NOT NULL DEFAULT 'redact',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_team_budgets_key_hash ON team_budgets(api_key_hash);

CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    team_id TEXT NOT NULL,
    tool_name TEXT NOT NULL DEFAULT '',
    inbound_protocol TEXT NOT NULL DEFAULT 'openai',
    requested_model TEXT NOT NULL,
    routed_model TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    route_reason TEXT NOT NULL DEFAULT '',
    fallback_used INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    pii_detected INTEGER NOT NULL DEFAULT 0,
    pii_types_json TEXT NOT NULL DEFAULT '{}',
    pii_action TEXT NOT NULL DEFAULT 'none',
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cost_micros INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    prompt_sha256 TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_team_created ON audit_logs(team_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_status ON audit_logs(status);

