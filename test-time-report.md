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
| `github.com/thomas/go-ai-gateway` | `1.013s` | `80.6%` | OK |
| `internal/config` | `1.010s` | `100.0%` | OK |
| `internal/gateway` | `1.036s` | `84.4%` | OK |
| `internal/provider` | `1.026s` | `82.7%` | OK |
| `internal/redact` | `1.020s` | `98.1%` | OK |
| `internal/router` | `1.018s` | `93.5%` | OK |
| `internal/store` | `1.027s` | `84.3%` | OK |
| `internal/types` | `1.012s` | `100.0%` | OK |
| **Aggregate / Benchmarks** | `~2.0s wall` (`0.342s` bench) | `86.1%` | OK |

