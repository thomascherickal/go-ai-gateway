#!/usr/bin/env bash
set -euo pipefail

# Pre-commit helper: generates the next 10 AI improvements & feature additions
# in next-steps.md and stages the updated file before every commit.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
# shellcheck source=scripts/lib/next_steps_ai.sh
source "${ROOT_DIR}/scripts/lib/next_steps_ai.sh"

load_ai_env_silent

DIFF_STAT="$(git diff --cached --stat 2>/dev/null | head -n 25 || true)"
RECENT_LOG="$(git log -n 5 --oneline 2>/dev/null || true)"
STAMP="$(date -u '+%Y-%m-%d %H:%M UTC')"

PROMPT="You are advising on the go-ai-gateway repository (Go + TinyGo Wasm + Cloudflare Workers + D1 + Workers AI + multi-vendor LLM proxy + PII redaction + per-team budgets).
Recent commits:
${RECENT_LOG}
Staged changes:
${DIFF_STAT}
Output ONLY a numbered markdown list (1. through 10.) of the next 10 concise, high-impact improvements and suitable feature additions (each 1 short sentence, <= 18 words, no preamble or closing text)."

AI_LIST="$(generate_ai_suggestions "$PROMPT" | grep -E '^[[:space:]]*[0-9]+\.' | head -n 10 || true)"
SOURCE_LABEL="AI-generated"

if [[ "$(printf '%s\n' "$AI_LIST" | grep -cE '^[[:space:]]*[0-9]+\.' || true)" -lt 10 ]]; then
  SOURCE_LABEL="Fallback (configure GEMINI_API_KEY, ANTHROPIC_API_KEY, OPENAI_API_KEY, or OLLAMA_BASE_URL for live AI)"
  AI_LIST="$(cat <<'EOF'
1. Wire `internal/features` (rate limiter, response cache, circuit breaker, DLP, alerts) directly into `Gateway` config and request pipeline.
2. Expose `/admin/audit/export?format=csv|json` and `/admin/forecast` HTTP endpoints backed by `internal/features/export.go`.
3. Add `AUDITOR_TOKEN` read-only RBAC enforcement in `internal/gateway/admin.go` using `internal/features/rbac.go`.
4. Add prompt-injection and jailbreak heuristic detection alongside PII redaction in `internal/redact`.
5. Add Cloudflare Durable Objects adapter for globally consistent sub-millisecond RPM/TPM rate limiting.
6. Add interactive cost-forecasting charts and CSV/JSON export download buttons to the embedded dashboard UI.
7. Add per-team model allowlists/denylists (e.g. restricting teams to specific approved vendor models).
8. Add scheduled Cloudflare Worker Cron Trigger to archive `audit_logs` to R2 as daily NDJSON snapshots.
9. Add mTLS / Cloudflare Zero Trust JWT header verification for enterprise `/admin/*` and `/v1/*` routes.
10. Add automated load-testing suite (`k6` or Go `testing.B` concurrent HTTP benchmark) to CI with p95 latency gates.
EOF
)"
fi

cat > "${ROOT_DIR}/next-steps.md" <<EOF
# Post-Commit & Post-Push Checklist (\`next-steps.md\`)

Reusable checklist and AI-generated roadmap updated automatically on every commit.

\`\`\`mermaid
flowchart LR
  A["1. Run Tests & Race"] --> B["2. 100% Coverage Gate"]
  B --> C["3. 100-Line & Mermaid"]
  C --> D["4. Profile CPU/GPU"]
  D --> E["5. Update Docs & Push"]
  style A fill:#1e3a5f,stroke:#38bdf8,color:#f8fafc
  style B fill:#14532d,stroke:#4ade80,color:#f8fafc
  style C fill:#4c1d95,stroke:#c084fc,color:#f8fafc
  style D fill:#7c2d12,stroke:#fb923c,color:#f8fafc
  style E fill:#1e293b,stroke:#94a3b8,color:#f8fafc
\`\`\`

## Post-Commit Checklist

1. Verify all tests pass (\`go test -race -count=1 ./...\`) with **100.0%** statement coverage.
2. Verify test runtime stays under 30 minutes and update \`test-time-report.md\`.
3. Verify all source/test files are \`<= 100\` lines and docs meet the colored Mermaid rule.
4. Run \`gofmt\`, \`go vet\`, \`bash -n\`, and CPU/GPU profiling (\`system-profile.md\`).
5. Update \`metrics.md\`, \`readme.md\`, and confirm \`./scripts/check-gate.sh\` passes cleanly.

## Next 10 AI-Generated Improvements & Feature Additions

_Updated: ${STAMP} (${SOURCE_LABEL})_

\`\`\`mermaid
flowchart LR
  Diff["Staged Diff"] --> AI["AI Roadmap Generator"]
  AI --> File["next-steps.md"]
  File --> Stage["git add next-steps.md"]
  style Diff fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style AI fill:#f3e8ff,stroke:#9333ea,color:#581c87
  style File fill:#fef9c3,stroke:#ca8a04,color:#713f12
  style Stage fill:#dcfce7,stroke:#16a34a,color:#14532d
\`\`\`

${AI_LIST}
EOF

git add "${ROOT_DIR}/next-steps.md"
echo "==> Updated and staged next-steps.md (${SOURCE_LABEL})"

