# Go AI Gateway — AI Traffic Control Plane

**Problem**: Every team calls LLMs its own way, so spend, data leaks, and audit are invisible.

**Solution**: A single Go gateway compiled with **TinyGo** to **Cloudflare Workers**, backed by **Cloudflare D1** and **Workers AI** (plus native support for **Anthropic**, **Google Gemini**, **xAI Grok**, **Meta Llama**, **DeepSeek**, **Alibaba Qwen**, **OpenAI**, and **Mistral**). Internal tools point their existing SDKs to the gateway instead of calling model providers directly.

## Key Capabilities

```mermaid
flowchart LR
  SDK["SDK Clients"] --> GW["Go AI Gateway"]
  GW --> PII["PII Redaction"]
  PII --> BUD["Budget Gate"]
  BUD --> RT["Model Router"]
  RT --> UP["AI Providers"]
  style SDK fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style GW fill:#dcfce7,stroke:#16a34a,color:#14532d
  style PII fill:#fef9c3,stroke:#ca8a04,color:#713f12
  style BUD fill:#ffedd5,stroke:#ea580c,color:#7c2d12
  style RT fill:#f3e8ff,stroke:#9333ea,color:#581c87
  style UP fill:#e0f2fe,stroke:#0284c7,color:#0c4a6e
```

- **Multi-Protocol Inbound Compatibility**:
  - OpenAI Chat Completions: `POST /v1/chat/completions`
  - Anthropic Messages API: `POST /v1/messages`
  - Google Gemini API: `POST /v1beta/models/{model}:generateContent`
- **Multi-Vendor Model Routing & Fallbacks**:
  - Native routing to **Workers AI** (Meta Llama 3.1/3.3/4, Qwen 2.5 Coder, DeepSeek R1 Distill, Mistral 7B), **Anthropic** (`claude-haiku-4-5`, `claude-sonnet-4-5`, `claude-opus-4-1`), **Google Gemini** (`gemini-2.5-flash`, `gemini-2.5-pro`), **xAI Grok** (`grok-3`, `grok-3-mini`), **DeepSeek** (`deepseek-chat`, `deepseek-reasoner`), **Qwen** (`qwen-turbo`, `qwen-plus`, `qwen-max`), **OpenAI** (`gpt-4o-mini`, `gpt-4o`, `o3-mini`), **Mistral**, or any custom OpenAI-compatible endpoint (`UPSTREAM_BASE_URL`).
  - Automatic `auto` complexity/code/cost routing, per-team `max_tier` enforcement, and cross-provider fallback chains.
- **Per-Team Budgets (D1)**:
  - `SHA-256` hashed team keys, integer micro-dollar accounting, atomic single-statement reservation in D1, UTC monthly rollover, post-flight cost reconciliation, and full refunds on upstream failure.
- **Personal-Data & Secret Redaction**:
  - Detects and redacts emails, US SSNs, Luhn-validated credit cards, phone numbers, IPv4 addresses, JWTs, Bearer tokens, and API keys (`sk-...`, `ghp_...`, `AKIA...`) before leaving the gateway.
  - Per-team `pii_mode`: `redact`, `block`, or `audit_only`.
- **Full Audit Log & Embedded Dashboard**:
  - Immutable `audit_logs` records for every allowed, blocked, or failed request (logging finding counts and redacted prompt SHA-256 hashes — never raw PII) plus a built-in web dashboard at `/`.

## Quick Start (Local Standalone Mode)

```mermaid
flowchart LR
  Run["go run ."] --> Port["localhost:9900"]
  Port --> UI["Dashboard UI"]
  Port --> API["OpenAI / Anthropic / Gemini"]
  API --> Mock["Local Mock or Live Keys"]
  style Run fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style Port fill:#dcfce7,stroke:#16a34a,color:#14532d
  style UI fill:#f3e8ff,stroke:#9333ea,color:#581c87
  style API fill:#fef9c3,stroke:#ca8a04,color:#713f12
  style Mock fill:#e0f2fe,stroke:#0284c7,color:#0c4a6e
```

```bash
go test -race ./...
go run .
```

Local mode starts on `http://localhost:9900` with an in-memory store pre-seeded with demo teams (`gw_platform_demo_key`, `gw_support_demo_key`, `gw_sandbox_demo_key`) and dev admin token `gw_admin_dev`. If no vendor API keys are exported, local mock mode simulates all providers so every model and SDK protocol can be tested immediately.

Open `http://localhost:9900/` for the Control Plane Dashboard, or test with `curl`:

```bash
curl -s http://localhost:9900/v1/chat/completions \
  -H "Authorization: Bearer gw_platform_demo_key" \
  -H "X-Tool-Name: support-cli" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [{"role": "user", "content": "Help alice@acme.com with SSN 123-45-6789"}]
  }'
```

## Quick Deploy Scripts (`.env` + Automated Scripts)

```mermaid
flowchart TD
  Env[".env / .env.company"] --> C["deploy-company.sh"]
  Env --> P["deploy-personal.sh"]
  Env --> CF["deploy-cloudflare.sh"]
  C --> Prod["Prod Worker + D1"]
  P --> Pers["Local or Free Tier"]
  CF --> Demo["Worker + Pages Demo"]
  style Env fill:#fef9c3,stroke:#ca8a04,color:#713f12
  style C fill:#fee2e2,stroke:#dc2626,color:#7f1d1d
  style P fill:#dcfce7,stroke:#16a34a,color:#14532d
  style CF fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style Prod fill:#f3e8ff,stroke:#9333ea,color:#581c87
  style Pers fill:#e0f2fe,stroke:#0284c7,color:#0c4a6e
  style Demo fill:#ffedd5,stroke:#ea580c,color:#7c2d12
```

```bash
# 1. Enterprise production deployment (hardened guardrails + team manifest provisioning):
./deploy-company.sh --init-only
./deploy-company.sh --teams-file teams.json

# 2. Personal multi-subscription deployment (local systemd or Cloudflare Free Tier):
./deploy-personal.sh --init-only
./deploy-personal.sh --mode local --install-service

# 3. Direct Cloudflare Worker / Pages demo deployment:
cp .env.example .env && chmod 600 .env
./deploy-cloudflare.sh --target both --seed-demo
```

## Documentation

```mermaid
flowchart LR
  Docs["Documentation"] --> Dep["Deploy Guides"]
  Docs --> Ref["API & Architecture"]
  Docs --> Gov["Metrics & Profiles"]
  style Docs fill:#dbeafe,stroke:#2563eb,color:#1e3a8a
  style Dep fill:#dcfce7,stroke:#16a34a,color:#14532d
  style Ref fill:#f3e8ff,stroke:#9333ea,color:#581c87
  style Gov fill:#fef9c3,stroke:#ca8a04,color:#713f12
```

- [Local Machine Demo Guide](local-deployment.md) — 2-minute local setup + 5-minute live `curl` walkthrough (PII, budgets, routing, Anthropic/Gemini/OpenAI protocols)
- [Cloudflare Pages Demo & Deployment Bundle Guide](docs/cloudflare-pages-demo.md) — Where & how to deploy with `deploy-cloudflare.sh` + `.env` and run the live Cloudflare Pages demo
- [Claude Desktop Deployment Instructions](claude-desktop-deployment.md) — MCP setup, project instructions, and safety playbook for Claude Desktop to deploy to Cloudflare
- [Enterprise Deployment Guide](company-deployment.md) — Production Cloudflare Workers, D1, Zero Trust, multi-team onboarding, and compliance
- [Personal Multi-Subscription Deployment Guide](personal-deployment.md) — Running locally (`PERSONAL_MODE=true`) or on Cloudflare Free Tier with per-provider spend caps (`PROVIDER_LIMITS_USD`) and Ollama/Workers AI overflow
- [Architecture Guide](docs/architecture.md) — Request lifecycle, package layout, TinyGo/Wasm and D1 design decisions
- [Providers, Models & Routing](docs/providers-and-routing.md) — Anthropic, Gemini, Grok, Meta, DeepSeek, Qwen, OpenAI, Mistral, Workers AI, and routing rules
- [Budgets, PII Redaction & Audit Policy](docs/policies-budgets-pii.md) — Micro-dollar accounting, atomic D1 reservation, PII/secret detection, and audit guarantees
- [API Reference](docs/api-reference.md) — OpenAI/Anthropic/Gemini inbound endpoints, headers, SDK snippets, and Admin API
- [Deploying to Cloudflare Workers](docs/deployment-cloudflare.md) — TinyGo Wasm build, D1 migrations, secrets, and `wrangler` deployment
- [Project Metrics](metrics.md) | [Test Time Report](test-time-report.md) | [System Profile](system-profile.md) | [Next Steps](next-steps.md) | [License (AGPL-v3)](LICENSE)
