# Local Machine Demonstration Guide (`local-deployment.md`)

Run and demonstrate every feature of **Go AI Gateway** on a local machine in under 2 minutes—no Cloudflare account or external API keys required (with optional hooks for live vendor keys, Ollama, and local Miniflare D1).

---

## Mode 1: Zero-Config Instant Demo (Recommended for First Run)

If you want to run a live demo immediately using the built-in mock multi-vendor provider and pre-seeded demo teams (`platform`, `support`, `sandbox`):

```bash
# Run with an isolated empty env if you already created a .env with PERSONAL_MODE=true,
# or simply run `go run .` if you do not have a .env file yet.
PERSONAL_MODE=false go run .
```

The gateway starts on **`http://localhost:9900`** (override with `PORT=8080`) with:
- **Admin Token**: `gw_admin_dev` (override with `ADMIN_TOKEN=...`)
- **Pre-seeded Demo Teams**:
  - `platform` — key `gw_platform_demo_key` (`$50.00/mo`, max tier `reasoning`, PII mode `redact`)
  - `support` — key `gw_support_demo_key` (`$15.00/mo`, max tier `balanced`, PII mode `block`)
  - `sandbox` — key `gw_sandbox_demo_key` (`$1.00/mo`, max tier `fast`, PII mode `redact`)

Open **`http://localhost:9900/`** in your browser, enter `gw_admin_dev` in the **Admin token** box, and click **Refresh**.

---

## Mode 2: Local Demo with Real AI Keys, Ollama & Persistent State (`.env`)

To demonstrate live inference against Anthropic, Gemini, Grok, DeepSeek, Qwen, OpenAI, or local **Ollama** on your machine:

1. Create `.env` from [.env.example](.env.example):
   ```bash
   cp .env.example .env && chmod 600 .env
   ```
2. Edit `.env`:
   - Set `PERSONAL_MODE=false` if you want the 3 demo teams (`gw_platform_demo_key`, `gw_support_demo_key`, `gw_sandbox_demo_key`) seeded automatically; or keep `PERSONAL_MODE=true` for a single `"personal"` workspace (`gw_personal_secret_change_me`).
   - Add API keys for any providers you want live (leave others blank to fall back to Workers AI mock).
   - Optionally set `GATEWAY_DATA_FILE=./gateway-state.json` to persist spend and audit logs across restarts.
   - Optionally start Ollama (`ollama serve && ollama pull llama3.2`) and set `OLLAMA_BASE_URL=http://localhost:11434/v1`.
3. Launch via the bundled script:
   ```bash
   ./deploy-cloudflare.sh --target local
   ```

---

## Mode 3: Local Cloudflare Workers / Pages Wasm Simulator (Miniflare + D1)

To demonstrate the **exact TinyGo WebAssembly binary** and **local Cloudflare D1 SQLite** before deploying to Cloudflare:

```bash
npm install
make build-wasm
npx wrangler d1 migrations apply ai-gateway-db --local
cp .dev.vars.example .dev.vars

# Option A: Local Worker API + embedded dashboard (http://localhost:8787)
npm run dev

# Option B: Local Cloudflare Pages UI + Pages Functions (http://localhost:8788)
npm run dev:pages
```

---

## 5-Minute Live Demo Script (`curl`)

Run these commands in a second terminal while the gateway is running on `http://localhost:9900`:

### 1. Health & Multi-Vendor Model Catalog

```bash
curl -s http://localhost:9900/healthz | jq .
curl -s http://localhost:9900/v1/models | jq '.data | length, .[0:5]'
```

### 2. Automatic Personal-Data & Secret Redaction (`platform` team)

```bash
curl -s http://localhost:9900/v1/chat/completions \
  -H "Authorization: Bearer gw_platform_demo_key" \
  -H "X-Tool-Name: support-cli" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{
      "role": "user",
      "content": "Customer alice@acme.com from 10.24.8.15 sent SSN 123-45-6789 and key sk-proj-abcdefghijklmnopqrstuvwxyz1234."
    }]
  }' | jq '{model, choice: .choices[0].message.content, gateway}'
```
Point out:
- `gateway.pii_action` is `"redacted"` and `gateway.pii_findings` reports counts (`email`, `ipv4`, `ssn`, `api_key`) without storing raw values.
- The echoed prompt received by the model contains `[REDACTED_EMAIL_1]`, `[REDACTED_IPV4_1]`, `[REDACTED_SSN_1]`, and `[REDACTED_API_KEY_1]`.

### 3. Strict PII Blocking (`support` team, `pii_mode: block`)

```bash
curl -i -s http://localhost:9900/v1/chat/completions \
  -H "Authorization: Bearer gw_support_demo_key" \
  -H "X-Tool-Name: ticket-bot" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "balanced",
    "messages": [{"role": "user", "content": "Email reset link to bob@customer.example"}]
  }'
```
Point out: returns **`HTTP 400`** (`pii_policy_violation`), charges `$0`, and records `status: "blocked_pii"` in the audit log.

### 4. Smart Model Routing & Team Tier Policy (`sandbox` team)

```bash
# Auto-routes code/SQL prompts to Qwen 2.5 Coder 32B for `platform`:
curl -s http://localhost:9900/v1/chat/completions \
  -H "Authorization: Bearer gw_platform_demo_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Refactor this Golang SQL JOIN query"}]}' \
  | jq '.gateway | {routed_model, route_reason}'

# `sandbox` (max_tier: fast) explicitly requesting reasoning model -> HTTP 403 policy_error:
curl -i -s http://localhost:9900/v1/chat/completions \
  -H "Authorization: Bearer gw_sandbox_demo_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-r1","messages":[{"role":"user","content":"Analyze architecture"}]}'
```

### 5. Anthropic & Gemini Inbound Protocols (with Streaming & Tools)

```bash
# Anthropic Messages API (Claude Code / Anthropic SDK format, SSE streaming):
curl -N -s http://localhost:9900/v1/messages \
  -H "x-api-key: gw_platform_demo_key" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "stream": true,
    "max_tokens": 128,
    "messages": [{"role": "user", "content": "Summarize why gateways prevent PII leaks."}]
  }'

# Google Gemini API (Antigravity CLI / Gemini SDK format):
curl -s http://localhost:9900/v1beta/models/gemini-2.5-flash:generateContent \
  -H "x-goog-api-key: gw_platform_demo_key" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"role":"user","parts":[{"text":"Explain per-team AI budgets in one line."}]}]}' \
  | jq .
```

### 6. Per-Team Budget Exhaustion (`HTTP 429`)

Create a tiny `$0.0005` (`500` micro-dollar) demo team and trigger a `429 insufficient_quota` block:

```bash
curl -s -X POST http://localhost:9900/admin/teams \
  -H "Authorization: Bearer gw_admin_dev" \
  -H "Content-Type: application/json" \
  -d '{"team_id":"tiny","team_name":"Tiny Cap Demo","api_key":"gw_tiny_demo","monthly_limit_micros":500,"max_tier":"fast","pii_mode":"redact"}' | jq .

curl -i -s http://localhost:9900/v1/chat/completions \
  -H "Authorization: Bearer gw_tiny_demo" \
  -H "Content-Type: application/json" \
  -d '{"model":"fast","max_tokens":5000,"messages":[{"role":"user","content":"Hello"}]}'
```

### 7. Inspect Full Audit Log, Spend Summary & Provider Caps

```bash
curl -s http://localhost:9900/admin/summary -H "Authorization: Bearer gw_admin_dev" | jq .
curl -s "http://localhost:9900/admin/audit?limit=10" -H "Authorization: Bearer gw_admin_dev" | jq .
curl -s http://localhost:9900/admin/providers -H "Authorization: Bearer gw_admin_dev" | jq .
```

