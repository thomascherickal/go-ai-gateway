# API Reference

## Inference Endpoints

```mermaid
flowchart LR
  OAI["/v1/chat/completions"] --> Pipe["Gateway Pipeline"]
  ANT["/v1/messages"] --> Pipe
  GEM["/v1beta/models/..."] --> Pipe
  Pipe --> Resp["Native Schema + Headers"]
  style OAI fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style ANT fill:#f3e8ff,stroke:#9333ea,color:#581c87
  style GEM fill:#e0f2fe,stroke:#0284c7,color:#0c4a6e
  style Pipe fill:#dcfce7,stroke:#16a34a,color:#14532d
  style Resp fill:#fef9c3,stroke:#ca8a04,color:#713f12
```

### 1. OpenAI Chat Completions — `POST /v1/chat/completions`

**Headers**:
- `Authorization: Bearer <TEAM_API_KEY>` (or `X-Team-Key: <TEAM_API_KEY>`)
- `X-Tool-Name: <tool-id>` (optional, recorded in audit logs)
- `X-Gateway-Route: cost | quality | latency` (optional routing hint when `model` is `auto`)
- `X-Gateway-Provider: anthropic | gemini | grok | deepseek | qwen | openai | mistral | workers-ai` (optional vendor hint)
- `X-Gateway-Strict-Provider: true` (optional: fail with `503` if vendor key is unconfigured instead of falling back to Workers AI)

**Example (`curl`)**:
```bash
curl -s http://localhost:9900/v1/chat/completions \
  -H "Authorization: Bearer gw_platform_demo_key" \
  -H "X-Tool-Name: incident-bot" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [
      {"role": "system", "content": "Summarize concisely."},
      {"role": "user", "content": "Customer alice@acme.com hit error 502 from 10.0.1.42"}
    ]
  }'
```

**Response Headers**:
- `X-Request-ID`
- `X-Gateway-Protocol`: `openai` | `anthropic` | `gemini`
- `X-Gateway-Team`: resolved `team_id`
- `X-Gateway-Routed-Model`: actual model invoked
- `X-Gateway-Provider`: upstream provider invoked
- `X-Gateway-PII-Action`: `none` | `redacted` | `blocked` | `audited`
- `X-Gateway-PII-Detected`: integer count of findings
- `X-Gateway-Cost-Micros`: micro-dollars charged
- `X-Gateway-Remaining-Micros`: remaining team monthly budget

**Python OpenAI SDK**:
```python
from openai import OpenAI

client = OpenAI(
    base_url="https://your-worker.workers.dev/v1",
    api_key="gw_platform_demo_key",
    default_headers={"X-Tool-Name": "billing-sync"},
)
resp = client.chat.completions.create(
    model="auto",  # or "grok-3", "deepseek-chat", "qwen-plus", "llama-4-scout"
    messages=[{"role": "user", "content": "Explain retry jitter in 2 sentences."}],
)
print(resp.choices[0].message.content)
```

---

### 2. Anthropic Messages API — `POST /v1/messages` & `POST /v1/messages/count_tokens`

```mermaid
flowchart LR
  CC["Claude Code / SDK"] --> Ant["POST /v1/messages"]
  Ant --> SSE["SSE & Tool Calls"]
  Ant --> Cnt["count_tokens"]
  style CC fill:#f3e8ff,stroke:#9333ea,color:#581c87
  style Ant fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style SSE fill:#dcfce7,stroke:#16a34a,color:#14532d
  style Cnt fill:#fef9c3,stroke:#ca8a04,color:#713f12
```

Compatible with **Claude Code** (`ANTHROPIC_BASE_URL`) and Anthropic SDK clients (supports `stream: true` SSE, `tools` / `tool_use` / `tool_result`, and `count_tokens`).

```bash
curl -s http://localhost:9900/v1/messages \
  -H "x-api-key: gw_platform_demo_key" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "max_tokens": 256,
    "system": "Be concise.",
    "messages": [{"role": "user", "content": "Contact bob@corp.io"}]
  }'
```

**Python Anthropic SDK**:
```python
import anthropic

client = anthropic.Anthropic(
    base_url="https://your-worker.workers.dev",
    api_key="gw_platform_demo_key",
)
msg = client.messages.create(
    model="claude-sonnet-4-5",
    max_tokens=256,
    messages=[{"role": "user", "content": "Summarize Q3 latency regressions."}],
)
print(msg.content[0].text)
```

---

### 3. Google Gemini API — `POST /v1beta/models/{model}:generateContent` & `:streamGenerateContent`

```mermaid
flowchart LR
  AGY["Antigravity / GenAI"] --> Gem[":generateContent"]
  Gem --> Fn["functionDeclarations"]
  Gem --> Str["?alt=sse Stream"]
  style AGY fill:#e0f2fe,stroke:#0284c7,color:#0c4a6e
  style Gem fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style Fn fill:#dcfce7,stroke:#16a34a,color:#14532d
  style Str fill:#fef9c3,stroke:#ca8a04,color:#713f12
```

Also available at `/v1/models/{model}:generateContent` and `GET /v1beta/models`. Compatible with **Google Antigravity CLI/SDK** (`GOOGLE_GEMINI_BASE_URL`), Gemini CLI, and Google GenAI SDKs (supports SSE `?alt=sse` and `functionDeclarations` tool calling). Accepts team API key in `x-goog-api-key`, `Authorization: Bearer`, `X-Team-Key`, or `?key=`.

```bash
curl -s http://localhost:9900/v1beta/models/gemini-2.5-flash:generateContent \
  -H "x-goog-api-key: gw_platform_demo_key" \
  -H "Content-Type: application/json" \
  -d '{
    "systemInstruction": {"parts": [{"text": "Answer in one line."}]},
    "contents": [{"role": "user", "parts": [{"text": "What is edge caching?"}]}]
  }'
```

---

## Discovery & Health Endpoints

- `GET /healthz`: Returns `{ "status": "ok", "store": "...", "provider": "...", "time": "..." }`.
- `GET /v1/models`: Returns the OpenAI-formatted model list including tier, aliases, and micro-dollar pricing.
- `GET /`: Serves the embedded HTML Control Plane Dashboard (or JSON endpoint list with `Accept: application/json`).

---

## Admin Control-Plane Endpoints

```mermaid
flowchart LR
  Tok["ADMIN_TOKEN"] --> T["/admin/teams"]
  Tok --> A["/admin/audit"]
  Tok --> S["/admin/summary"]
  Tok --> P["/admin/providers"]
  style Tok fill:#fee2e2,stroke:#dc2626,color:#7f1d1d
  style T fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style A fill:#f3e8ff,stroke:#9333ea,color:#581c87
  style S fill:#dcfce7,stroke:#16a34a,color:#14532d
  style P fill:#fef9c3,stroke:#ca8a04,color:#713f12
```

All `/admin/*` endpoints require `Authorization: Bearer <ADMIN_TOKEN>` or `X-Admin-Token: <ADMIN_TOKEN>`.

### List Teams — `GET /admin/teams`
Returns all teams, monthly limits, current spend, remaining budget, `max_tier`, `pii_mode`, and `enabled` status.

### Create or Update Team — `POST /admin/teams`
```bash
curl -s http://localhost:9900/admin/teams \
  -H "Authorization: Bearer gw_admin_dev" \
  -H "Content-Type: application/json" \
  -d '{
    "team_id": "data-eng",
    "team_name": "Data Engineering",
    "api_key": "gw_dataeng_secret_key",
    "monthly_limit_micros": 100000000,
    "max_tier": "reasoning",
    "pii_mode": "redact",
    "enabled": true
  }'
```
*(When updating an existing team, `api_key` is optional unless rotating credentials.)*

### Get Team — `GET /admin/teams/{team_id}`
Returns the single team policy and spend status.

### Reset Team Budget — `POST /admin/teams/{team_id}/reset-budget`
Resets `spent_micros` to `0` for the current month.

### Query Audit Log — `GET /admin/audit`
Optional query parameters:
- `team_id` — filter by team
- `status` — `ok`, `blocked_budget`, `blocked_pii`, `blocked_policy`, `upstream_error`, `unauthorized`
- `model` — match either `requested_model` or `routed_model`
- `pii_only=1` — return only requests with `pii_detected > 0`
- `limit` — max rows (default `50`, max `200`)

### Aggregate Summary — `GET /admin/summary`
Returns global totals (`total_requests`, `successful_count`, `blocked_budget`, `blocked_pii`, `upstream_errors`, `total_pii_detected`, `total_tokens`, `total_cost_micros`) and per-team spend/request breakdowns.

