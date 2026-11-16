# Where & How to Deploy + Cloudflare Pages Live Demo Guide

This guide explains **where** each component runs, **how** to deploy in one command with `./deploy-cloudflare.sh` and your `.env` API keys, and **how to run a live interactive demo on Cloudflare Pages**.

---

## 1. Where to Deploy (Choose Your Target)

| `--target` | Where It Runs | Best For | URLs Created |
| --- | --- | --- | --- |
| **`worker`** *(default)* | Cloudflare Workers + D1 + Workers AI | Production enterprise or personal API gateway | `https://<worker>.workers.dev` |
| **`pages`** | Cloudflare Pages (`pages/index.html` + `functions/` TinyGo Wasm + D1 + Workers AI) | Single-URL web demo + gateway API on Pages | `https://<project>.pages.dev` |
| **`both`** | Worker backend (`worker.mjs`) + static Pages frontend (`pages/index.html`) | Production separation: static Pages UI calling Worker API via CORS | `https://<project>.pages.dev` + `https://<worker>.workers.dev` |
| **`local`** | Your laptop / VPS (`go run .` + `.env` + local JSON file store) | Local testing or personal self-hosting | `http://localhost:9900` |

### Where Secrets, Data, and Models Live

- **API Keys & Secrets** (`ADMIN_TOKEN`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `XAI_API_KEY`, `DEEPSEEK_API_KEY`, `QWEN_API_KEY`, `OPENAI_API_KEY`, `MISTRAL_API_KEY`, `PROVIDER_LIMITS_USD`): Stored encrypted in **Cloudflare Worker/Pages Secrets** (uploaded from `.env` by `deploy-cloudflare.sh`).
- **Team Budgets & Audit Logs**: Stored in **Cloudflare D1** (`ai-gateway-db`, SQLite at the edge).
- **Zero-Key Default Models**: Executed directly on **Cloudflare Workers AI** via the `env.AI` binding (Meta Llama 3.1/3.3/4, Qwen 2.5 Coder, DeepSeek R1 Distill, Mistral 7B).

---

## 2. One-Command Deployment with `.env` (`deploy-cloudflare.sh`)

### Step 1: Create `.env` with Your API Keys

```bash
cp .env.example .env
chmod 600 .env
```

Edit `.env` and fill in:
- `ADMIN_TOKEN` — strong secret for `/admin/*` and the web dashboard
- Optional `CLOUDFLARE_API_TOKEN` & `CLOUDFLARE_ACCOUNT_ID` (if not logged in via `npx wrangler login`)
- Any external vendor keys you want active (`ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `XAI_API_KEY`, `DEEPSEEK_API_KEY`, `QWEN_API_KEY`, `OPENAI_API_KEY`, `MISTRAL_API_KEY`)
- Optional `PROVIDER_LIMITS_USD` (e.g. `anthropic=5,openai=5,grok=3`)

### Step 2: Dry-Run (Recommended First Check)

```bash
./deploy-cloudflare.sh --target both --seed-demo --dry-run
```

### Step 3: Deploy!

Choose one:

```bash
# Option 1 (Recommended for Demo): Worker API + Cloudflare Pages UI
./deploy-cloudflare.sh --target both --seed-demo

# Option 2: Single-project Cloudflare Pages (UI + Pages Functions Wasm)
./deploy-cloudflare.sh --target pages --seed-demo

# Option 3: API-only Cloudflare Worker (for production without demo UI)
./deploy-cloudflare.sh --target worker
```

`deploy-cloudflare.sh` automatically:
1. Runs `go test -race ./...`
2. Builds `./build/app.wasm` (`~2.4 MB`) with TinyGo and `workers-assets-gen`
3. Creates the Cloudflare D1 database if it does not exist and patches `wrangler.jsonc` / `wrangler.pages.jsonc`
4. Applies `migrations/0001_init.sql` (and `migrations/0002_seed.sql` when `--seed-demo` is passed)
5. Uploads every non-empty API key from `.env` via `wrangler secret put`
6. Deploys the Worker and/or Pages project and verifies `/healthz`

---

## 3. How to Demo on Cloudflare Pages

Open your deployed Pages URL (`https://<PAGES_PROJECT_NAME>.pages.dev`):

1. **Connect the Dashboard**:
   - If you deployed with `--target both`, **Gateway URL** is pre-populated with your `https://<worker>.workers.dev` URL.
   - If you deployed with `--target pages`, leave **Gateway URL** blank (same-origin Pages Functions).
   - Enter your `ADMIN_TOKEN` from `.env` and click **Connect & Refresh**.

2. **Walk Through the 5 Built-In Demo Scenarios**:
   - **Scenario 1 — PII Redaction (`1. PII Redaction`)**:
     Uses `gw_platform_demo_key` (`pii_mode: redact`). Sends an email, IPv4, SSN, and `sk-proj-...` key. Show the response's `gateway.pii_findings` and notice the model only ever receives `[REDACTED_EMAIL_1]`, `[REDACTED_IPV4_1]`, `[REDACTED_SSN_1]`, and `[REDACTED_API_KEY_1]`.
   - **Scenario 2 — Strict PII Blocking (`2. PII Strict Block`)**:
     Uses `gw_support_demo_key` (`pii_mode: block`). Returns `HTTP 400` (`pii_policy_violation`), charges `$0`, and logs `status = blocked_pii` in the Audit Log table.
   - **Scenario 3 — Auto Complexity/Code Routing (`3. Auto Code Routing`)**:
     Sends a Golang + SQL prompt with `model: "auto"`. The router automatically selects `@cf/qwen/qwen2.5-coder-32b-instruct` (`route_reason: auto_code_detected`).
   - **Scenario 4 — Multi-Vendor Protocol (`4. Anthropic / Claude Route`)**:
     Switches inbound protocol to Anthropic `POST /v1/messages` (`x-api-key`) requesting `claude-sonnet-4-5`. If `ANTHROPIC_API_KEY` is set in `.env`, it calls Anthropic directly; if unset (or capped by `PROVIDER_LIMITS_USD`), it seamlessly falls back to Workers AI Llama 3.3 70B!
   - **Scenario 5 — Gemini / Antigravity Protocol (`5. Gemini / Antigravity Protocol`)**:
     Sends a Google Gemini `POST /v1beta/models/gemini-2.5-flash:generateContent` request (`x-goog-api-key`) and inspects `usageMetadata` + unified audit logging.

3. **Show Budget Enforcement & Audit Visibility**:
   - Point out the **Per-Team Budgets & Policies** table, **Per-Provider Spend vs. Monthly Caps** table (`/admin/providers`), and the **Audit Log** confirming zero raw PII is stored.

---

## 4. Cleaning Up After a Demo

If you used `--seed-demo`, remove or disable the public demo keys before using the deployment for real workloads:

```bash
# Disable demo keys via Admin API or delete rows in D1:
npx wrangler d1 execute ai-gateway-db --remote --yes \
  --command="DELETE FROM team_budgets WHERE team_id IN ('platform','support','sandbox');"
```

