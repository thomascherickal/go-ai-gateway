#!/usr/bin/env bash
set -euo pipefail

# Checks every documentation .md file (excluding uppercase tool files) has at least
# 1 colored Mermaid diagram per 200 words of prose.
FAILED=0

while IFS= read -r -d '' file; do
  base="$(basename "$file")"
  case "$base" in
    AGENTS.md|GEMINI.md|CLAUDE.md|SKILL.md) continue ;;
  esac

  # Strip fenced mermaid blocks when counting prose words.
  words="$(awk '/^```mermaid/{in_m=1;next} /^```/{if(in_m){in_m=0;next}} !in_m{print}' "$file" | wc -w)"
  required=$(( (words + 199) / 200 ))
  (( required < 1 )) && required=1

  diagrams="$(grep -c '^```mermaid' "$file" || true)"
  colored="$(grep -Ec 'style |classDef ' "$file" || true)"

  if (( diagrams < required )); then
    printf 'FAIL: %s has %d words -> needs >= %d Mermaid diagram(s), found %d\n' "$file" "$words" "$required" "$diagrams" >&2
    FAILED=1
  elif (( colored < 1 )); then
    printf 'FAIL: %s Mermaid diagram(s) missing color styling (style/classDef)\n' "$file" >&2
    FAILED=1
  fi
done < <(find . -not -path './.git/*' -not -path './node_modules/*' -not -path './build/*' -name '*.md' -print0)

if (( FAILED == 0 )); then
  echo "OK: all documentation pages meet the colored Mermaid diagram rule (1 per 200 words)."
else
  exit 1
fi

