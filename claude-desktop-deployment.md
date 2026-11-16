# Claude Desktop Instructions for Cloudflare Deployment

Use this guide to let **Claude Desktop** handle building, configuring, and deploying **Go AI Gateway** to Cloudflare Workers, D1, Workers AI, and Cloudflare Pages.

---

## 1. Prerequisites for Claude Desktop

Claude Desktop needs tool access to your local workspace and shell (via MCP servers) to run `deploy-cloudflare.sh` and `wrangler`.

### Recommended `claude_desktop_config.json` Setup

Add a filesystem/terminal MCP server scoped to your `go-ai-gateway` directory (plus optionally the official Cloudflare MCP server) in your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS, `~/.config/Claude/claude_desktop_config.json` on Linux):

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/go-ai-gateway"]
    },
    "desktop-commander": {
      "command": "npx",
      "args": ["-y", "@wonderwhy-er/desktop-commander"]
    }
  }
}
```

> **Security Rule**: Put your real API keys directly into `.env` on disk (`cp .env.example .env && chmod 600 .env`) rather than pasting secret values into Claude Desktop chat messages. Claude Desktop can run `./deploy-cloudflare.sh` without ever reading or printing `.env` secrets.

---

## 2. Project Instructions for Claude Desktop

Create a Claude Desktop **Project** for `go-ai-gateway` and paste the block below into the Project's **Custom Instructions**:

````markdown
You are assisting with deploying the `go-ai-gateway` repository (Go + TinyGo + Cloudflare Workers + D1 + Workers AI + Cloudflare Pages) to Cloudflare.

STRICT SAFETY RULES:
1. NEVER read, print, echo, or display the contents of `.env` or `.dev.vars`, and NEVER ask the user to paste raw API keys into chat.
2. Verify `.env` exists (without printing its values) by checking `test -f .env`. If missing, instruct the user to copy `.env.example` to `.env` locally.
3. For enterprise/production deployments, NEVER pass `--seed-demo` and confirm `SEED_DEMO_TEAMS` is not `true`. Only pass `--seed-demo` when the user explicitly asks for a demo environment.
4. Always run `./deploy-cloudflare.sh --dry-run` first and show the planned steps before executing the live deployment.

DEPLOYMENT PLAYBOOK:
1. Verify workspace and tools:
   `go version && tinygo version && node -v && npm -v`
2. Verify `.env` exists (`test -f .env`) and confirm `.gitignore` excludes `.env`.
3. Verify Cloudflare authentication:
   `npx wrangler whoami`
   (If unauthenticated, tell the user to run `npx wrangler login` in their terminal or set `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in `.env`.)
4. Run Go tests and check the TinyGo Wasm build:
   `go test -race ./... && make build-wasm`
5. Preview deployment:
   `./deploy-cloudflare.sh --target <worker|pages|both> [--seed-demo] --dry-run`
6. Execute deployment:
   `./deploy-cloudflare.sh --target <worker|pages|both> [--seed-demo]`
7. Verify deployed endpoints (never printing secret tokens):
   - `curl -fsS <DEPLOYED_URL>/healthz`
   - `curl -fsS <DEPLOYED_URL>/v1/models`
   - Summarize the deployed Worker/Pages URLs, D1 database ID, and next steps (such as onboarding teams via `POST /admin/teams` or opening the Pages demo UI).
````

---

## 3. Ready-to-Use Prompt for Claude Desktop

Once your `.env` file is saved on disk and `npx wrangler login` is completed, send this prompt to Claude Desktop:

> **"Deploy `/path/to/go-ai-gateway` to Cloudflare using `--target both` (or `--target worker` for production without demo seed). Follow `claude-desktop-deployment.md`: do not read or print `.env`, run tests and `--dry-run` first, execute `./deploy-cloudflare.sh`, and verify `/healthz` and `/v1/models` after deployment."**

---

## 4. What Claude Desktop Should Check If Any Step Fails

| Symptom | Cause | Resolution for Claude Desktop |
| --- | --- | --- |
| `tinygo: command not found` or TinyGo `< 0.42` | TinyGo missing or outdated (TinyGo `< 0.42` cannot compile `net/http` to Wasm) | Ask user to install TinyGo `0.42+` (e.g. `0.44.0`) and re-run `make build-wasm` |
| `Not authenticated to Cloudflare` | `wrangler login` session expired and `CLOUDFLARE_API_TOKEN` not in `.env` | Ask user to run `npx wrangler login` or set `CLOUDFLARE_API_TOKEN` + `CLOUDFLARE_ACCOUNT_ID` in `.env` |
| `D1 create` fails with duplicate name | Database already exists on the Cloudflare account | `deploy-cloudflare.sh` checks `npx wrangler d1 list --json` first; verify `D1_DATABASE_NAME` in `.env` matches the existing DB |
| Pages demo gets CORS or `401` on `/admin/*` | Wrong `ADMIN_TOKEN` or Pages pointing to wrong Worker URL | Confirm `ADMIN_TOKEN` is set in Worker/Pages secrets (`npx wrangler secret list`) and matches the dashboard input |
| Wasm size exceeds Cloudflare Workers 3 MB free limit | Debug symbols included | Ensure build uses `-no-debug` (`tinygo build -o ./build/app.wasm -target wasm -no-debug ./...`; outputs ~2.4 MB uncompressed, ~700 KB gzipped) |

---

## 5. Manual Step-by-Step Alternative (If Claude Desktop Has Cloudflare MCP Only, No Local Shell)

If your Claude Desktop only has the **Cloudflare MCP** (and no local shell MCP):
1. Run `make build-wasm` locally to produce `./build/`.
2. Ask Claude Desktop (via Cloudflare MCP) to:
   - Create/verify the D1 database `ai-gateway-db` and apply the SQL from `migrations/0001_init.sql`
   - Verify the Workers AI binding (`AI`) and Worker deployment status
   - Inspect D1 `team_budgets` and `audit_logs` tables cleanly without touching local `.env` files.

