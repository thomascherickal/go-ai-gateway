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
| `github.com/thomas/go-ai-gateway` | `1.016s` | `65.7%` | OK |
| `internal/config` | `1.010s` | `91.2%` | OK |
| `internal/gateway` | `1.032s` | `78.4%` | OK |
| `internal/provider` | `1.027s` | `77.2%` | OK |
| `internal/redact` | `1.020s` | `95.4%` | OK |
| `internal/router` | `1.017s` | `88.7%` | OK |
| `internal/store` | `1.026s` | `78.9%` | OK |
| `internal/types` | `1.012s` | `100.0%` | OK |
| **Aggregate / Benchmarks** | `~2.0s wall` (`0.342s` bench) | `80.1%` | OK |

