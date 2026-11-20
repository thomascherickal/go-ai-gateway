#!/usr/bin/env bash
set -euo pipefail

# Commit & CI Gate enforcing AGENTS.md standards.
MIN_COVERAGE="${MIN_COVERAGE:-85.0}"
MAX_SECONDS="${MAX_SECONDS:-1800}" # 30 minutes

echo "==> [1/6] Checking formatting (gofmt)..."
UNFMT="$(gofmt -l .)"
if [[ -n "$UNFMT" ]]; then
  echo "FAIL: unformatted Go files:" >&2
  echo "$UNFMT" >&2
  exit 1
fi

echo "==> [2/6] Running static analysis (go vet)..."
go vet ./...

echo "==> [3/6] Enforcing 100-line file limit..."
./scripts/check-line-limit.sh

echo "==> [4/6] Checking documentation Mermaid diagrams..."
./scripts/check-docs-mermaid.sh

echo "==> [5/6] Validating shell scripts..."
bash -n deploy-*.sh scripts/*.sh scripts/lib/*.sh

echo "==> [6/6] Running race-enabled tests with coverage & runtime gate..."
START_TS="$(date +%s)"
COV_FILE="$(mktemp)"
trap 'rm -f "$COV_FILE"' EXIT
go test -race -count=1 -coverprofile="$COV_FILE" ./...
END_TS="$(date +%s)"
ELAPSED=$(( END_TS - START_TS ))

if (( ELAPSED > MAX_SECONDS )); then
  echo "FAIL: test suite took ${ELAPSED}s (limit ${MAX_SECONDS}s)" >&2
  exit 1
fi

TOTAL_COV="$(go tool cover -func="$COV_FILE" | awk '/^total:/ {gsub("%","",$3); print $3}')"
echo "Test suite passed in ${ELAPSED}s with ${TOTAL_COV}% statement coverage (gate >= ${MIN_COVERAGE}%)."
awk -v got="$TOTAL_COV" -v min="$MIN_COVERAGE" 'BEGIN { exit (got + 0 >= min + 0) ? 0 : 1 }' \
  || { echo "FAIL: coverage ${TOTAL_COV}% is below gate ${MIN_COVERAGE}%" >&2; exit 1; }

