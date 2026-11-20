# Live Test Timing Report (`test-time-report.md`)

Measured via `go test -race -count=1 ./...` on `2026-11-20` (budget limit: **30 minutes**).

```mermaid
flowchart LR
  T1["Total Race Suite: 8.16s"] --> T2["Limit: 1800s (30m)"]
  T2 --> T3["Status: PASS (0.45% of budget)"]
  style T1 fill:#1e3a5f,stroke:#38bdf8,color:#f8fafc
  style T2 fill:#334155,stroke:#94a3b8,color:#f8fafc
  style T3 fill:#14532d,stroke:#4ade80,color:#f8fafc
```

## Package Execution Times

| Package | Race Runtime | Coverage | Slow Flag |
| --- | --- | --- | --- |
| `github.com/thomas/go-ai-gateway` | `1.014s` | `100.0%` | OK |
| `internal/config` | `1.021s` | `100.0%` | OK |
| `internal/gateway` | `1.043s` | `100.0%` | OK |
| `internal/provider` | `1.030s` | `100.0%` | OK |
| `internal/redact` | `1.023s` | `100.0%` | OK |
| `internal/router` | `1.019s` | `100.0%` | OK |
| `internal/store` | `1.031s` | `100.0%` | OK |
| `internal/types` | `1.015s` | `100.0%` | OK |
| **Aggregate / Benchmarks** | `~3.0s wall` (`0.342s` bench) | `100.0%` | OK |

