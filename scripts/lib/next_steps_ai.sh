#!/usr/bin/env bash
# AI backend helper for scripts/update-next-steps.sh (never prints secrets).

load_ai_env_silent() {
  local f
  for f in ".env.personal" ".env" "${HOME}/.config/go-ai-gateway/env"; do
    [[ -f "$f" ]] || continue
    while IFS='=' read -r k v; do
      [[ -z "$k" || "$k" =~ ^[[:space:]]*# ]] && continue
      k="${k#export }"
      k="${k//[[:space:]]/}"
      [[ "$k" =~ ^[A-Z0-9_]+$ ]] || continue
      [[ -n "${!k:-}" ]] && continue
      v="${v#\"}" ; v="${v%\"}" ; v="${v#\'}" ; v="${v%\'}"
      export "${k}=${v}"
    done < "$f"
  done
}

call_openai_compat_ai() {
  local url="$1" key="$2" model="$3" prompt="$4"
  local payload resp
  payload="$(python3 -c 'import json,sys; print(json.dumps({"model":sys.argv[1],"max_tokens":700,"messages":[{"role":"user","content":sys.argv[2]}]}))' "$model" "$prompt")"
  resp="$(curl -fsS --max-time 20 "$url" \
    -H "Content-Type: application/json" \
    ${key:+-H "Authorization: Bearer ${key}"} \
    -d "$payload" 2>/dev/null)" || return 1
  python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["choices"][0]["message"]["content"])' <<< "$resp" 2>/dev/null
}

call_anthropic_ai() {
  local prompt="$1" payload resp
  payload="$(python3 -c 'import json,sys; print(json.dumps({"model":"claude-haiku-4-5","max_tokens":700,"messages":[{"role":"user","content":sys.argv[1]}]}))' "$prompt")"
  resp="$(curl -fsS --max-time 20 "https://api.anthropic.com/v1/messages" \
    -H "Content-Type: application/json" \
    -H "x-api-key: ${ANTHROPIC_API_KEY}" \
    -H "anthropic-version: 2023-06-01" \
    -d "$payload" 2>/dev/null)" || return 1
  python3 -c 'import json,sys; d=json.load(sys.stdin); print("".join(b.get("text","") for b in d.get("content",[])))' <<< "$resp" 2>/dev/null
}

call_gemini_ai() {
  local prompt="$1" payload resp
  payload="$(python3 -c 'import json,sys; print(json.dumps({"contents":[{"role":"user","parts":[{"text":sys.argv[1]}]}]}))' "$prompt")"
  resp="$(curl -fsS --max-time 20 "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent" \
    -H "Content-Type: application/json" \
    -H "x-goog-api-key: ${GEMINI_API_KEY}" \
    -d "$payload" 2>/dev/null)" || return 1
  python3 -c 'import json,sys; d=json.load(sys.stdin); print("".join(p.get("text","") for p in d["candidates"][0]["content"]["parts"]))' <<< "$resp" 2>/dev/null
}

generate_ai_suggestions() {
  local prompt="$1" out=""
  if [[ "${SKIP_AI_NEXT_STEPS:-0}" != "1" ]]; then
    if [[ -n "${NEXT_STEPS_AI_URL:-}" ]]; then
      out="$(call_openai_compat_ai "$NEXT_STEPS_AI_URL" "${PERSONAL_API_KEY:-}" "${NEXT_STEPS_AI_MODEL:-auto}" "$prompt" || true)"
    fi
    if [[ -z "$out" && -n "${GEMINI_API_KEY:-}" ]]; then
      out="$(call_gemini_ai "$prompt" || true)"
    fi
    if [[ -z "$out" && -n "${ANTHROPIC_API_KEY:-}" ]]; then
      out="$(call_anthropic_ai "$prompt" || true)"
    fi
    if [[ -z "$out" && -n "${OPENAI_API_KEY:-}" ]]; then
      out="$(call_openai_compat_ai "https://api.openai.com/v1/chat/completions" "$OPENAI_API_KEY" "gpt-4o-mini" "$prompt" || true)"
    fi
    if [[ -z "$out" && -n "${OLLAMA_BASE_URL:-}" ]]; then
      out="$(call_openai_compat_ai "${OLLAMA_BASE_URL%/}/chat/completions" "" "${OLLAMA_MODEL:-llama3.2}" "$prompt" || true)"
    fi
  fi
  printf '%s' "$out"
}
