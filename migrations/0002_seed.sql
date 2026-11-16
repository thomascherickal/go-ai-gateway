-- Demo teams for local development and testing.
-- Key hashes below are SHA-256 hex digests of:
--   platform: gw_platform_demo_key
--   support:  gw_support_demo_key
--   sandbox:  gw_sandbox_demo_key

INSERT OR IGNORE INTO team_budgets (
    team_id, team_name, api_key_hash, monthly_limit_micros, spent_micros,
    period_ym, max_tier, pii_mode, enabled, created_at, updated_at
) VALUES
(
    'platform',
    'Platform Engineering',
    '4fb6af9066b11a62202dadf8521e3e7084c1e3f05f8569477749d25165ad0da6',
    50000000, -- $50.00 / month
    0,
    strftime('%Y-%m', 'now'),
    'reasoning',
    'redact',
    1,
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
),
(
    'support',
    'Customer Support',
    '66044d16d88d81c9beac74c1bfd12293d9b1623f3ca4e34508100b7b1281949a',
    15000000, -- $15.00 / month
    0,
    strftime('%Y-%m', 'now'),
    'balanced',
    'block',
    1,
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
),
(
    'sandbox',
    'Internal Sandbox',
    '7d2adda40717be8bfa30ee7ce3b8759ae99e7fc0770365f8b03fe597ce1956c1',
    1000000, -- $1.00 / month
    0,
    strftime('%Y-%m', 'now'),
    'fast',
    'redact',
    1,
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
    strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
);
