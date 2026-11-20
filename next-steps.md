# Post-Commit & Post-Push Checklist (`next-steps.md`)

Reusable checklist executed after every commit and push in `go-ai-gateway`.

```mermaid
flowchart LR
  A["1. Run Tests & Race"] --> B["2. Check Coverage & Gate"]
  B --> C["3. Verify 100-Line & Mermaid"]
  C --> D["4. Profile CPU/GPU"]
  D --> E["5. Update Docs & Push"]
  style A fill:#1e3a5f,stroke:#38bdf8,color:#f8fafc
  style B fill:#14532d,stroke:#4ade80,color:#f8fafc
  style C fill:#4c1d95,stroke:#c084fc,color:#f8fafc
  style D fill:#7c2d12,stroke:#fb923c,color:#f8fafc
  style E fill:#1e293b,stroke:#94a3b8,color:#f8fafc
```

## Checklist

1. Verify all unit, integration, smoke, E2E, security, and benchmark tests pass (`go test -race -count=1 ./...`).
2. Confirm coverage meets the CI gate and work toward 100% statement coverage with zero regressions.
3. Verify total test runtime is under 30 minutes and update `test-time-report.md`.
4. Verify all source and test files are <= 100 lines (`./scripts/check-line-limit.sh`) and all docs satisfy the Mermaid rule (`./scripts/check-docs-mermaid.sh`).
5. Review for duplicate code, dead code, and per-commit optimizations.
6. Run `gofmt -l .`, `go vet ./...`, and `bash -n`.
7. Profile CPU/GPU usage and update `system-profile.md`.
8. Confirm `./scripts/check-gate.sh` and `.githooks/pre-commit` pass cleanly.
9. Update `metrics.md` with latest coverage, latency, and binary size metrics.
10. Update `readme.md` for any architectural or CLI changes.
11. Review this checklist for next-iteration follow-ups (raising statement coverage to 100%).
12. Commit and push changes to `origin/main`.

