#!/usr/bin/env bash
set -euo pipefail

# Enforces the 100-line maximum for all source code and test files.
MAX_LINES="${MAX_LINES:-100}"
FAILED=0

while IFS= read -r -d '' file; do
  lines="$(wc -l < "$file")"
  if (( lines > MAX_LINES )); then
    printf 'FAIL: %s has %d lines (limit %d)\n' "$file" "$lines" "$MAX_LINES" >&2
    FAILED=1
  fi
done < <(find . -not -path './.git/*' -not -path './node_modules/*' -not -path './build/*' \
  \( -name '*.go' -o -name '*.sh' -o -name '*.mjs' -o -name '*.js' \) -print0)

if (( FAILED == 0 )); then
  echo "OK: all source and test files are <= ${MAX_LINES} lines."
else
  exit 1
fi

